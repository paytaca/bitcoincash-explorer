import { decodeCashAddress, encodeCashAddress } from '@bitauth/libauth'

export type AddressDisplayMode = 'cash' | 'token'

function normalizeInputAddress(addr: string) {
  return addr.trim()
}

function isCashAddrLike(v: string) {
  // Minimal guard to avoid trying to decode non-address strings.
  return v.includes(':') || /^[qpzr][0-9a-z]{20,}$/i.test(v)
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

export function convertCashAddrDisplay(addr: string, mode: AddressDisplayMode): string {
  const input = normalizeInputAddress(addr)
  if (!input || input === '—') return input
  if (!isCashAddrLike(input)) return input

  const hadPrefix = input.includes(':')
  const decoded = decodeAnyCashAddress(input)
  if (!decoded) return input

  // decoded.type is one of: p2pkh, p2sh, p2pkhWithTokens, p2shWithTokens
  const targetType =
    mode === 'token'
      ? decoded.type === 'p2pkh'
        ? 'p2pkhWithTokens'
        : decoded.type === 'p2sh'
          ? 'p2shWithTokens'
          : decoded.type
      : decoded.type === 'p2pkhWithTokens'
        ? 'p2pkh'
        : decoded.type === 'p2shWithTokens'
          ? 'p2sh'
          : decoded.type

  // In libauth v3.x, encodeCashAddress takes a single object argument and returns
  // either an error string or { address }.
  const encoded = encodeCashAddress({ prefix: decoded.prefix, type: targetType as any, payload: decoded.payload })
  if (typeof encoded === 'string') return input
  if (hadPrefix) return encoded.address
  // Keep prefixless addresses prefixless for display.
  return encoded.address.includes(':') ? encoded.address.split(':').slice(1).join(':') : encoded.address
}

