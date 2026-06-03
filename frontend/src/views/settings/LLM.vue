<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  NButton,
  NCard,
  NEmpty,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NList,
  NListItem,
  NModal,
  NSelect,
  NSpace,
  NAlert,
  NTag,
  NButtonGroup,
  type SelectOption,
  NCode,
} from 'naive-ui'
import { Icon } from '@iconify/vue'
import { useI18n } from '../../i18n'
import { ConfigService } from '../../../bindings/wvmail/services'
import type { LLMConfig, LLMProvider } from '../../types/config'
import { LLM_PROVIDER_DEFAULTS, LLM_PROVIDER_LABELS } from '../../types/config'
import { renderProviderOption, getProviderIconUrl } from '../../composables/provider-icons'

const { t } = useI18n()

const configs = ref<LLMConfig[]>([])
const showModal = ref(false)
const editingIndex = ref<number | null>(null)
const saving = ref(false)

const form = ref<LLMConfig>({
  Name: '',
  Provider: 'openai',
  Model: '',
  BaseURL: '',
  APIKey: '',
})

const testing = ref(false)
const testResult = ref<{ success: boolean; message: string } | null>(null)

const testingChat = ref(false)
const testChatResult = ref<{ success: boolean; message: string } | null>(null)

const testSummary = computed(() => {
  if (!testResult.value) return ''
  const msg = testResult.value.message
  const idx = msg.indexOf('\n')
  return idx === -1 ? msg : msg.slice(0, idx)
})
const testBody = computed(() => {
  if (!testResult.value) return ''
  const msg = testResult.value.message
  const idx = msg.indexOf('\n')
  return idx === -1 ? '' : msg.slice(idx + 1)
})

const testChatSummary = computed(() => {
  if (!testChatResult.value) return ''
  const msg = testChatResult.value.message
  const idx = msg.indexOf('\n')
  return idx === -1 ? msg : msg.slice(0, idx)
})
const testChatBody = computed(() => {
  if (!testChatResult.value) return ''
  const msg = testChatResult.value.message
  const idx = msg.indexOf('\n')
  return idx === -1 ? '' : msg.slice(idx + 1)
})

const testingIndex = ref<number | null>(null)
const testResultIndex = ref<number | null>(null)
const testSuccess = ref(false)

const providerOptions: SelectOption[] = Object.entries(LLM_PROVIDER_LABELS).map(
  ([value, label]) => ({ label, value }),
)

const providerLabelMap = Object.fromEntries(
  Object.entries(LLM_PROVIDER_LABELS).map(([value, label]) => [value, label]),
)

