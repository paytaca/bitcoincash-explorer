import { bchRpc } from '../../../utils/bchRpc'

export default defineEventHandler(async (event) => {
  const hash = getRouterParam(event, 'hash')
  if (!hash) {
    throw createError({ statusCode: 400, statusMessage: 'Missing hash' })
  }

  // verbosity=2 includes tx objects
  return await bchRpc<any>('getblock', [hash, 2])
})

