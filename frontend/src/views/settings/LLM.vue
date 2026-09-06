<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from '~/i18n'
import { useFeedbackStore } from '~/stores/feedback'
import { ConfigService } from '#/wvmail/services'
import type { LLMConfig, LLMProvider } from '~/types/config'
import { LLM_PROVIDER_DEFAULTS, LLM_PROVIDER_LABELS } from '~/types/config'
import AppIcon from '~/components/AppIcon.vue'
import UiSelect from '~/components/UiSelect.vue'
import { getProviderIconConfig } from '~/composables/provider-icons'

const { t } = useI18n()
const fb = useFeedbackStore()

const configs = ref<LLMConfig[]>([])
const showModal = ref(false)
const editingIndex = ref<number | null>(null)
const saving = ref(false)
const confirmingDelete = ref<number | null>(null)

const form = ref<LLMConfig>({
  Name: '',
  Provider: 'openai',
  Model: '',
  BaseURL: '',
  APIKey: '',
})

const testing = ref(false)
const testingChat = ref(false)
const testSummary = ref('')
const testBody = ref('')
const testOk = ref(false)
const testChatSummary = ref('')
const testChatBody = ref('')
const testChatOk = ref(false)

const testingIndex = ref<number | null>(null)
const testResultIndex = ref<number | null>(null)
const testSuccess = ref(false)

const providerOptions = Object.entries(LLM_PROVIDER_LABELS).map(([value, label]) => ({
  value,
  label,
}))

const apiKeyRequired = computed(() => form.value.Provider !== 'ollama')

function providerIconProps(provider: string, size = 14, radius = 3) {
  const cfg = getProviderIconConfig(provider)
  if (!cfg) return null
  return { ...cfg, size, radius }
}

async function loadConfigs() {
  try {
    const cfg = await ConfigService.GetConfig()
    configs.value = (cfg.LLMConfigs ?? []) as typeof configs.value
  } catch {
    configs.value = []
  }
}
onMounted(loadConfigs)

function splitMsg(msg: string) {
  const idx = msg.indexOf('\n')
  return {
    summary: idx === -1 ? msg : msg.slice(0, idx),
    body: idx === -1 ? '' : msg.slice(idx + 1),
  }
}

function openAdd() {
  editingIndex.value = null
  form.value = {
    Name: '',
    Provider: 'openai',
    Model: '',
    BaseURL: LLM_PROVIDER_DEFAULTS.openai,
    APIKey: '',
  }
  testSummary.value = testBody.value = ''
  testChatSummary.value = testChatBody.value = ''
  confirmDebounce.value = null
  showModal.value = true
}

const confirmDebounce = ref<ReturnType<typeof setTimeout> | null>(null)

function openEdit(index: number) {
  editingIndex.value = index
  form.value = { ...configs.value[index] }
  testSummary.value = testBody.value = testChatSummary.value = testChatBody.value = ''
  showModal.value = true
}

async function handleSave() {
  if (!form.value.Name || !form.value.BaseURL) return
  saving.value = true
  try {
    const list = [...configs.value]
    if (editingIndex.value !== null) list[editingIndex.value] = { ...form.value }
    else list.push({ ...form.value })
    await ConfigService.SaveLLMConfigs(list)
    configs.value = list
    showModal.value = false
    fb.success(t.value['settings.llm.save'])
  } catch {
    fb.error(t.value['settings.llm.test.fail'])
  } finally {
    saving.value = false
  }
}

async function deleteRow(index: number) {
  if (confirmingDelete.value !== index) {
    confirmingDelete.value = index
    setTimeout(() => {
      if (confirmingDelete.value === index) confirmingDelete.value = null
    }, 3000)
    return
  }
  confirmingDelete.value = null
  configs.value = configs.value.filter((_, i) => i !== index)
  try {
    await ConfigService.SaveLLMConfigs(configs.value)
  } catch {
    /* ignore */
  }
}

function onProvider(p: string) {
  form.value.Provider = p as LLMProvider
  if (editingIndex.value === null && LLM_PROVIDER_DEFAULTS[p as LLMProvider])
    form.value.BaseURL = LLM_PROVIDER_DEFAULTS[p as LLMProvider]
  if (p === 'ollama' && editingIndex.value === null && apiKeyRequired.value === false)
    form.value.APIKey = ''
}

