// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  // Nuxt/Vue devtools can crash SSR in some environments (seen as:
  // "null is not an object (evaluating 'instance.__vrv_devtools = info')").
  // Re-enable once your environment is stable.
  devtools: { enabled: false },
  css: ['~/assets/main.css'],

  runtimeConfig: {
    // NOTE: These are runtime-config values and should be provided via environment
    // variables at runtime (e.g. Docker/Compose). Nuxt maps these from:
    // - NUXT_BCH_RPC_URL / NUXT_BCH_RPC_USER / NUXT_BCH_RPC_PASS
    bchRpcUrl: '',
    bchRpcUser: '',
    bchRpcPass: '',

    // - NUXT_BCMR_BASE_URL
    bcmrBaseUrl: 'https://bcmr.paytaca.com',

    public: {
      // - NUXT_PUBLIC_CHAIN
      chain: 'mainnet'
    }
  },

  nitro: {
    routeRules: {
      // BCMR metadata rarely changes; cache for fast repeated lookups.
      '/api/bcmr/token/**': { cache: { swr: true, maxAge: 60 * 60 } }
    }
  }
})
