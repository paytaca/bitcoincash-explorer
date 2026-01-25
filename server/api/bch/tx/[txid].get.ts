import { bchRpc } from '../../../utils/bchRpc'

export default defineEventHandler(async (event) => {
  const txid = getRouterParam(event, 'txid')
  if (!txid) {
    throw createError({ statusCode: 400, statusMessage: 'Missing txid' })
  }

  // verbose=2 includes tokenData (CashTokens) in vin/vout
  const tx = await bchRpc<any>('getrawtransaction', [txid, 2])

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

  return tx
})

