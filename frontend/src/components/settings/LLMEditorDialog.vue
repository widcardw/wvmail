<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Dialog } from '@ark-ui/vue/dialog'
import { Field } from '@ark-ui/vue/field'
import { useI18n } from '~/i18n'
import BusyButton from '~/components/BusyButton.vue'
import UiSelect from '~/components/UiSelect.vue'
import UiPasswordInput from '~/components/UiPasswordInput.vue'
import UiNumberInput from '~/components/UiNumberInput.vue'
import { getLLMProviderLimits, LLM_PROVIDER_DEFAULTS } from '~/types/config'
import type { LLMConfig, LLMProvider } from '~/types/config'
import JsonResponseViewer from '~/components/settings/JsonResponseViewer.vue'
import LLMModelPicker from '~/components/settings/LLMModelPicker.vue'

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
  /** Model the last chat test used, so the result names what was tested. */
  testChatModel: string
}>()

const form = defineModel<LLMConfig>({ required: true })
const emit = defineEmits<{
  'update:open': [open: boolean]
  test: [kind: 'conn' | 'chat', model?: string]
  /** The tested model changed: drop the previous response, it belongs to it. */
  clearChat: []
  save: []
}>()

const { t } = useI18n()

const canSave = computed(() =>
  Boolean(form.value.Name && form.value.BaseURL && (!props.apiKeyRequired || form.value.APIKey)),
)
const canConn = computed(() =>
  Boolean(form.value.BaseURL && (!props.apiKeyRequired || form.value.APIKey)),
)

function onProvider(provider: string) {
  form.value.Provider = provider as LLMProvider
  if (!props.editing && LLM_PROVIDER_DEFAULTS[provider as LLMProvider]) {
    form.value.BaseURL = LLM_PROVIDER_DEFAULTS[provider as LLMProvider]
  }
  if (!props.editing) {
    const limits = getLLMProviderLimits(provider)
    form.value.ContextWindowTokens = limits.context
    form.value.MaxOutputTokens = limits.output
  }
}

// Ark's field controls are controlled: `Field.Input` emits the string value,
// the UiNumberInput wrapper a number, and UiPasswordInput a DOM event.
function setText(key: 'Name' | 'BaseURL', value: unknown) {
  form.value[key] = String(value ?? '')
}

function onApiKey(event: Event) {
  form.value.APIKey = (event.target as HTMLInputElement).value
}

function onNumber(key: 'ContextWindowTokens' | 'MaxOutputTokens', value: number) {
  form.value[key] = (Number.isFinite(value) ? value : 0) as never
}

function close() {
  emit('update:open', false)
}

/** Model set panel: fetching it is optional, typing a model name always works. */
const pickerOpen = ref(false)
/** Name currently being typed into the "add model" box. */
const typedModel = ref('')

/**
 * Every model this config can use. Configs saved before the catalog feature only
 * carry the single `Model`, which is shown as the initial entry.
 */
const selectedModels = computed(() => {
  const models = form.value.Models ?? []
  if (models.length > 0) return models
  return form.value.Model ? [form.value.Model] : []
})

/**
 * `Model` is kept in sync with the first entry: callers that don't pick a model
 * explicitly (the chat test, translations without a chosen model) keep working.
 */
function setModels(models: string[]) {
  form.value.Models = models
  form.value.Model = models[0] ?? ''
}

function addTypedModel() {
  const name = typedModel.value.trim()
  if (!name) return
  if (!selectedModels.value.includes(name)) setModels([...selectedModels.value, name])
  typedModel.value = ''
}

function removeModel(model: string) {
  setModels(selectedModels.value.filter((m) => m !== model))
}

function onModelsPicked(models: string[]) {
  setModels(models)
  typedModel.value = ''
  pickerOpen.value = false
}

/**
 * Which of the ticked models the chat test sends to. The connection test needs no
 * model at all (it probes the catalog), so only the chat test is model-bound.
 */
const testModel = ref('')
const testModelOptions = computed(() =>
  selectedModels.value.map((model) => ({ label: model, value: model })),
)
// Keep the choice inside the ticked set: removing the tested model falls back to
// the first remaining one ('' when nothing is ticked, which disables the button).
watch(
  testModelOptions,
  (options) => {
    if (!options.some((option) => option.value === testModel.value)) {
      testModel.value = options[0]?.value ?? ''
    }
  },
  { immediate: true },
)

