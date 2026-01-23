<template>
  <header class="header">
    <div class="inner">
      <NuxtLink class="brand" to="/">
        <span class="title">Bitcoin Cash Explorer</span>
      </NuxtLink>

      <form class="search" @submit.prevent="onSubmit">
        <input
          v-model="q"
          class="input"
          :class="{ invalid }"
          type="text"
          inputmode="search"
          autocomplete="off"
          autocapitalize="off"
          spellcheck="false"
          placeholder="Search transaction…"
          aria-label="Search transaction by txid"
        />
      </form>
    </div>
  </header>
</template>

<script setup lang="ts">
const route = useRoute()
const q = ref('')
const invalid = ref(false)

function normalizeTxid(v: string) {
  return v.trim().toLowerCase().replace(/^0x/, '')
}

async function onSubmit() {
  const txid = normalizeTxid(q.value)
  const ok = /^[0-9a-f]{64}$/.test(txid)
  invalid.value = !ok
  if (!ok) return
  q.value = txid
  await navigateTo(`/tx/${txid}`)
}

watch(
  () => route.path,
  (path) => {
    if (/^\/tx\/[0-9a-f]{64}$/.test(path)) {
      q.value = ''
      invalid.value = false
    }
  },
  { immediate: true }
)
</script>

<style scoped>
.header {
  position: sticky;
  top: 0;
  z-index: 50;
  background: rgba(249, 250, 251, 0.9);
  backdrop-filter: blur(8px);
  border-bottom: 1px solid rgba(17, 24, 39, 0.08);
}
.inner {
  max-width: 960px;
  margin: 0 auto;
  padding: 14px 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
}
.brand {
  display: inline-flex;
  align-items: center;
  text-decoration: none;
  color: rgba(17, 24, 39, 1);
  cursor: pointer;
  padding: 6px 0;
  border-radius: 10px;
}
.brand:hover {
  color: rgba(29, 78, 216, 1);
}
.title {
  font-weight: 700;
  letter-spacing: -0.02em;
  font-size: 16px;
}
.search {
  flex: 1;
  display: flex;
  justify-content: flex-end;
}
.input {
  width: min(520px, 100%);
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid rgba(17, 24, 39, 0.12);
  background: rgba(255, 255, 255, 0.75);
  outline: none;
  font-size: 14px;
}
.input:focus {
  border-color: rgba(29, 78, 216, 0.65);
  box-shadow: 0 0 0 3px rgba(29, 78, 216, 0.12);
}
.input.invalid {
  border-color: rgba(180, 35, 24, 0.6);
  box-shadow: 0 0 0 3px rgba(180, 35, 24, 0.12);
}
</style>

