import { bchRpc } from '../../../utils/bchRpc'

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
const MEMPOOL_LIMIT = 25
const CONFIRMED_LIMIT = 25
const RECENT_LIMIT = 15
const BLOCKS_TO_SCAN = 3

let cache: { at: number; value: RecentTxResponse } | null = null

function toNum(v: unknown): number | undefined {
  return typeof v === 'number' && Number.isFinite(v) ? v : undefined
}

export default defineEventHandler(async () => {
  const now = Date.now()
  if (cache && now - cache.at < CACHE_MS) return cache.value

  const tip = await bchRpc<number>('getblockcount')

  // Confirmed txids from last few blocks (lightweight: verbosity=1 returns txids only).
  const heights = Array.from({ length: BLOCKS_TO_SCAN }, (_, i) => tip - i).filter((h) => h >= 0)
  const hashes = await Promise.all(heights.map((h) => bchRpc<string>('getblockhash', [h])))

  const blocks = await Promise.all(
    hashes.map((hash) =>
      bchRpc<any>('getblock', [hash, 1]).catch(() => null) // avoid breaking if one block fetch fails
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

    for (const t of txs) {
      if (typeof t !== 'string') continue
      confirmed.push({ txid: t, status: 'confirmed', time, blockHeight: height, confirmations })
      if (confirmed.length >= CONFIRMED_LIMIT) break
    }
    if (confirmed.length >= CONFIRMED_LIMIT) break
  }

  // Recent mempool txids (best-effort; nodes can have huge mempools, so always slice early).
  let mempool: RecentTxItem[] = []
  try {
    const mp = await bchRpc<Record<string, any>>('getrawmempool', [true])
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
  const items = [...mempool, ...confirmedDeduped].slice(0, RECENT_LIMIT)

  // Fetch tx details for amount and token detection (verbosity 2 includes tokenData in vin/vout).
  const txDetails = await Promise.all(
    items.map(async (item) => {
      try {
        const tx = await bchRpc<any>('getrawtransaction', [item.txid, 2])
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
    })
  )

  const itemsWithAmount: RecentTxItem[] = items.map((item, i) => ({
    ...item,
    amount: txDetails[i].amount,
    hasTokens: txDetails[i].hasTokens
  }))

  const value: RecentTxResponse = { updatedAt: Math.floor(now / 1000), items: itemsWithAmount }
  cache = { at: now, value }
  return value
})

