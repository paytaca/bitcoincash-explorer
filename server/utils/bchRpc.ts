type JsonRpcSuccess<T> = { result: T; error: null; id: string }
type JsonRpcFailure = { result: null; error: { code: number; message: string }; id: string }
type JsonRpcResponse<T> = JsonRpcSuccess<T> | JsonRpcFailure

export async function bchRpc<T>(method: string, params: unknown[] = []): Promise<T> {
  const config = useRuntimeConfig()
  // Prefer Nuxt runtime config (NUXT_*), but allow direct env vars (BCH_RPC_*)
  // for local dev and simpler Docker setups.
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

  const res = await $fetch<JsonRpcResponse<T>>(bchRpcUrl, {
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

