import { bchRpc } from '../../utils/bchRpc'

export default defineEventHandler(async (event) => {
  const body = await readBody(event)
  const hex = body?.hex?.trim() as string | undefined

  if (!hex || typeof hex !== 'string') {
    throw createError({ statusCode: 400, statusMessage: 'Missing transaction hex' })
  }

  if (!/^[0-9a-fA-F]+$/.test(hex)) {
    throw createError({ statusCode: 400, statusMessage: 'Invalid transaction hex format' })
  }

  if (hex.length < 20) {
    throw createError({ statusCode: 400, statusMessage: 'Transaction hex is too short' })
  }

  try {
    const txid = await bchRpc<string>('sendrawtransaction', [hex], 30_000, { maxRetries: 2 })
    return { success: true, txid }
  } catch (e: any) {
    const msg = typeof e?.message === 'string' ? e.message : String(e)
    
    if (msg.includes('transaction already in block chain') || msg.includes('already have transaction')) {
      throw createError({ statusCode: 409, statusMessage: 'Transaction already exists in blockchain or mempool' })
    }
    
    if (msg.includes('bad-txns') || msg.includes('bad-tx')) {
      throw createError({ statusCode: 400, statusMessage: `Invalid transaction: ${msg}` })
    }
    
    if (msg.includes('insufficient fee')) {
      throw createError({ statusCode: 400, statusMessage: 'Transaction rejected: insufficient fee' })
    }
    
    if (msg.includes('too-long-mempool-chain')) {
      throw createError({ statusCode: 400, statusMessage: 'Transaction chain too long for mempool' })
    }

    console.error('Broadcast error:', msg)
    throw createError({ statusCode: 500, statusMessage: `Broadcast failed: ${msg}` })
  }
})
