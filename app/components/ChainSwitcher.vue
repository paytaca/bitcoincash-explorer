<template>
  <div class="chainSwitcher" ref="dropdownRef">
    <button
      type="button"
      class="trigger"
      @click="toggle"
      :aria-expanded="isOpen"
      aria-haspopup="listbox"
    >
      <span class="badge" :class="chainClass">{{ chainLabel }}</span>
      <svg class="chevron" :class="{ open: isOpen }" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M6 9l6 6 6-6" />
      </svg>
    </button>

    <div v-if="isOpen" class="dropdown" role="listbox" @click.stop>
      <button
        v-for="option in options"
        :key="option.chain"
        type="button"
        class="option"
        :class="{ active: option.chain === chain, disabled: !option.url }"
        role="option"
        :aria-selected="option.chain === chain"
        :disabled="!option.url"
        @click="handleOptionClick(option.url)"
      >
        <span class="optionBadge" :class="option.class">{{ option.label }}</span>
        <span v-if="option.chain === chain" class="check">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
            <path d="M20 6L9 17l-5-5" />
          </svg>
        </span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
const config = useRuntimeConfig()
const chain = config.public.chain
const mainnetUrl = config.public.mainnetUrl
const chipnetUrl = config.public.chipnetUrl

const isOpen = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)

const chainLabel = computed(() => {
  switch (chain) {
    case 'chipnet': return 'Chipnet'
    case 'testnet': return 'Testnet'
    case 'regtest': return 'Regtest'
    default: return 'Mainnet'
  }
})

const chainClass = computed(() => {
  switch (chain) {
    case 'chipnet': return 'isChipnet'
    case 'testnet': return 'isTestnet'
    case 'regtest': return 'isRegtest'
    default: return 'isMainnet'
  }
})

const options = computed(() => [
  { chain: 'mainnet', label: 'Mainnet', class: 'isMainnet', url: mainnetUrl },
  { chain: 'chipnet', label: 'Chipnet', class: 'isChipnet', url: chipnetUrl }
])

function toggle() {
  isOpen.value = !isOpen.value
}

function close() {
  isOpen.value = false
}

function handleOptionClick(url: string | undefined) {
  if (!url) return
  close()
  window.location.href = url
}

function handleClickOutside(event: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
    close()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.chainSwitcher {
  position: relative;
  display: inline-block;
}

.trigger {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 0;
  border: none;
  background: transparent;
  cursor: pointer;
  color: inherit;
  font: inherit;
}

.trigger:hover .badge {
  opacity: 0.85;
}

.badge {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  transition: opacity 0.15s;
}

.badge.isMainnet {
  background: var(--color-badge-confirmed-bg);
  color: var(--color-badge-confirmed-fg);
}

.badge.isChipnet {
  background: var(--color-badge-mempool-bg);
  color: var(--color-badge-mempool-fg);
}

.badge.isTestnet {
  background: #fef3c7;
  color: #92400e;
}

.badge.isRegtest {
  background: #e0e7ff;
  color: #3730a3;
}

.chevron {
  width: 16px;
  height: 16px;
  color: var(--color-text-muted);
  transition: transform 0.2s;
}

.chevron.open {
  transform: rotate(180deg);
}

.dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  margin-top: 6px;
  min-width: 140px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border);
  border-radius: 10px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
  z-index: 100;
  overflow: hidden;
}

.option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
  padding: 10px 12px;
  border: none;
  background: transparent;
  text-decoration: none;
  color: var(--color-text);
  font: inherit;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  transition: background 0.1s;
}

.option:hover {
  background: var(--color-surface);
}

.option.active {
  background: var(--color-surface);
}

.option.disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.optionBadge {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
}

.optionBadge.isMainnet {
  background: var(--color-badge-confirmed-bg);
  color: var(--color-badge-confirmed-fg);
}

.optionBadge.isChipnet {
  background: var(--color-badge-mempool-bg);
  color: var(--color-badge-mempool-fg);
}

.check {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  color: var(--color-text-secondary);
}

.check svg {
  width: 14px;
  height: 14px;
}
</style>
