import { bchRpc } from '../utils/bchRpc'
import { createFulcrumClient } from '../utils/fulcrumRpc'

type NodeStatus = {
  ok: boolean
  error?: string
  latencyMs?: number
  chain?: string
  blocks?: number
  headers?: number
  bestblockhash?: string
  difficulty?: number
  verificationprogress?: number
  initialblockdownload?: boolean
  mediantime?: number
  warnings?: string
  version?: number
  subversion?: string
  connections?: number
  bestBlockTime?: number
}

type FulcrumStatus = {
  ok: boolean
  error?: string
  latencyMs?: number
  host?: string
  port?: number
  version?: unknown
  banner?: unknown
  height?: number
  headerTime?: number
}

function toNum(v: unknown): number | undefined {
  return typeof v === 'number' && Number.isFinite(v) ? v : undefined
}

function headerTimeFromHex(headerHex?: string): number | undefined {
  if (typeof headerHex !== 'string') return undefined
  if (!/^[0-9a-fA-F]+$/.test(headerHex) || headerHex.length < 160) return undefined
  const header = Buffer.from(headerHex, 'hex')
  if (header.length < 80) return undefined
  // Timestamp is 4 bytes LE at offset 68
  return header.readUInt32LE(68)
}

export default defineEventHandler(async () => {
  const generatedAt = Math.floor(Date.now() / 1000)

  const config = useRuntimeConfig()
  const fulcrumHost = String((config as any).fulcrumHost || process.env.FULCRUM_HOST || '127.0.0.1')
  const fulcrumPort = Number((config as any).fulcrumPort || process.env.FULCRUM_PORT || 60001)

  const node: NodeStatus = { ok: false }
  const fulcrum: FulcrumStatus = { ok: false, host: fulcrumHost, port: fulcrumPort }

  // Node status
  try {
    const t0 = Date.now()
    const chainInfo = await bchRpc<any>('getblockchaininfo')
    const netInfo = await bchRpc<any>('getnetworkinfo')
    node.latencyMs = Date.now() - t0
    node.ok = true
    node.chain = typeof chainInfo?.chain === 'string' ? chainInfo.chain : undefined
    node.blocks = toNum(chainInfo?.blocks)
    node.headers = toNum(chainInfo?.headers)
    node.bestblockhash = typeof chainInfo?.bestblockhash === 'string' ? chainInfo.bestblockhash : undefined
    node.difficulty = toNum(chainInfo?.difficulty)
    node.verificationprogress = toNum(chainInfo?.verificationprogress)
    node.initialblockdownload = typeof chainInfo?.initialblockdownload === 'boolean' ? chainInfo.initialblockdownload : undefined
    node.mediantime = toNum(chainInfo?.mediantime)
    node.warnings = typeof chainInfo?.warnings === 'string' ? chainInfo.warnings : undefined
    node.version = toNum(netInfo?.version)
    node.subversion = typeof netInfo?.subversion === 'string' ? netInfo.subversion : undefined
    node.connections = toNum(netInfo?.connections)

    if (node.bestblockhash) {
      try {
        const h = await bchRpc<any>('getblockheader', [node.bestblockhash])
        node.bestBlockTime = toNum(h?.time)
      } catch {
        // optional
      }
    }
  } catch (e: any) {
    node.ok = false
    node.error = String(e?.statusMessage || e?.message || e)
  }

  // Fulcrum status
  const f = createFulcrumClient()
  try {
    const t0 = Date.now()
    // Prefer a tip height via headers.subscribe.
    const tip = await f.request<any>('blockchain.headers.subscribe', [])
    fulcrum.height = toNum(tip?.height)

    try {
      fulcrum.version = await f.request<any>('server.version', ['bchexplorer', '1.4'])
    } catch {
      // optional
    }
    try {
      fulcrum.banner = await f.request<any>('server.banner', [])
    } catch {
      // optional
    }

    if (fulcrum.height && fulcrum.height > 0) {
      try {
        const headerHex = await f.request<string>('blockchain.block.header', [fulcrum.height])
        fulcrum.headerTime = headerTimeFromHex(headerHex)
      } catch {
        // optional
      }
    }

    fulcrum.latencyMs = Date.now() - t0
    fulcrum.ok = true
  } catch (e: any) {
    fulcrum.ok = false
    fulcrum.error = String(e?.statusMessage || e?.message || e)
  } finally {
    f.close()
  }

  const heightDiff =
    typeof node.blocks === 'number' && typeof fulcrum.height === 'number' ? node.blocks - fulcrum.height : undefined
  const timeDiffSeconds =
    typeof node.bestBlockTime === 'number' && typeof fulcrum.headerTime === 'number'
      ? node.bestBlockTime - fulcrum.headerTime
      : undefined

  const inSync =
    node.ok &&
    fulcrum.ok &&
    typeof heightDiff === 'number' &&
    heightDiff >= 0 &&
    heightDiff <= 1 &&
    (typeof timeDiffSeconds !== 'number' || Math.abs(timeDiffSeconds) <= 2 * 60 * 60)

  return {
    generatedAt,
    node,
    fulcrum,
    comparison: {
      heightDiff,
      timeDiffSeconds,
      inSync
    }
  }
})

