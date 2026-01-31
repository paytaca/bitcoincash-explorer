/**
 * SSR/client-stable "now" timestamp (ms).
 *
 * Hydration mismatches often happen when SSR renders relative times based on
 * server-side `Date.now()` and the client hydrates with a different `Date.now()`.
 * Storing the server timestamp in Nuxt payload ensures the client uses the same
 * reference during hydration.
 */
export function useStableNow() {
  const now = useState<number>('bchexplorer.now', () => Date.now())
  if (import.meta.client && (typeof now.value !== 'number' || !Number.isFinite(now.value))) {
    now.value = Date.now()
  }
  return now
}

