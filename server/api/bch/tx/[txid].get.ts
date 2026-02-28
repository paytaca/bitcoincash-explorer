import { bchRpc } from '../../../utils/bchRpc'
import { getTokenMeta } from '../../../utils/bcmr'
import { withConditionalFileCache } from '../../../utils/cache'

function isTxNotFoundError(e: unknown): boolean {
  const msg = typeof (e as any)?.message === 'string' ? (e as any).message : String(e)
  const s = msg.toLowerCase()
  return (
    s.includes('no such mempool or blockchain') ||
    s.includes('invalid or non-wallet transaction') ||
    s.includes('transaction not in block') ||
    s.includes('not in block chain')
  )
}

function collectTokenCategories(tx: any): string[] {
  const cats = new Set<string>()
  const vin = Array.isArray(tx?.vin) ? tx.vin : []
  const vout = Array.isArray(tx?.vout) ? tx.vout : []
  for (const v of vin) {
    const c = v?.tokenData?.category
    if (typeof c === 'string' && c) cats.add(c)
  }
  for (const o of vout) {
    const c = o?.tokenData?.category
    if (typeof c === 'string' && c) cats.add(c)
  }
  return Array.from(cats)
}

function isConfirmedTx(tx: any): boolean {
  return typeof tx?.blockhash === 'string' && tx.blockhash.length > 0
}

// Fetch token metadata with concurrency limit
async function fetchTokenMetaWithLimit(
  categories: string[],
  bcmrBaseUrl: string,
  concurrency = 3
): Promise<Record<string, { name?: string; symbol?: string; decimals?: number }>> {
  if (categories.length === 0) return {}
  
  const results: Record<string, { name?: string; symbol?: string; decimals?: number }> = {}
  const executing: Promise<void>[] = []
  
  for (const cat of categories) {
    const promise = getTokenMeta(cat, bcmrBaseUrl)
      .then((meta) => {
        if (meta && (meta.name || meta.symbol)) {
          results[cat] = meta
        }
      })
      .catch(() => {
        // Ignore individual token fetch failures
      })
    
    executing.push(promise)
    
    if (executing.length >= concurrency) {
      await Promise.race(executing)
      executing.splice(executing.findIndex(p => p === promise || p === executing[0]), 1)
    }
  }
  
  await Promise.all(executing)
  return results
}

export default defineEventHandler(async (event) => {
  const txid = getRouterParam(event, 'txid')
  if (!txid) {
    throw createError({ statusCode: 400, statusMessage: 'Missing txid' })
  }

  return await withConditionalFileCache(
    `tx:${txid}`,
    60 * 60 * 1000,
    async () => {
      let tx: any
      try {
        tx = await bchRpc<any>('getrawtransaction', [txid, 2], 30000, { maxRetries: 2 })
      } catch (e) {
        if (isTxNotFoundError(e)) {
          throw createError({ statusCode: 404, statusMessage: 'Transaction not found' })
        }
        throw e
      }

      // Fetch mempool entry for unconfirmed transactions
      if (typeof tx?.time !== 'number' || !Number.isFinite(tx.time)) {
        try {
          const entry = await bchRpc<any>('getmempoolentry', [txid], 10000, { maxRetries: 1 })
          if (typeof entry?.time === 'number' && Number.isFinite(entry.time)) {
            tx.seenTime = entry.time
          }
        } catch {
          // Ignore mempool entry errors
        }
      }

      // Fetch token metadata with concurrency limits
      const config = useRuntimeConfig()
      const bcmrBaseUrl = String(config.bcmrBaseUrl || '').trim()
      let tokenMeta: Record<string, { name?: string; symbol?: string; decimals?: number }> = {}
      
      if (bcmrBaseUrl) {
        const categories = collectTokenCategories(tx)
        // Limit to first 10 categories to prevent abuse
        const limitedCategories = categories.slice(0, 10)
        tokenMeta = await fetchTokenMetaWithLimit(limitedCategories, bcmrBaseUrl, 3)
      }

      return { ...tx, tokenMeta }
    },
    (result) => isConfirmedTx(result)
  )
})
