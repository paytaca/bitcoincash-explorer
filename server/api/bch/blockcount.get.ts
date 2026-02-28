import { bchRpc } from '../../utils/bchRpc'
import { getRedisClient, withCache } from '../../utils/redis'

export default defineEventHandler(async () => {
  const redis = getRedisClient()

  // Try Redis pre-processed list first (zero RPC calls)
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

  // Fallback to RPC with Redis cache (30 seconds TTL for frequently changing data)
  return await withCache(redis, 'blockcount', 30, async () => {
    console.log('Fetching blockcount from RPC...')
    return await bchRpc<number>('getblockcount')
  })
})