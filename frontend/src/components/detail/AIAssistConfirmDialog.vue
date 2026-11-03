<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Dialog } from '@ark-ui/vue/dialog'
import { ConfigService } from '#/wvmail/services'
import { useI18n } from '~/i18n'
import { llmModels } from '~/composables/llm-models'
import type { AppConfig, LLMConfig } from '~/types/config'
import type { AIAssistMode, AIAssistSelection } from '~/types/ai-assist'
import UiAlert from '~/components/ui/UiAlert.vue'
import UiCheckbox from '~/components/ui/UiCheckbox.vue'
import UiDialog from '~/components/ui/UiDialog.vue'
import UiDialogHeader from '~/components/ui/UiDialogHeader.vue'
import UiSelect from '~/components/ui/UiSelect.vue'

const props = defineProps<{
  open: boolean
  mode: AIAssistMode
}>()

const emit = defineEmits<{
  'update:open': [open: boolean]
  confirm: [selection: AIAssistSelection, dontAsk: boolean]
}>()

const { t } = useI18n()

const loading = ref(false)
const error = ref('')
const config = ref<AppConfig | null>(null)
const llmConfigs = ref<LLMConfig[]>([])
const selectedProvider = ref('')
const selectedModel = ref('')
const dontAsk = ref(false)
const defaultConfigName = ref('')
const defaultModel = ref('')

const isTranslation = computed(() => props.mode === 'translate')
const title = computed(
  () =>
    t.value[
      isTranslation.value ? 'mail.aiConfirm.title.translate' : 'mail.aiConfirm.title.summarize'
    ],
)
const body = computed(
  () =>
    t.value[
      isTranslation.value ? 'mail.aiConfirm.body.translate' : 'mail.aiConfirm.body.summarize'
    ],
)
const confirmLabel = computed(
  () =>
    t.value[
      isTranslation.value ? 'mail.aiConfirm.confirm.translate' : 'mail.aiConfirm.confirm.summarize'
    ],
)

const providerOptions = computed(() => {
  const options = llmConfigs.value.map((cfg) => ({
    label: cfg.Name,
    value: `llm:${cfg.Name}`,
  }))
  if (isTranslation.value && config.value?.TranslationConfig?.DeepLAuthKey) {
    options.push({ label: 'DeepL', value: 'deepl' })
  }
  return options
})

const selectedConfig = computed(() => {
  if (!selectedProvider.value.startsWith('llm:')) return null
  const name = selectedProvider.value.slice(4)
  return llmConfigs.value.find((cfg) => cfg.Name === name) ?? null
})

const modelOptions = computed(() =>
  llmModels(selectedConfig.value ?? undefined).map((model) => ({
    label: model,
    value: model,
  })),
)

const canConfirm = computed(() => {
  if (loading.value || !selectedProvider.value) return false
  if (selectedProvider.value === 'deepl') return true
  return Boolean(selectedConfig.value && selectedModel.value)
})

const warning = computed(() => {
  if (
    isTranslation.value &&
    config.value?.TranslationConfig?.DefaultEngine === 'deepl' &&
    !config.value?.TranslationConfig?.DeepLAuthKey
  ) {
    return t.value['mail.aiConfirm.deepl_no_key']
  }
  if (
    isTranslation.value &&
    selectedProvider.value === 'deepl' &&
    !config.value?.TranslationConfig?.DeepLAuthKey
  ) {
    return t.value['mail.aiConfirm.deepl_no_key']
  }
  if (selectedProvider.value.startsWith('llm:') && !selectedModel.value) {
    return t.value['mail.aiConfirm.no_model']
  }
  if (
    llmConfigs.value.length === 0 &&
    (!isTranslation.value || !config.value?.TranslationConfig?.DeepLAuthKey)
  ) {
    return t.value['mail.aiConfirm.no_llm']
  }
  return ''
})

function preferredConfigName(cfg: AppConfig): string {
  const saved = cfg.TranslationConfig?.LLMConfigName ?? ''
  if (saved && llmConfigs.value.some((item) => item.Name === saved)) return saved
  return llmConfigs.value[0]?.Name ?? ''
}

function modelsFor(name: string): string[] {
  return llmModels(llmConfigs.value.find((item) => item.Name === name))
}

function reset() {
  loading.value = true
  error.value = ''
  dontAsk.value = false
  selectedProvider.value = ''
  selectedModel.value = ''
  llmConfigs.value = []
  config.value = null
}

