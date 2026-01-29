<template>
  <header class="header">
    <div class="inner">
      <NuxtLink class="brand" to="/">
        <img class="logo" src="/logo.svg" alt="" aria-hidden="true" />
        <span class="title">Bitcoin Cash Explorer</span>
      </NuxtLink>

      <form class="search" action="/search" method="get" @submit.prevent="onSubmit">
        <div class="searchWrap">
          <input
            v-model="q"
            name="q"
            class="input"
            :class="{ invalid }"
            type="text"
            inputmode="search"
            autocomplete="off"
            autocapitalize="off"
            spellcheck="false"
            placeholder="Search transaction or address…"
            aria-label="Search by txid or address"
            @keydown.enter.prevent="onSubmit"
            @input="invalid = false"
          />
        </div>
      </form>

      <button
        type="button"
        class="themeSwitcher"
        data-theme-toggle
        :data-dark="isDark"
        :aria-label="isDark ? 'Switch to light mode' : 'Switch to dark mode'"
        :title="isDark ? 'Light mode' : 'Dark mode'"
      >
        <span class="iconWrap" aria-hidden="true">
          <svg data-icon="sun" class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="4" />
            <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
          </svg>
          <svg data-icon="moon" class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
          </svg>
        </span>
      </button>
    </div>
  </header>
</template>

<script setup lang="ts">
const route = useRoute()
const router = useRouter()
const { isDark } = useTheme()
const q = ref('')
const invalid = ref(false)

function normalizeTxid(v: string) {
  return v.trim().toLowerCase().replace(/^0x/, '')
}

function normalizeQuery(v: string) {
  return v.trim()
}

function looksLikeCashAddress(input: string): boolean {
  const v = input.trim().toLowerCase()
  if (!v) return false

  const hasPrefix = v.includes(':')
  const prefix = hasPrefix ? v.split(':', 1)[0] : ''
  const payload = hasPrefix ? v.split(':').slice(1).join(':') : v

  if (hasPrefix && !['bitcoincash', 'bchtest', 'bchreg'].includes(prefix)) return false

  // Accept common cashaddr payload starts: q/p (cash), z/r (token-aware).
  return /^[qpzr][0-9a-z]{20,}$/i.test(payload)
}

async function onSubmit() {
  const raw = normalizeQuery(q.value)
  const txid = normalizeTxid(raw)
  if (/^[0-9a-f]{64}$/.test(txid)) {
    invalid.value = false
    q.value = txid
    await router.push(`/tx/${txid}`)
    return
  }

  if (!looksLikeCashAddress(raw)) {
    // Fallback to server-side /search (works even if something subtle is off client-side)
    invalid.value = false
    window.location.href = `/search?q=${encodeURIComponent(raw)}`
    return
  }

  // Keep whatever the user typed (with/without prefix). Our server endpoint can
  // handle both, and we also decodeURIComponent server-side.
  invalid.value = false
  q.value = raw
  await router.push(`/address/${encodeURIComponent(raw)}`)
}

watch(
  () => route.path,
  (path) => {
    if (/^\/tx\/[0-9a-f]{64}$/.test(path) || /^\/address\/[^/]+$/.test(path)) {
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
  background: var(--color-bg-header);
  backdrop-filter: blur(8px);
  border-bottom: 1px solid var(--color-border);
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
  gap: 10px;
  text-decoration: none;
  color: var(--color-text);
  cursor: pointer;
  padding: 6px 0;
  border-radius: 10px;
}
.brand:hover {
  color: var(--color-link);
}
.logo {
  width: 22px;
  height: 22px;
  flex: 0 0 auto;
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
.searchWrap {
  width: min(520px, 100%);
  display: block;
}
.input {
  width: 100%;
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid var(--color-input-border);
  background: var(--color-bg-input);
  color: var(--color-text);
  outline: none;
  font-size: 14px;
}
.input::placeholder {
  color: var(--color-text-muted);
}
.input:focus {
  border-color: var(--color-border-focus);
  box-shadow: 0 0 0 3px var(--color-focus-ring);
}
.input.invalid {
  border-color: var(--color-error-border);
  box-shadow: 0 0 0 3px var(--color-error-ring);
}
.themeSwitcher {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  padding: 0;
  border: 1px solid var(--color-border);
  border-radius: 12px;
  background: var(--color-bg-input);
  color: var(--color-text);
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}
.themeSwitcher:hover {
  background: var(--color-surface);
  border-color: var(--color-border-subtle);
}
.iconWrap {
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.icon {
  width: 20px;
  height: 20px;
}
</style>