function renderProviderLabel(option: SelectOption) {
  const value = option.value as string
  const label = providerLabelMap[value] ?? (option.label as string)
  return renderProviderOption(value, label)
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

function openAdd() {
  editingIndex.value = null
  form.value = { Name: '', Provider: 'openai', Model: '', BaseURL: LLM_PROVIDER_DEFAULTS.openai, APIKey: '' }
  testResult.value = null
  showModal.value = true
}

function openEdit(index: number) {
  editingIndex.value = index
  const item = configs.value[index]
  form.value = { ...item }
  testResult.value = null
  showModal.value = true
}

async function handleSave() {
  saving.value = true
  try {
    const list = [...configs.value]
    if (editingIndex.value !== null) {
      list[editingIndex.value] = { ...form.value }
    } else {
      list.push({ ...form.value })
    }
    await ConfigService.SaveLLMConfigs(list)
    configs.value = list
    showModal.value = false
  } catch {
    // Save failed
  } finally {
    saving.value = false
  }
}

async function handleDelete(index: number) {
  const list = configs.value.filter((_, i) => i !== index)
  try {
    await ConfigService.SaveLLMConfigs(list)
    configs.value = list
  } catch {
    // Delete failed
  }
}

async function handleTestConnect() {
  testing.value = true
  testResult.value = null
  try {
    const [success, message] = await ConfigService.TestLLMConnection({
      Name: form.value.Name,
      Provider: form.value.Provider,
      Model: '',
      BaseURL: form.value.BaseURL,
      APIKey: form.value.APIKey,
    })
    testResult.value = { success, message }
  } catch {
    testResult.value = { success: false, message: 'Connection test failed' }
  } finally {
    testing.value = false
  }
}

async function handleTestChat() {
  testingChat.value = true
  testChatResult.value = null
  try {
    const [success, message] = await ConfigService.TestLLMConnection({
      Name: form.value.Name,
      Provider: form.value.Provider,
      Model: form.value.Model,
      BaseURL: form.value.BaseURL,
      APIKey: form.value.APIKey,
    })
    testChatResult.value = { success, message }
  } catch {
    testChatResult.value = { success: false, message: 'Chat test failed' }
  } finally {
    testingChat.value = false
  }
}

async function handleTestItem(index: number) {
  const item = configs.value[index]
  testingIndex.value = index
  testResultIndex.value = null
  testSuccess.value = false
  try {
    const [success] = await ConfigService.TestLLMConnection({ ...item })
    testResultIndex.value = index
    testSuccess.value = success
    // Reset after 3 seconds
    setTimeout(() => {
      if (testResultIndex.value === index) {
        testResultIndex.value = null
        testSuccess.value = false
      }
    }, 3000)
  } catch {
    testResultIndex.value = index
    testSuccess.value = false
    setTimeout(() => {
      if (testResultIndex.value === index) {
        testResultIndex.value = null
        testSuccess.value = false
      }
    }, 3000)
  } finally {
    testingIndex.value = null
  }
}

function onProviderChange(value: string) {
  const provider = value as LLMProvider
  form.value.Provider = provider
  // Only auto-fill base URL when adding a new config, not when editing
  if (LLM_PROVIDER_DEFAULTS[provider] && !editingIndex.value) {
    form.value.BaseURL = LLM_PROVIDER_DEFAULTS[provider]
  }
  // Clear API key when switching to Ollama (not needed)
  if (provider === 'ollama' && !editingIndex.value) {
    form.value.APIKey = ''
  }
}

const apiKeyRequired = computed(() => form.value.Provider !== 'ollama')

function maskKey(key: string): string {
  if (key.length <= 8) return '****'
  return key.slice(0, 4) + '****' + key.slice(-4)
}

const providerTagType: Record<string, string> = {
  deepseek: 'info',
  openai: 'success',
  anthropic: 'warning',
  gemini: 'primary',
  minimax: 'info',
  kimi: 'warning',
  ollama: 'default',
  bytedance: 'info',
  custom: 'default',
}
</script>

<template>
  <div style="padding: 1rem">
    <NCard :title="t['settings.llm.title']">
      <template #header-extra>
        <NButton type="primary" @click="openAdd">
          {{ t['settings.llm.add'] }}
        </NButton>
      </template>

      <NEmpty
        v-if="configs.length === 0"
        :description="t['settings.llm.empty']"
      />

      <NList v-else bordered>
        <NListItem v-for="(item, index) in configs" :key="index">
          <template #default>
            <div style="display: flex; align-items: center; justify-content: space-between">
              <div style="display: flex; align-items: center; gap: 10px">
                <div>
                  <strong>{{ item.Name }}</strong>
                  <NTag
                    :type="(providerTagType[item.Provider] as any) ?? 'default'"
                    size="small"
                    style="margin-left: 8px"
                  >
                    <span style="display: inline-flex; align-items: center; gap: 4px">
                      <img
                        v-if="getProviderIconUrl(item.Provider)"
                        :src="getProviderIconUrl(item.Provider)!"
                        width="14"
                        height="14"
                        style="display: inline-block; border-radius: 3px;"
                      />
                      {{ LLM_PROVIDER_LABELS[item.Provider] ?? item.Provider }}
                    </span>
                  </NTag>
                  <div style="font-size: 12px; opacity: 0.6; margin-top: 4px">
                    <template v-if="item.Model"><strong>Model:</strong> {{ item.Model }} &middot; </template>
                    {{ item.BaseURL }}
                    <template v-if="item.APIKey"> | {{ maskKey(item.APIKey) }}</template>
                    <template v-else-if="item.Provider === 'ollama'"> | No API key required</template>
                  </div>
                </div>
              </div>
              <NButtonGroup>
                <NButton
                  size="small"
                  :loading="testingIndex === index"
                  :disabled="testingIndex === index"
                  :type="testResultIndex === index ? (testSuccess ? 'success' : 'error') : 'default'"
                  @click="handleTestItem(index)"
                >
                  <template v-if="testResultIndex === index && testSuccess" #icon>
                    <NIcon><Icon style="color: white" icon="carbon:checkmark-filled" /></NIcon>
                  </template>
                  <template v-else-if="testingIndex === index" #icon>
                    <NIcon><Icon style="color: white" icon="eos-icons:loading" /></NIcon>
                  </template>
                  {{ t['settings.llm.test'] }}
                </NButton>
                <NButton size="small" @click="openEdit(index)">
                  {{ t['settings.llm.edit'] }}
                </NButton>
                <NButton size="small" type="error" @click="handleDelete(index)">
                  {{ t['settings.llm.delete'] }}
                </NButton>
              </NButtonGroup>
            </div>
          </template>
        </NListItem>
      </NList>

      <NModal
        v-model:show="showModal"
        preset="card"
        style="width: 520px"
        :title="editingIndex !== null ? t['settings.llm.edit'] : t['settings.llm.add']"
        :mask-closable="false"
      >
        <NForm label-placement="top">
          <NFormItem :label="t['settings.llm.provider']" path="provider">
            <NSelect
              v-model:value="form.Provider"
              :options="providerOptions"
              :render-label="renderProviderLabel"
              @update:value="onProviderChange"
            />
          </NFormItem>
          <NFormItem :label="t['settings.llm.name']" path="name">
            <NInput v-model:value="form.Name" placeholder="My LLM" />
          </NFormItem>
          <NFormItem label="Model" path="model">
            <NInput
              v-model:value="form.Model"
              placeholder="gpt-4o, claude-sonnet-4-20250514, ..."
              :autocapitalize="'none'"
              autocorrect="off"
            />
          </NFormItem>
          <NFormItem :label="t['settings.llm.baseURL']" path="baseURL">
            <NInput v-model:value="form.BaseURL" placeholder="https://api.example.com/v1" />
          </NFormItem>
          <NFormItem
            :label="apiKeyRequired ? t['settings.llm.apiKey'] : (t['settings.llm.apiKey'] + ' (optional)')"
            path="apiKey"
          >
            <NInput
              v-model:value="form.APIKey"
              type="password"
              show-password-on="click"
              :placeholder="apiKeyRequired ? 'sk-...' : 'Optional for local Ollama'"
            />
          </NFormItem>
        </NForm>

        <div style="margin-bottom: 12px">
          <NSpace>
            <NButton
              :loading="testing"
              :disabled="!form.BaseURL || (apiKeyRequired && !form.APIKey)"
              @click="handleTestConnect"
            >
              <template #icon>
                <NIcon><Icon icon="carbon:network-2" /></NIcon>
              </template>
              {{ testing ? t['settings.llm.testing'] : 'Test Connection' }}
            </NButton>
            <NButton
              :loading="testingChat"
              :disabled="!form.Model || !form.BaseURL || (apiKeyRequired && !form.APIKey)"
              @click="handleTestChat"
            >
              <template #icon>
                <NIcon><Icon icon="carbon:chat" /></NIcon>
              </template>
              {{ testingChat ? t['settings.llm.testing'] : 'Test Chat' }}
            </NButton>
          </NSpace>
          <NAlert
            v-if="testResult"
            :type="testResult.success ? 'success' : 'error'"
            :title="'Connectivity: ' + testSummary"
            style="margin-top: 8px"
          >
            <details v-if="testBody">
              <summary style="cursor: pointer; user-select: none">Response body</summary>
              <NCode :code="testBody" word-wrap />
            </details>
          </NAlert>
          <NAlert
            v-if="testChatResult"
            :type="testChatResult.success ? 'success' : 'error'"
            :title="'Chat: ' + testChatSummary"
            style="margin-top: 8px"
          >
            <details v-if="testChatBody">
              <summary style="cursor: pointer; user-select: none">Response body</summary>
              <NCode :code="testChatBody" word-wrap />
            </details>
          </NAlert>
        </div>

        <template #footer>
          <NSpace justify="end">
            <NButton @click="showModal = false">
              {{ t['settings.llm.cancel'] }}
            </NButton>
            <NButton
              type="primary"
              :loading="saving"
              :disabled="!form.Name || !form.BaseURL || (apiKeyRequired && !form.APIKey)"
              @click="handleSave"
            >
              {{ t['settings.llm.save'] }}
            </NButton>
          </NSpace>
        </template>
      </NModal>
    </NCard>
  </div>
</template>
