<template>
  <main class="container">
    <NuxtLink class="back" to="/">← Back</NuxtLink>

    <h1 class="title">Block</h1>
    <p class="mono">{{ hash }}</p>

    <section v-if="pending">Loading…</section>
    <section v-else-if="error" class="error">Error: {{ error.message }}</section>

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
  color: inherit;
  opacity: 0.8;
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
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-radius: 14px;
  padding: 14px;
}
.grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}
.label {
  font-size: 12px;
  opacity: 0.7;
}
.value {
  font-size: 14px;
}
.h2 {
  margin: 10px 0;
  font-size: 18px;
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
  background: rgba(0, 0, 0, 0.03);
}
.link {
  text-decoration: none;
  color: inherit;
}
.error {
  color: #b42318;
}
</style>

