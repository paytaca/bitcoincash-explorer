// Cache utilities for server-side caching with size limits and eviction

const MAX_CACHE_ENTRIES = 10000
const MAX_CACHE_SIZE_MB = 500
const CACHE_CLEANUP_INTERVAL_MS = 60 * 60 * 1000  // 1 hour

let lastCleanup = 0

type CacheEntry<T> = {
  value: T
  expires: number
}

async function cleanupOldEntries(): Promise<void> {
  const now = Date.now()
  if (now - lastCleanup < CACHE_CLEANUP_INTERVAL_MS) return
  
  const storage = useStorage('cachedData')
  const keys = await storage.getKeys('cache:')
  
  // Get all entries with their metadata
  const entries: { key: string; expires: number }[] = []
  for (const key of keys) {
    try {
      const item = await storage.getItem<{ expires: number }>(key)
      if (item && typeof item.expires === 'number') {
        entries.push({ key, expires: item.expires })
      }
    } catch {
      // Skip problematic entries
    }
  }
  
  // Remove expired entries
  const expiredKeys = entries.filter(e => e.expires < now).map(e => e.key)
  for (const key of expiredKeys) {
    await storage.removeItem(key)
  }
  
  // If still over limit, remove oldest entries (LRU eviction)
  if (entries.length - expiredKeys.length > MAX_CACHE_ENTRIES) {
    const sorted = entries
      .filter(e => e.expires >= now)
      .sort((a, b) => a.expires - b.expires)
    const toRemove = sorted.slice(0, sorted.length - MAX_CACHE_ENTRIES)
    for (const e of toRemove) {
      await storage.removeItem(e.key)
    }
  }
  
  lastCleanup = now
}

export async function withFileCache<T>(
  key: string,
  ttlMs: number,
  fn: () => Promise<T>
): Promise<T> {
  const storage = useStorage('cachedData')
  const cacheKey = `cache:${key}`
  
  // Periodic cleanup
  await cleanupOldEntries()
  
  const cached = await storage.getItem<CacheEntry<T>>(cacheKey)
  
  if (cached && Date.now() < cached.expires) {
    return cached.value
  }
  
  const value = await fn()
  await storage.setItem(cacheKey, { value, expires: Date.now() + ttlMs })
  return value
}

export async function withConditionalFileCache<T>(
  key: string,
  ttlMs: number,
  fn: () => Promise<T & { _cacheable?: boolean }>,
  shouldCache: (result: T) => boolean
): Promise<T> {
  const storage = useStorage('cachedData')
  const cacheKey = `cache:${key}`
  
  // Periodic cleanup
  await cleanupOldEntries()
  
  const cached = await storage.getItem<CacheEntry<T>>(cacheKey)
  
  if (cached && Date.now() < cached.expires) {
    return cached.value
  }
  
  const value = await fn()
  
  if (shouldCache(value)) {
    await storage.setItem(cacheKey, { value, expires: Date.now() + ttlMs })
  }
  
  return value
}
