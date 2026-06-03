<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  NButton,
  NCard,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NList,
  NListItem,
  NModal,
  NSelect,
  NSpace,
  NAlert,
  NTag,
  NButtonGroup,
  type FormInst,
  type SelectOption,
} from 'naive-ui'
import { useI18n } from '../../i18n'
import { ConfigService } from '../../../bindings/wvmail/services'
import type { LLMConfig, LLMProvider } from '../../types/config'
import { LLM_PROVIDER_DEFAULTS, LLM_PROVIDER_LABELS } from '../../types/config'

const { t } = useI18n()

const configs = ref<LLMConfig[]>([])
const showModal = ref(false)
const editingIndex = ref<number | null>(null)
const saving = ref(false)

const form = ref<LLMConfig>({
  Name: '',
  Provider: 'openai',
  BaseURL: '',
  APIKey: '',
})

const testing = ref(false)
const testResult = ref<{ success: boolean; message: string } | null>(null)

const providerOptions: SelectOption[] = Object.entries(LLM_PROVIDER_LABELS).map(
  ([value, label]) => ({ label, value }),
)

async function loadConfigs() {
  try {
    const cfg = await ConfigService.GetConfig()
    configs.value = cfg.LLMConfigs ?? []
  } catch {
    configs.value = []
  }
}

onMounted(loadConfigs)

function openAdd() {
  editingIndex.value = null
  form.value = { Name: '', Provider: 'openai', BaseURL: LLM_PROVIDER_DEFAULTS.openai, APIKey: '' }
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

async function handleTest() {
  testing.value = true
  testResult.value = null
  try {
    const [success, message] = await ConfigService.TestLLMConnection({
      Name: form.value.Name,
      Provider: form.value.Provider,
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

function onProviderChange(value: string) {
  const provider = value as LLMProvider
  form.value.Provider = provider
  if (LLM_PROVIDER_DEFAULTS[provider] && !editingIndex.value) {
    form.value.BaseURL = LLM_PROVIDER_DEFAULTS[provider]
  }
}

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
  custom: 'default',
}
</script>

<template>
  <NCard :title="t['settings.llm.title']" style="margin: 16px">
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
            <div>
              <strong>{{ item.Name }}</strong>
              <NTag
                :type="(providerTagType[item.Provider] as any) ?? 'default'"
                size="small"
                style="margin-left: 8px"
              >
                {{ LLM_PROVIDER_LABELS[item.Provider] ?? item.Provider }}
              </NTag>
              <div style="font-size: 12px; opacity: 0.6; margin-top: 4px">
                {{ item.BaseURL }} | {{ maskKey(item.APIKey) }}
              </div>
            </div>
            <NButtonGroup>
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
            @update:value="onProviderChange"
          />
        </NFormItem>
        <NFormItem :label="t['settings.llm.name']" path="name">
          <NInput v-model:value="form.Name" placeholder="My LLM" />
        </NFormItem>
        <NFormItem :label="t['settings.llm.baseURL']" path="baseURL">
          <NInput v-model:value="form.BaseURL" placeholder="https://api.example.com/v1" />
        </NFormItem>
        <NFormItem :label="t['settings.llm.apiKey']" path="apiKey">
          <NInput
            v-model:value="form.APIKey"
            type="password"
            show-password-on="click"
            placeholder="sk-..."
          />
        </NFormItem>
      </NForm>

      <div style="margin-bottom: 12px">
        <NButton
          :loading="testing"
          :disabled="!form.BaseURL || !form.APIKey"
          @click="handleTest"
        >
          {{ testing ? t['settings.llm.testing'] : t['settings.llm.test'] }}
        </NButton>
        <NAlert
          v-if="testResult"
          :type="testResult.success ? 'success' : 'error'"
          :title="testResult.success ? t['settings.llm.test.success'] : t['settings.llm.test.fail']"
          style="margin-top: 8px"
        >
          {{ testResult.message }}
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
            :disabled="!form.Name || !form.BaseURL || !form.APIKey"
            @click="handleSave"
          >
            {{ t['settings.llm.save'] }}
          </NButton>
        </NSpace>
      </template>
    </NModal>
  </NCard>
</template>
