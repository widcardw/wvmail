<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Field } from '@ark-ui/vue/field'
import { ConfigService, AIAssistService } from '#/wvmail/services'
import { useI18n } from '~/i18n'
import { llmModels } from '~/composables/llm-models'
import type { AppConfig, LLMConfig } from '~/types/config'
import UiAlert from '~/components/ui/UiAlert.vue'
import UiDialog from '~/components/ui/UiDialog.vue'
import UiDialogHeader from '~/components/ui/UiDialogHeader.vue'
import UiSelect from '~/components/ui/UiSelect.vue'

const props = defineProps<{
  open: boolean
  sourceText: string
}>()

const emit = defineEmits<{
  'update:open': [open: boolean]
  replace: [text: string]
}>()

const { t } = useI18n()

const loading = ref(false)
const generating = ref(false)
const error = ref('')
const config = ref<AppConfig | null>(null)
const llmConfigs = ref<LLMConfig[]>([])
const selectedProvider = ref('')
const selectedModel = ref('')
const sourceDraft = ref('')
const instruction = ref('')
const result = ref('')
/** Incremented on each request so stale responses are ignored. */
let requestToken = 0

const providerOptions = computed(() =>
  llmConfigs.value.map((cfg) => ({ label: cfg.Name, value: cfg.Name })),
)

const selectedConfig = computed(
  () => llmConfigs.value.find((cfg) => cfg.Name === selectedProvider.value) ?? null,
)

const modelOptions = computed(() =>
  llmModels(selectedConfig.value ?? undefined).map((model) => ({ label: model, value: model })),
)

const canGenerate = computed(
  () =>
    !generating.value &&
    !loading.value &&
    Boolean(sourceDraft.value.trim()) &&
    Boolean(selectedProvider.value && selectedModel.value),
)

const canReplace = computed(() => !generating.value && Boolean(result.value.trim()))

function resetState() {
  requestToken++
  generating.value = false
  loading.value = true
  error.value = ''
  result.value = ''
  instruction.value = ''
  sourceDraft.value = props.sourceText
  llmConfigs.value = []
  config.value = null
  selectedProvider.value = ''
  selectedModel.value = ''
}

function preferredModel(cfg: AppConfig, name: string): string {
  const models = llmModels(llmConfigs.value.find((c) => c.Name === name))
  const preferred = cfg.TranslationConfig?.PolishLLMModel ?? ''
  if (preferred && models.includes(preferred)) return preferred
  const fallback = cfg.TranslationConfig?.LLMModel ?? ''
  if (fallback && models.includes(fallback)) return fallback
  return models[0] ?? ''
}

