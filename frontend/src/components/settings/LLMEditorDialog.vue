<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '~/i18n'
import UiSelect from '~/components/UiSelect.vue'
import { LLM_PROVIDER_DEFAULTS } from '~/types/config'
import type { LLMConfig, LLMProvider } from '~/types/config'
import JsonResponseViewer from '~/components/settings/JsonResponseViewer.vue'

const props = defineProps<{
  open: boolean
  editing: boolean
  providerOptions: Array<{ value: string; label: string }>
  apiKeyRequired: boolean
  testing: boolean
  testingChat: boolean
  saving: boolean
  testSummary: string
  testBody: string
  testOk: boolean
  testChatSummary: string
  testChatBody: string
  testChatOk: boolean
}>()

const form = defineModel<LLMConfig>({ required: true })
const emit = defineEmits<{
  'update:open': [open: boolean]
  test: [kind: 'conn' | 'chat']
  save: []
}>()

const { t } = useI18n()

const canSave = computed(() =>
  Boolean(form.value.Name && form.value.BaseURL && (!props.apiKeyRequired || form.value.APIKey)),
)
const canConn = computed(() =>
  Boolean(form.value.BaseURL && (!props.apiKeyRequired || form.value.APIKey)),
)
const canChat = computed(() =>
  Boolean(
    form.value.Model && form.value.BaseURL && (!props.apiKeyRequired || form.value.APIKey),
  ),
)

function onProvider(provider: string) {
  form.value.Provider = provider as LLMProvider
  if (!props.editing && LLM_PROVIDER_DEFAULTS[provider as LLMProvider]) {
    form.value.BaseURL = LLM_PROVIDER_DEFAULTS[provider as LLMProvider]
  }
}

function onInput(key: 'Name' | 'Model' | 'BaseURL' | 'APIKey', event: Event) {
  form.value[key] = (event.target as HTMLInputElement).value
}

function close() {
  emit('update:open', false)
}
</script>

<template>
  <Teleport to="body">
    <div v-if="props.open" class="overlay">
      <div class="modal">
        <header class="modal__head">
          <h3>{{ props.editing ? t['settings.llm.edit'] : t['settings.llm.add'] }}</h3>
          <button type="button" class="btn--ghost" :aria-label="t['settings.accounts.cancel']" @click="close">
            <i class="i-ri:close-line" aria-hidden="true" />
          </button>
        </header>

        <div class="modal__body">
          <div class="field">
            <label>{{ t['settings.llm.provider'] }}</label>
            <UiSelect
              :options="props.providerOptions"
              :model-value="form.Provider"
              width="100%"
              @update:model-value="onProvider"
            />
          </div>
          <div class="field">
            <label>{{ t['settings.llm.name'] }}</label>
            <input
              class="ctl"
              :value="form.Name"
              @input="onInput('Name', $event)"
              placeholder="My LLM"
            />
          </div>
          <div class="field">
            <label>Model</label>
            <input
              class="ctl"
              :value="form.Model"
              @input="onInput('Model', $event)"
              placeholder="gpt-4o, claude-sonnet-4-20250514, ..."
            />
          </div>
          <div class="field">
            <label>{{ t['settings.llm.baseURL'] }}</label>
            <input
              class="ctl"
              :value="form.BaseURL"
              @input="onInput('BaseURL', $event)"
              placeholder="https://api.example.com/v1"
            />
          </div>
          <div class="field">
            <label>
              {{ t['settings.llm.apiKey'] }}<span v-if="!props.apiKeyRequired"> (optional)</span>
            </label>
            <input
              class="ctl pass"
              type="password"
              :value="form.APIKey"
              @input="onInput('APIKey', $event)"
              :placeholder="props.apiKeyRequired ? 'sk-...' : 'Optional for local Ollama'"
            />
          </div>

          <div class="test-actions">
            <button
              type="button"
              class="btn"
              :disabled="!canConn || props.testing"
              @click="emit('test', 'conn')"
            >
              {{ props.testing ? '…' : t['settings.llm.test_connection'] }}
            </button>
            <button
              type="button"
              class="btn"
              :disabled="!canChat || props.testingChat"
              @click="emit('test', 'chat')"
            >
              {{ props.testingChat ? '…' : t['settings.llm.test_chat'] }}
            </button>
          </div>

          <div v-if="props.testSummary" class="result" :class="props.testOk ? 'ok' : 'bad'">
            <strong>Connectivity: {{ props.testSummary }}</strong>
            <JsonResponseViewer v-if="props.testBody" :body="props.testBody" />
          </div>
          <div v-if="props.testChatSummary" class="result" :class="props.testChatOk ? 'ok' : 'bad'">
            <strong>Chat: {{ props.testChatSummary }}</strong>
            <JsonResponseViewer v-if="props.testChatBody" :body="props.testChatBody" />
          </div>
        </div>

        <footer class="modal__foot">
          <button type="button" class="btn" @click="close">
            {{ t['settings.llm.cancel'] }}
          </button>
          <button type="button" class="btn btn--primary" :disabled="!canSave || props.saving" @click="emit('save')">
            {{ props.saving ? '…' : t['settings.llm.save'] }}
          </button>
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  z-index: 900;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 10vh 16px 16px;
  overflow: auto;
  background: rgb(0 0 0 / 45%);
}
.modal {
  width: min(720px, 100%);
  max-width: 100%;
  overflow: hidden;
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-lg);
}
.modal__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  border-bottom: 1px solid var(--n-border-color);
}
.modal__head h3 {
  margin: 0;
  font-size: 14px;
}
.modal__body {
  padding: 12px 16px;
}
.modal__foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid var(--n-border-color);
}
.field {
  margin-bottom: 12px;
}
.field label {
  display: block;
  margin-bottom: 4px;
  color: var(--color-text-2);
  font-size: 13px;
}
.ctl {
  width: 100%;
}
.pass {
  letter-spacing: 1px;
}
.test-actions {
  display: flex;
  gap: 8px;
  margin-bottom: 6px;
}
.result {
  margin-top: 8px;
  padding: 8px 10px;
  font-size: 13px;
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
}
.result.ok {
  background: rgba(24, 160, 88, 0.12);
}
.result.bad {
  background: rgba(208, 48, 80, 0.12);
}
</style>
