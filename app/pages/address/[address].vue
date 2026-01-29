<template>
  <main class="container">
    <NuxtLink class="back" to="/">← Back</NuxtLink>

    <h1 class="title">Address</h1>
    <p class="mono">{{ address }}</p>

    <section class="card">
      <div class="cardHeader">
        <h2 class="h2">BCH Balance</h2>
      </div>

      <div v-if="pending">Loading…</div>
      <div v-else-if="error" class="error">Error: {{ error.message }}</div>
      <div v-else class="balanceOne">
        <div class="balanceValue amountCell">{{ formatBch(bchBalanceBch) }} <span class="unit">BCH</span></div>
      </div>
    </section>

    <section class="card">
      <div class="cardHeader">
        <h2 class="h2">Token balances</h2>
      </div>

      <div v-if="pending">Loading…</div>
      <div v-else-if="error" class="error">Error: {{ error.message }}</div>
      <div v-else-if="(data?.tokenBalances?.length || 0) === 0" class="muted">No tokens found for this address.</div>
      <div v-else class="txTableWrap">
        <table class="txTable">
          <thead>
            <tr>
              <th>Token</th>
              <th class="right thRight">Fungible</th>
              <th class="right thRight">NFTs</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="t in data!.tokenBalances" :key="t.category">
              <td class="tokenCell">
                <div class="tokenName">{{ tokenLabel(t.category) }}</div>
                <div class="mono muted small">{{ t.category }}</div>
              </td>
              <td class="amountCell right">
                <template v-if="t.fungibleAmount !== '0'">
                  {{ formatTokenAmount(t.fungibleAmount, tokenDecimals(t.category)) }}
                  <span v-if="tokenSymbol(t.category)" class="unit">{{ ' ' + tokenSymbol(t.category) }}</span>
                </template>
                <template v-else>—</template>
              </td>
              <td class="right">
                <template v-if="t.nftCount">
                  <span class="mono">{{ t.nftCount }}</span>
                </template>
                <template v-else>—</template>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="card">
      <div class="cardHeader">
        <h2 class="h2">Transactions</h2>
      </div>

      <div v-if="pending">Loading…</div>
      <div v-else-if="error" class="error">
        Error: {{ error.message }}
      </div>

      <div v-else-if="(data?.items?.length || 0) === 0" class="muted">
        No recent transactions found for this address.
      </div>

      <div v-else class="txTableWrap">
        <table class="txTable">
          <thead>
            <tr>
              <th>Txid</th>
              <th>Direction</th>
              <th class="right">Amount</th>
              <th>Time</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="t in data!.items" :key="t.txid">
              <td class="txIdCell">
                <NuxtLink class="txLink mono" :to="`/tx/${t.txid}`">{{ t.txid }}</NuxtLink>
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

              <td>
                <span class="dirBadge" :class="t.direction === 'sent' ? 'isSent' : 'isReceived'">
                  {{ t.direction === 'sent' ? 'SENT' : 'RECEIVED' }}
                </span>
              </td>

              <td class="amountCell right">
                {{ formatSignedBch(t.net) }} <span class="unit">BCH</span>
                <div v-if="t.hasTokens" class="tokensHint">with tokens</div>
              </td>

              <td class="timeCell">
                <div class="timeCellInner">
                  <span class="timeRel">{{ formatRelativeTime(t.time) }}</span>
                  <span class="timeAbs">
                    <template v-if="t.status === 'mempool'">Seen time: {{ formatAbsoluteTime(t.time) }}</template>
                    <template v-else>{{ formatAbsoluteTime(t.time) }}</template>
                  </span>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <div class="pager">
          <template v-if="hasNewer">
            <a class="pagerBtn" :href="newerHref">Newer</a>
          </template>
          <template v-else>
            <span class="pagerBtn isDisabled" aria-disabled="true">Newer</span>
          </template>

          <template v-if="olderHref">
            <a class="pagerBtn" :href="olderHref">Older</a>
          </template>
          <template v-else>
            <span class="pagerBtn isDisabled" aria-disabled="true">Older</span>
          </template>
        </div>
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
type AddressTxItem = {
  txid: string
  status: 'mempool' | 'confirmed'
  time?: number
  blockHeight?: number
  confirmations?: number
  direction: 'sent' | 'received'
  net: number
  inValue: number
  outValue: number
  hasTokens?: boolean
}

type AddressTxResponse = {
  address: string
  scanned: { source: 'fulcrum'; scripthash: string; tipHeight?: number; cursor: number; window: number }
  nextCursor: number | null
  balance: { confirmed: number; unconfirmed: number }
  tokenBalances: { category: string; fungibleAmount: string; nftCount: number; nft: { none: number; mutable: number; minting: number }; utxoCount: number }[]
  tokenMeta: Record<string, { name?: string; symbol?: string; decimals?: number }>
  items: AddressTxItem[]
}

const route = useRoute()
const address = String(route.params.address || '')

const locale = (() => {
  if (import.meta.client) return navigator.language || 'en-US'
  const al = useRequestHeaders(['accept-language'])['accept-language']
  return al?.split(',')?.[0] || 'en-US'
})()

const { data, pending, error } = await useFetch<AddressTxResponse>(
  () => {
    const qs = new URLSearchParams()
    if (route.query.cursor != null) qs.set('cursor', String(route.query.cursor))
    if (route.query.window != null) qs.set('window', String(route.query.window))
    const suffix = qs.toString() ? `?${qs.toString()}` : ''
    return `/api/bch/address/${encodeURIComponent(address)}/txs${suffix}`
  },
  { watch: [() => route.params.address, () => route.query.cursor, () => route.query.window] }
)

const hasNewer = computed(() => route.query.cursor != null)

