import { bchRpc } from '../../../utils/bchRpc'

export default defineEventHandler(async (event) => {
  const height = Number(getRouterParam(event, 'height'))
  if (!Number.isFinite(height)) {
    throw createError({ statusCode: 400, statusMessage: 'Invalid height' })
  }
  return await bchRpc<string>('getblockhash', [height])
})

