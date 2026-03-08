// https://nuxt.com/docs/api/configuration/nuxt-config
const env = process.env as Record<string, string | undefined>
const siteUrl = (env.NUXT_PUBLIC_SITE_URL || env.SITE_URL || 'https://bchexplorer.info').replace(/\/$/, '')

export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  // Disable SSR - Go API server handles all backend
  ssr: false,
  devtools: { enabled: false },
  css: ['~/assets/main.css'],

  vite: {
    build: {
      // Cloudflare/Safari can intermittently fail fetching some module chunks over HTTP/2
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
        { rel: 'apple-touch-icon', href: '/apple-touch-icon.png', sizes: '180x180' },
        { rel: 'apple-touch-icon-precomposed', href: '/apple-touch-icon-precomposed.png', sizes: '180x180' },
        { rel: 'manifest', href: '/favicon/site.webmanifest' }
      ]
    }
  },

  runtimeConfig: {
    public: {
      chain: (env.CHAIN || 'mainnet') as string,
      mainnetUrl: (env.MAINNET_URL || '') as string,
      chipnetUrl: (env.CHIPNET_URL || '') as string
    }
  },

  nitro: {
    output: {
      publicDir: '.output/public'
    }
  }
})
