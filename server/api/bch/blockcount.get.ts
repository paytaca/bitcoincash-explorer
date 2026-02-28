import { bchRpc } from '../../utils/bchRpc'
import { withFileCache } from '../../utils/cache'
import { getRedisClient } from '../../utils/redis'

export default defineEventHandler(async () => {
  // Try Redis first (zero RPC calls)
  const redis = getRedisClient()
  if (redis) {
    try {
      const blockData = await redis.lindex('bch:blocks:latest', 0)
      if (blockData) {
        const block = JSON.parse(blockData)
        if (block.height !== undefined) {
          return block.height
        }
      }
    } catch (error) {
      console.warn('Redis fetch failed for blockcount, falling back to RPC:', error)
    }
  }

  // Fallback to RPC with file cache
  return await withFileCache('blockcount', 10_000, async () => {
    console.log('Fetching blockcount from RPC...')
    return await bchRpc<number>('getblockcount')
  })
})