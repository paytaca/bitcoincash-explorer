import { getRedisClient } from './redis'

/**
 * Request timing middleware for debugging timeouts
 * Logs requests that take longer than threshold
 */
export default defineEventHandler(async (event) => {
  // Only monitor API routes
  if (!event.path?.startsWith('/api/')) return

  const start = Date.now()
  const path = event.path
  const method = event.method

  // Log when request starts (for debugging hanging requests)
  console.log(`[${new Date().toISOString()}] START ${method} ${path}`)

  // Track if request completed
  let completed = false

  // Check for timeout every 10 seconds
  const checkInterval = setInterval(() => {
    if (completed) {
      clearInterval(checkInterval)
      return
    }

    const elapsed = Date.now() - start
    if (elapsed > 30000) {
      console.error(`[${new Date().toISOString()}] WARNING: ${method} ${path} is taking ${elapsed}ms and still running`)
    }
  }, 10000)

  // Clean up when response is sent
  event.node.res.on('finish', () => {
    completed = true
    clearInterval(checkInterval)
    const duration = Date.now() - start
    const status = event.node.res.statusCode
    
    // Log slow requests (> 5s) or errors
    if (duration > 5000 || status >= 400) {
      console.log(`[${new Date().toISOString()}] END ${method} ${path} - ${status} - ${duration}ms`)
    }
  })

  event.node.res.on('close', () => {
    completed = true
    clearInterval(checkInterval)
    const duration = Date.now() - start
    
    if (!event.node.res.writableEnded) {
      console.error(`[${new Date().toISOString()}] ABORTED ${method} ${path} - client disconnected after ${duration}ms`)
    }
  })
})
