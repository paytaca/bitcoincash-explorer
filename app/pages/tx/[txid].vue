<template>
  <main class="container">
    <NuxtLink class="back" to="/">← Back</NuxtLink>

    <h1 class="title">Transaction</h1>
    <p class="mono">{{ txid }}</p>

    <section v-if="pending">Loading…</section>
    <section v-else-if="error" class="notFoundCard">
      <div class="notFoundIcon" aria-hidden="true">
        <svg viewBox="0 0 64 64" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="28" cy="28" r="14"/>
          <path d="M38 38 L52 52"/>
        </svg>
      </div>
      <h2 class="notFoundTitle">Transaction not found</h2>
      <p class="notFoundText">
        We couldn't find a transaction with that ID. Double-check the txid or head back to the explorer.
      </p>
      <NuxtLink class="notFoundBack" to="/">← Back to explorer</NuxtLink>
    </section>

    <section v-else class="card">
      <h2 class="h2">Summary</h2>
      <div class="grid">
        <div>
          <div class="label">Confirmations</div>
          <div class="value">{{ tx.confirmations ?? '—' }}</div>
        </div>
        <div>
          <div class="label">{{ timeLabel }}</div>
          <div class="value">
            {{ timeValue ? new Date(timeValue * 1000).toLocaleString(undefined, { timeZoneName: 'short' }) : '—' }}
            <div v-if="timeHint" class="hint">{{ timeHint }}</div>
          </div>
        </div>
        <div>
          <div class="label">Size</div>
          <div class="value">{{ tx.size ?? '—' }} bytes</div>
        </div>
        <div>
          <div class="label">Inputs</div>
          <div class="value">{{ inputs.length }}</div>
        </div>
        <div>
          <div class="label">Outputs</div>
          <div class="value">{{ outputs.length }}</div>
        </div>
        <div>
          <div class="label">Fee</div>
          <div class="value amount">{{ formatBch(tx.fee) }} <span class="unit">BCH</span></div>
        </div>
      </div>

      <div v-if="hasAnyToken" class="addressToggle">
        <div class="segmented" role="group" aria-label="Address display mode">
          <button class="segBtn" :class="{ active: addressMode === 'cash' }" type="button" @click="addressMode = 'cash'">
            Cash address
          </button>
          <button class="segBtn" :class="{ active: addressMode === 'token' }" type="button" @click="addressMode = 'token'">
            Token address
          </button>
        </div>
      </div>

      <div class="totals">
        <div class="pill">
          <span class="muted">Total in</span>
          <span class="amount">{{ formatBch(totalIn) }} <span class="unit">BCH</span></span>
        </div>
        <div class="pill">
          <span class="muted">Total out</span>
          <span class="amount">{{ formatBch(totalOut) }} <span class="unit">BCH</span></span>
        </div>
      </div>

      <h2 class="h2">Inputs</h2>
      <ul class="list">
        <li v-for="(i, idx) in inputs" :key="i.key" class="row">
          <div class="rowTop">
            <div class="mono muted">#{{ idx }}</div>
            <div class="amount">{{ formatBch(i.value) }} <span class="unit">BCH</span></div>
          </div>

          <div class="line">
            <span class="label">From</span>
            <span class="addr">
              <template v-if="i.address">
                <NuxtLink class="addrLink" :to="addressLink(i.address)">{{ formatAddress(i.address) }}</NuxtLink>
              </template>
              <template v-else>{{ i.type || '—' }}</template>
            </span>
          </div>

          <div class="line">
            <span class="label">Outpoint</span>
            <span class="mono">{{ i.txid }}:{{ i.vout }}</span>
          </div>

          <div v-if="i.tokenData?.category" class="tokenBox">
            <div class="tokenTitle">Token</div>
            <div class="line">
              <span class="label">Category</span>
              <span class="addr">{{ i.tokenData.category }}</span>
            </div>
            <div v-if="metaByCategory[i.tokenData.category]?.name" class="line">
              <span class="label">Name</span>
              <span class="tokenName">{{ metaByCategory[i.tokenData.category]?.name }}</span>
            </div>
            <div v-if="i.tokenData.amount && BigInt(i.tokenData.amount) !== 0n" class="line">
              <span class="label">Amount</span>
              <span class="amount">
                {{ formatAmount(i.tokenData.amount, metaByCategory[i.tokenData.category]?.decimals) }}
                <template v-if="metaByCategory[i.tokenData.category]?.symbol">
                  {{ ' ' + metaByCategory[i.tokenData.category]!.symbol }}
                </template>
              </span>
            </div>
            <div v-if="i.tokenData.nft" class="line">
              <span class="label">NFT</span>
              <span class="addr">
                capability={{ i.tokenData.nft.capability }}, commitment={{ i.tokenData.nft.commitment || '(none)' }}
              </span>
            </div>
          </div>
        </li>
      </ul>

      <h2 class="h2">Outputs</h2>
      <ul class="list">
        <li v-for="(o, idx) in outputs" :key="o.key" class="row">
          <div class="rowTop">
            <div class="mono muted">#{{ idx }}</div>
            <div class="amountRow">
              <span class="amount">{{ formatBch(o.value) }} <span class="unit">BCH</span></span>
              <span v-if="o.type !== 'nulldata'" class="spentIcon" :class="spentStatusClass(o)">
                <template v-if="outpointStatus[o.n] === 'unspent'">
                  <span class="iconBadge" aria-label="Unspent" title="Unspent">
                    <svg viewBox="0 0 24 24" aria-hidden="true">
                      <path
                        d="M12 2.5a9.5 9.5 0 1 0 0 19a9.5 9.5 0 0 0 0-19Z"
                        fill="currentColor"
                        opacity="0.16"
                      />
                      <path
                        d="M8.2 11.2V9.3a3.8 3.8 0 1 1 7.6 0v1.9"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                      <rect
                        x="7.6"
                        y="11.2"
                        width="8.8"
                        height="7.6"
                        rx="1.6"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                      />
                      <path
                        d="M12 14.3v1.6"
                        stroke="currentColor"
                        stroke-width="2"
                        stroke-linecap="round"
                      />
                    </svg>
                  </span>
                </template>
                <template v-else-if="outpointStatus[o.n] === 'spent'">
                  <span class="iconBadge" aria-label="Spent" title="Spent">
                    <svg viewBox="0 0 24 24" aria-hidden="true">
                      <path
                        d="M12 2.5a9.5 9.5 0 1 0 0 19a9.5 9.5 0 0 0 0-19Z"
                        fill="currentColor"
                        opacity="0.12"
                      />
                      <path
                        d="M9.3 11.2V9.3a3.8 3.8 0 0 1 7.2-1.9"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                      <rect
                        x="7.6"
                        y="11.2"
                        width="8.8"
                        height="7.6"
                        rx="1.6"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                      />
                      <path
                        d="M8.2 16.9L15.8 9.3"
                        stroke="currentColor"
                        stroke-width="2.4"
                        stroke-linecap="round"
                      />
                    </svg>
                  </span>
                </template>
                <template v-else>
                  <span aria-label="Unknown" title="Unknown">•</span>
                </template>
              </span>
            </div>
          </div>

          <div class="line">
            <span class="label">{{ o.type === 'nulldata' ? 'Data' : 'To' }}</span>
            <span class="addr">
              <template v-if="o.type === 'nulldata'">{{ o.asm || '—' }}</template>
              <template v-else>
                <template v-if="outputAddressList(o).length">
                  <template v-for="(a, idx) in outputAddressList(o)" :key="a.raw">
                    <NuxtLink class="addrLink" :to="addressLink(a.raw)">{{ a.display }}</NuxtLink
                    ><span v-if="idx < outputAddressList(o).length - 1">, </span>
                  </template>
                </template>
                <template v-else>{{ o.type || '—' }}</template>
              </template>
            </span>
          </div>

          <div v-if="o.tokenData?.category" class="tokenBox">
            <div class="tokenTitle">Token</div>
            <div class="line">
              <span class="label">Category</span>
              <span class="addr">{{ o.tokenData.category }}</span>
            </div>
            <div v-if="metaByCategory[o.tokenData.category]?.name" class="line">
              <span class="label">Name</span>
              <span class="tokenName">{{ metaByCategory[o.tokenData.category]?.name }}</span>
            </div>
            <div v-if="o.tokenData.amount && BigInt(o.tokenData.amount) !== 0n" class="line">
              <span class="label">Amount</span>
              <span class="amount">
                {{ formatAmount(o.tokenData.amount, metaByCategory[o.tokenData.category]?.decimals) }}
                <template v-if="metaByCategory[o.tokenData.category]?.symbol">
                  {{ ' ' + metaByCategory[o.tokenData.category]!.symbol }}
                </template>
              </span>
            </div>
            <div v-if="o.tokenData.nft" class="line">
              <span class="label">NFT</span>
              <span class="addr">
                capability={{ o.tokenData.nft.capability }}, commitment={{ o.tokenData.nft.commitment || '(none)' }}
              </span>
            </div>
          </div>
        </li>
      </ul>

      <details class="details">
        <summary>Raw tx</summary>
        <pre class="pre">{{ tx }}</pre>
      </details>
    </section>
  </main>
