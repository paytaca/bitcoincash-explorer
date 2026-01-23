export default defineEventHandler(async (event) => {
  const category = getRouterParam(event, 'category')
  if (!category) {
    throw createError({ statusCode: 400, statusMessage: 'Missing category' })
  }

  const config = useRuntimeConfig()
  const configured = String(config.bcmrBaseUrl || '').trim()
  if (!configured) {
    throw createError({ statusCode: 500, statusMessage: 'BCMR_BASE_URL not set' })
  }

  // Support both:
  // - https://bcmr.paytaca.com
  // - https://bcmr.paytaca.com/api
  const base = configured.replace(/\/+$/, '')
  const root = base.endsWith('/api') ? base.slice(0, -4) : base

  const candidates = [
    // Current Paytaca BCMR indexer token endpoint (single object response)
    `${root}/api/tokens/${category}/`,
    // If user configured BCMR_BASE_URL=https://.../api, try /tokens as well
    `${base}/tokens/${category}/`,
    // Backwards compatible fallback (older endpoint returns an array of candidates)
    `${root}/bcmr/token/${category}/all`
  ]

  let lastErr: unknown = undefined
  for (const url of candidates) {
    try {
      return await $fetch(url, { method: 'GET', headers: { Accept: 'application/json' } })
    } catch (e) {
      lastErr = e
    }
  }

  // Surface a useful error to the client (tx page will ignore and continue).
  throw createError({
    statusCode: 502,
    statusMessage: `BCMR lookup failed for category ${category}`,
    cause: lastErr
  })
})

