<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from '~/i18n'
import UiSelect from '~/components/ui/UiSelect.vue'
import UiPasswordInput from '~/components/ui/UiPasswordInput.vue'
import UiFieldRow from '~/components/ui/UiFieldRow.vue'
import UiAlert from '~/components/ui/UiAlert.vue'
import UiSwitch from '~/components/ui/UiSwitch.vue'
import { useAutoPersist } from '~/composables/use-autopersist'
import { useFeedbackStore } from '~/stores/feedback'
import { usePreferencesStore } from '~/stores/preferences'
import { ConfigService } from '#/wvmail/services'
import type { LLMConfig } from '~/types/config'
import { llmModels } from '~/composables/llm-models'

const { t } = useI18n()
const fb = useFeedbackStore()
const preferences = usePreferencesStore()

const targetLangOptions = computed(() => [
  { label: t.value['settings.translation.language.zh'], value: 'zh-CN' },
  { label: t.value['settings.translation.language.en'], value: 'en' },
])

const engineOptions = computed(() => [
  { label: t.value['settings.translation.engine.llm'], value: 'llm' },
  { label: t.value['settings.translation.engine.deepl'], value: 'deepl' },
])

const llmConfigs = ref<LLMConfig[]>([])
const llmOptions = computed(() => llmConfigs.value.map((c) => ({ label: c.Name, value: c.Name })))

const form = ref({
  DefaultTargetLang: 'zh-CN',
  DefaultEngine: 'llm',
  DeepLAuthKey: '',
  LLMConfigName: '',
  LLMModel: '',
})

function modelListFor(configName: string): string[] {
  return llmModels(llmConfigs.value.find((c) => c.Name === configName))
}

const modelOptions = computed(() =>
  modelListFor(form.value.LLMConfigName).map((model) => ({ label: model, value: model })),
)

onMounted(async () => {
  try {
    const cfg = await ConfigService.GetConfig()
    if (cfg.TranslationConfig) {
      const targetLang = cfg.TranslationConfig.DefaultTargetLang
      form.value.DefaultTargetLang = targetLang === 'en' ? 'en' : 'zh-CN'
      form.value.DefaultEngine = cfg.TranslationConfig.DefaultEngine || 'llm'
      form.value.DeepLAuthKey = cfg.TranslationConfig.DeepLAuthKey || ''
    }
    llmConfigs.value = cfg.LLMConfigs || []
    const savedName = cfg.TranslationConfig?.LLMConfigName
    if (savedName && llmConfigs.value.some((c) => c.Name === savedName)) {
      form.value.LLMConfigName = savedName
    } else if (llmConfigs.value.length > 0) {
      form.value.LLMConfigName = llmConfigs.value[0].Name
    }
    // A saved model can disappear when its config is edited, so fall back to the
    // config's own default instead of translating with a stale name.
    const savedModel = cfg.TranslationConfig?.LLMModel || ''
    const models = modelListFor(form.value.LLMConfigName)
    form.value.LLMModel = savedModel && models.includes(savedModel) ? savedModel : (models[0] ?? '')
  } catch (e) {
    console.error('Failed to load config', e)
  }
})

const persist = useAutoPersist()

function persistNow() {
  persist.schedule('translation', () => doPersist(), 250)
}
async function doPersist() {
  // Debounced auto-save: only failures toast, success is the visible norm.
  try {
    await ConfigService.SaveTranslationConfig({
      DefaultTargetLang: form.value.DefaultTargetLang,
      DefaultEngine: form.value.DefaultEngine,
      LLMConfigName: form.value.LLMConfigName,
      LLMModel: form.value.LLMModel,
      DeepLAuthKey: form.value.DeepLAuthKey,
    })
  } catch (e) {
    fb.error(String(e))
  }
}

function onSelect(key: string, v: string) {
  form.value[key as keyof typeof form.value] = v as never
  persistNow()
}

/** Switching config invalidates the chosen model, which belonged to the old one. */
function onSelectConfig(name: string) {
  form.value.LLMConfigName = name
  form.value.LLMModel = modelListFor(name)[0] ?? ''
  persistNow()
}

function onSkipConfirmationChange(checked: boolean) {
  void preferences.setSkipAIAssistConfirmation(checked)
}
</script>

<template>
  <div class="wrap">
    <section class="surface">
      <header class="surface__head">
        <h2>{{ t['settings.translation.title'] }}</h2>
      </header>

      <p class="hint">{{ t['settings.translation.ai_hint'] }}</p>

      <UiAlert
        v-if="llmConfigs.length === 0 && form.DefaultEngine === 'llm'"
        class="translation-warning"
        tone="warning"
      >
        {{ t['settings.translation.no_llm'] }}
      </UiAlert>

      <UiFieldRow :label="t['settings.translation.target_lang']">
        <UiSelect
          :options="targetLangOptions"
          :model-value="form.DefaultTargetLang"
          width="300px"
          @update:model-value="(v) => onSelect('DefaultTargetLang', v)"
        />
      </UiFieldRow>

      <UiFieldRow :label="t['settings.translation.engine']">
        <UiSelect
          :options="engineOptions"
          :model-value="form.DefaultEngine"
          width="300px"
          @update:model-value="(v) => onSelect('DefaultEngine', v)"
        />
      </UiFieldRow>

      <template v-if="form.DefaultEngine === 'llm'">
        <UiFieldRow :label="t['settings.translation.llm_config']">
          <UiSelect
            :options="llmOptions"
            :model-value="form.LLMConfigName"
            :disabled="llmConfigs.length === 0"
            width="300px"
            @update:model-value="onSelectConfig"
          />
        </UiFieldRow>

        <UiFieldRow v-if="modelOptions.length > 0" :label="t['settings.translation.llm_model']">
          <UiSelect
            :options="modelOptions"
            :model-value="form.LLMModel"
            width="300px"
            @update:model-value="(v) => onSelect('LLMModel', v)"
          />
        </UiFieldRow>
      </template>

      <template v-else-if="form.DefaultEngine === 'deepl'">
        <UiFieldRow :label="t['settings.translation.deepl_key']">
          <UiPasswordInput
            width="300px"
            :value="form.DeepLAuthKey"
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
            @input="
              (event: Event) => (form.DeepLAuthKey = (event.target as HTMLInputElement).value)
            "
          />
        </UiFieldRow>
      </template>
    </section>

    <section class="surface surface--spaced">
      <header class="surface__head">
        <h2>{{ t['settings.translation.confirm_title'] }}</h2>
      </header>

      <UiFieldRow
        :label="t['settings.translation.skip_confirm']"
        :hint="t['settings.translation.skip_confirm_hint']"
      >
        <UiSwitch
          :checked="preferences.skipAIAssistConfirmation"
          @update:checked="onSkipConfirmationChange"
        />
      </UiFieldRow>
    </section>
  </div>
</template>

<style scoped>
.translation-warning {
  margin: 14px 18px 0;
}
</style>
