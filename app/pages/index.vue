<template>
  <main class="container">
    <h1 class="title">Bitcoin Cash Explorer</h1>
    <div class="chainRow">
      <span class="muted">Chain:</span>
      <ChainSwitcher />
    </div>
    <section class="card">
      <div class="cardHeader">
        <h2 class="h2">Latest transactions</h2>
      </div>

      <div v-if="recentPending">Loading…</div>
      <div v-else-if="recentError" class="error">Error: {{ recentError.message }}</div>

      <div v-else class="txTableWrap">
        <table class="txTable">
          <thead>
            <tr>
              <th>Txid</th>
              <th>Amount</th>
              <th>Time</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="t in (recentTxs?.items || [])" :key="t.txid">
              <td class="txIdCell">
                <NuxtLink class="txLink mono" :to="`/tx/${t.txid}`">{{ truncateTxid(t.txid) }}</NuxtLink>
              </td>
              <td class="amountCell">
                <template v-if="t.amount !== undefined">
                  {{ formatBch(t.amount) }} <span class="unit">BCH</span>
                  <div v-if="t.hasTokens" class="tokensHint">with tokens</div>
                </template>
                <template v-else>—</template>
              </td>
              <td class="timeCell">
                <div class="timeCellInner">
                  <span class="timeRel">{{ formatRelativeTime(t.time) }}</span>
                  <span class="timeAbs">{{ formatAbsoluteTime(t.time) }}</span>
                </div>
              </td>
              <td class="statusCell">
                <div class="txStatus">
                  <span class="badge" :class="t.status === 'mempool' ? 'isMempool' : 'isConfirmed'">
                    <template v-if="t.status === 'mempool'">Mempool</template>
                    <template v-else>Confirmed</template>
                  </span>
                  <span v-if="t.status === 'confirmed' && t.confirmations" class="muted small">
                    {{ t.confirmations }} conf
                  </span>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="card">
      <div class="cardHeader">
        <h2 class="h2">Latest blocks</h2>
        <button v-if="blocksError" class="refreshBtn" @click="refreshBlocks()">Retry</button>
      </div>

      <div v-if="tipPending || blocksPending">Loading…</div>
      <div v-else-if="tipError" class="error">Error: {{ tipError.message }}</div>
      <div v-else-if="blocksError" class="error">Error loading blocks: {{ blocksError.message }}</div>

      <div v-else class="blockListWrap">
        <ul class="list">
          <li class="row headerRow" aria-hidden="true">
          <div class="headerLink">
            <span>Height</span>
            <span>Hash</span>
            <span>Miner</span>
            <span>Time</span>
          </div>
        </li>
        <li v-for="b in blocks" :key="b.hash" class="row">
          <NuxtLink class="link" :to="`/block/${b.hash}`">
            <span class="blockHeightCell">
              <span class="mono">#{{ b.height }}</span>
              <span class="muted blockTxCount">{{ formatBlockSize(b.size) }}, {{ b.txCount }} {{ b.txCount === 1 ? 'tx' : 'txs' }}</span>
            </span>
            <span class="mono hash">{{ truncateHash(b.hash) }}</span>
            <span class="miner">{{ b.miner || 'Unknown' }}</span>
            <span class="time">
              <span class="timeRel">{{ formatRelativeTime(b.time) }}</span>
              <span class="timeAbs">{{ formatAbsoluteTime(b.time) }}</span>
            </span>
          </NuxtLink>
        </li>
        </ul>
      </div>
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
  amount?: number
  hasTokens?: boolean
}

type RecentTxResponse = {
  updatedAt: number
  items: RecentTxItem[]
}

const fallbackTimestamp = computed(() =>
  new Date().toLocaleString('en-US', {
    timeZone: 'UTC',
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    second: '2-digit',
    timeZoneName: 'short'
  })
)

const clientLocaleDate = computed(() => {
  if (typeof navigator === 'undefined') return ''
  return new Date().toLocaleString(navigator.language, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    second: '2-digit',
    timeZoneName: 'short'
  })
})

const locale = usePageLocale()
const stableNow = useStableNow()

const {
  data: recentTxs,
  pending: recentPending,
  error: recentError
} = await useFetch<RecentTxResponse>('/api/bch/tx/recent')

const { data: tip, pending: tipPending, error: tipError } = await useFetch<number>('/api/bch/blockcount')

const { data: blocks, pending: blocksPending, error: blocksError, refresh: refreshBlocks } = await useAsyncData('latestBlocks', async () => {
  return await $fetch<Array<{
    hash: string
    height: number
    time?: number
    txCount: number
    miner?: string
  }>>('/api/bch/blocks/latest')
}, {
  watch: [tip],
})

function formatAbsoluteTime(unixSeconds?: number) {
  if (!unixSeconds) return '—'
  return new Date(unixSeconds * 1000).toLocaleString(locale.value, { timeZone: 'UTC', timeZoneName: 'short' })
}

