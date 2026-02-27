import { bchRpc } from '../../utils/bchRpc'
import { withFileCache } from '../../utils/cache'

export default defineEventHandler(async () => {
  return await withFileCache('blockcount', 10_000, async () => {
    return await bchRpc<number>('getblockcount')
  })
})

