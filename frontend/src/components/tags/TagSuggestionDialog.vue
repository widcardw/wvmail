<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ConfigService, TagService } from '#/wvmail/services'
import type { EmailAccount } from '#/wvmail/services'
import { useI18n } from '~/i18n'
import { useFeedbackStore } from '~/stores/feedback'
import { useTagsStore } from '~/stores/tags'
import { llmModels } from '~/composables/llm-models'
import type { LLMConfig } from '~/types/config'
import type {
  TagMatchField,
  TagSuggestion,
  TagSuggestionPreview,
  TagSuggestionRequest,
} from '~/types/tag'
import BusyButton from '~/components/ui/BusyButton.vue'
import UiCheckbox from '~/components/ui/UiCheckbox.vue'
import UiDialog from '~/components/ui/UiDialog.vue'
import UiDialogHeader from '~/components/ui/UiDialogHeader.vue'
import UiNumberInput from '~/components/ui/UiNumberInput.vue'
import UiSelect from '~/components/ui/UiSelect.vue'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [open: boolean] }>()

const { t, tf } = useI18n()
const feedback = useFeedbackStore()
const tagsStore = useTagsStore()
const configs = ref<LLMConfig[]>([])
const accounts = ref<EmailAccount[]>([])
const selectedConfigName = ref('')
const selectedModel = ref('')
const selectedAccountIDs = ref<string[]>([])
const sampleLimit = ref(120)
const optionalFields = ref<string[]>([])
const preview = ref<TagSuggestionPreview | null>(null)
const suggestions = ref<Array<TagSuggestion & { selected: boolean }>>([])
const previewing = ref(false)
const generating = ref(false)
const saving = ref(false)
const loadingConfig = ref(false)
const error = ref('')

const optionalFieldOptions = [
  'from_name',
  'from_address',
  'domain',
  'date',
  'body_excerpt',
] as const

const configOptions = computed(() =>
  configs.value.map((config) => ({ label: config.Name, value: config.Name })),
)
const selectedConfig = computed(
  () => configs.value.find((config) => config.Name === selectedConfigName.value) ?? null,
)
const modelOptions = computed(() =>
  llmModels(selectedConfig.value ?? undefined).map((model) => ({ label: model, value: model })),
)
const canPreview = computed(
  () =>
    Boolean(selectedConfig.value && selectedModel.value) &&
    selectedAccountIDs.value.length > 0 &&
    sampleLimit.value > 0,
)
const selectedSuggestionCount = computed(
  () => suggestions.value.filter((suggestion) => suggestion.selected).length,
)

watch(
  () => props.open,
  async (open) => {
    if (!open) return
    reset()
    loadingConfig.value = true
    try {
      const config = await ConfigService.GetConfig()
      configs.value = (config.LLMConfigs ?? []) as LLMConfig[]
      accounts.value = (config.EmailAccounts ?? []) as EmailAccount[]
      selectedAccountIDs.value = accounts.value.map((account) => account.ID)
      selectedConfigName.value = configs.value[0]?.Name ?? ''
      selectedModel.value = llmModels(configs.value[0])[0] ?? ''
    } catch (e) {
      console.error('Failed to load AI tag suggestion configuration', e)
    } finally {
      loadingConfig.value = false
    }
  },
)

watch(selectedConfigName, () => {
  selectedModel.value = llmModels(selectedConfig.value ?? undefined)[0] ?? ''
  invalidatePreview()
})

watch([selectedModel, selectedAccountIDs, sampleLimit, optionalFields], () => invalidatePreview(), {
  deep: true,
})

function reset() {
  preview.value = null
  suggestions.value = []
  error.value = ''
  optionalFields.value = []
  sampleLimit.value = 120
}

function invalidatePreview() {
  preview.value = null
  suggestions.value = []
  error.value = ''
}

function toggleAccount(id: string, checked: boolean) {
  selectedAccountIDs.value = checked
    ? [...new Set([...selectedAccountIDs.value, id])]
    : selectedAccountIDs.value.filter((value) => value !== id)
}

function toggleField(field: string, checked: boolean) {
  optionalFields.value = checked
    ? [...new Set([...optionalFields.value, field])]
    : optionalFields.value.filter((value) => value !== field)
}

function request(): TagSuggestionRequest {
  return {
    account_ids: [...selectedAccountIDs.value],
    sample_limit: sampleLimit.value,
    fields: ['subject', ...optionalFields.value],
    llm_config_name: selectedConfigName.value,
    llm_model: selectedModel.value,
  }
}

async function loadPreview() {
  if (!canPreview.value || previewing.value) return
  previewing.value = true
  error.value = ''
  suggestions.value = []
  try {
    preview.value = await TagService.PreviewTagSuggestionPrompt(request())
  } catch (e) {
    error.value = String(e)
  } finally {
    previewing.value = false
  }
}

