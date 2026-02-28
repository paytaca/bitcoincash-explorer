import { createHash } from 'crypto'
import { Subscriber } from 'zeromq'
import { config } from './config.js'
import { redis, BlockData, TransactionData } from './redis-client.js'
import { rpc } from './rpc-client.js'

export class ZmqClient {
  private subscriber: Subscriber
  private isRunning = false
  private reconnectDelay = 1000
  private readonly maxReconnectDelay = 30000
  private lastBlockHash: string | null = null

  constructor() {
    this.subscriber = new Subscriber()
  }

  async start(): Promise<void> {
    console.log(`Connecting to ZMQ at ${config.zmqHost}:${config.zmqPort}...`)
    
    try {
      await this.subscriber.connect(`tcp://${config.zmqHost}:${config.zmqPort}`)
      
      // Subscribe to topics (using what user has configured)
      await this.subscriber.subscribe('rawblock')
      await this.subscriber.subscribe('rawtx')
      // Note: 'sequence' topic not available in user's config, handling reorgs differently
      
      console.log('Subscribed to ZMQ topics: rawblock, rawtx')
      console.log('Note: sequence topic not configured - reorgs handled via block validation')
      
      // Get the latest block hash for reorg detection
      const latestBlock = await redis.getLatestBlock()
      if (latestBlock) {
        this.lastBlockHash = latestBlock.hash
        console.log(`Last known block: #${latestBlock.height} - ${this.lastBlockHash.slice(0, 16)}...`)
      }
      
      this.isRunning = true
      this.reconnectDelay = 1000
      
      // Process messages
      this.processMessages()
      
    } catch (error) {
      console.error('Failed to connect to ZMQ:', error)
      this.scheduleReconnect()
    }
  }

  private async processMessages(): Promise<void> {
    while (this.isRunning) {
      try {
        const [topic, message] = await this.subscriber.receive()
        const topicStr = topic.toString()
        
        switch (topicStr) {
          case 'rawblock':
            await this.handleRawBlock(message)
            break
          case 'rawtx':
            await this.handleRawTx(message)
            break
          case 'hashblock':
            // Optional: Use for quick validation without parsing full block
            break
          case 'hashtx':
            // Optional: Use for quick tx notifications
            break
        }
        
      } catch (error) {
        if (this.isRunning) {
          console.error('Error processing ZMQ message:', error)
          this.scheduleReconnect()
        }
      }
    }
  }

  private async handleRawBlock(data: Buffer): Promise<void> {
    try {
      // Get block hash from the header
      const blockHash = this.calculateBlockHash(data.slice(0, 80))
      
      // Fetch full block details from RPC
      const blockInfo = await rpc.getBlock(blockHash, 2)
      
      // Check for reorgs - if this block doesn't connect to our chain
      if (this.lastBlockHash && blockInfo.previousblockhash !== this.lastBlockHash) {
        console.log(`Potential reorg detected: new block ${blockHash.slice(0, 16)}... doesn't connect to ${this.lastBlockHash.slice(0, 16)}...`)
        await this.handleReorg(blockInfo)
      }
      
      const block: BlockData = {
        hash: blockInfo.hash,
        height: blockInfo.height,
        time: blockInfo.time,
        size: blockInfo.size,
        txCount: blockInfo.tx?.length || 0,
        miner: this.extractMinerFromBlock(blockInfo)
      }
      
      await redis.pushBlock(block)
      this.lastBlockHash = blockHash
      console.log(`Added block #${block.height} - ${block.miner || 'Unknown'}`)
      
      // Process all transactions in this block
      const txs: any[] = blockInfo.tx || []
      for (const tx of txs) {
        const txid = tx.txid

        // Check if this tx was in our mempool
        const isInMempool = await redis.isInMempool(txid)
        if (isInMempool) {
          await redis.markTransactionConfirmed(txid, block.height, 1)
          await redis.removeFullTransaction(txid)
        } else {
          // Add confirmed transaction that wasn't in our mempool
          const vout = tx.vout || []
          const amount = vout.reduce((sum: number, output: any) => {
            return sum + (typeof output.value === 'number' ? output.value : 0)
          }, 0)

          const hasTokens = (tx.vin || []).some((v: any) => v.tokenData != null) ||
                           (tx.vout || []).some((o: any) => o.tokenData != null)

          const txData: TransactionData = {
            txid,
            status: 'confirmed',
            time: blockInfo.time,
            amount,
            hasTokens,
            blockHeight: block.height,
            confirmations: 1
          }
          await redis.pushTransaction(txData)
        }

        // Remove from mempool set regardless
        await redis.removeFromMempool(txid)
      }
      
    } catch (error) {
      console.error('Error handling raw block:', error)
    }
  }

