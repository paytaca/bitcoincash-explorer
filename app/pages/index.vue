<template>
  <main class="container">
    <header class="header">
      <div>
        <h1 class="title">Bitcoin Cash Explorer</h1>
        <p class="muted">Chain: {{ chain }}</p>
      </div>
    </header>

    <section class="card">
      <h2 class="h2">Latest blocks</h2>
      <div v-if="tipPending">Loading…</div>
      <div v-else-if="tipError" class="error">Error: {{ tipError.message }}</div>
      <ul v-else class="list">
        <li v-for="b in blocks" :key="b.hash" class="row">
          <NuxtLink class="link" :to="`/block/${b.hash}`">
            <span class="mono">#{{ b.height }}</span>
            <span class="mono">{{ b.hash.slice(0, 16) }}…</span>
            <span class="muted">{{ b.txCount }} tx</span>
          </NuxtLink>
        </li>
      </ul>
    </section>
  </main>
</template>

<script setup lang="ts">
const chain = useRuntimeConfig().public.chain

const { data: tip, pending: tipPending, error: tipError } = await useFetch<number>('/api/bch/blockcount')

const { data: blocks } = await useAsyncData(async () => {
  if (!tip.value) return []
  const heights = Array.from({ length: 15 }, (_, i) => tip.value! - i)
  const hashes = await Promise.all(heights.map((h) => $fetch<string>(`/api/bch/blockhash/${h}`)))
  const full = await Promise.all(hashes.map((hash) => $fetch<any>(`/api/bch/block/${hash}`)))
  return full.map((b) => ({
    hash: b.hash as string,
    height: b.height as number,
    txCount: Array.isArray(b.tx) ? b.tx.length : 0
  }))
})
</script>

<style scoped>
.container {
  max-width: 960px;
  margin: 24px auto;
  padding: 0 16px;
  font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial, "Apple Color Emoji",
    "Segoe UI Emoji";
}
.header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}
.title {
  font-size: 28px;
  margin: 0;
}
.h2 {
  margin: 0 0 10px 0;
  font-size: 18px;
}
.card {
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-radius: 14px;
  padding: 14px;
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
  display: flex;
  gap: 12px;
  align-items: baseline;
  text-decoration: none;
  color: inherit;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
}
.muted {
  opacity: 0.7;
}
.error {
  color: #b42318;
}
</style>

