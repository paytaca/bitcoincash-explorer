type JsonRpcSuccess<T> = { result: T; error: null; id: string }
type JsonRpcFailure = { result: null; error: { code: number; message: string }; id: string }
type JsonRpcResponse<T> = JsonRpcSuccess<T> | JsonRpcFailure

export async function bchRpc<T>(method: string, params: unknown[] = []): Promise<T> {
  const config = useRuntimeConfig()
  if (!config.bchRpcUrl) {
    throw createError({ statusCode: 500, statusMessage: 'BCH_RPC_URL not set' })
  }

  const auth =
    config.bchRpcUser && config.bchRpcPass
      ? 'Basic ' + Buffer.from(`${config.bchRpcUser}:${config.bchRpcPass}`).toString('base64')
      : undefined

  const res = await $fetch<JsonRpcResponse<T>>(config.bchRpcUrl, {
    method: 'POST',
    headers: {
      ...(auth ? { Authorization: auth } : {}),
      'Content-Type': 'application/json'
    },
    body: {
      jsonrpc: '1.0',
      id: 'nuxt',
      method,
      params
    }
  })

  if (res.error) {
    throw createError({
      statusCode: 502,
      statusMessage: `BCH RPC error (${res.error.code}): ${res.error.message}`
    })
  }

  return res.result
}