  private async handleReorg(newBlock: any): Promise<void> {
    try {
      // Find the common ancestor
      let currentHeight = newBlock.height - 1
      let foundCommonAncestor = false
      const blocksToRemove: string[] = []
      
      // Check our recent blocks to find where the fork happened
      const ourBlocks = await redis.getBlocks()
      const ourBlockHashes = new Set(ourBlocks.map(b => b.hash))
      
      // Walk back from the new block to find common ancestor
      let checkBlock = newBlock.previousblockhash
      while (currentHeight > 0 && !foundCommonAncestor) {
        if (ourBlockHashes.has(checkBlock)) {
          foundCommonAncestor = true
          console.log(`Found common ancestor at height ${currentHeight}: ${checkBlock.slice(0, 16)}...`)
          break
        }
        
        // This block is part of the orphaned chain
        try {
          const blockInfo = await rpc.getBlock(checkBlock, 1)
          blocksToRemove.push(checkBlock)
          checkBlock = blockInfo.previousblockhash
          currentHeight--
        } catch (e) {
          console.error(`Failed to fetch block ${checkBlock} during reorg handling`)
          break
        }
        
        // Safety limit - don't go back more than 100 blocks
        if (blocksToRemove.length > 100) {
          console.warn('Reorg detection hit safety limit, clearing all blocks')
          // Clear all blocks and let initial sync repopulate
          for (const block of ourBlocks) {
            await redis.removeBlock(block.hash)
          }
          return
        }
      }
      
      // Remove orphaned blocks
      for (const hash of blocksToRemove) {
        console.log(`Removing orphaned block: ${hash.slice(0, 16)}...`)
        await redis.removeBlock(hash)
      }
      
      // Reset last block hash to trigger full validation on next block
      this.lastBlockHash = null
      
    } catch (error) {
      console.error('Error handling reorg:', error)
    }
  }

  private async handleRawTx(data: Buffer): Promise<void> {
    try {
      const partialTx = this.parseRawTransaction(data)

      let fullTx: any
      try {
        fullTx = await rpc.getRawTransaction(partialTx.txid, true)
      } catch (rpcError) {
        console.warn(`Failed to fetch tx details from RPC for ${partialTx.txid.slice(0, 16)}..., using parsed data`)
        fullTx = null
      }

      let tx: TransactionData
      if (fullTx) {
        const vout = fullTx.vout || []
        const amount = vout.reduce((sum: number, output: any) => {
          return sum + (typeof output.value === 'number' ? output.value : 0)
        }, 0)

        const hasTokens = (fullTx.vin || []).some((v: any) => v.tokenData != null) ||
                         (fullTx.vout || []).some((o: any) => o.tokenData != null)

        tx = {
          txid: partialTx.txid,
          status: 'mempool',
          time: Math.floor(Date.now() / 1000),
          amount,
          hasTokens,
          size: partialTx.size
        }

        await redis.storeFullTransaction(fullTx)
      } else {
        tx = partialTx
      }

      await redis.pushTransaction(tx)
      await redis.addToMempool(tx.txid)

      const tokenInfo = tx.hasTokens ? ' [tokens]' : ''
      console.log(`Added mempool tx: ${tx.txid.slice(0, 16)}... (${tx.amount?.toFixed(8)} BCH)${tokenInfo}`)

    } catch (error) {
      console.error('Error handling raw tx:', error)
    }
  }

  private calculateBlockHash(header: Buffer): string {
    const hash1 = createHash('sha256').update(header).digest()
    const hash2 = createHash('sha256').update(hash1).digest()
    return Buffer.from(hash2).reverse().toString('hex')
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

  private parseRawTransaction(data: Buffer): TransactionData {
    let offset = 0
    
    // Version (4 bytes)
    offset += 4
    
    // Input count (varint)
    const inputCount = this.readVarInt(data, offset)
    offset = inputCount.offset
    
    // Skip all inputs
    for (let i = 0; i < inputCount.value; i++) {
      // Previous output hash (32 bytes)
      offset += 32
      // Previous output index (4 bytes)
      offset += 4
      // Script length (varint)
      const scriptLen = this.readVarInt(data, offset)
      offset = scriptLen.offset
      // Script
      offset += scriptLen.value
      // Sequence (4 bytes)
      offset += 4
    }
    
    // Output count (varint)
    const outputCount = this.readVarInt(data, offset)
    offset = outputCount.offset
    
    // Parse outputs to sum values
    let totalValue = 0
    for (let i = 0; i < outputCount.value; i++) {
      // Value (8 bytes)
      const value = data.readBigUInt64LE(offset)
      offset += 8
      totalValue += Number(value) / 100000000  // Convert satoshis to BCH
      
      // Script length (varint)
      const scriptLen = this.readVarInt(data, offset)
      offset = scriptLen.offset
      // Script
      offset += scriptLen.value
    }
    
    // Locktime (4 bytes)
    offset += 4
    
    // Calculate txid
    const txidHash1 = createHash('sha256').update(data).digest()
    const txidHash2 = createHash('sha256').update(txidHash1).digest()
    const txid = Buffer.from(txidHash2).reverse().toString('hex')
    
    return {
      txid,
      status: 'mempool',
      time: Math.floor(Date.now() / 1000),
      amount: totalValue,
      hasTokens: false,
      size: data.length
    }
  }

  private readVarInt(buffer: Buffer, offset: number): { value: number; offset: number } {
    const first = buffer[offset]
    if (first < 0xfd) {
      return { value: first, offset: offset + 1 }
    } else if (first === 0xfd) {
      return { value: buffer.readUInt16LE(offset + 1), offset: offset + 3 }
    } else if (first === 0xfe) {
      return { value: buffer.readUInt32LE(offset + 1), offset: offset + 5 }
    } else {
      const low = buffer.readUInt32LE(offset + 1)
      const high = buffer.readUInt32LE(offset + 5)
      return { value: low + (high * 0x100000000), offset: offset + 9 }
    }
  }

  private scheduleReconnect(): void {
    if (!this.isRunning) return
    
    console.log(`Reconnecting in ${this.reconnectDelay}ms...`)
    setTimeout(() => {
      this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay)
      this.start()
    }, this.reconnectDelay)
  }

  async stop(): Promise<void> {
    this.isRunning = false
    try {
      await this.subscriber.close()
    } catch (error) {
      // Ignore close errors
    }
  }
}