function onInput(key: 'Name' | 'Model' | 'BaseURL' | 'APIKey', e: Event) {
  form.value[key] = (e.target as HTMLInputElement).value
}

async function testItem(index: number) {
  const item = configs.value[index]
  testingIndex.value = index
  testResultIndex.value = null
  testSuccess.value = false
  try {
    const [success] = await ConfigService.TestLLMConnection({ ...item })
    testResultIndex.value = index
    testSuccess.value = success
  } catch {
    testResultIndex.value = index
    testSuccess.value = false
  } finally {
    testingIndex.value = null
    setTimeout(() => {
      if (testResultIndex.value === index) {
        testResultIndex.value = null
        testSuccess.value = false
      }
    }, 3000)
  }
}

async function runTest(kind: 'conn' | 'chat') {
  const cfg = {
    Name: form.value.Name,
    Provider: form.value.Provider,
    Model: kind === 'chat' ? form.value.Model : '',
    BaseURL: form.value.BaseURL,
    APIKey: form.value.APIKey,
  }
  if (kind === 'conn') testing.value = true
  else testingChat.value = true
  try {
    const [ok, message] = await ConfigService.TestLLMConnection(cfg)
    const { summary, body } = splitMsg(message)
    if (kind === 'conn') {
      testOk.value = ok
      testSummary.value = summary
      testBody.value = body
    } else {
      testChatOk.value = ok
      testChatSummary.value = summary
      testChatBody.value = body
    }
  } catch {
    if (kind === 'conn') {
      testOk.value = false
      testSummary.value = 'Connection test failed'
      testBody.value = ''
    } else {
      testChatOk.value = false
      testChatSummary.value = 'Chat test failed'
      testChatBody.value = ''
    }
  } finally {
    if (kind === 'conn') testing.value = false
    else testingChat.value = false
  }
}

function maskKey(key: string): string {
  if (key.length <= 8) return '****'
  return key.slice(0, 4) + '****' + key.slice(-4)
}

function canSave() {
  return Boolean(
    form.value.Name && form.value.BaseURL && (!apiKeyRequired.value || form.value.APIKey),
  )
}
function canConn() {
  return Boolean(form.value.BaseURL && (!apiKeyRequired.value || form.value.APIKey))
}
function canChat() {
  return Boolean(
    form.value.Model && form.value.BaseURL && (!apiKeyRequired.value || form.value.APIKey),
  )
}
</script>

