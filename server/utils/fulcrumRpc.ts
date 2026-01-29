import net from 'node:net'

type JsonRpcResponse<T> = { id: number; result?: T; error?: { code: number; message: string } }

export type FulcrumClient = {
  request<T>(method: string, params?: unknown[]): Promise<T>
  close(): void
}

export function createFulcrumClient(): FulcrumClient {
  const config = useRuntimeConfig()
  // Prefer runtime config (NUXT_*), but allow direct env vars (FULCRUM_*) for simple setups.
  const host = String((config as any).fulcrumHost || process.env.FULCRUM_HOST || '127.0.0.1')
  const port = Number((config as any).fulcrumPort || process.env.FULCRUM_PORT || 60002)
  const timeoutMs = Number((config as any).fulcrumTimeoutMs || process.env.FULCRUM_TIMEOUT_MS || 10_000)

  if (!Number.isInteger(port) || port <= 0) {
    throw createError({ statusCode: 500, statusMessage: 'FULCRUM_PORT invalid (set FULCRUM_PORT or NUXT_FULCRUM_PORT)' })
  }

  let nextId = 1
  const pending = new Map<number, { resolve: (v: any) => void; reject: (e: any) => void; timer: NodeJS.Timeout }>()
  let buffer = ''

  const socket = net.createConnection({ host, port })
  socket.setNoDelay(true)

  socket.on('data', (chunk) => {
    buffer += chunk.toString('utf8')
    while (true) {
      const idx = buffer.indexOf('\n')
      if (idx === -1) break
      const line = buffer.slice(0, idx).trim()
      buffer = buffer.slice(idx + 1)
      if (!line) continue

      let msg: JsonRpcResponse<any>
      try {
        msg = JSON.parse(line)
      } catch {
        continue
      }

      const p = pending.get(msg.id)
      if (!p) continue
      pending.delete(msg.id)
      clearTimeout(p.timer)

      if (msg.error) {
        p.reject(createError({ statusCode: 502, statusMessage: `Fulcrum RPC error (${msg.error.code}): ${msg.error.message}` }))
      } else {
        p.resolve(msg.result)
      }
    }
  })

  socket.on('error', (err) => {
    for (const [id, p] of pending) {
      pending.delete(id)
      clearTimeout(p.timer)
      p.reject(createError({ statusCode: 502, statusMessage: `Fulcrum connection error: ${String((err as any)?.message || err)}` }))
    }
  })

  function close() {
    try {
      socket.end()
      socket.destroy()
    } catch {
      // ignore
    }
  }

  async function request<T>(method: string, params: unknown[] = []): Promise<T> {
    // Wait for connect (or error) before sending.
    if (socket.connecting) {
      try {
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
            socket.off('connect', onConnect)
            socket.off('error', onError)
          }
          socket.on('connect', onConnect)
          socket.on('error', onError)
        })
      } catch (e: any) {
        const code = String(e?.code || '')
        const msg = String(e?.message || e || '')
        throw createError({
          statusCode: 502,
          statusMessage: `Cannot connect to Fulcrum at ${host}:${port}${code ? ` (${code})` : ''}: ${msg}`
        })
      }
    }

    if (socket.destroyed) {
      throw createError({ statusCode: 502, statusMessage: `Fulcrum connection is closed (${host}:${port})` })
    }

    const id = nextId++
    const payload = JSON.stringify({ jsonrpc: '2.0', id, method, params }) + '\n'

    return await new Promise<T>((resolve, reject) => {
      const timer = setTimeout(() => {
        pending.delete(id)
        reject(createError({ statusCode: 504, statusMessage: `Fulcrum RPC timeout calling ${method}` }))
      }, timeoutMs)

      pending.set(id, { resolve, reject, timer })
      socket.write(payload, 'utf8', (err) => {
        if (!err) return
        pending.delete(id)
        clearTimeout(timer)
        reject(createError({ statusCode: 502, statusMessage: `Fulcrum write error: ${String(err.message || err)}` }))
      })
    })
  }

  return { request, close }
}

