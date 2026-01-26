import { bchRpc } from '../../../utils/bchRpc'

function isBlockNotFoundError(e: unknown): boolean {
  const msg = typeof (e as any)?.message === 'string' ? (e as any).message : String(e)
  const s = msg.toLowerCase()
  return (
    s.includes('block not found') ||
    s.includes('block height out of range') ||
    s.includes('bad block hash') ||
    s.includes('invalid block hash')
  )
}

export default defineEventHandler(async (event) => {
  const hash = getRouterParam(event, 'hash')
  if (!hash) {
    throw createError({ statusCode: 400, statusMessage: 'Missing hash' })
  }

  try {
    return await bchRpc<any>('getblock', [hash, 2])
  } catch (e) {
    if (isBlockNotFoundError(e)) {
      throw createError({ statusCode: 404, statusMessage: 'Block not found' })
    }
    throw e
  }
})