</template>

<script setup lang="ts">
import type { AddressDisplayMode } from '~/utils/addressFormat'
import { convertCashAddrDisplay } from '~/utils/addressFormat'

const route = useRoute()
const txid = String(route.params.txid)

const locale = (() => {
  if (import.meta.client) return navigator.language || 'en-US'
  const al = useRequestHeaders(['accept-language'])['accept-language']
  return al?.split(',')?.[0] || 'en-US'
})()

const addressMode = ref<AddressDisplayMode>('cash')

if (import.meta.client) {
  const saved = localStorage.getItem('bchexplorer.addressMode') as AddressDisplayMode | null
  if (saved === 'cash' || saved === 'token') addressMode.value = saved
  watch(addressMode, (v) => localStorage.setItem('bchexplorer.addressMode', v))
}

type TokenData = {
  category: string
  amount?: string
  nft?: { capability: string; commitment?: string }
}

const { data: tx, pending, error } = await useFetch<any>(`/api/bch/tx/${txid}`)

const timeValue = computed<number | undefined>(() => {
  const t = tx.value
  const blockTime = typeof t?.time === 'number' ? t.time : typeof t?.blocktime === 'number' ? t.blocktime : undefined
  const seenTime = typeof t?.seenTime === 'number' ? t.seenTime : undefined
  return seenTime ?? blockTime
})

