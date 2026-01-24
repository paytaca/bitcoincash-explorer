// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  // Nuxt/Vue devtools can crash SSR in some environments (seen as:
  // "null is not an object (evaluating 'instance.__vrv_devtools = info')").
  // Re-enable once your environment is stable.
  devtools: { enabled: false },
  css: ['~/assets/main.css'],
  app: {
    head: {
      title: 'Bitcoin Cash Explorer',
      titleTemplate: 'Bitcoin Cash Explorer',
      meta: [
        { name: 'description', content: 'Explore Bitcoin Cash blocks, transactions, and CashTokens with BCMR metadata.' },
        { name: 'robots', content: 'index,follow' },

        // Open Graph
        { property: 'og:site_name', content: 'Bitcoin Cash Explorer' },
        { property: 'og:type', content: 'website' },
        { property: 'og:title', content: 'Bitcoin Cash Explorer' },
        { property: 'og:description', content: 'Explore Bitcoin Cash blocks, transactions, and CashTokens with BCMR metadata.' },
        { property: 'og:image', content: '/og-image.png' },
        { property: 'og:image:width', content: '1200' },
        { property: 'og:image:height', content: '630' },

        // Twitter
        { name: 'twitter:card', content: 'summary_large_image' },
        { name: 'twitter:title', content: 'Bitcoin Cash Explorer' },
        { name: 'twitter:description', content: 'Explore Bitcoin Cash blocks, transactions, and CashTokens with BCMR metadata.' },
        { name: 'twitter:image', content: '/og-image.png' }
      ],
      link: [
        { rel: 'icon', type: 'image/png', href: '/favicon/favicon-96x96.png', sizes: '96x96' },
        { rel: 'icon', type: 'image/svg+xml', href: '/favicon/favicon.svg' },
        { rel: 'shortcut icon', href: '/favicon/favicon.ico' },
        // Some clients request these at the site root regardless of configured paths.
        { rel: 'apple-touch-icon', href: '/apple-touch-icon.png', sizes: '180x180' },
        { rel: 'apple-touch-icon-precomposed', href: '/apple-touch-icon-precomposed.png', sizes: '180x180' },
        { rel: 'manifest', href: '/favicon/site.webmanifest' }
      ]
    }
  },

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
