import { BlockData } from './redis-client.js'

interface BlockTx {
  txid?: string
  vin?: Array<{
    coinbase?: string
    txid?: string
    vout?: number
    scriptSig?: { hex?: string }
  }>
  vout?: Array<{
    value?: number
    n?: number
    scriptPubKey?: { hex?: string; asm?: string }
  }>
}

interface BlockInfo {
  hash: string
  height: number
  time: number
  size: number
  tx?: BlockTx[]
}

export function extractMinerFromBlock(block: BlockInfo): string | undefined {
  const coinbaseTx = block.tx?.[0]
  if (!coinbaseTx) return undefined

  const coinbaseInput = coinbaseTx.vin?.[0]
  if (!coinbaseInput?.coinbase) return undefined

  return extractMinerFromCoinbaseHex(coinbaseInput.coinbase)
}

function extractMinerFromCoinbaseHex(coinbaseHex: string): string | undefined {
  if (!coinbaseHex || !/^[0-9a-fA-F]+$/.test(coinbaseHex) || coinbaseHex.length % 2 !== 0) {
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

export function processBlock(rawBlock: Buffer): BlockData {
  // Parse the block header and transactions from raw bytes
  let offset = 0
  
  // Block header (80 bytes)
  // Version (4 bytes)
  const version = rawBlock.readUInt32LE(offset)
  offset += 4
  
  // Previous block hash (32 bytes)
  offset += 32
  
  // Merkle root (32 bytes)
  offset += 32
  
  // Time (4 bytes)
  const time = rawBlock.readUInt32LE(offset)
  offset += 4
  
  // Bits (4 bytes)
  offset += 4
  
  // Nonce (4 bytes)
  offset += 4
  
  // Transaction count (varint)
  const txCount = readVarInt(rawBlock, offset)
  offset = txCount.offset
  
  // We need the block hash and height, which we don't have from raw data alone
  // We'll need to get this from the ZMQ sequence or compute hash from header
  // For now, we'll return partial data and let the caller fill in the rest
  
  return {
    hash: '', // Will be filled by caller
    height: 0, // Will be filled by caller
    time,
    size: rawBlock.length,
    txCount: txCount.value,
    miner: undefined // Will be extracted from coinbase
  }
}

function readVarInt(buffer: Buffer, offset: number): { value: number; offset: number } {
  const first = buffer[offset]
  if (first < 0xfd) {
    return { value: first, offset: offset + 1 }
  } else if (first === 0xfd) {
    return { value: buffer.readUInt16LE(offset + 1), offset: offset + 3 }
  } else if (first === 0xfe) {
    return { value: buffer.readUInt32LE(offset + 1), offset: offset + 5 }
  } else {
    // 0xff - 64 bit, but we can't read full 64 bits safely in JS
    // For our purposes, blocks won't have this many txs
    const low = buffer.readUInt32LE(offset + 1)
    const high = buffer.readUInt32LE(offset + 5)
    return { value: low + (high * 0x100000000), offset: offset + 9 }
  }
}