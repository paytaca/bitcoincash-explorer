import { Redis } from 'ioredis'
import { config, getRedisKey } from './config.js'

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

class RedisClient {
  private client: Redis

  constructor() {
    this.client = new Redis(config.redisUrl, {
      retryStrategy: (times: number) => {
        const delay = Math.min(times * 50, 2000)
        return delay
      },
      maxRetriesPerRequest: 3
    })

    this.client.on('error', (err: Error) => {
      console.error('Redis error:', err)
    })

    this.client.on('connect', () => {
      console.log('Connected to Redis')
    })
  }

  async pushBlock(block: BlockData): Promise<void> {
    const key = getRedisKey('blocks:latest')
    const existingJson = await this.client.lrange(key, 0, config.maxBlocks - 1)
    const existingIndex = existingJson.findIndex((item: string) => {
      try {
        const parsed = JSON.parse(item)
        return parsed.hash === block.hash
      } catch {
        return false
      }
    })

    if (existingIndex !== -1) {
      // Remove existing entry to avoid duplicates
      await this.client.lrem(key, 0, existingJson[existingIndex])
    }

    await this.client.lpush(key, JSON.stringify(block))
    await this.client.ltrim(key, 0, config.maxBlocks - 1)
  }

  async getBlocks(): Promise<BlockData[]> {
    const key = getRedisKey('blocks:latest')
    const data = await this.client.lrange(key, 0, config.maxBlocks - 1)
    return data.map((item: string) => JSON.parse(item))
  }

  async getLatestBlock(): Promise<BlockData | null> {
    const key = getRedisKey('blocks:latest')
    const data = await this.client.lindex(key, 0)
    return data ? JSON.parse(data) : null
  }

  async pushTransaction(tx: TransactionData): Promise<void> {
    const key = getRedisKey('txs:latest')
    const existingJson = await this.client.lrange(key, 0, config.maxTransactions - 1)
    const existingIndex = existingJson.findIndex((item: string) => {
      try {
        const parsed = JSON.parse(item)
        return parsed.txid === tx.txid
      } catch {
        return false
      }
    })

    if (existingIndex !== -1) {
      // Remove existing entry to avoid duplicates
      const existing = JSON.parse(existingJson[existingIndex])
      await this.client.lrem(key, 0, existingJson[existingIndex])
      // Preserve hasTokens from existing entry if new one doesn't have it
      if (!tx.hasTokens && existing.hasTokens) {
        tx.hasTokens = existing.hasTokens
      }
    }

    await this.client.lpush(key, JSON.stringify(tx))
    await this.client.ltrim(key, 0, config.maxTransactions - 1)
  }

  async getTransactions(): Promise<TransactionData[]> {
    const key = getRedisKey('txs:latest')
    const data = await this.client.lrange(key, 0, config.maxTransactions - 1)
    return data.map((item: string) => JSON.parse(item))
  }

  async addToMempool(txid: string): Promise<void> {
    const key = getRedisKey('mempool:txids')
    await this.client.sadd(key, txid)
  }

  async removeFromMempool(txid: string): Promise<void> {
    const key = getRedisKey('mempool:txids')
    await this.client.srem(key, txid)
  }

  async isInMempool(txid: string): Promise<boolean> {
    const key = getRedisKey('mempool:txids')
    return await this.client.sismember(key, txid) === 1
  }

  async getMempoolTxids(): Promise<string[]> {
    const key = getRedisKey('mempool:txids')
    return await this.client.smembers(key)
  }

  async markTransactionConfirmed(txid: string, blockHeight: number, confirmations: number): Promise<void> {
    // Update the transaction in the list
    const key = getRedisKey('txs:latest')
    const txs = await this.getTransactions()
    
    const updatedTxs = txs.map(tx => {
      if (tx.txid === txid) {
        return {
          ...tx,
          status: 'confirmed' as const,
          blockHeight,
          confirmations
        }
      }
      return tx
    })

    // Replace the list
    await this.client.del(key)
    if (updatedTxs.length > 0) {
      await this.client.lpush(key, ...updatedTxs.map(tx => JSON.stringify(tx)))
    }

    // Remove from mempool set
    await this.removeFromMempool(txid)
  }

  async removeBlock(hash: string): Promise<void> {
    const key = getRedisKey('blocks:latest')
    const blocks = await this.getBlocks()
    const filtered = blocks.filter(b => b.hash !== hash)
    
    await this.client.del(key)
    if (filtered.length > 0) {
      await this.client.lpush(key, ...filtered.map(b => JSON.stringify(b)))
    }
  }

  async disconnect(): Promise<void> {
    await this.client.quit()
  }
}

export const redis = new RedisClient()