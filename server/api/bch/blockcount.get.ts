import { bchRpc } from '../../utils/bchRpc'

export default defineEventHandler(async () => {
  return await bchRpc<number>('getblockcount')
})

