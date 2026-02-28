// https://nuxt.com/docs/api/configuration/nuxt-config
const env = ((globalThis as any).process?.env || {}) as Record<string, string | undefined>
const siteUrl = (env.NUXT_PUBLIC_SITE_URL || env.SITE_URL || 'https://bchexplorer.info').replace(/\/$/, '')

export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  // Nuxt/Vue devtools can crash SSR in some environments (seen as:
  // "null is not an object (evaluating 'instance.__vrv_devtools = info')").
  // Re-enable once your environment is stable.
  devtools: { enabled: false },
  css: ['~/assets/main.css'],
  vite: {
    build: {
      // Cloudflare/Safari can intermittently fail fetching some module chunks over HTTP/2
      // (e.g. filenames ending in `-.js`). Using hex hashes avoids `-`/`_` characters in filenames.
      rollupOptions: {
        output: {
          hashCharacters: 'hex'
        }
      }
    }
  },
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
        { property: 'og:url', content: siteUrl },
        { property: 'og:title', content: 'Bitcoin Cash Explorer' },
        { property: 'og:description', content: 'Explore Bitcoin Cash blocks, transactions, and CashTokens with BCMR metadata.' },
        { property: 'og:image', content: `${siteUrl}/og-image.png` },
        { property: 'og:image:secure_url', content: `${siteUrl}/og-image.png` },
        { property: 'og:image:alt', content: 'Bitcoin Cash Explorer' },
        { property: 'og:image:width', content: '1200' },
        { property: 'og:image:height', content: '630' },

        // Twitter
        { name: 'twitter:card', content: 'summary_large_image' },
        { name: 'twitter:domain', content: 'bchexplorer.info' },
        { name: 'twitter:title', content: 'Bitcoin Cash Explorer' },
        { name: 'twitter:description', content: 'Explore Bitcoin Cash blocks, transactions, and CashTokens with BCMR metadata.' },
        { name: 'twitter:image', content: `${siteUrl}/og-image.png` },
        { name: 'twitter:image:alt', content: 'Bitcoin Cash Explorer' }
      ],
      script: [
        { key: 'theme-init', src: '/theme-init.js', tagPosition: 'head' }
      ],
      link: [
        { rel: 'canonical', href: siteUrl },
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

    // Fulcrum (Electrum server)
    // - NUXT_FULCRUM_HOST / NUXT_FULCRUM_PORT / NUXT_FULCRUM_TIMEOUT_MS
    fulcrumHost: '127.0.0.1',
    fulcrumPort: 60001,
    fulcrumTimeoutMs: 10_000,

    // Redis cache (for ZMQ-based data)
    // - NUXT_REDIS_URL or REDIS_URL
    redisUrl: '',

    public: {
      // - NUXT_PUBLIC_CHAIN
      chain: 'mainnet',
      // - NUXT_PUBLIC_MAINNET_URL
      mainnetUrl: '',
      // - NUXT_PUBLIC_CHIPNET_URL
      chipnetUrl: ''
    }
  },

  nitro: {
    storage: {
      // Note: API routes now use Redis directly via ioredis.
      // The cachedData mount is kept as a fallback for any legacy code.
      cachedData: { driver: 'fs', base: './.cache/nitro' }
    },
    routeRules: {
      // Blocks are immutable - cache for 1 week (reduced from 1 year to save memory)
      '/api/bch/block/**': { cache: { swr: true, maxAge: 60 * 60 * 24 * 7 } },
      // Block hash by height is a permanent mapping - cache for 1 week
      '/api/bch/blockhash/**': { cache: { swr: true, maxAge: 60 * 60 * 24 * 7 } },
      // BCMR metadata rarely changes; cache for fast repeated lookups.
      '/api/bcmr/token/**': { cache: { swr: true, maxAge: 60 * 60 } }
    },
    // Worker configuration to limit concurrent memory usage
    worker: true,
    minWorkers: 1,
    maxWorkers: 2,
    // Memory optimization: close connections more aggressively
    timing: false
  }
})
