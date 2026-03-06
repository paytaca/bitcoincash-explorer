import net from 'node:net'

export type FulcrumClient = {
  request<T>(method: string, params?: unknown[]): Promise<T>
  close(): void
}

type JsonRpcResponse<T> = { id: number; result?: T; error?: { code: number; message: string } }
type PendingRequest = { resolve: (v: any) => void; reject: (e: any) => void; timer: NodeJS.Timeout }

// Connection pooling with simpler, more reliable implementation
// Increased to 100 to handle slow address lookups (many concurrent long-running requests)
const MAX_POOL_SIZE = 100
const IDLE_TIMEOUT_MS = 30000

interface PooledConnection {
  socket: net.Socket
  nextId: number
  pending: Map<number, PendingRequest>
  buffer: string
  lastUsed: number
  destroyed: boolean
  inUse: boolean
}

let connectionPool: PooledConnection[] = []
let poolConfig: { host: string; port: number; timeoutMs: number } | null = null

function getConfig(): { host: string; port: number; timeoutMs: number } {
  const config = useRuntimeConfig()
  const host = String((config as any).fulcrumHost || process.env.FULCRUM_HOST || '127.0.0.1')
  const port = Number((config as any).fulcrumPort || process.env.FULCRUM_PORT || 60002)
  const timeoutMs = Number((config as any).fulcrumTimeoutMs || process.env.FULCRUM_TIMEOUT_MS || 30_000)

  if (!Number.isInteger(port) || port <= 0) {
    throw createError({ statusCode: 500, statusMessage: 'FULCRUM_PORT invalid (set FULCRUM_PORT or NUXT_FULCRUM_PORT)' })
  }

  return { host, port, timeoutMs }
}

function isConnectionHealthy(conn: PooledConnection): boolean {
  return !conn.destroyed && !conn.socket.destroyed && conn.socket.readyState === 'open'
}

function destroyConnection(conn: PooledConnection): void {
  if (conn.destroyed) return
  conn.destroyed = true
  conn.inUse = false
  
  // Remove from pool immediately to prevent leaks
  const idx = connectionPool.indexOf(conn)
  if (idx !== -1) {
    connectionPool.splice(idx, 1)
  }
  
  try {
    conn.socket.end()
    conn.socket.destroy()
  } catch {
    // ignore
  }
  // Reject all pending requests
  for (const [id, p] of conn.pending) {
    conn.pending.delete(id)
    clearTimeout(p.timer)
    p.reject(createError({ statusCode: 502, statusMessage: 'Fulcrum connection closed' }))
  }
}

function createConnection(host: string, port: number, timeoutMs: number): PooledConnection {
  const conn: PooledConnection = {
    socket: net.createConnection({ host, port }),
    nextId: 1,
    pending: new Map(),
    buffer: '',
    lastUsed: Date.now(),
    destroyed: false,
    inUse: false
  }

  conn.socket.setNoDelay(true)
  conn.socket.setKeepAlive(true, 30000)

  conn.socket.on('data', (chunk) => {
    conn.buffer += chunk.toString('utf8')
    while (true) {
      const idx = conn.buffer.indexOf('\n')
      if (idx === -1) break
      const line = conn.buffer.slice(0, idx).trim()
      conn.buffer = conn.buffer.slice(idx + 1)
      if (!line) continue

      let msg: JsonRpcResponse<any>
      try {
        msg = JSON.parse(line)
      } catch {
        continue
      }

      const p = conn.pending.get(msg.id)
      if (!p) continue
      conn.pending.delete(msg.id)
      clearTimeout(p.timer)

      if (msg.error) {
        p.reject(createError({ statusCode: 502, statusMessage: `Fulcrum RPC error (${msg.error.code}): ${msg.error.message}` }))
      } else {
        p.resolve(msg.result)
      }
    }
  })

  conn.socket.on('error', (err) => {
    console.error(`Fulcrum connection error: ${err.message}`)
    destroyConnection(conn)
  })

  conn.socket.on('close', () => {
    destroyConnection(conn)
  })

  return conn
}

