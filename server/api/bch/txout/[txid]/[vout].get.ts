import { bchRpc } from '../../../../utils/bchRpc'

export default defineEventHandler(async (event) => {
  const txid = getRouterParam(event, 'txid')
  const voutStr = getRouterParam(event, 'vout')
  const vout = Number(voutStr)

  if (!txid) {
    throw createError({ statusCode: 400, statusMessage: 'Missing txid' })
  }
  if (!Number.isInteger(vout) || vout < 0) {
    throw createError({ statusCode: 400, statusMessage: 'Invalid vout' })
  }

  // gettxout returns null if spent or not found in UTXO set (and optionally mempool)
  // include_mempool=true helps for unconfirmed transactions.
  try {
    const res = await bchRpc<any | null>('gettxout', [txid, vout, true])
    return { status: res ? 'unspent' : 'spent' }
  } catch {
    // Don't break tx page if backend can't answer (e.g. pruned node edge cases)
    return { status: 'unknown' }
  }
})

