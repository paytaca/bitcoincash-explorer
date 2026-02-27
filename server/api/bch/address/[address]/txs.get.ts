import { decodeCashAddress } from '@bitauth/libauth'
import crypto from 'node:crypto'
import { createFulcrumClient } from '../../../../utils/fulcrumRpc'
import { getTokenMeta } from '../../../../utils/bcmr'
import { bchRpc } from '../../../../utils/bchRpc'

type AddressTxStatus = 'mempool' | 'confirmed'
type AddressTxDirection = 'sent' | 'received'

export type AddressTxItem = {
  txid: string
  status: AddressTxStatus
  time?: number
  blockHeight?: number
  confirmations?: number
  direction: AddressTxDirection
  net: number // positive = received, negative = sent (relative to address)
  inValue: number // sum of inputs from address (BCH)
  outValue: number // sum of outputs to address (BCH)
  hasTokens?: boolean
}

type AddressBalance = {
  confirmed: number // satoshis
  unconfirmed: number // satoshis
}

export type AddressTokenBalance = {
  category: string
  fungibleAmount: string // bigint as string
  nftCount: number
  nft: { none: number; mutable: number; minting: number }
  utxoCount: number
}

function toNum(v: unknown): number | undefined {
  return typeof v === 'number' && Number.isFinite(v) ? v : undefined
}

function normalizeAddrCandidate(v: string): string[] {
  const s = v.trim()
  if (!s) return []
  if (s.includes(':')) return [s]
  // Try common prefixes for decoding (mainnet/testnet/regtest).
  return [`bitcoincash:${s}`, `bchtest:${s}`, `bchreg:${s}`, s]
}

function decodeAnyCashAddress(addr: string) {
  for (const candidate of normalizeAddrCandidate(addr)) {
    const decoded = decodeCashAddress(candidate)
    if (typeof decoded !== 'string') return decoded
  }
  return undefined
}

function safeDecodePathParam(v: string): string {
  // Nuxt router params are usually decoded, but if the client encoded the path segment
  // (e.g. ':' => '%3A'), we may still receive percent-encoded strings here.
  try {
    return decodeURIComponent(v)
  } catch {
    return v
  }
}

function baseCashAddrType(t: string) {
  return t === 'p2pkhWithTokens' ? 'p2pkh' : t === 'p2shWithTokens' ? 'p2sh' : t
}

function lockingScriptFromDecodedCashaddr(decoded: any): Buffer {
  const t = baseCashAddrType(String(decoded?.type || ''))
  const payload = decoded?.payload
  if (!(payload instanceof Uint8Array)) {
    throw createError({ statusCode: 400, statusMessage: 'Unsupported address payload' })
  }
  const hash = Buffer.from(payload)

  if (t === 'p2pkh') {
    if (hash.length !== 20) {
      throw createError({ statusCode: 400, statusMessage: 'Unsupported P2PKH address payload' })
    }
    // OP_DUP OP_HASH160 PUSH20 <hash160> OP_EQUALVERIFY OP_CHECKSIG
    return Buffer.concat([Buffer.from('76a914', 'hex'), hash, Buffer.from('88ac', 'hex')])
  }
  if (t === 'p2sh') {
    if (hash.length === 20) {
      // P2SH20: OP_HASH160 PUSH20 <hash160> OP_EQUAL
      return Buffer.concat([Buffer.from('a914', 'hex'), hash, Buffer.from('87', 'hex')])
    }
    if (hash.length === 32) {
      // P2SH32: OP_HASH256 PUSH32 <hash256> OP_EQUAL
      return Buffer.concat([Buffer.from('aa20', 'hex'), hash, Buffer.from('87', 'hex')])
    }
    throw createError({ statusCode: 400, statusMessage: 'Unsupported P2SH address payload' })
  }

  throw createError({ statusCode: 400, statusMessage: 'Unsupported address type' })
}

function sha256(buf: Buffer): Buffer {
  return crypto.createHash('sha256').update(buf).digest()
}

function reverseBytes(buf: Buffer): Buffer {
  const out = Buffer.allocUnsafe(buf.length)
  for (let i = 0; i < buf.length; i++) out[i] = buf[buf.length - 1 - i]
  return out
}

function toScripthashHex(lockingScript: Buffer): string {
  return reverseBytes(sha256(lockingScript)).toString('hex')
}