async function generate() {
  if (!preview.value || generating.value || preview.value.sample_count === 0) return
  generating.value = true
  error.value = ''
  try {
    const result = (await TagService.GenerateTagSuggestions(request())) as TagSuggestion[]
    suggestions.value = result.map((suggestion) => ({ ...suggestion, selected: true }))
  } catch (e) {
    error.value = String(e)
  } finally {
    generating.value = false
  }
}

async function saveSelected() {
  const chosen = suggestions.value.filter((suggestion) => suggestion.selected)
  if (chosen.length === 0 || saving.value) return
  saving.value = true
  let saved = 0
  let failed = 0
  for (const suggestion of chosen) {
    try {
      await TagService.SaveMailTag({
        id: '',
        label: suggestion.label,
        color: suggestion.color,
        keywords: suggestion.keywords,
        fields: suggestion.fields,
        enabled: true,
        created_at: 0,
        updated_at: 0,
      })
      saved++
    } catch (e) {
      failed++
      console.error('Failed to save an AI tag suggestion', e)
    }
  }
  await tagsStore.reload()
  saving.value = false
  if (saved > 0) feedback.success(tf('tags.ai.saveSuccess', { count: saved }))
  if (failed > 0) feedback.error(t.value['tags.ai.saveFailed'])
  if (saved > 0 && failed === 0) emit('update:open', false)
}

function fieldLabel(field: string): string {
  return t.value[`tags.ai.field.${field}` as keyof typeof t.value] ?? field
}

function matchFieldLabel(field: TagMatchField): string {
  return t.value[`tags.field.${field}` as keyof typeof t.value] ?? field
}
</script>

<template>
  <UiDialog
    :open="open"
    size="top"
    :busy="generating || saving"
    :close-on-interact-outside="false"
    @update:open="emit('update:open', $event)"
  >
    <UiDialogHeader
      :title="t['tags.ai.title']"
      :close-label="t['cancel']"
      :disabled="generating || saving"
      @close="emit('update:open', false)"
    />
    <div class="dlg-body suggestion-body">
      <p class="intro">{{ t['tags.ai.intro'] }}</p>
      <div class="privacy-warning">
        <i class="i-ri:shield-keyhole-line" aria-hidden="true" />
        <span>{{ t['tags.ai.warning'] }}</span>
      </div>

      <div v-if="loadingConfig" class="loading-row"><span class="spinner" /></div>
      <div v-else-if="configs.length === 0" class="no-config">
        {{ t['tags.ai.noLlm'] }}
      </div>
      <template v-else>
        <div class="settings-grid">
          <label class="field">
            <span class="field__label">{{ t['tags.ai.llmConfig'] }}</span>
            <UiSelect
              :options="configOptions"
              :model-value="selectedConfigName"
              width="100%"
              @update:model-value="selectedConfigName = $event"
            />
          </label>
          <label class="field">
            <span class="field__label">{{ t['tags.ai.model'] }}</span>
            <UiSelect
              :options="modelOptions"
              :model-value="selectedModel"
              width="100%"
              @update:model-value="selectedModel = $event"
            />
          </label>
          <label class="field">
            <span class="field__label">{{ t['tags.ai.sampleLimit'] }}</span>
            <UiNumberInput
              :value="sampleLimit"
              :min="1"
              :max="300"
              :step="10"
              width="100%"
              @value="sampleLimit = $event"
            />
          </label>
        </div>

        <section class="choice-section">
          <h3>{{ t['tags.ai.accounts'] }}</h3>
          <div class="choice-grid">
            <UiCheckbox
              v-for="account in accounts"
              :key="account.ID"
              :checked="selectedAccountIDs.includes(account.ID)"
              @update:checked="toggleAccount(account.ID, $event)"
            >
              {{ account.Name || account.Email }}
            </UiCheckbox>
          </div>
        </section>

        <section class="choice-section">
          <h3>{{ t['tags.ai.fields'] }}</h3>
          <div class="choice-grid">
            <UiCheckbox checked disabled>{{ t['tags.ai.field.subject'] }}</UiCheckbox>
            <UiCheckbox
              v-for="field in optionalFieldOptions"
              :key="field"
              :checked="optionalFields.includes(field)"
              @update:checked="toggleField(field, $event)"
            >
              {{ fieldLabel(field) }}
            </UiCheckbox>
          </div>
        </section>

        <div class="preview-action">
          <BusyButton
            class="btn"
            :label="t['tags.ai.preview']"
            :busy="previewing"
            :disabled="!canPreview"
            @click="loadPreview"
          />
        </div>

        <template v-if="preview">
          <div class="preview-meta">
            <span>{{ tf('tags.ai.samples', { count: preview.sample_count }) }}</span>
            <span v-if="preview.sample_from && preview.sample_to">
              {{
                tf('tags.ai.range', {
                  from: preview.sample_from,
                  to: preview.sample_to,
                })
              }}
            </span>
          </div>
          <p v-if="preview.sample_count === 0" class="no-config">
            {{ t['tags.ai.noSamples'] }}
          </p>
          <label class="field prompt-field">
            <span class="field__label">{{ t['tags.ai.prompt'] }}</span>
            <textarea class="input prompt-preview" readonly :value="preview.prompt" />
          </label>
          <div class="preview-action">
            <BusyButton
              class="btn btn--primary"
              :label="
                tf('tags.ai.send', {
                  provider: preview.llm_name || selectedConfigName,
                  model: preview.llm_model || selectedModel,
                })
              "
              :busy="generating"
              :disabled="preview.sample_count === 0"
              @click="generate"
            />
          </div>
        </template>

        <p v-if="error" class="suggestion-error">{{ error }}</p>

        <section v-if="suggestions.length" class="suggestion-results">
          <h3>{{ t['tags.ai.generated'] }}</h3>
          <div class="suggestion-list">
            <label v-for="suggestion in suggestions" :key="suggestion.label" class="suggestion">
              <input v-model="suggestion.selected" type="checkbox" />
              <span class="suggestion-color" :style="{ background: suggestion.color }" />
              <span class="suggestion-main">
                <strong>{{ suggestion.label }}</strong>
                <span>{{ suggestion.keywords.join(', ') }}</span>
                <small>{{ suggestion.fields.map(matchFieldLabel).join(' · ') }}</small>
              </span>
            </label>
          </div>
          <div class="preview-action">
            <button
              type="button"
              class="btn btn--primary"
              :disabled="saving || selectedSuggestionCount === 0"
              @click="saveSelected"
            >
              {{ t['tags.ai.saveSelected'] }}
              <span v-if="selectedSuggestionCount">({{ selectedSuggestionCount }})</span>
            </button>
          </div>
        </section>
      </template>
    </div>
  </UiDialog>