const timeLabel = computed(() => (typeof tx.value?.seenTime === 'number' ? 'Seen time' : 'Block time'))

const timeHint = computed(() => {
  if (typeof tx.value?.seenTime === 'number') return 'From mempool (not mined yet)'
  return ''
})

type TxInput = {
  key: string
  txid: string
  vout: number
  value?: number
  address?: string
  type?: string
  tokenData?: TokenData
}

type TxOutput = {
  key: string
  n: number
  value?: number
  address?: string
  addresses?: string[]
  type?: string
  asm?: string
  tokenData?: TokenData
}

type OutpointStatus = 'unspent' | 'spent' | 'unknown'

const inputs = computed<TxInput[]>(() => {
  const vin = tx.value?.vin
  if (!Array.isArray(vin)) return []
  return vin.map((v: any, idx: number) => {
    const spk = v?.scriptPubKey || {}
    return {
      key: `${idx}:${v?.txid || ''}:${v?.vout ?? ''}`,
      txid: String(v?.txid || ''),
      vout: Number(v?.vout ?? 0),
      value: typeof v?.value === 'number' ? v.value : undefined,
      address: spk?.address || (Array.isArray(spk?.addresses) ? spk.addresses[0] : undefined),
      type: spk?.type,
      tokenData: v?.tokenData
    }
  })
})

const outputs = computed<TxOutput[]>(() => {
  const vout = tx.value?.vout
  if (!Array.isArray(vout)) return []
  return vout.map((o: any, idx: number) => {
    const spk = o?.scriptPubKey || {}
    const addrs = Array.isArray(spk?.addresses) ? spk.addresses : undefined
    return {
      key: `${idx}:${o?.n ?? idx}:${spk?.hex || ''}`,
      n: Number(o?.n ?? idx),
      value: typeof o?.value === 'number' ? o.value : undefined,
      address: spk?.address,
      addresses: addrs,
      type: spk?.type,
      asm: typeof spk?.asm === 'string' ? spk.asm : undefined,
      tokenData: o?.tokenData
    }
  })
})

const totalIn = computed(() => inputs.value.reduce((sum, i) => sum + (i.value || 0), 0))
const totalOut = computed(() => outputs.value.reduce((sum, o) => sum + (o.value || 0), 0))

const { data: outpointStatusData } = await useAsyncData<Record<number, OutpointStatus>>(
  () => `txout-${txid}-${(tx.value?.vout?.length ?? 0)}`,
  async () => {
    const t = tx.value
    if (!t?.vout || !Array.isArray(t.vout)) return {}
    const result: Record<number, OutpointStatus> = {}
    const vouts = t.vout as any[]
    await Promise.all(
      vouts
        .filter((o: any) => o?.type !== 'nulldata')
        .map(async (o: any) => {
          const n = typeof o.n === 'number' ? o.n : vouts.indexOf(o)
          try {
            const res = await $fetch<{ status: OutpointStatus }>(`/api/bch/txout/${txid}/${n}`)
            result[n] = res?.status || 'unknown'
          } catch {
            result[n] = 'unknown'
          }
        })
    )
    return result
  },
  { watch: [tx] }
)
const outpointStatus = computed(() => outpointStatusData.value ?? {})

