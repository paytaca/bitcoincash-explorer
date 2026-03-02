<template>
  <main class="container">
    <h1 class="title">Broadcast Transaction</h1>
    <p class="description">Enter a raw transaction in hex format to broadcast it to the Bitcoin Cash network.</p>

    <section class="card">
      <form class="broadcastForm" @submit.prevent="onSubmit">
        <div class="formRow">
          <div class="formGroup networkGroup">
            <label for="network" class="label">Network</label>
            <select
              id="network"
              v-model="selectedNetwork"
              class="select"
              :disabled="pending"
              @change="handleNetworkChange"
            >
              <option value="mainnet">Mainnet</option>
              <option value="chipnet">Chipnet</option>
            </select>
          </div>
        </div>

        <div class="formGroup">
          <label for="tx-hex" class="label">Transaction Hex</label>
          <textarea
            id="tx-hex"
            v-model="hex"
            class="input"
            :class="{ invalid: error, success }"
            rows="6"
            placeholder="Paste raw transaction hex here..."
            :disabled="pending"
          />
        </div>

        <div class="formActions">
          <button 
            type="submit" 
            class="submitBtn"
            :disabled="pending || !hex.trim()"
          >
            <template v-if="pending">Broadcasting...</template>
            <template v-else>Broadcast Transaction</template>
          </button>
        </div>
      </form>

      <div v-if="error" class="result error">
        <div class="resultTitle">Error</div>
        <div class="resultMessage">{{ error }}</div>
      </div>

      <div v-else-if="success && result" class="result success">
        <div class="resultTitle">Success!</div>
        <div class="resultMessage">
          Transaction broadcast successfully.
        </div>
        <div class="resultTxid">
          <span class="txidLabel">Txid:</span>
          <NuxtLink class="txidLink" :to="`/tx/${result.txid}`">{{ result.txid }}</NuxtLink>
        </div>
      </div>
    </section>

    <section class="infoCard">
      <h2 class="infoTitle">About Transaction Broadcasting</h2>
      <ul class="infoList">
        <li>Select the network (Mainnet or Chipnet) where you want to broadcast</li>
        <li>The transaction must be valid and properly signed</li>
        <li>The transaction will be validated by the network before acceptance</li>
        <li>Once broadcast, the transaction will enter the mempool and await confirmation</li>
        <li>You can track the transaction status using the provided txid</li>
      </ul>
    </section>
  </main>
</template>

<script setup lang="ts">
const config = useRuntimeConfig()
const currentChain = (config.public?.chain as string) || 'mainnet'
const mainnetUrl = (config.public?.mainnetUrl as string) || ''
const chipnetUrl = (config.public?.chipnetUrl as string) || ''

const selectedNetwork = ref(currentChain)
const hex = ref('')
const pending = ref(false)
const error = ref<string | null>(null)
const success = ref(false)
const result = ref<{ txid: string } | null>(null)

function handleNetworkChange() {
  // If user selected a different network, redirect to that network's explorer
  if (selectedNetwork.value !== currentChain) {
    const targetUrl = selectedNetwork.value === 'mainnet' ? mainnetUrl : chipnetUrl
    if (targetUrl) {
      window.location.href = `${targetUrl}/broadcast`
    }
  }
}

async function onSubmit() {
  if (!hex.value.trim()) return

  pending.value = true
  error.value = null
  success.value = false
  result.value = null

  try {
    const data = await $fetch<{ success: boolean; txid: string }>('/api/bch/broadcast', {
      method: 'POST',
      body: { hex: hex.value.trim() }
    })
    
    result.value = data
    success.value = true
    hex.value = ''
  } catch (e: any) {
    error.value = e?.data?.statusMessage || e?.message || 'Failed to broadcast transaction'
  } finally {
    pending.value = false
  }
}
</script>

<style scoped>
.container {
  max-width: 800px;
  margin: 24px auto;
  padding: 0 16px;
}

.title {
  margin: 0 0 6px;
  font-size: 24px;
  letter-spacing: -0.02em;
}

.description {
  margin: 0 0 20px;
  color: var(--color-text-secondary);
  font-size: 14px;
}

.card {
  border: 1px solid var(--color-border);
  border-radius: 16px;
  padding: 20px;
  background: var(--color-bg-card);
}

.broadcastForm {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.formGroup {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.label {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.input {
  width: 100%;
  box-sizing: border-box;
  padding: 12px;
  border-radius: 12px;
  border: 1px solid var(--color-input-border);
  background: var(--color-bg-input);
  color: var(--color-text);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 13px;
  line-height: 1.5;
  resize: vertical;
  min-height: 120px;
  outline: none;
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

.formActions {
  display: flex;
  gap: 12px;
}

.formRow {
  display: flex;
  gap: 16px;
}

.networkGroup {
  flex: 0 0 auto;
  min-width: 150px;
}

.select {
  width: 100%;
  box-sizing: border-box;
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid var(--color-input-border);
  background: var(--color-bg-input);
  color: var(--color-text);
  font-size: 14px;
  cursor: pointer;
  outline: none;
}

.select:focus {
  border-color: var(--color-border-focus);
  box-shadow: 0 0 0 3px var(--color-focus-ring);
}

.submitBtn {
  padding: 12px 24px;
  border: none;
  border-radius: 12px;
  background: var(--color-link);
  color: white;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: opacity 0.15s;
}

.submitBtn:hover:not(:disabled) {
  opacity: 0.9;
}

.submitBtn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.result {
  margin-top: 20px;
  padding: 16px;
  border-radius: 12px;
}

.result.error {
  background: var(--color-error-bg, #fef2f2);
  border: 1px solid var(--color-error-border, #fee2e2);
}

.result.success {
  background: var(--color-success-bg, #f0fdf4);
  border: 1px solid var(--color-success-border, #bbf7d0);
}

.resultTitle {
  font-size: 14px;
  font-weight: 700;
  margin-bottom: 8px;
}

.result.error .resultTitle {
  color: var(--color-error, #dc2626);
}

.result.success .resultTitle {
  color: var(--color-success, #16a34a);
}

.resultMessage {
  font-size: 13px;
  color: var(--color-text-secondary);
  margin-bottom: 12px;
}

.resultTxid {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  padding: 12px;
  background: var(--color-surface);
  border-radius: 8px;
}

.txidLabel {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.txidLink {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 13px;
  color: var(--color-link);
  text-decoration: none;
  word-break: break-all;
}

.txidLink:hover {
  text-decoration: underline;
}

.infoCard {
  margin-top: 20px;
  padding: 20px;
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-bg-card);
}

.infoTitle {
  margin: 0 0 12px;
  font-size: 14px;
  font-weight: 700;
  color: var(--color-text-secondary);
}

.infoList {
  margin: 0;
  padding-left: 20px;
  font-size: 13px;
  color: var(--color-text-secondary);
  line-height: 1.8;
}
</style>
