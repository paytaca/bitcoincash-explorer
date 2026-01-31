<template>
  <main class="container">
    <NuxtLink class="back" to="/">← Back</NuxtLink>

    <h1 class="title">Status</h1>
    <p class="muted">Last updated: <span class="mono">{{ formatAbsoluteTime(data?.generatedAt) }}</span></p>

    <section class="card">
      <div class="cardHeader">
        <h2 class="h2">Summary</h2>
        <span v-if="data" class="badge" :class="data.comparison.inSync ? 'isOk' : 'isWarn'">
          {{ data.comparison.inSync ? 'OK' : 'Check sync' }}
        </span>
      </div>

      <div v-if="pending">Loading…</div>
      <div v-else-if="error" class="error">Error: {{ error.message }}</div>
      <div v-else class="grid">
        <div class="kv">
          <div class="k">Height diff</div>
          <div class="v mono">{{ formatMaybeNumber(data?.comparison.heightDiff) }}</div>
        </div>
        <div class="kv">
          <div class="k">Time diff</div>
          <div class="v mono">{{ formatSeconds(data?.comparison.timeDiffSeconds) }}</div>
        </div>
      </div>
      <p class="hint muted">Height diff is \(nodeHeight - fulcrumHeight\). Large diffs usually mean Fulcrum is still syncing.</p>
    </section>

    <section class="card">
      <div class="cardHeader">
        <h2 class="h2">Blockchain node</h2>
        <span class="badge" :class="data?.node.ok ? 'isOk' : 'isErr'">{{ data?.node.ok ? 'Reachable' : 'Error' }}</span>
      </div>

      <div v-if="pending">Loading…</div>
      <div v-else-if="error" class="error">Error: {{ error.message }}</div>
      <div v-else>
        <div v-if="data?.node.error" class="errorBox">
          <div class="mono">{{ data.node.error }}</div>
        </div>

        <div class="tableWrap">
          <table class="table">
            <tbody>
              <tr>
                <td class="k">Chain</td>
                <td class="v mono">{{ data?.node.chain || '—' }}</td>
              </tr>
              <tr>
                <td class="k">Blocks</td>
                <td class="v mono">{{ formatMaybeNumber(data?.node.blocks) }}</td>
              </tr>
              <tr>
                <td class="k">Headers</td>
                <td class="v mono">{{ formatMaybeNumber(data?.node.headers) }}</td>
              </tr>
              <tr>
                <td class="k">Best block time</td>
                <td class="v mono">{{ formatAbsoluteTime(data?.node.bestBlockTime) }}</td>
              </tr>
              <tr>
                <td class="k">IBD</td>
                <td class="v mono">{{ data?.node.initialblockdownload === true ? 'true' : data?.node.initialblockdownload === false ? 'false' : '—' }}</td>
              </tr>
              <tr>
                <td class="k">Verification progress</td>
                <td class="v mono">{{ formatPercent(data?.node.verificationprogress) }}</td>
              </tr>
              <tr>
                <td class="k">Connections</td>
                <td class="v mono">{{ formatMaybeNumber(data?.node.connections) }}</td>
              </tr>
              <tr>
                <td class="k">Subversion</td>
                <td class="v mono">{{ data?.node.subversion || '—' }}</td>
              </tr>
              <tr>
                <td class="k">RPC latency</td>
                <td class="v mono">{{ formatMs(data?.node.latencyMs) }}</td>
              </tr>
              <tr v-if="data?.node.warnings">
                <td class="k">Warnings</td>
                <td class="v">{{ data?.node.warnings }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>

    <section class="card">
      <div class="cardHeader">
        <h2 class="h2">Fulcrum (Electrum)</h2>
        <span class="badge" :class="data?.fulcrum.ok ? 'isOk' : 'isErr'">{{ data?.fulcrum.ok ? 'Reachable' : 'Error' }}</span>
      </div>

      <div v-if="pending">Loading…</div>
      <div v-else-if="error" class="error">Error: {{ error.message }}</div>
      <div v-else>
        <div v-if="data?.fulcrum.error" class="errorBox">
          <div class="mono">{{ data.fulcrum.error }}</div>
        </div>

        <div class="tableWrap">
          <table class="table">
            <tbody>
              <tr>
                <td class="k">Endpoint</td>
                <td class="v mono">{{ `${data?.fulcrum.host}:${data?.fulcrum.port}` }}</td>
              </tr>
              <tr>
                <td class="k">Tip height</td>
                <td class="v mono">{{ formatMaybeNumber(data?.fulcrum.height) }}</td>
              </tr>
              <tr>
                <td class="k">Tip block time</td>
                <td class="v mono">{{ formatAbsoluteTime(data?.fulcrum.headerTime) }}</td>
              </tr>
              <tr>
                <td class="k">Server version</td>
                <td class="v mono">{{ formatJsonOneLine(data?.fulcrum.version) }}</td>
              </tr>
              <tr>
                <td class="k">Banner</td>
                <td class="v mono">{{ formatJsonOneLine(data?.fulcrum.banner) }}</td>
              </tr>
              <tr>
                <td class="k">RPC latency</td>
                <td class="v mono">{{ formatMs(data?.fulcrum.latencyMs) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
type StatusResponse = {
  generatedAt: number
  node: {
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
  fulcrum: {
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
  comparison: {
    heightDiff?: number
    timeDiffSeconds?: number
    inSync: boolean
  }
}

const locale = usePageLocale()

const { data, pending, error } = await useFetch<StatusResponse>('/api/status')

function formatAbsoluteTime(unixSeconds?: number) {
  if (!unixSeconds) return '—'
  return new Date(unixSeconds * 1000).toLocaleString(locale.value, { timeZone: 'UTC', timeZoneName: 'short' })
}

function formatMaybeNumber(v: number | undefined) {
  if (typeof v !== 'number' || !Number.isFinite(v)) return '—'
  return new Intl.NumberFormat(locale.value, { maximumFractionDigits: 0 }).format(v)
}

function formatMs(v: number | undefined) {
  if (typeof v !== 'number' || !Number.isFinite(v)) return '—'
  return `${Math.round(v)} ms`
}

function formatSeconds(v: number | undefined) {
  if (typeof v !== 'number' || !Number.isFinite(v)) return '—'
  const abs = Math.abs(v)
  const sign = v < 0 ? '-' : '+'
  if (abs < 120) return `${sign}${Math.round(abs)}s`
  const min = Math.round(abs / 60)
  if (min < 120) return `${sign}${min}m`
  const hr = Math.round(abs / 3600)
  return `${sign}${hr}h`
}

function formatPercent(v: number | undefined) {
  if (typeof v !== 'number' || !Number.isFinite(v)) return '—'
  return `${(v * 100).toFixed(2)}%`
}

function formatJsonOneLine(v: unknown) {
  if (v == null) return '—'
  if (typeof v === 'string') return v
  try {
    return JSON.stringify(v)
  } catch {
    return String(v)
  }
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
.muted {
  color: var(--color-text-muted);
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  word-break: break-all;
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
.badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.04em;
}
.badge.isOk {
  background: rgba(16, 185, 129, 0.12);
  color: rgba(6, 95, 70, 1);
}
html.dark .badge.isOk {
  background: rgba(34, 197, 94, 0.18);
  color: rgba(134, 239, 172, 1);
}
.badge.isWarn {
  background: rgba(245, 158, 11, 0.15);
  color: rgba(146, 64, 14, 1);
}
html.dark .badge.isWarn {
  background: rgba(251, 191, 36, 0.18);
  color: rgba(253, 230, 138, 1);
}
.badge.isErr {
  background: rgba(185, 28, 28, 0.12);
  color: rgba(185, 28, 28, 1);
}
html.dark .badge.isErr {
  background: rgba(248, 113, 113, 0.18);
  color: rgba(248, 113, 113, 1);
}
.grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 6px;
}
.kv {
  padding: 10px;
  border-radius: 14px;
  background: var(--color-surface);
  border: 1px solid var(--color-surface-border);
}
.kv .k {
  font-size: 12px;
  font-weight: 700;
  color: var(--color-text-secondary);
  margin-bottom: 4px;
}
.kv .v {
  font-size: 14px;
  font-weight: 800;
}
.hint {
  margin: 8px 0 0;
  font-size: 12px;
}
.tableWrap {
  overflow-x: auto;
  border-radius: 12px;
}
.table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.table td {
  padding: 10px 8px;
  border-bottom: 1px solid var(--color-border-subtle);
  vertical-align: top;
}
.table td.k {
  width: 220px;
  color: var(--color-text-secondary);
  font-weight: 700;
}
.table td.v {
  color: var(--color-text);
}
.error {
  color: var(--color-error);
}
.errorBox {
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  border-radius: 12px;
  padding: 10px;
  margin-bottom: 10px;
}
@media (max-width: 640px) {
  .grid {
    grid-template-columns: 1fr;
  }
  .table td.k {
    width: 140px;
  }
}
</style>

