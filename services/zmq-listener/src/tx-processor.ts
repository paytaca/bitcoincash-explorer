import { TransactionData } from './redis-client.js'

interface TxInput {
  txid?: string
  vout?: number
  scriptSig?: { hex?: string; asm?: string }
  sequence?: number
  coinbase?: string
  tokenData?: any
}

interface TxOutput {
  value?: number
  n?: number
  scriptPubKey?: { hex?: string; asm?: string }
  tokenData?: any
}

interface Transaction {
  txid?: string
  hash?: string
  version?: number
  size?: number
  vin?: TxInput[]
  vout?: TxOutput[]
  locktime?: number
  time?: number
  blocktime?: number
}

export function processRawTransaction(rawTx: Buffer, time?: number): TransactionData {
  // Parse the transaction from raw bytes
  let offset = 0
  
  // Version (4 bytes)
  const version = rawTx.readUInt32LE(offset)
  offset += 4
  
  // Check for witness flag (segwit - Bitcoin Cash doesn't use this, but for completeness)
  let isWitness = false
  if (rawTx[offset] === 0 && rawTx[offset + 1] === 1) {
    isWitness = true
    offset += 2
  }
  
  // Input count (varint)
  const inputCount = readVarInt(rawTx, offset)
  offset = inputCount.offset
  
  // Skip inputs (we just need to count them and detect coinbase)
  let hasCoinbase = false
  for (let i = 0; i < inputCount.value; i++) {
    // Previous output hash (32 bytes)
    const prevHash = rawTx.slice(offset, offset + 32)
    offset += 32
    
    // Previous output index (4 bytes)
    const prevIndex = rawTx.readUInt32LE(offset)
    offset += 4
    
    // Check if this is a coinbase (null hash and 0xffffffff index)
    if (i === 0 && prevHash.equals(Buffer.alloc(32)) && prevIndex === 0xffffffff) {
      hasCoinbase = true
    }
    
    // Script length (varint)
    const scriptLen = readVarInt(rawTx, offset)
    offset = scriptLen.offset
    
    // Script
    offset += scriptLen.value
    
    // Sequence (4 bytes)
    offset += 4
  }
  
  // Output count (varint)
  const outputCount = readVarInt(rawTx, offset)
  offset = outputCount.offset
  
  // Parse outputs to sum values
  let totalValue = 0
  for (let i = 0; i < outputCount.value; i++) {
    // Value (8 bytes)
    const value = readUInt64LE(rawTx, offset)
    offset += 8
    
    // Script length (varint)
    const scriptLen = readVarInt(rawTx, offset)
    offset = scriptLen.offset
    
    // Script
    offset += scriptLen.value
    
    totalValue += value
  }
  
  // Skip witness data if present
  if (isWitness) {
    for (let i = 0; i < inputCount.value; i++) {
      const witnessCount = readVarInt(rawTx, offset)
      offset = witnessCount.offset
      for (let j = 0; j < witnessCount.value; j++) {
        const witnessLen = readVarInt(rawTx, offset)
        offset = witnessLen.offset
        offset += witnessLen.value
      }
    }
  }
  
  // Locktime (4 bytes)
  offset += 4
  
  // Calculate txid from the raw transaction
  const txid = calculateTxid(rawTx)
  
  return {
    txid,
    status: 'mempool',
    time: time || Math.floor(Date.now() / 1000),
    amount: totalValue,
    hasTokens: false, // Would need to parse token data from outputs
    size: rawTx.length
  }
}

export function processVerboseTransaction(tx: Transaction): TransactionData {
  const vout = tx.vout || []
  const vin = tx.vin || []
  
  // Calculate total output value
  const amount = vout.reduce((sum, output) => {
    return sum + (typeof output.value === 'number' ? output.value : 0)
  }, 0)
  
  // Check for tokens
  const hasTokens = vin.some(v => v.tokenData != null) || 
                    vout.some(o => o.tokenData != null)
  
  // Calculate fee if possible (inputs - outputs)
  // Note: For mempool txs, we don't have input values easily
  let fee: number | undefined
  if (tx.vin?.every(v => v.coinbase === undefined)) {
    // Can't easily calculate without looking up input values
    fee = undefined
  }
  
  return {
    txid: tx.txid || '',
    status: 'mempool',
    time: tx.time || tx.blocktime || Math.floor(Date.now() / 1000),
    amount,
    hasTokens,
    fee,
    size: tx.size
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
    const low = buffer.readUInt32LE(offset + 1)
    const high = buffer.readUInt32LE(offset + 5)
    return { value: low + (high * 0x100000000), offset: offset + 9 }
  }
}

function readUInt64LE(buffer: Buffer, offset: number): number {
  // Read as Number (will lose precision above 2^53, but BCH amounts are in satoshis)
  const low = buffer.readUInt32LE(offset)
  const high = buffer.readUInt32LE(offset + 4)
  return low + (high * 0x100000000)
}

function calculateTxid(rawTx: Buffer): string {
  // Calculate double SHA256 of raw tx and reverse bytes for txid
  const crypto = require('crypto')
  const hash1 = crypto.createHash('sha256').update(rawTx).digest()
  const hash2 = crypto.createHash('sha256').update(hash1).digest()
  // Reverse the hash bytes
  const reversed = Buffer.from(hash2).reverse()
  return reversed.toString('hex')
}