<template>
  <div class="wrap">
    <section class="surface">
      <header class="surface__head">
        <h2>{{ t['settings.llm.title'] }}</h2>
        <button type="button" class="btn btn--primary btn--sm" @click="openAdd">
          {{ t['settings.llm.add'] }}
        </button>
      </header>

      <p v-if="configs.length === 0" class="empty">{{ t['settings.llm.empty'] }}</p>

      <ul v-else class="rows">
        <li v-for="(item, index) in configs" :key="index" class="row">
          <div class="row__main">
            <div class="row__head">
              <strong class="row__name">{{ item.Name }}</strong>
              <span class="chip">
                <AppIcon
                  v-if="providerIconProps(item.Provider)"
                  v-bind="providerIconProps(item.Provider)!"
                />
                {{ LLM_PROVIDER_LABELS[item.Provider] ?? item.Provider }}
              </span>
            </div>
            <dl class="meta">
              <template v-if="item.Model">
                <dt>Model</dt>
                <dd>{{ item.Model }}</dd>
              </template>
              <dt>URL</dt>
              <dd>{{ item.BaseURL }}</dd>
              <dt>API Key</dt>
              <dd v-if="item.APIKey">{{ maskKey(item.APIKey) }}</dd>
              <dd v-else-if="item.Provider === 'ollama'">No API key required</dd>
            </dl>
          </div>
          <div class="row__actions">
            <button
              type="button"
              class="btn btn--sm"
              :class="{ 'row-testing': testingIndex === index }"
              @click="testItem(index)"
            >
              <template v-if="testResultIndex === index">
                {{ testSuccess ? '✓' : '✕' }}
              </template>
              {{ testingIndex === index ? '…' : t['settings.llm.test'] }}
            </button>
            <button type="button" class="btn btn--sm" @click="openEdit(index)">
              {{ t['settings.llm.edit'] }}
            </button>
            <button
              type="button"
              class="btn btn--sm btn--danger"
              :class="{ 'btn--danger-c': confirmingDelete === index }"
              @click="deleteRow(index)"
            >
              {{
                confirmingDelete === index
                  ? t['settings.llm.delete'] + '?'
                  : t['settings.llm.delete']
              }}
            </button>
          </div>
        </li>
      </ul>
    </section>

    <!-- Modal dialog -->
    <Teleport to="body">
      <div v-if="showModal" class="overlay">
        <div class="modal">
          <header class="modal__head">
            <h3>{{ editingIndex !== null ? t['settings.llm.edit'] : t['settings.llm.add'] }}</h3>
            <button type="button" class="btn--ghost" title="close" @click="showModal = false">
              <i class="i-ri:close-line" />
            </button>
          </header>
          <div class="modal__body">
            <div class="field">
              <label>{{ t['settings.llm.provider'] }}</label>
              <UiSelect
                :options="providerOptions"
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
              <label
                >{{ t['settings.llm.apiKey']
                }}<span v-if="!apiKeyRequired"> (optional)</span></label
              >
              <input
                class="ctl pass"
                type="password"
                :value="form.APIKey"
                @input="onInput('APIKey', $event)"
                :placeholder="apiKeyRequired ? 'sk-...' : 'Optional for local Ollama'"
              />
            </div>

            <div class="test-actions">
              <button
                type="button"
                class="btn"
                :disabled="!canConn() || testing"
                @click="runTest('conn')"
              >
                {{ testing ? '…' : t['settings.llm.test_connection'] }}
              </button>
              <button
                type="button"
                class="btn"
                :disabled="!canChat() || testingChat"
                @click="runTest('chat')"
              >
                {{ testingChat ? '…' : t['settings.llm.test_chat'] }}
              </button>
            </div>

            <div v-if="testSummary" class="result" :class="testOk ? 'ok' : 'bad'">
              <strong>Connectivity: {{ testSummary }}</strong>
              <details v-if="testBody">
                <summary>Response body</summary>
                <pre>{{ testBody }}</pre>
              </details>
            </div>
            <div v-if="testChatSummary" class="result" :class="testChatOk ? 'ok' : 'bad'">
              <strong>Chat: {{ testChatSummary }}</strong>
              <details v-if="testChatBody">
                <summary>Response body</summary>
                <pre>{{ testChatBody }}</pre>
              </details>
            </div>
          </div>
          <footer class="modal__foot">
            <button type="button" class="btn" @click="showModal = false">
              {{ t['settings.llm.cancel'] }}
            </button>
            <button
              type="button"
              class="btn btn--primary"
              :disabled="!canSave() || saving"
              @click="handleSave"
            >
              {{ saving ? '…' : t['settings.llm.save'] }}
            </button>
          </footer>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.wrap {
  padding: 16px;
  max-width: 760px;
  color: var(--n-text-color);
}
.surface__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.empty {
  padding: 32px 18px;
  color: var(--color-text-3);
  font-size: 13px;
}
.rows {
  list-style: none;
  margin: 0;
  padding: 0;
}
.row {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--n-border-color);
}
.row:last-child {
  border-bottom: none;
}
.row__main {
  min-width: 0;
  flex: 1;
}
.row__head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.row__name {
  font-size: 13px;
}
.meta {
  margin: 6px 0 0;
  display: grid;
  grid-template-columns: 70px 1fr;
  column-gap: 10px;
  row-gap: 2px;
  font-size: 12px;
  opacity: 0.7;
}
.meta dt {
  font-weight: 600;
}
.meta dd {
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.row__actions {
  display: flex;
  flex-direction: column;
  gap: 4px;
  align-items: stretch;
  flex-shrink: 0;
}

/* Modal (page-specific) */
.overlay {
  position: fixed;
  inset: 0;
  z-index: 900;
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 10vh 16px 16px;
  overflow: auto;
}
.modal {
  width: 520px;
  max-width: 100%;
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-lg);
  overflow: hidden;
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
  font-size: 13px;
  margin-bottom: 4px;
  color: var(--color-text-2);
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
  border-radius: 6px;
  border: 1px solid var(--n-border-color);
}
.result.ok {
  background: rgba(24, 160, 88, 0.12);
}
.result.bad {
  background: rgba(208, 48, 80, 0.12);
}
.result summary {
  cursor: pointer;
  user-select: none;
}
.result pre {
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
  max-height: 180px;
  overflow: auto;
}
</style>