async function load() {
  resetState()
  try {
    const cfg = await ConfigService.GetConfig()
    config.value = cfg
    llmConfigs.value = (cfg.LLMConfigs ?? []) as LLMConfig[]
    const preferredName =
      cfg.TranslationConfig?.PolishLLMConfigName || cfg.TranslationConfig?.LLMConfigName || ''
    const name = llmConfigs.value.some((c) => c.Name === preferredName)
      ? preferredName
      : (llmConfigs.value[0]?.Name ?? '')
    selectedProvider.value = name
    selectedModel.value = name ? preferredModel(cfg, name) : ''
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

function onProviderChange(name: string) {
  selectedProvider.value = name
  if (!config.value) {
    selectedModel.value = modelOptions.value[0]?.value ?? ''
    return
  }
  selectedModel.value = name ? preferredModel(config.value, name) : ''
  result.value = ''
}

async function generate() {
  if (!canGenerate.value) return
  const token = ++requestToken
  generating.value = true
  error.value = ''
  try {
    const text = await AIAssistService.PolishText({
      Text: sourceDraft.value,
      Instruction: instruction.value,
      LLMConfigName: selectedProvider.value,
      LLMModel: selectedModel.value,
      Engine: 'llm',
      TargetLang: '',
    })
    if (token !== requestToken) return
    result.value = (text ?? '').trim()
  } catch (e) {
    if (token !== requestToken) return
    error.value = String(e)
  } finally {
    if (token === requestToken) generating.value = false
  }
}

function replaceSelection() {
  if (!canReplace.value) return
  emit('replace', result.value)
  emit('update:open', false)
}

function close() {
  requestToken++
  emit('update:open', false)
}
</script>

<template>
  <UiDialog
    :open="open"
    :busy="generating"
    :close-on-interact-outside="false"
    size="wide"
    @update:open="emit('update:open', $event)"
  >
    <UiDialogHeader :title="t('compose.ai.polish_title')" :close-label="t('cancel')" @close="close" />
    <div class="dlg-body polish-dlg">
      <UiAlert class="polish-alert" tone="warning" icon="i-ri:shield-keyhole-line">
        {{ t('compose.ai.polish_intro') }}
      </UiAlert>

      <p v-if="loading" class="polish-state">{{ t('mail.aiConfirm.loading') }}</p>
      <UiAlert v-else-if="llmConfigs.length === 0" tone="warning">
        {{ t('compose.ai.no_llm') }}
      </UiAlert>

      <Field.Root class="field">
        <Field.Label class="field__label">{{ t('compose.ai.source') }}</Field.Label>
        <Field.Textarea v-model="sourceDraft" class="ctl polish-textarea" rows="6" />
      </Field.Root>

      <Field.Root class="field">
        <Field.Label class="field__label">{{ t('compose.ai.instruction') }}</Field.Label>
        <Field.Textarea
          v-model="instruction"
          class="ctl polish-textarea polish-textarea--sm"
          rows="2"
          :placeholder="t('compose.ai.instruction_placeholder')"
        />
      </Field.Root>

      <div class="polish-model-row">
        <label class="field polish-model-field">
          <span class="field__label">{{ t('mail.aiConfirm.provider') }}</span>
          <UiSelect
            :options="providerOptions"
            :model-value="selectedProvider"
            :disabled="providerOptions.length === 0 || generating"
            width="100%"
            @update:model-value="onProviderChange"
          />
        </label>
        <label class="field polish-model-field">
          <span class="field__label">{{ t('mail.aiConfirm.model') }}</span>
          <UiSelect
            :options="modelOptions"
            :model-value="selectedModel"
            :disabled="modelOptions.length === 0 || generating"
            width="100%"
            @update:model-value="selectedModel = $event"
          />
        </label>
      </div>

      <UiAlert v-if="error" tone="error">{{ error }}</UiAlert>

      <Field.Root v-if="result || generating" class="field">
        <Field.Label class="field__label">{{ t('compose.ai.result') }}</Field.Label>
        <Field.Textarea
          v-model="result"
          class="ctl polish-textarea"
          rows="6"
          :readonly="generating"
          :placeholder="generating ? t('compose.ai.generating') : ''"
        />
      </Field.Root>
    </div>

    <div class="dlg-foot">
      <button type="button" class="btn" :disabled="generating" @click="close">
        {{ t('cancel') }}
      </button>
      <template v-if="result">
        <button type="button" class="btn" :disabled="!canGenerate" @click="generate">
          {{ t('compose.ai.regenerate') }}
        </button>
        <button type="button" class="btn btn--primary" :disabled="!canReplace" @click="replaceSelection">
          {{ t('compose.ai.replace') }}
        </button>
      </template>
      <button v-else type="button" class="btn btn--primary" :disabled="!canGenerate" @click="generate">
        {{ t('compose.ai.generate') }}
      </button>
    </div>
  </UiDialog>
</template>

<style scoped>
.polish-dlg {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.polish-alert {
  margin: 0;
}
.polish-state {
  margin: 0;
  color: var(--color-text-2);
  font-size: 13px;
}
.polish-textarea {
  width: 100%;
  resize: vertical;
  min-height: 96px;
  font: inherit;
  font-size: 13px;
  line-height: 1.55;
}
.polish-textarea--sm {
  min-height: 56px;
}
.polish-model-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.polish-model-field {
  min-width: 0;
}
@media (max-width: 640px) {
  .polish-model-row {
    grid-template-columns: 1fr;
  }
}
</style>
