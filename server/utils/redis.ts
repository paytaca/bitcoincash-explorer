import Redis from 'ioredis'

const config = useRuntimeConfig()

export interface BlockData {
  hash: string
  height: number
  time: number
  size: number
  txCount: number
  miner?: string
}

export interface TransactionData {
  txid: string
  status: 'mempool' | 'confirmed'
  time: number
  amount?: number
  hasTokens: boolean
  fee?: number
  size?: number
  blockHeight?: number
  confirmations?: number
}

let redisClient: Redis | null = null

export function getRedisClient(): Redis | null {
  if (redisClient) return redisClient
  
  const redisUrl = config.redisUrl || process.env.REDIS_URL
  if (!redisUrl) {
    console.warn('REDIS_URL not configured, using RPC fallback')
    return null
  }

  try {
    redisClient = new Redis(redisUrl as string, {
      retryStrategy: (times) => {
        const delay = Math.min(times * 50, 2000)
        return delay
      },
      maxRetriesPerRequest: 2,
      lazyConnect: true
    })

    redisClient.on('error', (err) => {
      console.error('Redis error:', err.message)
    })

    return redisClient
  } catch (error) {
    console.error('Failed to create Redis client:', error)
    return null
  }
}

export async function getLatestBlocks(redis: Redis | null, limit: number = 15): Promise<BlockData[] | null> {
  if (!redis) return null

  try {
    const data = await redis.lrange('bch:blocks:latest', 0, limit - 1)
    if (!data || data.length === 0) return null
    return data.map(item => JSON.parse(item))
  } catch (error) {
    console.error('Error fetching blocks from Redis:', error)
    return null
  }
}

export async function getLatestTransactions(redis: Redis | null, limit: number = 20): Promise<TransactionData[] | null> {
  if (!redis) return null

  try {
    const data = await redis.lrange('bch:txs:latest', 0, limit - 1)
    if (!data || data.length === 0) return null
    return data.map(item => JSON.parse(item))
  } catch (error) {
    console.error('Error fetching transactions from Redis:', error)
    return null
  }
}

export async function getMempoolTxids(redis: Redis | null): Promise<Set<string> | null> {
  if (!redis) return null
  
  try {
    const txids = await redis.smembers('bch:mempool:txids')
    return new Set(txids)
  } catch (error) {
    console.error('Error fetching mempool from Redis:', error)
    return null
  }
}