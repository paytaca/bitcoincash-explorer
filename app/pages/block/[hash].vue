<template>
  <main class="container">
    <NuxtLink class="back" to="/">← Back</NuxtLink>

    <h1 class="title">Block</h1>
    <p class="mono">{{ hash }}</p>

    <section v-if="pending">Loading…</section>
    <section v-else-if="error" class="notFoundCard">
      <div class="notFoundIcon" aria-hidden="true">
        <svg viewBox="0 0 64 64" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="28" cy="28" r="14"/>
          <path d="M38 38 L52 52"/>
        </svg>
      </div>
      <h2 class="notFoundTitle">Block not found</h2>
      <p class="notFoundText">
        We couldn't find a block with that hash. Double-check the hash or head back to the explorer.
      </p>
      <NuxtLink class="notFoundBack" to="/">← Back to explorer</NuxtLink>
    </section>

    <section v-else class="card">
      <div class="grid">
        <div>
          <div class="label">Height</div>
          <div class="value mono">#{{ block.height }}</div>
        </div>
        <div>
          <div class="label">Time</div>
          <div class="value">{{ new Date(block.time * 1000).toLocaleString() }}</div>
        </div>
        <div>
          <div class="label">Tx count</div>
          <div class="value">{{ Array.isArray(block.tx) ? block.tx.length : 0 }}</div>
        </div>
      </div>

      <h2 class="h2">Transactions</h2>
      <ul class="list">
        <li v-for="tx in txids" :key="tx" class="row">
          <NuxtLink class="link mono" :to="`/tx/${tx}`">{{ tx }}</NuxtLink>
        </li>
      </ul>
    </section>
  </main>
</template>

<script setup lang="ts">
const route = useRoute()
const hash = String(route.params.hash)

const { data: block, pending, error } = await useFetch<any>(`/api/bch/block/${hash}`)

const txids = computed(() => {
  const tx = block.value?.tx
  if (!Array.isArray(tx)) return []
  // verbosity=2 returns tx objects; fall back if strings.
  return tx.map((t: any) => (typeof t === 'string' ? t : t?.txid)).filter(Boolean)
})
</script>

<style scoped>
.container {
  max-width: 960px;
  margin: 24px auto;
  padding: 0 16px;
  font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial;
}
.back {
  text-decoration: none;
  color: var(--color-text);
  opacity: 0.8;
}
.back:hover {
  color: var(--color-link);
}
.title {
  margin: 12px 0 6px;
  font-size: 24px;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  word-break: break-all;
}
.card {
  margin-top: 14px;
  border: 1px solid var(--color-border);
  border-radius: 14px;
  padding: 14px;
  background: var(--color-bg-card);
}
.grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}
.label {
  font-size: 12px;
  color: var(--color-text-muted);
}
.value {
  font-size: 14px;
  color: var(--color-text);
}
.h2 {
  margin: 10px 0;
  font-size: 18px;
  color: var(--color-text-secondary);
}
.list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  gap: 8px;
}
.row {
  border-radius: 10px;
  padding: 10px 10px;
  background: var(--color-surface);
  border: 1px solid var(--color-surface-border);
}
.link {
  text-decoration: none;
  color: inherit;
}
.link:hover {
  color: var(--color-link);
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
</style>