async function load() {
  reset()
  try {
    const cfg = await ConfigService.GetConfig()
    config.value = cfg
    llmConfigs.value = (cfg.LLMConfigs ?? []) as LLMConfig[]
    defaultConfigName.value = preferredConfigName(cfg)
    const models = modelsFor(defaultConfigName.value)
    const savedModel = cfg.TranslationConfig?.LLMModel ?? ''
    defaultModel.value = models.includes(savedModel) ? savedModel : (models[0] ?? '')

    const useDeepL =
      isTranslation.value &&
      Boolean(cfg.TranslationConfig?.DeepLAuthKey) &&
      (cfg.TranslationConfig?.DefaultEngine === 'deepl' || !defaultConfigName.value)
    selectedProvider.value = useDeepL
      ? 'deepl'
      : defaultConfigName.value
        ? `llm:${defaultConfigName.value}`
        : ''
    selectedModel.value = defaultModel.value
  } catch (loadError) {
    error.value = String(loadError)
  } finally {
    loading.value = false
  }
}

watch(
  () => props.open,
  (open) => {
    if (open) void load()
  },
)

function onProviderChange(value: string) {
  selectedProvider.value = value
  if (!value.startsWith('llm:')) {
    selectedModel.value = ''
    return
  }
  const name = value.slice(4)
  const models = modelsFor(name)
  selectedModel.value =
    name === defaultConfigName.value && models.includes(defaultModel.value)
      ? defaultModel.value
      : (models[0] ?? '')
}

function confirm() {
  if (!canConfirm.value) return
  const selection: AIAssistSelection =
    selectedProvider.value === 'deepl'
      ? { engine: 'deepl' }
      : {
          engine: 'llm',
          llmConfigName: selectedConfig.value?.Name ?? '',
          llmModel: selectedModel.value,
        }
  emit('confirm', selection, dontAsk.value)
}
</script>

<template>
  <UiDialog
    :open="open"
    :busy="loading"
    :close-on-interact-outside="false"
    @update:open="emit('update:open', $event)"
  >
    <UiDialogHeader :title="title" :close-label="t['cancel']" @close="emit('update:open', false)" />
    <div class="dlg-body ai-confirm">
      <Dialog.Description class="ai-confirm__intro">{{ body }}</Dialog.Description>
      <UiAlert class="ai-confirm__alert" tone="warning" icon="i-ri:shield-keyhole-line">
        {{ t['mail.aiConfirm.warning'] }}
      </UiAlert>

      <p v-if="loading" class="ai-confirm__state">{{ t['mail.aiConfirm.loading'] }}</p>
      <UiAlert v-else-if="error" tone="error">{{ error }}</UiAlert>
      <template v-else>
        <label class="field">
          <span class="field__label">{{ t['mail.aiConfirm.provider'] }}</span>
          <UiSelect
            :options="providerOptions"
            :model-value="selectedProvider"
            :disabled="providerOptions.length === 0"
            width="100%"
            @update:model-value="onProviderChange"
          />
        </label>

        <label v-if="selectedProvider.startsWith('llm:')" class="field">
          <span class="field__label">{{ t['mail.aiConfirm.model'] }}</span>
          <UiSelect
            :options="modelOptions"
            :model-value="selectedModel"
            :disabled="modelOptions.length === 0"
            width="100%"
            @update:model-value="selectedModel = $event"
          />
        </label>

        <UiAlert v-if="warning" tone="warning">{{ warning }}</UiAlert>

        <UiCheckbox v-model:checked="dontAsk" class="ai-confirm__dont-ask">
          {{ t['mail.aiConfirm.dont_ask'] }}
        </UiCheckbox>
      </template>
    </div>
    <div class="dlg-foot">
      <button type="button" class="btn" @click="emit('update:open', false)">
        {{ t['cancel'] }}
      </button>
      <button type="button" class="btn btn--primary" :disabled="!canConfirm" @click="confirm">
        {{ confirmLabel }}
      </button>
    </div>
  </UiDialog>
</template>

<style scoped>
.ai-confirm__intro {
  margin: 0 0 12px;
  color: var(--color-text-2);
  font-size: 13px;
  line-height: 1.6;
}
.ai-confirm__alert {
  margin-bottom: 14px;
}
.ai-confirm .field {
  margin-bottom: 12px;
}
.ai-confirm__dont-ask {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 14px;
  font-size: 13px;
  cursor: pointer;
}
.ai-confirm__state {
  margin: 0;
  color: var(--color-text-3);
  font-size: 13px;
}
</style>
