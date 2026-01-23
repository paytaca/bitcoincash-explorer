import { bchRpc } from '../../../utils/bchRpc'

export default defineEventHandler(async (event) => {
  const txid = getRouterParam(event, 'txid')
  if (!txid) {
    throw createError({ statusCode: 400, statusMessage: 'Missing txid' })
  }

  // verbose=2 includes tokenData (CashTokens) in vin/vout
  return await bchRpc<any>('getrawtransaction', [txid, 2])
})