function formatRelativeTime(unixSeconds?: number) {
  if (!unixSeconds) return '—'
  const diffMs = unixSeconds * 1000 - stableNow.value
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

  const rtf = new Intl.RelativeTimeFormat(locale.value, { numeric: 'auto' })
  return rtf.format(value, unit)
}

function formatBch(v: number | undefined) {
  const n = typeof v === 'number' && Number.isFinite(v) ? v : 0
  return new Intl.NumberFormat(locale.value, { maximumFractionDigits: 8 }).format(n)
}

function truncateHash(hash: string, nonZeroChars = 10, endChars = 10): string {
  const match = hash.match(/^(0+)(.*)/)
  const leadingZeros = match ? match[1].length : 0
  const firstNonZeroIndex = leadingZeros
  const totalLength = leadingZeros + nonZeroChars + 3 + endChars

  if (hash.length <= totalLength) return hash

  return hash.slice(0, leadingZeros + nonZeroChars) + '...' + hash.slice(-endChars)
}

function truncateTxid(txid: string, startChars = 10, endChars = 10): string {
  if (txid.length <= startChars + 3 + endChars) return txid
  return txid.slice(0, startChars) + '...' + txid.slice(-endChars)
}

function formatBlockSize(bytes: number): string {
  const mb = bytes / 1024 / 1024
  if (mb >= 0.01) {
    return mb.toFixed(2) + ' MB'
  }
  const kb = bytes / 1024
  return kb.toFixed(2) + ' KB'
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
.chainRow {
  display: flex;
  align-items: center;
  gap: 8px;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  word-break: break-all;
}
.muted {
  color: var(--color-text-muted);
}
.card {
  margin-top: 14px;
  border: 1px solid var(--color-border);
  border-radius: 16px;
  padding: 14px;
  background: var(--color-bg-card);
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
  color: var(--color-text-secondary);
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
  border-bottom: 1px solid var(--color-border-subtle);
  vertical-align: top;
}
.txTable th {
  text-align: left;
  font-weight: 600;
  color: var(--color-text-secondary);
}
.txIdCell {
  vertical-align: top;
}
.txStatus {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.txLink {
  color: inherit;
  text-decoration: none;
  word-break: break-all;
}
.txLink:hover {
  text-decoration: underline;
}
.amountCell {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-weight: 700;
  color: var(--color-amount);
}
.amountCell .unit {
  font-weight: 600;
  color: var(--color-text-secondary);
}
.amountCell .tokensHint {
  margin: 0;
  margin-top: 2px;
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-secondary);
  font-family: inherit;
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
  background: var(--color-badge-mempool-bg);
  color: var(--color-badge-mempool-fg);
}
.badge.isConfirmed {
  background: var(--color-badge-confirmed-bg);
  color: var(--color-badge-confirmed-fg);
}
.small {
  font-size: 12px;
}
.timeCell {
  vertical-align: top;
}
.timeCellInner {
  display: grid;
  gap: 2px;
}
.timeCell .timeRel {
  font-weight: 600;
  color: var(--color-text-secondary);
}
.timeCell .timeAbs {
  font-size: 12px;
  color: var(--color-text-muted);
}
.blockListWrap {
  border-radius: 12px;
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
  background: var(--color-surface);
  border: 1px solid var(--color-surface-border);
}
.headerRow {
  background: transparent;
  border: 0;
  padding: 0 10px;
}
.headerLink {
  display: grid;
  grid-template-columns: 160px 1fr 140px 200px;
  gap: 16px;
  padding: 0 0 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-secondary);
}
.headerLink span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}
.link {
  display: grid;
  grid-template-columns: 160px 1fr 140px 200px;
  gap: 16px;
  width: 100%;
  font-size: 13px;
  text-decoration: none;
  color: inherit;
  align-items: baseline;
  box-sizing: border-box;
}
.miner {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.time {
  display: grid;
  gap: 2px;
  white-space: nowrap;
}
.time .timeRel {
  font-weight: 600;
  color: var(--color-text-secondary);
}
.time .timeAbs {
  font-size: 12px;
  color: var(--color-text-muted);
}
.blockHeightCell {
  display: grid;
  gap: 2px;
}
.blockTxCount {
  font-size: 12px;
}
.hash {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
@media (max-width: 768px) {
  .blockListWrap .headerLink span:nth-child(2) {
    display: none;
  }
  .blockListWrap .link > .hash {
    display: none;
  }
  .blockListWrap .headerLink,
  .blockListWrap .link {
    grid-template-columns: 160px 1fr 200px;
  }
}
.error {
  color: var(--color-error);
}
.refreshBtn {
  padding: 6px 12px;
  border: 1px solid var(--color-border);
  border-radius: 6px;
  background: var(--color-bg-card);
  color: var(--color-text);
  font-size: 13px;
  cursor: pointer;
}
.refreshBtn:hover {
  background: var(--color-surface);
}
</style>

