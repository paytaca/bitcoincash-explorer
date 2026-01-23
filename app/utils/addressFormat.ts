import { decodeCashAddress, encodeCashAddress } from '@bitauth/libauth'

export type AddressDisplayMode = 'cash' | 'token'

function normalizeInputAddress(addr: string) {
  return addr.trim()
}

function isCashAddrLike(v: string) {
  // Minimal guard to avoid trying to decode non-address strings.
  return v.includes(':') || /^[qpzr][0-9a-z]{20,}$/.test(v)
}

export function convertCashAddrDisplay(addr: string, mode: AddressDisplayMode): string {
  const input = normalizeInputAddress(addr)
  if (!input || input === '—') return input
  if (!isCashAddrLike(input)) return input

  const decoded = decodeCashAddress(input)
  if (typeof decoded === 'string') return input

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
  return typeof encoded === 'string' ? input : encoded.address
}