const hasAnyToken = computed(() => {
  return (
    inputs.value.some((i) => Boolean(i.tokenData?.category)) ||
    outputs.value.some((o) => Boolean(o.tokenData?.category))
  )
})

function normalizeTokenMeta(payload: any): { name?: string; symbol?: string; decimals?: number } {
  // New endpoint: single object
  if (payload && typeof payload === 'object' && !Array.isArray(payload)) {
    const t = payload?.token || payload
    return {
      name: payload?.name,
      symbol: t?.symbol,
      decimals: typeof t?.decimals === 'number' ? t.decimals : undefined
    }
  }

  // Legacy endpoint: array of candidates
  const candidates = payload
  if (!Array.isArray(candidates) || candidates.length === 0) return {}

  const scored = candidates
    .map((c) => {
      const t = c?.token || c
      const score =
        (t?.symbol ? 3 : 0) +
        (Number.isFinite(t?.decimals) ? 3 : 0) +
        (c?.name ? 2 : 0) +
        (c?.uris?.icon ? 1 : 0)
      return { c, t, score }
    })
    .sort((a, b) => b.score - a.score)

  const best = scored[0]?.c
  const bestToken = best?.token || best
  return {
    name: best?.name,
    symbol: bestToken?.symbol,
    decimals: typeof bestToken?.decimals === 'number' ? bestToken.decimals : undefined
  }
}

const metaByCategory = reactive<Record<string, { name?: string; symbol?: string; decimals?: number }>>({})

// Use server-provided token metadata when present (tx API enriches with BCMR data).
watch(
  () => tx.value?.tokenMeta,
  (tokenMeta) => {
    if (tokenMeta && typeof tokenMeta === 'object' && !Array.isArray(tokenMeta)) {
      Object.assign(metaByCategory, tokenMeta)
    }
  },
  { immediate: true }
)

// Fallback: fetch from /api/bcmr/token/ for categories not yet in metaByCategory.
watch(
  () => {
    const inCats = inputs.value.map((i) => i.tokenData?.category).filter(Boolean) as string[]
    const outCats = outputs.value.map((o) => o.tokenData?.category).filter(Boolean) as string[]
    return [...inCats, ...outCats]
  },
  async (cats) => {
    const uniq = Array.from(new Set(cats))
    await Promise.all(
      uniq.map(async (cat) => {
        if (metaByCategory[cat]) return
        try {
          const payload = await $fetch<any>(`/api/bcmr/token/${cat}`)
          metaByCategory[cat] = normalizeTokenMeta(payload)
        } catch {
          // Avoid repeated retries; token metadata is optional.
          metaByCategory[cat] = {}
        }
      })
    )
  },
  { immediate: true }
)

function formatBch(v: unknown) {
  const n = typeof v === 'number' && Number.isFinite(v) ? v : 0
  // BCH values are up to 8 decimals.
  return new Intl.NumberFormat(locale, { maximumFractionDigits: 8 }).format(n)
}

function formatAddress(addr?: string) {
  if (!addr) return ''
  return convertCashAddrDisplay(addr, addressMode.value)
}

function addressLink(addr: string) {
  return `/address/${encodeURIComponent(addr)}`
}

function outputAddressList(o: TxOutput): { raw: string; display: string }[] {
  const list = o.addresses?.length ? o.addresses : o.address ? [o.address] : []
  return list.filter(Boolean).map((raw) => ({ raw, display: convertCashAddrDisplay(raw, addressMode.value) }))
}

function formatOutputAddresses(o: TxOutput) {
  const list = o.addresses?.length ? o.addresses : o.address ? [o.address] : []
  if (list.length === 0) return ''
  return list.map((a) => convertCashAddrDisplay(a, addressMode.value)).join(', ')
}

function formatAmount(amountStr: string, decimals?: number) {
  const dec = Number.isFinite(decimals) ? Number(decimals) : 0
  const amt = BigInt(amountStr)
  const intFmt = new Intl.NumberFormat(locale, { maximumFractionDigits: 0 })

  if (dec <= 0) return intFmt.format(amt)

  const parts = new Intl.NumberFormat(locale).formatToParts(1.1)
  const decimalSep = parts.find((p) => p.type === 'decimal')?.value || '.'

  const base = BigInt(10) ** BigInt(dec)
  const whole = amt / base
  const frac = (amt % base).toString().padStart(dec, '0').replace(/0+$/, '')
  const wholeStr = intFmt.format(whole)
  return frac ? `${wholeStr}${decimalSep}${frac}` : wholeStr
}

