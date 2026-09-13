<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from '~/i18n'
import { useFeedbackStore } from '~/stores/feedback'
import { ConfigService } from '#/wvmail/services'
import type { LLMConfig } from '~/types/config'
import { getLLMProviderLimits, LLM_PROVIDER_DEFAULTS, LLM_PROVIDER_LABELS } from '~/types/config'
import AppIcon from '~/components/AppIcon.vue'
import { getProviderIconConfig } from '~/composables/provider-icons'
import LLMEditorDialog from '~/components/settings/LLMEditorDialog.vue'

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
  ContextWindowTokens: getLLMProviderLimits('openai').context,
  MaxOutputTokens: getLLMProviderLimits('openai').output,
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
  const config = getProviderIconConfig(provider)
  if (!config) return null
  return { ...config, size, radius }
}

async function loadConfigs() {
  try {
    const config = await ConfigService.GetConfig()
    configs.value = (config.LLMConfigs ?? []) as LLMConfig[]
  } catch {
    configs.value = []
  }
}

onMounted(loadConfigs)

function splitMsg(message: string) {
  const index = message.indexOf('\n')
  if (index === -1) {
    const httpError = message.match(/^(HTTP \d+):\s+([\s\S]+)$/)
    if (httpError) {
      return { summary: httpError[1], body: httpError[2] }
    }
  }
  return {
    summary: index === -1 ? message : message.slice(0, index),
    body: index === -1 ? '' : message.slice(index + 1),
  }
}

function clearTestResults() {
  testSummary.value = ''
  testBody.value = ''
  testChatSummary.value = ''
  testChatBody.value = ''
}

function openAdd() {
  editingIndex.value = null
  form.value = {
    Name: '',
    Provider: 'openai',
    Model: '',
    BaseURL: LLM_PROVIDER_DEFAULTS.openai,
    APIKey: '',
    ContextWindowTokens: getLLMProviderLimits('openai').context,
    MaxOutputTokens: getLLMProviderLimits('openai').output,
  }
  clearTestResults()
  showModal.value = true
}

function openEdit(index: number) {
  editingIndex.value = index
  const config = configs.value[index]
  const limits = getLLMProviderLimits(config.Provider)
  form.value = {
    ...config,
    ContextWindowTokens: config.ContextWindowTokens || limits.context,
    MaxOutputTokens: config.MaxOutputTokens || limits.output,
  }
  clearTestResults()
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
  configs.value = configs.value.filter((_, itemIndex) => itemIndex !== index)
  try {
    await ConfigService.SaveLLMConfigs(configs.value)
  } catch {
    // Keep the existing optimistic delete behavior.
  }
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
  const config = {
    Name: form.value.Name,
    Provider: form.value.Provider,
    Model: kind === 'chat' ? form.value.Model : '',
    BaseURL: form.value.BaseURL,
    APIKey: form.value.APIKey,
  }

  if (kind === 'conn') testing.value = true
  else testingChat.value = true

  try {
    const [ok, message] = await ConfigService.TestLLMConnection(config)
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
              <template v-if="testResultIndex === index">{{ testSuccess ? '✓' : '✕' }}</template>
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

    <LLMEditorDialog
      v-model="form"
      :open="showModal"
      :editing="editingIndex !== null"
      :provider-options="providerOptions"
      :api-key-required="apiKeyRequired"
      :testing="testing"
      :testing-chat="testingChat"
      :saving="saving"
      :test-summary="testSummary"
      :test-body="testBody"
      :test-ok="testOk"
      :test-chat-summary="testChatSummary"
      :test-chat-body="testChatBody"
      :test-chat-ok="testChatOk"
      @update:open="showModal = $event"
      @test="runTest"
      @save="handleSave"
    />
  </div>
</template>

<style scoped>
.wrap {
  max-width: 760px;
  padding: 16px;
  color: var(--n-text-color);
}
.empty {
  padding: 32px 18px;
  color: var(--color-text-3);
  font-size: 13px;
}
.rows {
  margin: 0;
  padding: 0;
  list-style: none;
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
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.row__name {
  font-size: 13px;
}
.meta {
  display: grid;
  grid-template-columns: 70px 1fr;
  gap: 2px 10px;
  margin: 6px 0 0;
  font-size: 12px;
  opacity: 0.7;
}
.meta dt {
  font-weight: 600;
}
.meta dd {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.row__actions {
  display: flex;
  flex-shrink: 0;
  flex-direction: column;
  align-items: stretch;
  gap: 4px;
}
@media (max-width: 560px) {
  .row {
    flex-direction: column;
  }
  .row__actions {
    flex-direction: row;
  }
}
</style>
