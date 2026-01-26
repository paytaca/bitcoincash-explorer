const BCMR_FETCH_TIMEOUT_MS = 15_000

export type TokenMeta = { name?: string; symbol?: string; decimals?: number }

function normalizeTokenMeta(payload: unknown): TokenMeta {
  if (payload && typeof payload === 'object' && !Array.isArray(payload)) {
    const p = payload as Record<string, unknown>
    const t = (p?.token ?? p) as Record<string, unknown>
    return {
      name: typeof p?.name === 'string' ? p.name : undefined,
      symbol: typeof t?.symbol === 'string' ? t.symbol : undefined,
      decimals: typeof t?.decimals === 'number' && Number.isFinite(t.decimals) ? t.decimals : undefined
    }
  }
  if (Array.isArray(payload) && payload.length > 0) {
    const candidates = payload as Record<string, unknown>[]
    const scored = candidates
      .map((c) => {
        const t = (c?.token ?? c) as Record<string, unknown>
        const score =
          (t?.symbol ? 3 : 0) +
          (Number.isFinite(t?.decimals) ? 3 : 0) +
          (c?.name ? 2 : 0) +
          ((c as any)?.uris?.icon ? 1 : 0)
        return { c, t, score }
      })
      .sort((a, b) => (b.score as number) - (a.score as number))
    const best = scored[0]?.c
    const bestToken = (best?.token ?? best) as Record<string, unknown>
    return {
      name: typeof best?.name === 'string' ? best.name : undefined,
      symbol: typeof bestToken?.symbol === 'string' ? bestToken.symbol : undefined,
      decimals:
        typeof bestToken?.decimals === 'number' && Number.isFinite(bestToken.decimals)
          ? bestToken.decimals
          : undefined
    }
  }
  return {}
}

/**
 * Fetch BCMR token metadata for a category. Returns normalized { name?, symbol?, decimals? }
 * or {} on failure. Used by the tx API to enrich responses so the client doesn't need a
 * separate BCMR request.
 */
export async function getTokenMeta(
  category: string,
  bcmrBaseUrl: string
): Promise<TokenMeta> {
  const configured = String(bcmrBaseUrl || '').trim()
  if (!configured) return {}

  const base = configured.replace(/\/+$/, '')
  const root = base.endsWith('/api') ? base.slice(0, -4) : base
  const encoded = encodeURIComponent(category)

  const candidates = [
    `${root}/api/tokens/${encoded}/`,
    `${root}/api/tokens/${encoded}`,
    `${base}/tokens/${encoded}/`,
    `${base}/tokens/${encoded}`,
    `${root}/bcmr/token/${encoded}/all`,
    `${root}/bcmr/token/${encoded}/all/`
  ]

  const fetchOptions: Record<string, unknown> = {
    method: 'GET',
    headers: {
      Accept: 'application/json',
      'User-Agent': 'BitcoinCashExplorer/1.0 (+https://github.com/paytaca/bitcoincash-explorer)'
    }
  }
  if (typeof (globalThis as any).AbortSignal?.timeout === 'function') {
    fetchOptions.signal = (globalThis as any).AbortSignal.timeout(BCMR_FETCH_TIMEOUT_MS)
  }

  for (const url of candidates) {
    try {
      const payload = await $fetch(url as string, fetchOptions as any)
      return normalizeTokenMeta(payload)
    } catch {
      // try next candidate
    }
  }
  return {}
}