function spentStatusClass(o: TxOutput) {
  const s = outpointStatus.value[o.n]
  return s === 'unspent' ? 'isUnspent' : s === 'spent' ? 'isSpent' : 'isUnknown'
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
.h2 {
  margin: 16px 0 10px 0;
  font-size: 14px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--color-text-secondary);
}
.grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}
.addressToggle {
  margin-top: 10px;
  display: flex;
  justify-content: flex-end;
}
.segmented {
  display: inline-flex;
  background: var(--color-segmented-bg);
  border: 1px solid var(--color-border-subtle);
  border-radius: 999px;
  padding: 3px;
  gap: 3px;
}
.segBtn {
  border: 0;
  background: transparent;
  padding: 8px 10px;
  border-radius: 999px;
  font-size: 12px;
  letter-spacing: 0.02em;
  cursor: pointer;
  color: var(--color-text-secondary);
}
.segBtn.active {
  background: var(--color-segmented-active);
  box-shadow: var(--shadow-segmented);
  color: var(--color-text);
  font-weight: 600;
}
.value {
  font-size: 14px;
  font-weight: 600;
}
.totals {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  margin-top: 12px;
}
.pill {
  padding: 8px 10px;
  border-radius: 999px;
  background: var(--color-surface);
  border: 1px solid var(--color-surface-border);
  display: inline-flex;
  gap: 8px;
  align-items: baseline;
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
.rowTop {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 6px;
}
.amountRow {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.spentIcon {
  line-height: 1;
}
.iconBadge {
  display: inline-flex;
  vertical-align: middle;
  transform: translateY(-1px);
}
.iconBadge svg {
  width: 18px;
  height: 18px;
}
.spentIcon.isUnspent {
  color: var(--color-icon-unspent);
}
.spentIcon.isSpent {
  color: var(--color-icon-spent);
}
.spentIcon.isUnknown {
  color: var(--color-text-muted);
}
.line {
  display: grid;
  grid-template-columns: 100px 1fr;
  gap: 10px;
  padding: 2px 0;
}
.tokenBox {
  margin-top: 8px;
  padding-top: 10px;
  border-top: 1px dashed var(--color-border);
}
.tokenTitle {
  font-weight: 600;
  margin-bottom: 6px;
  color: var(--color-link);
}
.label {
  font-size: 12px;
  letter-spacing: 0.02em;
  color: var(--color-text-muted);
}
.addr {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  word-break: break-all;
  color: var(--color-text);
}
.addrLink {
  color: inherit;
  text-decoration: none;
}
.addrLink:hover {
  text-decoration: underline;
  color: var(--color-link);
}
.tokenName {
  font-weight: 600;
  color: var(--color-text);
}
.amount {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-weight: 700;
  color: var(--color-amount);
}
.unit {
  font-weight: 600;
  color: var(--color-text-secondary);
}
.details {
  margin-top: 14px;
}
.pre {
  white-space: pre-wrap;
  color: var(--color-text);
}
.error {
  color: var(--color-error);
}
.notFoundCard {
  margin-top: 14px;
  padding: 28px 20px;
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-bg-card);
  text-align: center;
}
.notFoundIcon {
  margin: 0 auto 20px;
  width: 80px;
  height: 80px;
  color: var(--color-text-muted);
}
.notFoundIcon svg {
  width: 100%;
  height: 100%;
  display: block;
}
.notFoundTitle {
  margin: 0 0 8px;
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text);
}
.notFoundText {
  margin: 0 0 16px;
  font-size: 14px;
  color: var(--color-text-muted);
  max-width: 360px;
  margin-left: auto;
  margin-right: auto;
}
.notFoundBack {
  display: inline-block;
  padding: 8px 14px;
  border-radius: 999px;
  background: var(--color-surface);
  border: 1px solid var(--color-surface-border);
  font-size: 14px;
  font-weight: 600;
  text-decoration: none;
  color: var(--color-text);
}
.notFoundBack:hover {
  background: var(--color-surface-hover, var(--color-surface));
  border-color: var(--color-border);
}
.hint {
  margin-top: 2px;
  font-size: 12px;
  color: var(--color-text-muted);
  font-weight: 500;
}
</style>

