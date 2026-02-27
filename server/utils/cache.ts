import { useStorage } from '#imports'

type CacheEntry<T> = {
  value: T
  expires: number
}

export async function withFileCache<T>(
  key: string,
  ttlMs: number,
  fn: () => Promise<T>
): Promise<T> {
  const storage = useStorage('cachedData')
  const cacheKey = `cache:${key}`
  
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
