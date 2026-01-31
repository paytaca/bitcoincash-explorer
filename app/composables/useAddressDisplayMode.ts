import type { AddressDisplayMode } from '~/utils/addressFormat'

/**
 * SSR-safe address display mode ("cash" vs "token").
 *
 * Using a cookie (instead of localStorage) keeps SSR and client hydration in sync,
 * and works reliably in production deployments behind caches/CDNs.
 */
export function useAddressDisplayMode() {
  const raw = useCookie<AddressDisplayMode | string>('bchexplorer_addressMode', {
    default: () => 'cash',
    sameSite: 'lax'
  })

  const mode = computed<AddressDisplayMode>({
    get() {
      return raw.value === 'token' ? 'token' : 'cash'
    },
    set(v) {
      raw.value = v === 'token' ? 'token' : 'cash'
    }
  })

  return mode
}

