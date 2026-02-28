import { ZmqClient } from './zmq-client.js'
import { redis, BlockData, TransactionData } from './redis-client.js'
import { rpc } from './rpc-client.js'
import { config } from './config.js'

class ZmqListenerService {
  private zmqClient: ZmqClient
  private isShuttingDown = false

  constructor() {
    this.zmqClient = new ZmqClient()
  }

  async start(): Promise<void> {
    console.log('=== BCH ZMQ Listener Service ===')
    console.log(`Redis URL: ${config.redisUrl}`)
    console.log(`ZMQ: ${config.zmqHost}:${config.zmqPort}`)
    console.log(`RPC: ${config.rpcUrl}`)
    console.log('')

    try {
      // Initial sync from RPC
      console.log('Performing initial sync from RPC...')
      await this.initialSync()
      console.log('Initial sync complete!\n')

      // Start ZMQ listener
      console.log('Starting ZMQ listener...')
      await this.zmqClient.start()
      console.log('ZMQ listener started. Waiting for new blocks and transactions...\n')

      // Keep the process running
      this.setupShutdownHandlers()
      
    } catch (error) {
      console.error('Failed to start service:', error)
      process.exit(1)
    }
  }

  private async initialSync(): Promise<void> {
    // Check if we already have data in Redis
    const existingBlocks = await redis.getBlocks()
    if (existingBlocks.length > 0) {
      console.log(`Found ${existingBlocks.length} blocks in Redis, skipping initial sync`)
      return
    }

    console.log('No existing data in Redis, syncing from RPC...')

    try {
      // Get current tip
      const tip = await rpc.getBlockCount()
      console.log(`Current tip: ${tip}`)

      // Fetch last N blocks
      const blocksToFetch = config.maxBlocks
      console.log(`Fetching last ${blocksToFetch} blocks...`)

      const blocksToPush: BlockData[] = []
      for (let i = 0; i < blocksToFetch; i++) {
        const height = tip - i
        if (height < 0) break

        try {
          const hash = await rpc.getBlockHash(height)
          const block = await rpc.getBlock(hash, 2)

          const blockData: BlockData = {
            hash: block.hash,
            height: block.height,
            time: block.time,
            size: block.size,
            txCount: block.tx?.length || 0,
            miner: this.extractMinerFromBlock(block)
          }

          blocksToPush.push(blockData)
          console.log(`  Block #${block.height} - ${blockData.miner || 'Unknown'}`)

        } catch (error) {
          console.error(`  Error fetching block at height ${height}:`, error)
        }
      }

      // Push blocks in reverse order so newest ends up at index 0
      for (const block of blocksToPush.reverse()) {
        await redis.pushBlock(block)
      }

      // Fetch recent mempool transactions
      console.log('\nFetching mempool transactions...')
      const mempool = await rpc.getRawMempool()
      const mempoolEntries = Object.entries(mempool)
        .sort(([, a], [, b]) => (b.time || 0) - (a.time || 0))
        .slice(0, config.maxTransactions)

      const txsToPush: TransactionData[] = []
      for (const [txid, txInfo] of mempoolEntries) {
        try {
          // Get full transaction details
          const tx = await rpc.getRawTransaction(txid, true)

          const vout = tx.vout || []
          const amount = vout.reduce((sum: number, output: any) => {
            return sum + (typeof output.value === 'number' ? output.value : 0)
          }, 0)

          const hasTokens = (tx.vin || []).some((v: any) => v.tokenData != null) ||
                           (tx.vout || []).some((o: any) => o.tokenData != null)

          const txData: TransactionData = {
            txid,
            status: 'mempool',
            time: txInfo.time || Math.floor(Date.now() / 1000),
            amount,
            hasTokens,
            fee: txInfo.fee || txInfo.fees?.base,
            size: txInfo.size || tx.size
          }

          txsToPush.push(txData)

        } catch (error) {
          console.error(`  Error fetching tx ${txid}:`, error)
        }
      }

      // Push transactions in reverse order so newest ends up at index 0
      for (const tx of txsToPush.reverse()) {
        await redis.pushTransaction(tx)
        await redis.addToMempool(tx.txid)
      }

      console.log(`Added ${txsToPush.length} mempool transactions`)

    } catch (error) {
      console.error('Error during initial sync:', error)
      throw error
    }
  }

  private extractMinerFromBlock(block: any): string | undefined {
    const coinbaseTx = block.tx?.[0]
    if (!coinbaseTx) return undefined

    const coinbaseInput = coinbaseTx.vin?.[0]
    if (!coinbaseInput?.coinbase) return undefined

    const coinbaseHex: string = coinbaseInput.coinbase
    if (!coinbaseHex || !/^[0-9a-fA-F]+$/.test(coinbaseHex)) {
      return undefined
    }

    try {
      const buf = Buffer.from(coinbaseHex, 'hex')
      const ascii = buf.toString('latin1').replace(/[^\x20-\x7E]+/g, ' ')
      const cleaned = ascii.replace(/\s+/g, ' ').trim()

      if (!cleaned) return undefined

      // Look for pool name in slash-delimited tags
      const match = cleaned.match(/\/\s*([A-Za-z0-9][A-Za-z0-9 ._-]{0,40}?)\s*\//)
      if (match?.[1]) {
        return match[1].trim()
      }

      return undefined
    } catch {
      return undefined
    }
  }

  private setupShutdownHandlers(): void {
    const shutdown = async (signal: string) => {
      if (this.isShuttingDown) return
      this.isShuttingDown = true
      
      console.log(`\nReceived ${signal}, shutting down gracefully...`)
      
      try {
        await this.zmqClient.stop()
        await redis.disconnect()
        console.log('Shutdown complete')
        process.exit(0)
      } catch (error) {
        console.error('Error during shutdown:', error)
        process.exit(1)
      }
    }

    process.on('SIGINT', () => shutdown('SIGINT'))
    process.on('SIGTERM', () => shutdown('SIGTERM'))
    process.on('uncaughtException', (error) => {
      console.error('Uncaught exception:', error)
      shutdown('uncaughtException')
    })
    process.on('unhandledRejection', (reason, promise) => {
      console.error('Unhandled rejection at:', promise, 'reason:', reason)
    })
  }
}

// Start the service
const service = new ZmqListenerService()
service.start()