</template>

<style scoped>
.suggestion-body {
  padding: 18px 20px 22px;
}
.intro {
  margin: 0 0 10px;
  color: var(--color-text-2);
  font-size: 13px;
}
.privacy-warning {
  display: flex;
  gap: 9px;
  margin-bottom: 16px;
  padding: 10px 12px;
  color: var(--color-warning);
  background: var(--color-warning-bg);
  border: 1px solid color-mix(in srgb, var(--color-warning) 28%, transparent);
  border-radius: 7px;
  font-size: 12px;
  line-height: 1.55;
}
.privacy-warning i {
  flex: none;
  margin-top: 1px;
  font-size: 16px;
}
.settings-grid {
  display: grid;
  grid-template-columns: 1.1fr 1.3fr 0.7fr;
  gap: 12px;
}
.choice-section {
  margin: 4px 0 14px;
}
.choice-section h3,
.suggestion-results h3 {
  margin: 0 0 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-2);
}
.choice-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px 16px;
  font-size: 13px;
}
.preview-action {
  display: flex;
  justify-content: flex-end;
  margin: 10px 0;
}
.preview-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  margin-top: 12px;
  color: var(--color-text-3);
  font-size: 11px;
}
.prompt-preview {
  width: 100%;
  min-height: 220px;
  padding: 9px;
  resize: vertical;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  line-height: 1.5;
  white-space: pre;
}
.suggestion-error,
.no-config {
  margin: 10px 0;
  padding: 10px 12px;
  color: var(--color-danger);
  background: var(--color-danger-bg);
  border-radius: 6px;
  font-size: 12px;
}
.loading-row {
  display: flex;
  justify-content: center;
  padding: 28px;
}
.suggestion-results {
  margin-top: 16px;
  padding-top: 14px;
  border-top: 1px solid var(--n-border-color);
}
.suggestion-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.suggestion {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  padding: 9px;
  border: 1px solid var(--n-border-color);
  border-radius: 7px;
  cursor: pointer;
}
.suggestion:hover {
  background: var(--n-sidebar-hover);
}
.suggestion input {
  margin-top: 2px;
  accent-color: var(--n-primary-color);
}
.suggestion-color {
  width: 9px;
  height: 9px;
  flex: none;
  margin-top: 4px;
  border-radius: 999px;
}
.suggestion-main {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 2px;
  font-size: 12px;
}
.suggestion-main span {
  color: var(--color-text-2);
  word-break: break-word;
}
.suggestion-main small {
  color: var(--color-text-3);
}
@media (max-width: 760px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }
  .choice-grid {
    grid-template-columns: 1fr;
  }
}
</style>