function readVarInt(buf: Buffer, offset: number): { value: number; size: number } {
  const first = buf[offset]
  if (first < 0xfd) return { value: first, size: 1 }
  if (first === 0xfd) return { value: buf.readUInt16LE(offset + 1), size: 3 }
  if (first === 0xfe) return { value: buf.readUInt32LE(offset + 1), size: 5 }
  // 0xff
  const v = Number(buf.readBigUInt64LE(offset + 1))
  return { value: v, size: 9 }
}

type DecodedTx = {
  vin: { prevTxid: string; prevVout: number }[]
  vout: { valueSats: bigint; lockingScript: Buffer; hasToken: boolean }[]
}

function decodeTxHex(hex: string): DecodedTx {
  const buf = Buffer.from(hex, 'hex')
  let o = 0

  // version
  if (buf.length < 4) throw new Error('tx too short')
  o += 4

  // segwit marker/flag (BCH should not have it; still guard)
  if (buf[o] === 0x00 && buf[o + 1] !== 0x00) {
    throw new Error('segwit tx not supported')
  }

  // inputs
  const inCount = readVarInt(buf, o)
  o += inCount.size
  const vin: DecodedTx['vin'] = []
  for (let i = 0; i < inCount.value; i++) {
    const prevHashLE = buf.subarray(o, o + 32)
    o += 32
    const prevVout = buf.readUInt32LE(o)
    o += 4
    const scriptLen = readVarInt(buf, o)
    o += scriptLen.size + scriptLen.value
    o += 4 // sequence
    vin.push({ prevTxid: reverseBytes(prevHashLE as any as Buffer).toString('hex'), prevVout })
  }

  // outputs
  const outCount = readVarInt(buf, o)
  o += outCount.size
  const vout: DecodedTx['vout'] = []
  for (let i = 0; i < outCount.value; i++) {
    const valueSats = buf.readBigUInt64LE(o)
    o += 8
    const tokenPrefixAndScriptLen = readVarInt(buf, o)
    o += tokenPrefixAndScriptLen.size
    const segment = Buffer.from(buf.subarray(o, o + tokenPrefixAndScriptLen.value))
    o += tokenPrefixAndScriptLen.value

    // Token Output Format: https://reference.cash/protocol/blockchain/transaction#token-output-format
    // segment is either:
    // - <lockingScript>
    // - 0xef <categoryId:32> <bitfield:1> [commitmentLen+commitment] [ftAmount] <lockingScript>
    let hasToken = false
    let lockingScript = segment
    if (segment.length > 0 && segment[0] === 0xef) {
      hasToken = true
      let p = 1 + 32 // prefix + category id
      if (segment.length < p + 1) throw new Error('invalid token prefix')
      const bitfield = segment[p]
      p += 1

      const hasCommitmentLen = (bitfield & 0x40) !== 0
      const hasAmount = (bitfield & 0x10) !== 0

      if (hasCommitmentLen) {
        const clen = readVarInt(segment, p)
        p += clen.size
        p += clen.value
        if (p > segment.length) throw new Error('invalid token commitment length')
      }

      if (hasAmount) {
        const amt = readVarInt(segment, p)
        p += amt.size
        // NOTE: value is read but not used here; we only need to skip it.
        if (p > segment.length) throw new Error('invalid token amount')
      }

      lockingScript = Buffer.from(segment.subarray(p))
    }

    vout.push({ valueSats, lockingScript, hasToken })
  }

  // locktime (ignore)
  return { vin, vout }
}

function sumOutputsToScript(decoded: DecodedTx, targetScript: Buffer): bigint {
  let sum = BigInt(0)
  for (const o of decoded.vout) {
    if (o.lockingScript.length === targetScript.length && o.lockingScript.equals(targetScript)) sum += o.valueSats
  }
  return sum
}

async function getTxHex(fulcrum: ReturnType<typeof createFulcrumClient>, txid: string): Promise<string> {
  try {
    const res = await fulcrum.request<any>('blockchain.transaction.get', [txid, false])
    if (typeof res === 'string') return res
    if (typeof res?.hex === 'string') return res.hex
  } catch {
    // fallthrough
  }
  const res = await fulcrum.request<any>('blockchain.transaction.get', [txid])
  if (typeof res === 'string') return res
  if (typeof res?.hex === 'string') return res.hex
  throw createError({ statusCode: 502, statusMessage: 'Fulcrum returned unexpected transaction format' })
}

