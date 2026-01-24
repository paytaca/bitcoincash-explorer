<template>
  <main class="container">
    <h1 class="title">Bitcoin Cash Explorer</h1>
    <p class="muted">Chain: {{ chain }}</p>

    <section class="card">
      <div class="cardHeader">
        <h2 class="h2">Latest blocks</h2>
        <div class="muted" v-if="tip">Tip: <span class="mono">#{{ tip }}</span></div>
      </div>

      <div v-if="tipPending">Loading…</div>
      <div v-else-if="tipError" class="error">Error: {{ tipError.message }}</div>

      <ul v-else class="list">
        <li v-for="b in blocks" :key="b.hash" class="row">
          <NuxtLink class="link" :to="`/block/${b.hash}`">
            <span class="mono">#{{ b.height }}</span>
            <span class="mono hash">{{ b.hash }}</span>
            <span class="time">
              <span class="timeRel">{{ formatRelativeTime(b.time) }}</span>
              <span class="timeAbs">{{ formatAbsoluteTime(b.time) }}</span>
            </span>
            <span class="muted">{{ b.txCount }} tx</span>
          </NuxtLink>
        </li>
      </ul>
    </section>
  </main>
</template>

<script setup lang="ts">
const chain = useRuntimeConfig().public.chain

const locale = (() => {
  if (import.meta.client) return navigator.language || 'en-US'
  const al = useRequestHeaders(['accept-language'])['accept-language']
  return al?.split(',')?.[0] || 'en-US'
})()

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
    txCount: Array.isArray(b.tx) ? b.tx.length : 0
  }))
})

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
.link {
  display: grid;
  grid-template-columns: 110px 1fr 210px 70px;
  gap: 12px;
  text-decoration: none;
  color: inherit;
  align-items: baseline;
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

