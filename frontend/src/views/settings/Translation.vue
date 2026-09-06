<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from '~/i18n'
import UiSelect from '~/components/UiSelect.vue'
import { useFeedbackStore } from '~/stores/feedback'
import { ConfigService } from '#/wvmail/services'
import type { LLMConfig } from '~/types/config'

const { t } = useI18n()
const fb = useFeedbackStore()

const targetLangOptions = [
  { label: '中文', value: 'zh-CN' },
  { label: 'English', value: 'en' },
  { label: '日本語', value: 'ja' },
  { label: '한국어', value: 'ko' },
  { label: 'Français', value: 'fr' },
  { label: 'Deutsch', value: 'de' },
  { label: 'Español', value: 'es' },
  { label: 'Русский', value: 'ru' },
]

const engineOptions = [
  { label: t.value['settings.translation.engine.llm'], value: 'llm' },
  { label: t.value['settings.translation.engine.deepl'], value: 'deepl' },
]

const llmConfigs = ref<LLMConfig[]>([])
const llmOptions = computed(() => llmConfigs.value.map((c) => ({ label: c.Name, value: c.Name })))

const form = ref({
  DefaultTargetLang: 'zh-CN',
  DefaultEngine: 'llm',
  DeepLAuthKey: '',
  LLMConfigName: '',
})
const saving = ref(false)

onMounted(async () => {
  try {
    const cfg = await ConfigService.GetConfig()
    if (cfg.TranslationConfig) {
      form.value.DefaultTargetLang = cfg.TranslationConfig.DefaultTargetLang || 'zh-CN'
      form.value.DefaultEngine = cfg.TranslationConfig.DefaultEngine || 'llm'
      form.value.DeepLAuthKey = cfg.TranslationConfig.DeepLAuthKey || ''
    }
    llmConfigs.value = cfg.LLMConfigs || []
    if (llmConfigs.value.length > 0) form.value.LLMConfigName = llmConfigs.value[0].Name
  } catch (e) {
    console.error('Failed to load config', e)
  }
})

function onSelect(key: string, v: string) {
  form.value[key as keyof typeof form.value] = v as never
}

async function handleSave() {
  saving.value = true
  try {
    await ConfigService.SaveTranslationConfig({
      DefaultTargetLang: form.value.DefaultTargetLang,
      DefaultEngine: form.value.DefaultEngine,
      DeepLAuthKey: form.value.DeepLAuthKey,
    })
    fb.success(t.value['settings.translation.saved'])
  } catch (e) {
    fb.error(String(e))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="wrap">
    <section class="surface">
      <header class="surface__head">
        <h2>{{ t['settings.translation.title'] }}</h2>
      </header>

      <div v-if="llmConfigs.length === 0 && form.DefaultEngine === 'llm'" class="warning">
        No LLM configurations found. Add one in the LLM settings first.
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
          width="180px"
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
            @update:model-value="(v) => onSelect('LLMConfigName', v)"
          />
        </div>
      </template>

      <template v-else-if="form.DefaultEngine === 'deepl'">
        <div class="row">
          <label class="row__label">{{ t['settings.translation.deepl_key'] }}</label>
          <input
            type="password"
            class="ctl password"
            v-model="form.DeepLAuthKey"
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
          />
        </div>
      </template>

      <div class="actions">
        <button type="button" class="btn btn--primary" :disabled="saving" @click="handleSave">
          {{ saving ? '…' : t['settings.llm.save'] }}
        </button>
      </div>
    </section>
  </div>
</template>

<style scoped>
.wrap {
  padding: 16px;
  max-width: 720px;
  color: var(--n-text-color);
}
.surface {
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-md);
}
.surface__head {
  padding: 10px 16px;
  border-bottom: 1px solid var(--n-border-color);
}
.surface__head h2 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
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
.row__label {
  width: 220px;
  flex-shrink: 0;
  font-size: 13px;
  color: var(--color-text-2);
}
.ctl {
  width: 220px;
  height: 28px;
  padding: 0 8px;
  font-size: 13px;
  color: var(--n-text-color);
  background: var(--n-action-color);
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
}
.ctl--w160 {
  width: 160px;
}
.ctl:focus {
  border-color: var(--n-primary-color);
  outline: none;
}
.password {
  width: 320px;
}
.actions {
  display: flex;
  justify-content: flex-end;
  padding: 6px 18px 16px;
}
.btn--primary {
  padding: 5px 12px;
  font-size: 13px;
  color: #fff;
  background: var(--n-primary-color);
  border: none;
  border-radius: 6px;
  cursor: pointer;
}
.btn--primary:disabled {
  opacity: 0.6;
}
</style>