async function getBlockTimeByHeight(fulcrum: ReturnType<typeof createFulcrumClient>, height: number): Promise<number | undefined> {
  if (!Number.isInteger(height) || height <= 0) return undefined
  const headerHex = await fulcrum.request<string>('blockchain.block.header', [height])
  if (typeof headerHex !== 'string') return undefined
  const header = Buffer.from(headerHex, 'hex')
  if (header.length < 80) return undefined
  // Timestamp is 4 bytes LE at offset 68
  return header.readUInt32LE(68)
}

async function getMempoolSeenTime(txid: string): Promise<number | undefined> {
  // Fulcrum mempool list doesn't include a timestamp; best-effort pull from the node's mempool.
  try {
    const entry = await bchRpc<any>('getmempoolentry', [txid])
    const t = entry?.time
    return typeof t === 'number' && Number.isFinite(t) ? t : undefined
  } catch {
    return undefined
  }
}

export default defineEventHandler(async (event) => {
  const addressParam = getRouterParam(event, 'address')
  if (!addressParam) throw createError({ statusCode: 400, statusMessage: 'Missing address' })
  const address = safeDecodePathParam(addressParam)

  const q = getQuery(event)
  const limit = Math.min(100, Math.max(1, Number(q.limit ?? 25) || 25))
  const cursor = Number(q.cursor ?? -1) // to_height (exclusive). -1 means "tip + mempool"
  const window = Math.min(50_000, Math.max(100, Number(q.window ?? 5000) || 5000)) // height window to scan per request
  if (!Number.isFinite(cursor) || cursor < -1) {
    throw createError({ statusCode: 400, statusMessage: 'Invalid cursor' })
  }

  const decodedAddr = decodeAnyCashAddress(address)
  if (!decodedAddr) throw createError({ statusCode: 400, statusMessage: 'Invalid CashAddress' })
  const targetScript = lockingScriptFromDecodedCashaddr(decodedAddr)
  const scripthash = toScripthashHex(targetScript)

  const fulcrum = createFulcrumClient()
  try {
    const tip = await fulcrum.request<{ height: number }>('blockchain.headers.subscribe', [])
    const tipHeight = Number(tip?.height ?? 0)

    // Prefer scripthash-based calls for consistency with history queries.
    // Some Fulcrum setups can behave differently between `blockchain.address.*` and
    // `blockchain.scripthash.*` methods; using scripthash avoids any address parsing ambiguity.
    let bal: any
    try {
      bal = await fulcrum.request<any>('blockchain.scripthash.get_balance', [scripthash, 'include_tokens'])
    } catch {
      // fallback to address-based method (older/compat)
      bal = await fulcrum.request<any>('blockchain.address.get_balance', [address, 'include_tokens'])
    }
    const balance: AddressBalance = {
      confirmed: Number(bal?.confirmed ?? 0),
      unconfirmed: Number(bal?.unconfirmed ?? 0)
    }

    // Token balances: aggregate token UTXOs by category (Fulcrum provides token_data per UTXO).
    type FulcrumUtxo = {
      tx_hash: string
      tx_pos: number
      value: number
      height: number
      token_data?: { category: string; amount?: string; nft?: { capability: 'none' | 'mutable' | 'minting'; commitment?: string } }
    }

    let tokenBalances: AddressTokenBalance[] = []
    const tokenMeta: Record<string, { name?: string; symbol?: string; decimals?: number }> = {}
    try {
      let utxos: FulcrumUtxo[] = []
      try {
        utxos = await fulcrum.request<FulcrumUtxo[]>('blockchain.scripthash.listunspent', [scripthash, 'tokens_only'])
      } catch {
        // fallback to address-based method (older/compat)
        utxos = await fulcrum.request<FulcrumUtxo[]>('blockchain.address.listunspent', [address, 'tokens_only'])
      }
      const byCat = new Map<
        string,
        { fungible: bigint; nftNone: number; nftMutable: number; nftMinting: number; utxos: number }
      >()

      for (const u of utxos || []) {
        const td = u?.token_data
        const cat = typeof td?.category === 'string' ? td.category : ''
        if (!cat) continue
        const entry = byCat.get(cat) || { fungible: BigInt(0), nftNone: 0, nftMutable: 0, nftMinting: 0, utxos: 0 }
        entry.utxos++

        if (typeof td?.amount === 'string' && td.amount) {
          try {
            entry.fungible += BigInt(td.amount)
          } catch {
            // ignore malformed amounts
          }
        }
        const cap = td?.nft?.capability
        if (cap === 'minting') entry.nftMinting++
        else if (cap === 'mutable') entry.nftMutable++
        else if (cap === 'none') entry.nftNone++

        byCat.set(cat, entry)
      }

      tokenBalances = Array.from(byCat.entries()).map(([category, v]) => ({
        category,
        fungibleAmount: v.fungible.toString(),
        nftCount: v.nftNone + v.nftMutable + v.nftMinting,
        nft: { none: v.nftNone, mutable: v.nftMutable, minting: v.nftMinting },
        utxoCount: v.utxos
      }))

      // Sort by: any fungible amount desc, then nft count desc.
      tokenBalances.sort((a, b) => {
        const af = BigInt(a.fungibleAmount || '0')
        const bf = BigInt(b.fungibleAmount || '0')
        if (af !== bf) return af > bf ? -1 : 1
        return (b.nftCount || 0) - (a.nftCount || 0)
      })

      // BCMR token metadata (best-effort)
      const config = useRuntimeConfig()
      const bcmrBaseUrl = String((config as any).bcmrBaseUrl || '').trim()
      if (bcmrBaseUrl) {
        const categories = tokenBalances.map((t) => t.category)
        const entries = await Promise.all(
          categories.map(async (cat) => {
            const meta = await getTokenMeta(cat, bcmrBaseUrl)
            return [cat, meta] as const
          })
        )
        Object.assign(tokenMeta, Object.fromEntries(entries))
      }
    } catch {
      // token balances are optional; ignore if Fulcrum doesn't support tokens_only
    }

    type HistItem = { tx_hash: string; height: number }
    type Picked = { txid: string; status: AddressTxStatus; height?: number }

    // Fulcrum supports pagination by height range:
    // blockchain.scripthash.get_history(scripthash, from_height=0, to_height=-1)
    // See: https://electrum-cash-protocol.readthedocs.io/en/latest/protocol-methods.html#blockchain-scripthash-get-history
    const includeMempool = cursor === -1
    const confirmedToHeightExclusive = cursor === -1 ? Math.max(0, tipHeight + 1) : cursor

    const picked: Picked[] = []
    const seen = new Set<string>()

    // We avoid splitting within the same block height (since history items do not include tx position).
    // This can cause us to slightly exceed `limit`, but avoids missing transactions.
    function takeWithoutSplittingByHeight(items: HistItem[], remaining: number): HistItem[] {
      if (remaining <= 0) return []
      if (items.length <= remaining) return items
      const cut = items.slice(0, remaining)
      const lastHeight = cut[cut.length - 1]?.height
      if (!Number.isInteger(lastHeight) || lastHeight <= 0) return cut
      let i = remaining
      while (i < items.length && items[i]?.height === lastHeight) i++
      return items.slice(0, i)
    }

    // We'll scan backwards in fixed height windows until we have enough.
    // For the first page (cursor=-1), we request to_height=-1 to also include mempool txs.
    let to = confirmedToHeightExclusive
    while (picked.length < limit && to > 0) {
      const from = Math.max(0, to - window)
      const toParam = includeMempool && to === confirmedToHeightExclusive ? -1 : to
      const historyChunk = await fulcrum.request<HistItem[]>('blockchain.scripthash.get_history', [scripthash, from, toParam])

      const confirmedChunk = (historyChunk || []).filter((h) => Number(h?.height) > 0 && typeof h?.tx_hash === 'string') as HistItem[]
      // Fulcrum returns blockchain order; we want newest-first.
      const newestFirst = confirmedChunk.slice().reverse()
      const remaining = limit - picked.length
      const chosen = takeWithoutSplittingByHeight(newestFirst, remaining)

      for (const c of chosen) {
        if (seen.has(c.tx_hash)) continue
        seen.add(c.tx_hash)
        picked.push({ txid: c.tx_hash, status: 'confirmed', height: Number(c.height) })
      }

      // Next window goes older
      to = from
      // If nothing was added from this window and we're not at genesis, still continue; next window might contain txs.
      if (from === 0) break
    }

    // Mempool txs are only included when cursor=-1 (to_height=-1).
    if (includeMempool) {
      try {
        const mempool = await fulcrum.request<{ tx_hash: string; height: number; fee?: number }[]>(
          'blockchain.scripthash.get_mempool',
          [scripthash]
        )
        for (const m of mempool || []) {
          if (picked.length >= limit) break
          if (typeof m?.tx_hash !== 'string') continue
          if (seen.has(m.tx_hash)) continue
          seen.add(m.tx_hash)
          picked.unshift({ txid: m.tx_hash, status: 'mempool' })
        }
      } catch {
        // optional
      }
    }

    const headerTimeCache = new Map<number, number | undefined>()
    const txCache = new Map<string, DecodedTx>()

    async function getDecodedTx(txid: string): Promise<DecodedTx> {
      const existing = txCache.get(txid)
      if (existing) return existing
      const hex = await getTxHex(fulcrum, txid)
      const decoded = decodeTxHex(hex)
      txCache.set(txid, decoded)
      return decoded
    }

    async function inputsFromScript(decoded: DecodedTx): Promise<{ sum: bigint; sawToken: boolean }> {
      let sum = BigInt(0)
      let sawToken = false
      for (const i of decoded.vin) {
        // Coinbase input
        if (i.prevTxid === '0'.repeat(64) && i.prevVout === 0xffffffff) continue
        const prev = await getDecodedTx(i.prevTxid)
        const prevOut = prev.vout[i.prevVout]
        if (!prevOut) continue
        if (prevOut.hasToken) sawToken = true
        if (prevOut.lockingScript.length === targetScript.length && prevOut.lockingScript.equals(targetScript)) {
          sum += prevOut.valueSats
        }
      }
      return { sum, sawToken }
    }

    const results: AddressTxItem[] = []
    for (const p of picked) {
      const decodedTx = await getDecodedTx(p.txid)
      const outSats = sumOutputsToScript(decodedTx, targetScript)
      const { sum: inSats, sawToken } = await inputsFromScript(decodedTx)
      const netSats = outSats - inSats

      const direction: AddressTxDirection = netSats < BigInt(0) ? 'sent' : 'received'
      const hasTokens =
        decodedTx.vout.some((o) => o.hasToken) || sawToken

      let time: number | undefined
      let confirmations: number | undefined
      let blockHeight: number | undefined
      if (p.status === 'confirmed' && Number.isInteger(p.height) && (p.height as number) > 0) {
        blockHeight = p.height as number
        if (!headerTimeCache.has(blockHeight)) {
          headerTimeCache.set(blockHeight, await getBlockTimeByHeight(fulcrum, blockHeight))
        }
        time = headerTimeCache.get(blockHeight)
        if (Number.isInteger(tipHeight) && tipHeight > 0) confirmations = Math.max(0, tipHeight - blockHeight + 1)
      } else if (p.status === 'mempool') {
        time = await getMempoolSeenTime(p.txid)
      }

      const inValue = Number(inSats) / 1e8
      const outValue = Number(outSats) / 1e8
      const net = Number(netSats) / 1e8

      results.push({
        txid: p.txid,
        status: p.status,
        time,
        blockHeight,
        confirmations,
        direction,
        net,
        inValue,
        outValue,
        hasTokens
      })
    }

    // Newest-ish: mempool first, then descending time/height.
    results.sort((a, b) => {
      if (a.status !== b.status) return a.status === 'mempool' ? -1 : 1
      return (b.time || 0) - (a.time || 0)
    })

    const confirmedHeights = results.filter((r) => r.status === 'confirmed' && Number.isInteger(r.blockHeight)).map((r) => r.blockHeight as number)
    const oldestConfirmedHeight = confirmedHeights.length ? Math.min(...confirmedHeights) : undefined
    const nextCursor = oldestConfirmedHeight && oldestConfirmedHeight > 0 ? oldestConfirmedHeight : null

    return {
      address,
      scanned: {
        source: 'fulcrum',
        scripthash,
        tipHeight: Number.isInteger(tipHeight) ? tipHeight : undefined,
        cursor,
        window
      },
      nextCursor,
      balance,
      tokenBalances,
      tokenMeta,
      items: results
    }
  } finally {
    fulcrum.close()
  }
})