async function getConnection(): Promise<PooledConnection> {
  const config = poolConfig || getConfig()
  poolConfig = config

  // Clean up old/dead connections (regardless of inUse status)
  connectionPool = connectionPool.filter(conn => {
    const idle = Date.now() - conn.lastUsed
    if (conn.destroyed || !isConnectionHealthy(conn) || idle > IDLE_TIMEOUT_MS) {
      destroyConnection(conn)
      return false
    }
    return true
  })

  // Find an available healthy connection (skip destroyed ones)
  for (const conn of connectionPool) {
    if (!conn.destroyed && !conn.inUse && isConnectionHealthy(conn)) {
      conn.inUse = true
      conn.lastUsed = Date.now()
      console.log(`[Fulcrum Pool] Connection acquired (existing). Pool: ${connectionPool.length} total, ${connectionPool.filter(c => c.inUse).length} in use`)
      return conn
    }
  }

  // Create new connection if under limit
  if (connectionPool.length < MAX_POOL_SIZE) {
    const conn = createConnection(config.host, config.port, config.timeoutMs)
    conn.inUse = true
    connectionPool.push(conn)
    console.log(`[Fulcrum Pool] New connection created. Pool: ${connectionPool.length} total, ${connectionPool.filter(c => c.inUse).length} in use`)
    
    // Wait for connection to be ready
    if (conn.socket.connecting) {
      await new Promise<void>((resolve, reject) => {
        const onConnect = () => {
          cleanup()
          resolve()
        }
        const onError = (e: any) => {
          cleanup()
          reject(e)
        }
        const cleanup = () => {
          conn.socket.off('connect', onConnect)
          conn.socket.off('error', onError)
        }
        conn.socket.on('connect', onConnect)
        conn.socket.on('error', onError)
      })
    }
    
    return conn
  }

  // Pool is full, wait for a connection to become available
  console.log(`[Fulcrum Pool] Waiting for connection... Pool: ${connectionPool.length} total, ${connectionPool.filter(c => c.inUse).length} in use`)
  return new Promise((resolve, reject) => {
    const checkInterval = setInterval(() => {
      for (const conn of connectionPool) {
        if (!conn.destroyed && !conn.inUse && isConnectionHealthy(conn)) {
          clearInterval(checkInterval)
          conn.inUse = true
          conn.lastUsed = Date.now()
          console.log(`[Fulcrum Pool] Connection acquired (after wait). Pool: ${connectionPool.length} total, ${connectionPool.filter(c => c.inUse).length} in use`)
          resolve(conn)
          return
        }
      }
    }, 50)

    // Timeout after 30 seconds waiting for pool (increased from 5s to handle slow address lookups)
    setTimeout(() => {
      clearInterval(checkInterval)
      console.error(`[Fulcrum Pool] Timeout waiting for connection. Pool: ${connectionPool.length} total, ${connectionPool.filter(c => c.inUse).length} in use`)
      reject(createError({ statusCode: 503, statusMessage: 'Fulcrum connection pool exhausted' }))
    }, 30000)
  })
}

function releaseConnection(conn: PooledConnection): void {
  conn.inUse = false
  conn.lastUsed = Date.now()
  console.log(`[Fulcrum Pool] Connection released. Pool: ${connectionPool.length} total, ${connectionPool.filter(c => c.inUse).length} in use`)
}

export function createFulcrumClient(): FulcrumClient {
  let conn: PooledConnection | null = null

  async function request<T>(method: string, params: unknown[] = []): Promise<T> {
    const config = poolConfig || getConfig()
    
    conn = await getConnection()

    if (!isConnectionHealthy(conn)) {
      releaseConnection(conn)
      throw createError({ statusCode: 502, statusMessage: 'Fulcrum connection is not healthy' })
    }

    const id = conn.nextId++
    const payload = JSON.stringify({ jsonrpc: '2.0', id, method, params }) + '\n'

    return new Promise<T>((resolve, reject) => {
      const timer = setTimeout(() => {
        conn!.pending.delete(id)
        destroyConnection(conn!)
        reject(createError({ statusCode: 504, statusMessage: `Fulcrum RPC timeout calling ${method} after ${config.timeoutMs}ms` }))
      }, config.timeoutMs)

      conn!.pending.set(id, { resolve, reject, timer })
      
      conn!.socket.write(payload, 'utf8', (err) => {
        if (err) {
          conn!.pending.delete(id)
          clearTimeout(timer)
          destroyConnection(conn!)
          reject(createError({ statusCode: 502, statusMessage: `Fulcrum write error: ${String(err.message || err)}` }))
        }
      })
    })
  }

  function close(): void {
    if (conn) {
      releaseConnection(conn)
      conn = null
    }
  }

  return { request, close }
}

// Periodic cleanup every 10 seconds (was 30s) - more aggressive cleanup
setInterval(() => {
  const now = Date.now()
  const beforeCount = connectionPool.length
  connectionPool = connectionPool.filter(conn => {
    const idle = now - conn.lastUsed
    // Remove if: not healthy OR destroyed OR idle too long (regardless of inUse status)
    if (conn.destroyed || !isConnectionHealthy(conn) || idle > IDLE_TIMEOUT_MS) {
      destroyConnection(conn)
      return false
    }
    return true
  })
  const removed = beforeCount - connectionPool.length
  if (removed > 0 || connectionPool.length > 0) {
    console.log(`[Fulcrum Pool] Cleanup: ${removed} removed, ${connectionPool.length} remaining (${connectionPool.filter(c => c.inUse).length} in use)`)
  }
}, 10000)
