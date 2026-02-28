import { config } from './config.js'

interface JsonRpcResponse<T> {
  result: T
  error: null | { code: number; message: string }
  id: string
}

export class RpcClient {
  private auth: string | undefined

  constructor() {
    if (config.rpcUser && config.rpcPass) {
      this.auth = 'Basic ' + Buffer.from(`${config.rpcUser}:${config.rpcPass}`).toString('base64')
    }
  }

  async call<T>(method: string, params: unknown[] = [], timeoutMs: number = 30000): Promise<T> {
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), timeoutMs)

    try {
      const response = await fetch(config.rpcUrl, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(this.auth ? { 'Authorization': this.auth } : {})
        },
        body: JSON.stringify({
          jsonrpc: '1.0',
          id: 'zmq-listener',
          method,
          params
        }),
        signal: controller.signal
      })

      if (!response.ok) {
        throw new Error(`HTTP error: ${response.status}`)
      }

      const data: unknown = await response.json()
      const jsonData = data as JsonRpcResponse<T>

      if (jsonData.error) {
        throw new Error(`RPC error (${jsonData.error.code}): ${jsonData.error.message}`)
      }

      return jsonData.result
    } finally {
      clearTimeout(timeoutId)
    }
  }

  async getBlockCount(): Promise<number> {
    return this.call<number>('getblockcount')
  }

  async getBlockHash(height: number): Promise<string> {
    return this.call<string>('getblockhash', [height])
  }

  async getBlock(hash: string, verbosity: number = 1): Promise<any> {
    return this.call<any>('getblock', [hash, verbosity])
  }

  async getRawTransaction(txid: string, verbose: boolean = true): Promise<any> {
    return this.call<any>('getrawtransaction', [txid, verbose ? 2 : 0])
  }

  async getRawMempool(): Promise<Record<string, any>> {
    return this.call<Record<string, any>>('getrawmempool', [true])
  }
}

export const rpc = new RpcClient()