import type { AddressDisplayMode } from '~/utils/addressFormat'

/**
 * Address display mode persistence ("cash" vs "token").
 * Uses cookies for persistence across sessions.
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
