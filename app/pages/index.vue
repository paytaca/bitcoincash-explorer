<template>
  <main class="container">
    <h1 class="title">Bitcoin Cash Explorer</h1>
    <p class="muted">Chain: {{ chain }}</p>

    <section class="card">
      <div class="cardHeader">
        <h2 class="h2">Latest transactions</h2>
        <div class="muted" v-if="recentTxs?.updatedAt">
          Updated:
          <span class="mono">{{ formatAbsoluteTime(recentTxs.updatedAt) }}</span>
        </div>
      </div>

      <div v-if="recentPending">Loading…</div>
      <div v-else-if="recentError" class="error">Error: {{ recentError.message }}</div>

      <div v-else class="txTableWrap">
        <table class="txTable">
          <thead>
            <tr>
              <th>Status</th>
              <th>Txid</th>
              <th>Time</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="t in (recentTxs?.items || [])" :key="t.txid">
              <td>
                <span class="badge" :class="t.status === 'mempool' ? 'isMempool' : 'isConfirmed'">
                  <template v-if="t.status === 'mempool'">Mempool</template>
                  <template v-else>Confirmed</template>
                </span>
                <span v-if="t.status === 'confirmed' && t.confirmations" class="muted small">
                  {{ t.confirmations }} conf
                </span>
              </td>
              <td class="mono">
                <NuxtLink class="txLink" :to="`/tx/${t.txid}`">{{ t.txid }}</NuxtLink>
              </td>
              <td class="timeCell">
                <span class="timeRel">{{ formatRelativeTime(t.time) }}</span>
                <span class="timeAbs">{{ formatAbsoluteTime(t.time) }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="card">
      <div class="cardHeader">
        <h2 class="h2">Latest blocks</h2>
        <div class="muted" v-if="tip">Tip: <span class="mono">#{{ tip }}</span></div>
      </div>

      <div v-if="tipPending">Loading…</div>
      <div v-else-if="tipError" class="error">Error: {{ tipError.message }}</div>

      <ul v-else class="list">
        <li class="row headerRow" aria-hidden="true">
          <div class="link headerLink">
            <span>Height</span>
            <span>Hash</span>
            <span>Time</span>
            <span>Miner</span>
            <span>Txs</span>
          </div>
        </li>
        <li v-for="b in blocks" :key="b.hash" class="row">
          <NuxtLink class="link" :to="`/block/${b.hash}`">
            <span class="mono">#{{ b.height }}</span>
            <span class="mono hash">{{ b.hash }}</span>
            <span class="time">
              <span class="timeRel">{{ formatRelativeTime(b.time) }}</span>
              <span class="timeAbs">{{ formatAbsoluteTime(b.time) }}</span>
            </span>
            <span class="miner">{{ b.miner || 'Unknown' }}</span>
            <span class="muted">{{ b.txCount }} tx</span>
          </NuxtLink>
        </li>
      </ul>
    </section>
  </main>
</template>

<script setup lang="ts">
type RecentTxItem = {
  txid: string
  status: 'mempool' | 'confirmed'
  time?: number
  fee?: number
  size?: number
  blockHeight?: number
  confirmations?: number
}

type RecentTxResponse = {
  updatedAt: number
  items: RecentTxItem[]
}

const chain = useRuntimeConfig().public.chain

const locale = (() => {
  if (import.meta.client) return navigator.language || 'en-US'
  const al = useRequestHeaders(['accept-language'])['accept-language']
  return al?.split(',')?.[0] || 'en-US'
})()

const {
  data: recentTxs,
  pending: recentPending,
  error: recentError
} = await useFetch<RecentTxResponse>('/api/bch/tx/recent')

const { data: tip, pending: tipPending, error: tipError } = await useFetch<number>('/api/bch/blockcount')

const { data: blocks } = await useAsyncData('latestBlocks', async () => {
  if (!tip.value) return []
  const heights = Array.from({ length: 15 }, (_, i) => tip.value! - i)
  const hashes = await Promise.all(heights.map((h) => $fetch<string>(`/api/bch/blockhash/${h}`)))
  const full = await Promise.all(hashes.map((hash) => $fetch<any>(`/api/bch/block/${hash}`)))
  return full.map((b) => ({
    hash: b.hash as string,
    height: b.height as number,
    time: typeof b.time === 'number' ? b.time : undefined,
    miner: extractMinerFromBlock(b),
    txCount: Array.isArray(b.tx) ? b.tx.length : 0
  }))
})

function extractMinerFromCoinbaseHex(coinbaseHex?: string): string | undefined {
  if (!coinbaseHex || typeof coinbaseHex !== 'string') return undefined
  if (!/^[0-9a-fA-F]+$/.test(coinbaseHex) || coinbaseHex.length % 2 !== 0) return undefined

  // Decode to bytes, then keep printable ASCII. Many pools use slash-delimited tags like "/ViaBTC/".
  let ascii = ''
  try {
    if (import.meta.client) {
      const bytes = new Uint8Array(coinbaseHex.match(/.{1,2}/g)!.map((h) => parseInt(h, 16)))
      ascii = Array.from(bytes, (b) => (b >= 0x20 && b <= 0x7e ? String.fromCharCode(b) : ' ')).join('')
    } else {
      // Node SSR path
      // eslint-disable-next-line n/no-deprecated-api
      const buf = Buffer.from(coinbaseHex, 'hex')
      ascii = buf.toString('latin1').replace(/[^\x20-\x7E]+/g, ' ')
    }
  } catch {
    return undefined
  }

  const cleaned = ascii.replace(/\s+/g, ' ').trim()
  if (!cleaned) return undefined

  // Best-effort: take first slash-delimited tag as "pool" name.
  const m = cleaned.match(/\/\s*([A-Za-z0-9][A-Za-z0-9 ._-]{0,40}?)\s*\//)
  if (m?.[1]) return m[1].trim()
  return undefined
}

function extractMinerFromBlock(b: any): string | undefined {
  const tx0 = Array.isArray(b?.tx) ? b.tx[0] : undefined
  const vin0 = Array.isArray(tx0?.vin) ? tx0.vin[0] : undefined
  // BCHN typically exposes coinbase script as vin[0].coinbase (hex).
  const coinbaseHex = typeof vin0?.coinbase === 'string' ? vin0.coinbase : undefined
  return extractMinerFromCoinbaseHex(coinbaseHex)
}

function formatAbsoluteTime(unixSeconds?: number) {
  if (!unixSeconds) return '—'
  return new Date(unixSeconds * 1000).toLocaleString(locale)
}

function formatRelativeTime(unixSeconds?: number) {
  if (!unixSeconds) return '—'
  const diffMs = unixSeconds * 1000 - Date.now()
  const abs = Math.abs(diffMs)

  const minute = 60_000
  const hour = 60 * minute
  const day = 24 * hour
  const week = 7 * day

  let value: number
  let unit: Intl.RelativeTimeFormatUnit
  if (abs < hour) {
    value = Math.round(diffMs / minute)
    unit = 'minute'
  } else if (abs < day) {
    value = Math.round(diffMs / hour)
    unit = 'hour'
  } else if (abs < week) {
    value = Math.round(diffMs / day)
    unit = 'day'
  } else {
    value = Math.round(diffMs / week)
    unit = 'week'
  }

  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' })
  return rtf.format(value, unit)
}

</script>

<style scoped>
.container {
  max-width: 960px;
  margin: 24px auto;
  padding: 0 16px;
}
.title {
  margin: 0 0 6px;
  font-size: 24px;
  letter-spacing: -0.02em;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  word-break: break-all;
}
.muted {
  color: rgba(107, 114, 128, 1);
}
.card {
  margin-top: 14px;
  border: 1px solid rgba(17, 24, 39, 0.08);
  border-radius: 16px;
  padding: 14px;
  background: rgba(255, 255, 255, 1);
}
.cardHeader {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}
.h2 {
  margin: 0;
  font-size: 14px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgba(55, 65, 81, 1);
}
.txTableWrap {
  overflow-x: auto;
  border-radius: 12px;
}
.txTable {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.txTable th,
.txTable td {
  padding: 10px 8px;
  border-bottom: 1px solid rgba(17, 24, 39, 0.06);
  vertical-align: top;
}
.txTable th {
  text-align: left;
  font-weight: 600;
  color: rgba(55, 65, 81, 1);
}
.txLink {
  color: inherit;
  text-decoration: none;
}
.txLink:hover {
  text-decoration: underline;
}
.right {
  text-align: right;
}
.badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  margin-right: 8px;
}
.badge.isMempool {
  background: rgba(217, 119, 6, 0.12);
  color: rgba(146, 64, 14, 1);
}
.badge.isConfirmed {
  background: rgba(16, 185, 129, 0.12);
  color: rgba(6, 95, 70, 1);
}
.small {
  font-size: 12px;
}
.timeCell {
  display: grid;
  gap: 2px;
}
.timeRel {
  font-weight: 600;
  color: rgba(55, 65, 81, 1);
}
.timeAbs {
  font-size: 12px;
  color: rgba(107, 114, 128, 1);
}
.list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  gap: 8px;
}
.row {
  border-radius: 14px;
  padding: 10px 10px;
  background: rgba(17, 24, 39, 0.03);
  border: 1px solid rgba(17, 24, 39, 0.04);
}
.headerRow {
  background: transparent;
  border: 0;
  padding: 0 10px;
}
.headerLink {
  padding: 0 0 6px;
  color: rgba(107, 114, 128, 1);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
.headerLink span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.link {
  display: grid;
  grid-template-columns: 110px 1fr 210px 140px 70px;
  gap: 12px;
  text-decoration: none;
  color: inherit;
  align-items: baseline;
}
.miner {
  font-size: 12px;
  font-weight: 600;
  color: rgba(55, 65, 81, 1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.time {
  display: grid;
  gap: 2px;
}
.timeRel {
  font-weight: 600;
  color: rgba(55, 65, 81, 1);
}
.timeAbs {
  font-size: 12px;
  color: rgba(107, 114, 128, 1);
}
.hash {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.error {
  color: #b42318;
}
</style>

