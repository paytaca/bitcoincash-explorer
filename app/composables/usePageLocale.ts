/**
 * SSR/client-stable locale.
 *
 * Vue hydration warnings can happen if SSR renders with one locale/timezone and
 * the client renders with another. We store the server-chosen locale in Nuxt
 * payload so the client uses the same value during hydration.
 */
export function usePageLocale() {
  const locale = useState<string>('bchexplorer.locale', () => {
    const al = useRequestHeaders(['accept-language'])['accept-language']
    return al?.split(',')?.[0] || 'en-US'
  })

  // If we ever render purely client-side (no SSR payload), pick a sensible default.
  if (import.meta.client && (!locale.value || typeof locale.value !== 'string')) {
    locale.value = navigator.language || 'en-US'
  }

  return locale
}

