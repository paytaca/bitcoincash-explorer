export default defineEventHandler(async (event) => {
  const category = getRouterParam(event, 'category')
  if (!category) {
    throw createError({ statusCode: 400, statusMessage: 'Missing category' })
  }

  const config = useRuntimeConfig()
  const base = String(config.bcmrBaseUrl || '').replace(/\/+$/, '')
  if (!base) {
    throw createError({ statusCode: 500, statusMessage: 'BCMR_BASE_URL not set' })
  }

  // Current Paytaca BCMR indexer token endpoint (single object response)
  // Example: https://bcmr.paytaca.com/api/tokens/<category>/
  const urlV2 = `${base}/api/tokens/${category}/`

  try {
    return await $fetch(urlV2, { method: 'GET' })
  } catch (_e) {
    // Backwards compatible fallback (older endpoint returns an array of candidates)
    const urlV1 = `${base}/bcmr/token/${category}/all`
    return await $fetch(urlV1, { method: 'GET' })
  }
})

