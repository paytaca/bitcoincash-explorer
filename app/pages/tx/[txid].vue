<template>
  <main class="container">
    <NuxtLink class="back" to="/">← Back</NuxtLink>

    <h1 class="title">Transaction</h1>
    <p class="mono">{{ txid }}</p>

    <section v-if="pending">Loading…</section>
    <section v-else-if="error" class="error">Error: {{ error.message }}</section>

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
            {{ timeValue ? new Date(timeValue * 1000).toLocaleString() : '—' }}
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
            <span class="addr">{{ formatAddress(i.address) || i.type || '—' }}</span>
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
              {{
                o.type === 'nulldata'
                  ? o.asm || '—'
                  : formatOutputAddresses(o) || o.type || '—'
              }}
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

const outpointStatus = reactive<Record<number, OutpointStatus>>({})

const hasAnyToken = computed(() => {
  return (
    inputs.value.some((i) => Boolean(i.tokenData?.category)) ||
    outputs.value.some((o) => Boolean(o.tokenData?.category))
  )
})

watch(
  () => outputs.value.map((o) => o.n),
  async (ns) => {
    const uniq = Array.from(new Set(ns))
    await Promise.all(
      uniq.map(async (n) => {
        if (outpointStatus[n]) return
        try {
          const res = await $fetch<{ status: OutpointStatus }>(`/api/bch/txout/${txid}/${n}`)
          outpointStatus[n] = res?.status || 'unknown'
        } catch {
          outpointStatus[n] = 'unknown'
        }
      })
    )
  },
  { immediate: true }
)

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
  const s = outpointStatus[o.n]
  return s === 'unspent' ? 'isUnspent' : s === 'spent' ? 'isSpent' : 'isUnknown'
}
</script>

<style scoped>
.container {
  max-width: 960px;
  margin: 24px auto;
  padding: 0 16px;
  font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial;
  color: rgba(17, 24, 39, 1);
}
.back {
  text-decoration: none;
  color: inherit;
  opacity: 0.75;
  font-weight: 500;
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
  color: rgba(107, 114, 128, 1);
}
.card {
  margin-top: 14px;
  border: 1px solid rgba(17, 24, 39, 0.08);
  border-radius: 16px;
  padding: 14px;
  background: rgba(255, 255, 255, 1);
}
.h2 {
  margin: 16px 0 10px 0;
  font-size: 14px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgba(55, 65, 81, 1);
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
  background: rgba(17, 24, 39, 0.04);
  border: 1px solid rgba(17, 24, 39, 0.06);
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
  color: rgba(55, 65, 81, 1);
}
.segBtn.active {
  background: rgba(255, 255, 255, 1);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);
  color: rgba(17, 24, 39, 1);
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
  background: rgba(17, 24, 39, 0.04);
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
  background: rgba(17, 24, 39, 0.03);
  border: 1px solid rgba(17, 24, 39, 0.04);
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
  color: rgba(6, 95, 70, 1);
}
.spentIcon.isSpent {
  color: rgba(185, 28, 28, 1);
}
.spentIcon.isUnknown {
  color: rgba(107, 114, 128, 1);
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
  border-top: 1px dashed rgba(17, 24, 39, 0.14);
}
.tokenTitle {
  font-weight: 600;
  margin-bottom: 6px;
  color: rgba(29, 78, 216, 1);
}
.label {
  font-size: 12px;
  letter-spacing: 0.02em;
  color: rgba(107, 114, 128, 1);
}
.addr {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  word-break: break-all;
  color: rgba(17, 24, 39, 0.95);
}
.tokenName {
  font-weight: 600;
  color: rgba(17, 24, 39, 0.92);
}
.amount {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-weight: 700;
  color: rgba(6, 95, 70, 1);
}
.unit {
  font-weight: 600;
  color: rgba(55, 65, 81, 1);
}
.details {
  margin-top: 14px;
}
.pre {
  white-space: pre-wrap;
}
.error {
  color: #b42318;
}
.hint {
  margin-top: 2px;
  font-size: 12px;
  color: rgba(107, 114, 128, 1);
  font-weight: 500;
}
</style>

