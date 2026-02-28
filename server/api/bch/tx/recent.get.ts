import { bchRpc } from '../../../utils/bchRpc'
import { getRedisClient, getLatestTransactions, getMempoolTxids, TransactionData } from '../../../utils/redis'

// Helper for promisified delay (avoids import issues)
const delay = (ms: number): Promise<void> => new Promise(resolve => setTimeout(resolve, ms))

type RecentTxStatus = 'mempool' | 'confirmed'

export type RecentTxItem = {
  txid: string
  status: RecentTxStatus
  time?: number
  fee?: number
  size?: number
  blockHeight?: number
  confirmations?: number
  amount?: number
  hasTokens?: boolean
}

type RecentTxResponse = {
  updatedAt: number
  items: RecentTxItem[]
}

const CACHE_MS = 5_000

let cache: { at: number; value: RecentTxResponse } | null = null
let isFetching = false

function toNum(v: unknown): number | undefined {
  return typeof v === 'number' && Number.isFinite(v) ? v : undefined
}

export default defineEventHandler(async () => {
  const now = Date.now()
  if (cache && now - cache.at < CACHE_MS) return cache.value
  
  // Prevent concurrent fetches
  if (isFetching) {
    if (cache) return cache.value
    await delay(100)
    if (cache) return cache.value
  }

  isFetching = true
  
  try {
    // Try Redis first (zero RPC calls)
    const redis = getRedisClient()
    if (redis) {
      try {
        const [txs, mempoolSet] = await Promise.all([
          getLatestTransactions(redis, 20),
          getMempoolTxids(redis)
        ])
        
        if (txs && txs.length > 0) {
          const items: RecentTxItem[] = txs.map(tx => ({
            txid: tx.txid,
            status: mempoolSet?.has(tx.txid) ? 'mempool' : 'confirmed',
            time: tx.time,
            fee: tx.fee,
            size: tx.size,
            blockHeight: tx.blockHeight,
            confirmations: tx.confirmations,
            amount: tx.amount,
            hasTokens: tx.hasTokens
          }))
          
          const value: RecentTxResponse = { updatedAt: Math.floor(now / 1000), items }
          cache = { at: now, value }
          return value
        }
      } catch (error) {
        console.warn('Redis fetch failed, falling back to RPC:', error)
      }
    }
    
    // Fallback to RPC
    console.log('Fetching recent transactions from RPC...')
    return await fetchFromRpc(now)
    
  } finally {
    isFetching = false
  }
})

async function fetchFromRpc(now: number): Promise<RecentTxResponse> {
  const tip = await bchRpc<number>('getblockcount', [], 10000, { maxRetries: 2 })

  // Confirmed txids from last few blocks (lightweight: verbosity=1 returns txids only).
  const BLOCKS_TO_SCAN = 2
  const heights = Array.from({ length: BLOCKS_TO_SCAN }, (_, i) => tip - i).filter((h) => h >= 0)
  
  // Fetch block hashes sequentially to reduce load
  const hashes: string[] = []
  for (const h of heights) {
    try {
      const hash = await bchRpc<string>('getblockhash', [h], 5000, { maxRetries: 1 })
      hashes.push(hash)
    } catch {
      // Continue without this block
    }
  }

  // Fetch blocks with limited concurrency
  const CONFIRMED_LIMIT = 15
  const blocks = await Promise.all(
    hashes.map(hash => 
      bchRpc<any>('getblock', [hash, 1], 10000, { maxRetries: 1 }).catch(() => null)
    )
  )

  const confirmed: RecentTxItem[] = []
  for (const b of blocks) {
    if (!b) continue
    const txs: unknown = b?.tx
    const height = toNum(b?.height)
    const time = toNum(b?.time)
    const confirmations = height !== undefined ? Math.max(0, tip - height + 1) : undefined
    if (!Array.isArray(txs)) continue

    // Skip coinbase transaction (first tx in block)
    for (let i = 1; i < txs.length && confirmed.length < CONFIRMED_LIMIT; i++) {
      const t = txs[i]
      if (typeof t !== 'string') continue
      confirmed.push({ txid: t, status: 'confirmed', time, blockHeight: height, confirmations })
    }
    if (confirmed.length >= CONFIRMED_LIMIT) break
  }

  // Recent mempool txids (best-effort; nodes can have huge mempools, so always slice early).
  const MEMPOOL_LIMIT = 15
  let mempool: RecentTxItem[] = []
  try {
    const mp = await bchRpc<Record<string, any>>('getrawmempool', [true], 10000, { maxRetries: 1 })
    mempool = Object.entries(mp || {})
      .map(([txid, e]) => {
        const fees = e?.fees
        const fee = toNum(e?.fee) ?? toNum(fees?.base) ?? toNum(fees?.modified)
        return {
          txid,
          status: 'mempool' as const,
          time: toNum(e?.time),
          size: toNum(e?.size),
          fee
        }
      })
      .sort((a, b) => (b.time || 0) - (a.time || 0))
      .slice(0, MEMPOOL_LIMIT)
  } catch {
    // mempool is optional for the home page
  }

  // Order: mempool first (newest), then confirmed (newest).
  // Also dedupe in case a tx just got mined while we were fetching.
  const mempoolIds = new Set(mempool.map((m) => m.txid))
  const confirmedDeduped = confirmed.filter((c) => !mempoolIds.has(c.txid))
  const RECENT_LIMIT = 20
  const items = [...mempool, ...confirmedDeduped].slice(0, RECENT_LIMIT)

  // Fetch tx details with limited concurrency to avoid overwhelming the node
  const TX_DETAILS_CONCURRENCY = 3
  const txDetails = await processWithConcurrency(
    items,
    async (item) => {
      try {
        const tx = await bchRpc<any>('getrawtransaction', [item.txid, 2], 15000, { maxRetries: 1 })
        const vout = Array.isArray(tx?.vout) ? tx.vout : []
        const amount = vout.reduce(
          (sum: number, o: any) => sum + (typeof o?.value === 'number' && Number.isFinite(o.value) ? o.value : 0),
          0
        )
        const vin = Array.isArray(tx?.vin) ? tx.vin : []
        const hasTokens =
          vin.some((v: any) => v?.tokenData != null) || vout.some((o: any) => o?.tokenData != null)
        return { amount, hasTokens }
      } catch {
        return { amount: undefined, hasTokens: false }
      }
    },
    TX_DETAILS_CONCURRENCY
  )

  const itemsWithAmount: RecentTxItem[] = items.map((item, i) => ({
    ...item,
    amount: txDetails[i].amount,
    hasTokens: txDetails[i].hasTokens
  }))

  const value: RecentTxResponse = { updatedAt: Math.floor(now / 1000), items: itemsWithAmount }
  cache = { at: now, value }
  return value
}

// Process items with limited concurrency
async function processWithConcurrency<T, R>(
  items: T[],
  processor: (item: T) => Promise<R>,
  concurrency: number
): Promise<R[]> {
  const results: R[] = new Array(items.length)
  const iterator = items.entries()
  
  async function worker(): Promise<void> {
    for (const [index, item] of iterator) {
      results[index] = await processor(item)
    }
  }
  
  const workers = Array(Math.min(concurrency, items.length))
    .fill(null)
    .map(() => worker())
  
  await Promise.all(workers)
  return results
}