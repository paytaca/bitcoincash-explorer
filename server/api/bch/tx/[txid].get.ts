import { bchRpc } from '../../../utils/bchRpc'
import { getTokenMeta } from '../../../utils/bcmr'

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

export default defineEventHandler(async (event) => {
  const txid = getRouterParam(event, 'txid')
  if (!txid) {
    throw createError({ statusCode: 400, statusMessage: 'Missing txid' })
  }

  let tx: any
  try {
    tx = await bchRpc<any>('getrawtransaction', [txid, 2])
  } catch (e) {
    if (isTxNotFoundError(e)) {
      throw createError({ statusCode: 404, statusMessage: 'Transaction not found' })
    }
    throw e
  }

  // Some nodes omit `time` for mempool transactions in getrawtransaction.
  // Best-effort: if time is missing, attach mempool "seen time" (not block time).
  if (typeof tx?.time !== 'number' || !Number.isFinite(tx.time)) {
    try {
      const entry = await bchRpc<any>('getmempoolentry', [txid])
      if (typeof entry?.time === 'number' && Number.isFinite(entry.time)) {
        tx.seenTime = entry.time
      }
    } catch {
      // Not in mempool (or node doesn't support it); ignore.
    }
  }

  const config = useRuntimeConfig()
  const bcmrBaseUrl = String(config.bcmrBaseUrl || '').trim()
  let tokenMeta: Record<string, { name?: string; symbol?: string; decimals?: number }> = {}
  if (bcmrBaseUrl) {
    const categories = collectTokenCategories(tx)
    const entries = await Promise.all(
      categories.map(async (cat) => {
        const meta = await getTokenMeta(cat, bcmrBaseUrl)
        return [cat, meta] as const
      })
    )
    tokenMeta = Object.fromEntries(entries)
  }

  return { ...tx, tokenMeta }
})