const bchBalanceBch = computed(() => ((data.value?.balance?.confirmed ?? 0) + (data.value?.balance?.unconfirmed ?? 0)) / 1e8)

function queryString(params: Record<string, string | undefined>) {
  const qs = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v != null && String(v).length) qs.set(k, String(v))
  }
  const s = qs.toString()
  return s ? `?${s}` : ''
}

const newerHref = computed(() => {
  const w = route.query.window != null ? String(route.query.window) : undefined
  return `${route.path}${queryString({ ...(w ? { window: w } : {}) })}`
})

const olderHref = computed(() => {
  const next = data.value?.nextCursor
  if (!next) return ''
  const w = route.query.window != null ? String(route.query.window) : undefined
  return `${route.path}${queryString({ cursor: String(next), ...(w ? { window: w } : {}) })}`
})

function formatTokenAmount(amountStr: string, decimals?: number) {
  const dec = Number.isFinite(decimals) ? Number(decimals) : 0
  let amt = 0n
  try {
    amt = BigInt(amountStr || '0')
  } catch {
    amt = 0n
  }

  const intFmt = new Intl.NumberFormat(locale, { maximumFractionDigits: 0 })
  if (dec <= 0) return intFmt.format(amt)

  const parts = new Intl.NumberFormat(locale).formatToParts(1.1)
  const decimalSep = parts.find((p) => p.type === 'decimal')?.value || '.'

  const base = 10n ** BigInt(dec)
  const whole = amt / base
  const frac = (amt % base).toString().padStart(dec, '0').replace(/0+$/, '')
  const wholeStr = intFmt.format(whole)
  return frac ? `${wholeStr}${decimalSep}${frac}` : wholeStr
}

function tokenLabel(category: string) {
  const m = data.value?.tokenMeta?.[category]
  return m?.name || m?.symbol || category
}

function tokenDecimals(category: string) {
  const m = data.value?.tokenMeta?.[category]
  return typeof m?.decimals === 'number' ? m.decimals : 0
}

function tokenSymbol(category: string) {
  const m = data.value?.tokenMeta?.[category]
  return m?.symbol || ''
}

async function goNewest() {
  await navigateTo({ path: route.path, query: {} })
}

async function goOlder() {
  const next = data.value?.nextCursor
  if (!next) return
  await navigateTo({
    path: route.path,
    query: { cursor: String(next), ...(route.query.window != null ? { window: String(route.query.window) } : {}) }
  })
}

function formatAbsoluteTime(unixSeconds?: number) {
  if (!unixSeconds) return '—'
  return new Date(unixSeconds * 1000).toLocaleString(locale, { timeZoneName: 'short' })
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

function formatBch(v: number | undefined) {
  const n = typeof v === 'number' && Number.isFinite(v) ? v : 0
  // In some runtime/locale combinations, very small decimals can end up rendered
  // as "0". For BCH, we want up to 8 decimals (satoshis) deterministically.
  if (n !== 0 && Math.abs(n) < 1) {
    return n.toFixed(8).replace(/\.?0+$/, '')
  }
  return new Intl.NumberFormat(locale, { maximumFractionDigits: 8 }).format(n)
}

function formatSignedBch(net: number) {
  const n = typeof net === 'number' && Number.isFinite(net) ? net : 0
  const abs = Math.abs(n)
  const sign = n < 0 ? '-' : '+'
  return `${sign}${formatBch(abs)}`
}
</script>

<style scoped>
.container {
  max-width: 960px;
  margin: 24px auto;
  padding: 0 16px;
  font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial;
  color: var(--color-text);
}
.back {
  text-decoration: none;
  color: var(--color-text);
  opacity: 0.75;
  font-weight: 500;
}
.back:hover {
  color: var(--color-link);
}
.title {
  margin: 12px 0 6px;
  font-size: 24px;
  letter-spacing: -0.02em;
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
.txTable th.thRight {
  text-align: right;
}
.txIdCell {
  display: grid;
  gap: 4px;
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
.badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
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
.balanceOne {
  padding: 10px;
  border-radius: 14px;
  background: var(--color-surface);
  border: 1px solid var(--color-surface-border);
}
.balanceValue {
  font-size: 16px;
}
.tokenCell {
  display: grid;
  gap: 2px;
}
.tokenName {
  font-weight: 700;
  color: var(--color-text);
}
.dirBadge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.04em;
}
.dirBadge.isSent {
  background: rgba(185, 28, 28, 0.12);
  color: rgba(185, 28, 28, 1);
}
html.dark .dirBadge.isSent {
  background: rgba(248, 113, 113, 0.18);
  color: rgba(248, 113, 113, 1);
}
.dirBadge.isReceived {
  background: rgba(16, 185, 129, 0.12);
  color: rgba(6, 95, 70, 1);
}
html.dark .dirBadge.isReceived {
  background: rgba(34, 197, 94, 0.18);
  color: rgba(134, 239, 172, 1);
}
.amountCell {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-weight: 700;
  color: var(--color-amount);
}
.unit {
  font-weight: 600;
  color: var(--color-text-secondary);
  font-family: inherit;
}
.tokensHint {
  margin-top: 2px;
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-secondary);
  font-family: inherit;
}
.right {
  text-align: right;
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
.pager {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 12px;
}
.pagerBtn {
  border: 1px solid var(--color-border);
  background: var(--color-bg-input);
  color: var(--color-text);
  padding: 8px 12px;
  border-radius: 12px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
}
.pagerBtn.isDisabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.pagerBtn:not(.isDisabled):hover {
  background: var(--color-surface);
  border-color: var(--color-border-subtle);
}
.error {
  color: var(--color-error);
}
</style>