const canChat = computed(() =>
  Boolean(testModel.value && form.value.BaseURL && (!props.apiKeyRequired || form.value.APIKey)),
)

// A chat response belongs to the model that produced it, so switching the target
// clears the previous result instead of leaving a stale body on screen (two models
// often fail with byte-identical error JSON, which looks like "nothing updated").
watch(testModel, () => emit('clearChat'))

// "Test connection" doubles as the entry point for picking models: a successful
// connection test reveals the catalog panel, which fetches the list itself.
watch(
  () => props.testOk,
  (ok) => {
    if (ok && !pickerOpen.value) pickerOpen.value = true
  },
)
</script>

<template>
  <Dialog.Root
    :open="props.open"
    lazy-mount
    unmount-on-exit
    @update:open="emit('update:open', $event)"
  >
    <Teleport to="body">
      <Dialog.Backdrop class="dlg-back" />
      <Dialog.Positioner class="dlg-pos dlg-pos--top">
        <Dialog.Content class="dlg-card dlg-card--wide dlg-card--flush">
          <header class="dlg-head">
            <Dialog.Title class="dlg-title">
              {{ props.editing ? t['settings.llm.edit'] : t['settings.llm.add'] }}
            </Dialog.Title>
            <Dialog.CloseTrigger
              type="button"
              class="dlg-close"
              :aria-label="t['settings.accounts.cancel']"
              @click="close"
            >
              <i class="i-ri:close-line" aria-hidden="true" />
            </Dialog.CloseTrigger>
          </header>

          <div class="dlg-body">
            <Field.Root class="field">
              <Field.Label class="field__label">{{ t['settings.llm.provider'] }}</Field.Label>
              <UiSelect
                :options="props.providerOptions"
                :model-value="form.Provider"
                width="100%"
                @update:model-value="onProvider"
              />
            </Field.Root>
            <Field.Root class="field">
              <Field.Label class="field__label">{{ t['settings.llm.name'] }}</Field.Label>
              <Field.Input
                class="ctl"
                :model-value="form.Name"
                placeholder="My LLM"
                @update:model-value="setText('Name', $event)"
              />
            </Field.Root>
            <Field.Root class="field">
              <Field.Label class="field__label">{{ t['settings.llm.baseURL'] }}</Field.Label>
              <Field.Input
                class="ctl"
                :model-value="form.BaseURL"
                placeholder="https://api.example.com/v1"
                @update:model-value="setText('BaseURL', $event)"
              />
            </Field.Root>
            <Field.Root class="field">
              <Field.Label class="field__label">
                {{ t['settings.llm.apiKey'] }}<span v-if="!props.apiKeyRequired"> (optional)</span>
              </Field.Label>
              <UiPasswordInput
                class="ctl"
                :value="form.APIKey"
                :placeholder="props.apiKeyRequired ? 'sk-...' : 'Optional for local Ollama'"
                @input="onApiKey"
              />
            </Field.Root>
            <div class="limits">
              <Field.Root class="field">
                <Field.Label class="field__label">{{
                  t['settings.llm.contextWindow']
                }}</Field.Label>
                <UiNumberInput
                  :value="form.ContextWindowTokens"
                  :min="1024"
                  :step="1024"
                  @value="onNumber('ContextWindowTokens', $event)"
                />
              </Field.Root>
              <Field.Root class="field">
                <Field.Label class="field__label">{{ t['settings.llm.maxOutput'] }}</Field.Label>
                <UiNumberInput
                  :value="form.MaxOutputTokens"
                  :min="256"
                  :step="256"
                  @value="onNumber('MaxOutputTokens', $event)"
                />
              </Field.Root>
            </div>

            <!-- Model set lives after the connection fields: you need a working
               base URL + key before fetching the catalog, and the test section
               right below consumes the ticked models. -->
            <Field.Root class="field">
              <Field.Label class="field__label">{{ t['settings.llm.models'] }}</Field.Label>
              <div class="model-row">
                <Field.Input
                  v-model="typedModel"
                  class="ctl"
                  placeholder="gpt-4o, claude-sonnet-4-20250514, ..."
                  @change="addTypedModel()"
                />
                <button
                  type="button"
                  class="btn btn--sm"
                  :disabled="!typedModel.trim()"
                  @click="addTypedModel()"
                >
                  {{ t['settings.llm.modelAdd'] }}
                </button>
                <button
                  type="button"
                  class="btn btn--sm"
                  :disabled="!canConn"
                  :title="canConn ? undefined : t['settings.llm.fetchModelsNeeds']"
                  @click="pickerOpen = !pickerOpen"
                >
                  {{ t['settings.llm.fetchModels'] }}
                </button>
              </div>
              <Field.HelperText class="field__hint">
                {{ t['settings.llm.modelsHint'] }}
              </Field.HelperText>
              <div v-if="selectedModels.length > 0" class="model-tags">
                <span v-for="model in selectedModels" :key="model" class="model-tag" :title="model">
                  <span class="model-tag__text">{{ model }}</span>
                  <button
                    type="button"
                    class="model-tag__remove"
                    :aria-label="`${t['settings.llm.delete']} ${model}`"
                    @click="removeModel(model)"
                  >
                    ×
                  </button>
                </span>
              </div>
              <LLMModelPicker
                v-if="pickerOpen"
                :config="form"
                :selected="selectedModels"
                @confirm="onModelsPicked"
                @cancel="pickerOpen = false"
              />
            </Field.Root>

            <Field.Root class="field">
              <Field.Label class="field__label">{{ t['settings.llm.test_model'] }}</Field.Label>
              <div class="test-actions">
                <UiSelect
                  :options="testModelOptions"
                  :model-value="testModel"
                  :disabled="testModelOptions.length === 0"
                  :placeholder="t['settings.llm.test_no_model']"
                  @update:model-value="testModel = $event"
                  width="300px"
                />
                <BusyButton
                  class="btn"
                  :label="t['settings.llm.test_connection']"
                  :busy="props.testing"
                  :disabled="!canConn"
                  @click="emit('test', 'conn')"
                />
                <BusyButton
                  class="btn"
                  :label="t['settings.llm.test_chat']"
                  :busy="props.testingChat"
                  :disabled="!canChat"
                  @click="emit('test', 'chat', testModel)"
                />
              </div>
              <Field.HelperText class="field__hint">
                {{ t['settings.llm.test_hint'] }}
              </Field.HelperText>
            </Field.Root>

            <div v-if="props.testSummary" class="result" :class="props.testOk ? 'ok' : 'bad'">
              <strong>{{ t['settings.llm.test_connection'] }}: {{ props.testSummary }}</strong>
              <JsonResponseViewer v-if="props.testBody" :body="props.testBody" />
            </div>
            <div
              v-if="props.testChatSummary"
              class="result"
              :class="props.testChatOk ? 'ok' : 'bad'"
            >
              <strong>
                {{ t['settings.llm.test_chat'] }}
                <template v-if="props.testChatModel">· {{ props.testChatModel }}</template
                >:
                {{ props.testChatSummary }}
              </strong>
              <JsonResponseViewer v-if="props.testChatBody" :body="props.testChatBody" />
            </div>
          </div>

          <footer class="dlg-foot">
            <button type="button" class="btn" @click="close">
              {{ t['settings.llm.cancel'] }}
            </button>
            <BusyButton
              class="btn btn--primary"
              :label="t['settings.llm.save']"
              :busy="props.saving"
              :disabled="!canSave"
              @click="emit('save')"
            />
          </footer>
        </Dialog.Content>
      </Dialog.Positioner>
    </Teleport>
  </Dialog.Root>
</template>

<style scoped>
/* `.field` / `.field__label` / `.field__hint` come from ui.css. */
.ctl {
  width: 100%;
}
.pass {
  letter-spacing: 1px;
}
.limits {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.test-actions {
  display: flex;
  gap: 8px;
  margin-bottom: 6px;
}
.model-row {
  display: flex;
  gap: 6px;
}
.model-row .ctl {
  flex: 1;
  min-width: 0;
}
/* Shared tag styles live in ui.css; only the spacing is local. */
.model-tags {
  margin-top: 6px;
}
.result {
  margin-top: 8px;
  padding: 8px 10px;
  font-size: 13px;
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
}
/* Same treatment as the list's test button: matching text, border and tint. The
   embedded JSON tree keeps its own syntax colours. */
.result.ok {
  color: var(--color-success);
  background: var(--color-success-bg);
  border-color: var(--color-success);
}
.result.bad {
  color: var(--color-danger);
  background: var(--color-danger-bg);
  border-color: var(--color-danger);
}
</style>
