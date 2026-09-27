<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from '~/i18n'
import BusyButton from '~/components/BusyButton.vue'
import UiSelect from '~/components/UiSelect.vue'
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

function onInput(
  key: 'Name' | 'BaseURL' | 'APIKey' | 'ContextWindowTokens' | 'MaxOutputTokens',
  event: Event,
) {
  if (key === 'ContextWindowTokens' || key === 'MaxOutputTokens') {
    form.value[key] = Number((event.target as HTMLInputElement).value) as never
    return
  }
  form.value[key] = (event.target as HTMLInputElement).value
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
  <Teleport to="body">
    <div v-if="props.open" class="overlay">
      <div class="modal">
        <header class="modal__head">
          <h3>{{ props.editing ? t['settings.llm.edit'] : t['settings.llm.add'] }}</h3>
          <button
            type="button"
            class="btn--ghost"
            :aria-label="t['settings.accounts.cancel']"
            @click="close"
          >
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
          <div class="limits">
            <div class="field">
              <label>{{ t['settings.llm.contextWindow'] }}</label>
              <input
                class="ctl"
                type="number"
                min="1024"
                step="1024"
                :value="form.ContextWindowTokens"
                @input="onInput('ContextWindowTokens', $event)"
              />
            </div>
            <div class="field">
              <label>{{ t['settings.llm.maxOutput'] }}</label>
              <input
                class="ctl"
                type="number"
                min="256"
                step="256"
                :value="form.MaxOutputTokens"
                @input="onInput('MaxOutputTokens', $event)"
              />
            </div>
          </div>

          <!-- Model set lives after the connection fields: you need a working
               base URL + key before fetching the catalog, and the test section
               right below consumes the ticked models. -->
          <div class="field">
            <label>{{ t['settings.llm.models'] }}</label>
            <div class="model-row">
              <input
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
            <p class="field__hint">{{ t['settings.llm.modelsHint'] }}</p>
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
          </div>

          <div class="field">
            <label>{{ t['settings.llm.test_model'] }}</label>
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
            <p class="field__hint">{{ t['settings.llm.test_hint'] }}</p>
          </div>

          <div v-if="props.testSummary" class="result" :class="props.testOk ? 'ok' : 'bad'">
            <strong>{{ t['settings.llm.test_connection'] }}: {{ props.testSummary }}</strong>
            <JsonResponseViewer v-if="props.testBody" :body="props.testBody" />
          </div>
          <div v-if="props.testChatSummary" class="result" :class="props.testChatOk ? 'ok' : 'bad'">
            <strong>
              {{ t['settings.llm.test_chat'] }}
              <template v-if="props.testChatModel">· {{ props.testChatModel }}</template
              >:
              {{ props.testChatSummary }}
            </strong>
            <JsonResponseViewer v-if="props.testChatBody" :body="props.testChatBody" />
          </div>
        </div>

        <footer class="modal__foot">
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
.field__hint {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--color-text-3);
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
