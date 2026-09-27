<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from '~/i18n'
import UiSelect from '~/components/UiSelect.vue'
import { useAutoPersist } from '~/composables/use-autopersist'
import { useFeedbackStore } from '~/stores/feedback'
import { ConfigService } from '#/wvmail/services'
import type { LLMConfig } from '~/types/config'

const { t } = useI18n()
const fb = useFeedbackStore()

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

/** Models offered by one config: its picked set, or the single legacy `Model`. */
function modelListFor(configName: string): string[] {
  const cfg = llmConfigs.value.find((c) => c.Name === configName)
  if (!cfg) return []
  const models = cfg.Models ?? []
  if (models.length > 0) return models
  return cfg.Model ? [cfg.Model] : []
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
  try {
    await ConfigService.SaveTranslationConfig({
      DefaultTargetLang: form.value.DefaultTargetLang,
      DefaultEngine: form.value.DefaultEngine,
      LLMConfigName: form.value.LLMConfigName,
      LLMModel: form.value.LLMModel,
      DeepLAuthKey: form.value.DeepLAuthKey,
    })
    fb.success(t.value['settings.translation.saved'])
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
</script>

<template>
  <div class="wrap">
    <section class="surface">
      <header class="surface__head">
        <h2>{{ t['settings.translation.title'] }}</h2>
      </header>

      <div v-if="llmConfigs.length === 0 && form.DefaultEngine === 'llm'" class="warning">
        {{ t['settings.translation.no_llm'] }}
      </div>

      <div class="row">
        <label class="row__label">{{ t['settings.translation.target_lang'] }}</label>
        <UiSelect
          :options="targetLangOptions"
          :model-value="form.DefaultTargetLang"
          @update:model-value="(v) => onSelect('DefaultTargetLang', v)"
        />
      </div>

      <div class="row">
        <label class="row__label">{{ t['settings.translation.engine'] }}</label>
        <UiSelect
          :options="engineOptions"
          :model-value="form.DefaultEngine"
          @update:model-value="(v) => onSelect('DefaultEngine', v)"
        />
      </div>

      <template v-if="form.DefaultEngine === 'llm'">
        <div class="row">
          <label class="row__label">{{ t['settings.translation.llm_config'] }}</label>
          <UiSelect
            :options="llmOptions"
            :model-value="form.LLMConfigName"
            :disabled="llmConfigs.length === 0"
            @update:model-value="onSelectConfig"
          />
        </div>

        <div v-if="modelOptions.length > 0" class="row">
          <label class="row__label">{{ t['settings.translation.llm_model'] }}</label>
          <UiSelect
            :options="modelOptions"
            :model-value="form.LLMModel"
            @update:model-value="(v) => onSelect('LLMModel', v)"
          />
        </div>
      </template>

      <template v-else-if="form.DefaultEngine === 'deepl'">
        <div class="row">
          <label class="row__label">{{ t['settings.translation.deepl_key'] }}</label>
          <input
            v-model="form.DeepLAuthKey"
            type="password"
            class="ctl password"
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
            @change="persistNow()"
          />
        </div>
      </template>
    </section>
  </div>
</template>

<style scoped>
.wrap {
  padding: 16px;
  max-width: 800px;
  color: var(--n-text-color);
}
.warning {
  margin: 14px 18px 0;
  padding: 8px 12px;
  font-size: 13px;
  color: var(--n-text-color);
  background: rgba(240, 160, 32, 0.15);
  border-left: 3px solid #f0a020;
  border-radius: 4px;
}
.row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 18px;
}
.ctl {
  width: 220px;
}
.ctl--w160 {
  width: 160px;
}
.password {
  width: 320px;
}
</style>
