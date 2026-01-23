// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  // Nuxt/Vue devtools can crash SSR in some environments (seen as:
  // "null is not an object (evaluating 'instance.__vrv_devtools = info')").
  // Re-enable once your environment is stable.
  devtools: { enabled: false },
  css: ['~/assets/main.css'],

  runtimeConfig: {
    bchRpcUrl: process.env.BCH_RPC_URL,
    bchRpcUser: process.env.BCH_RPC_USER,
    bchRpcPass: process.env.BCH_RPC_PASS,

    bcmrBaseUrl: process.env.BCMR_BASE_URL || 'https://bcmr.paytaca.com',

    public: {
      chain: process.env.CHAIN || 'mainnet'
    }
  },

  nitro: {
    routeRules: {
      // BCMR metadata rarely changes; cache for fast repeated lookups.
      '/api/bcmr/token/**': { cache: { swr: true, maxAge: 60 * 60 } }
    }
  }
})
