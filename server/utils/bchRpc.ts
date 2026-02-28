type JsonRpcSuccess<T> = { result: T; error: null; id: string }

// Helper for promisified delay (avoids import issues)
const delay = (ms: number): Promise<void> => new Promise(resolve => setTimeout(resolve, ms))
type JsonRpcFailure = { result: null; error: { code: number; message: string }; id: string }
type JsonRpcResponse<T> = JsonRpcSuccess<T> | JsonRpcFailure

// Concurrency control to prevent overwhelming the BCH node
const MAX_CONCURRENT_RPC = 5
let activeRequests = 0
const requestQueue: Array<() => void> = []

async function acquireSlot(): Promise<void> {
  if (activeRequests < MAX_CONCURRENT_RPC) {
    activeRequests++
    return
  }
  return new Promise((resolve) => requestQueue.push(resolve))
}

function releaseSlot(): void {
  activeRequests--
  const next = requestQueue.shift()
  if (next) {
    activeRequests++
    next()
  }
}

// Circuit breaker state
let circuitState: 'closed' | 'open' | 'half-open' = 'closed'
let lastFailureTime = 0
const CIRCUIT_BREAKER_TIMEOUT_MS = 30_000
const CIRCUIT_BREAKER_THRESHOLD = 5
let consecutiveFailures = 0

function isWorkQueueError(e: unknown): boolean {
  const msg = typeof (e as any)?.message === 'string' ? (e as any).message : String(e)
  return msg.toLowerCase().includes('work queue depth exceeded')
}

async function withCircuitBreaker<T>(fn: () => Promise<T>): Promise<T> {
  if (circuitState === 'open') {
    const now = Date.now()
    if (now - lastFailureTime < CIRCUIT_BREAKER_TIMEOUT_MS) {
      throw createError({ 
        statusCode: 503, 
        statusMessage: 'BCH RPC temporarily unavailable (circuit breaker open)' 
      })
    }
    circuitState = 'half-open'
  }

  try {
    const result = await fn()
    // Success - reset circuit breaker
    if (circuitState === 'half-open') {
      circuitState = 'closed'
      consecutiveFailures = 0
    }
    return result
  } catch (e) {
    if (isWorkQueueError(e)) {
      consecutiveFailures++
      lastFailureTime = Date.now()
      if (consecutiveFailures >= CIRCUIT_BREAKER_THRESHOLD) {
        circuitState = 'open'
      }
    }
    throw e
  }
}

export async function bchRpc<T>(
  method: string, 
  params: unknown[] = [], 
  timeoutMs: number = 30_000,
  retryOptions?: { maxRetries?: number; baseDelayMs?: number }
): Promise<T> {
  const { maxRetries = 3, baseDelayMs = 500 } = retryOptions || {}
  
  const config = useRuntimeConfig()
  const bchRpcUrl = config.bchRpcUrl || process.env.BCH_RPC_URL
  const bchRpcUser = config.bchRpcUser || process.env.BCH_RPC_USER
  const bchRpcPass = config.bchRpcPass || process.env.BCH_RPC_PASS

  if (!bchRpcUrl) {
    throw createError({ statusCode: 500, statusMessage: 'BCH_RPC_URL not set (set BCH_RPC_URL or NUXT_BCH_RPC_URL)' })
  }

  const auth =
    bchRpcUser && bchRpcPass
      ? 'Basic ' + Buffer.from(`${bchRpcUser}:${bchRpcPass}`).toString('base64')
      : undefined

  let lastError: Error | undefined

  for (let attempt = 0; attempt <= maxRetries; attempt++) {
    await acquireSlot()
    
    try {
      const result = await withCircuitBreaker(async () => {
        const controller = new AbortController()
        const timeoutId = setTimeout(() => controller.abort(), timeoutMs)

        try {
          const res = await $fetch<JsonRpcResponse<T>>(bchRpcUrl, {
            method: 'POST',
            headers: {
              ...(auth ? { Authorization: auth } : {}),
              'Content-Type': 'application/json',
              'Connection': 'keep-alive'
            },
            body: {
              jsonrpc: '1.0',
              id: 'nuxt',
              method,
              params
            },
            signal: controller.signal
          })

          if (res.error) {
            throw createError({
              statusCode: 502,
              statusMessage: `BCH RPC error (${res.error.code}): ${res.error.message}`
            })
          }

          return res.result
        } catch (e: any) {
          if (e?.name === 'AbortError') {
            throw createError({ statusCode: 504, statusMessage: 'BCH RPC request timed out' })
          }
          throw e
        } finally {
          clearTimeout(timeoutId)
        }
      })

      return result
    } catch (e: any) {
      lastError = e
      
      // Don't retry on client errors (4xx)
      if (e?.statusCode >= 400 && e?.statusCode < 500 && !isWorkQueueError(e)) {
        throw e
      }
      
      // Circuit breaker is open, don't retry
      if (e?.statusCode === 503 && e?.statusMessage?.includes('circuit breaker')) {
        throw e
      }
      
      // Last attempt failed, throw the error
      if (attempt === maxRetries) {
        throw e
      }
      
      // Exponential backoff for work queue errors
      const backoffDelay = isWorkQueueError(e)
        ? baseDelayMs * Math.pow(2, attempt) * 2  // Extra delay for work queue
        : baseDelayMs * Math.pow(2, attempt)

      await delay(Math.min(backoffDelay, 10000)) // Cap at 10 seconds
    } finally {
      releaseSlot()
    }
  }

  throw lastError || createError({ statusCode: 500, statusMessage: 'BCH RPC failed after retries' })
}
