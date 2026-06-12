<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import {
  NCard,
  NForm,
  NFormItem,
  NSelect,
  NInput,
  NButton,
  NSpace,
  NAlert,
  useMessage,
  type SelectOption,
} from 'naive-ui'
import { useI18n } from '../../i18n'
import { ConfigService } from '../../../bindings/wvmail/services'
import type { LLMConfig } from '../../types/config'

const { t } = useI18n()
const message = useMessage()

const targetLangOptions: SelectOption[] = [
  { label: '中文', value: 'zh-CN' },
  { label: 'English', value: 'en' },
  { label: '日本語', value: 'ja' },
  { label: '한국어', value: 'ko' },
  { label: 'Français', value: 'fr' },
  { label: 'Deutsch', value: 'de' },
  { label: 'Español', value: 'es' },
  { label: 'Русский', value: 'ru' },
]

const engineOptions: SelectOption[] = [
  { label: t.value['settings.translation.engine.llm'], value: 'llm' },
  { label: t.value['settings.translation.engine.deepl'], value: 'deepl' },
]

const llmConfigs = ref<LLMConfig[]>([])
const llmOptions = computed<SelectOption[]>(() =>
  llmConfigs.value.map((c) => ({ label: c.Name, value: c.Name })),
)

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
    if (llmConfigs.value.length > 0) {
      form.value.LLMConfigName = llmConfigs.value[0].Name
    }
  } catch (e) {
    console.error('Failed to load config', e)
  }
})

async function handleSave() {
  saving.value = true
  try {
    await ConfigService.SaveTranslationConfig({
      DefaultTargetLang: form.value.DefaultTargetLang,
      DefaultEngine: form.value.DefaultEngine,
      DeepLAuthKey: form.value.DeepLAuthKey,
    })
    message.success(t.value['settings.translation.saved'])
  } catch (e) {
    message.error(String(e))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div style="padding: 16px; max-width: 720px">
    <NCard :title="t['settings.translation.title']" size="small">
      <NAlert v-if="llmConfigs.length === 0 && form.DefaultEngine === 'llm'" type="warning" style="margin-bottom: 12px">
        No LLM configurations found. Please add one in the LLM settings page first.
      </NAlert>

      <NForm label-placement="left" label-width="140">
        <NFormItem :label="t['settings.translation.target_lang']">
          <NSelect
            v-model:value="form.DefaultTargetLang"
            :options="targetLangOptions"
            style="width: 200px"
          />
        </NFormItem>
        <NFormItem :label="t['settings.translation.engine']">
          <NSelect
            v-model:value="form.DefaultEngine"
            :options="engineOptions"
            style="width: 160px"
          />
        </NFormItem>

        <template v-if="form.DefaultEngine === 'llm'">
          <NFormItem :label="t['settings.translation.llm_config']">
            <NSelect
              v-model:value="form.LLMConfigName"
              :options="llmOptions"
              style="width: 240px"
              :disabled="llmConfigs.length === 0"
            />
          </NFormItem>
        </template>

        <template v-if="form.DefaultEngine === 'deepl'">
          <NFormItem :label="t['settings.translation.deepl_key']">
            <NInput
              v-model:value="form.DeepLAuthKey"
              type="password"
              show-password-on="click"
              placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
              style="width: 320px"
            />
          </NFormItem>
        </template>
      </NForm>

      <NSpace justify="end" style="margin-top: 16px">
        <NButton type="primary" :loading="saving" @click="handleSave">
          {{ t['settings.llm.save'] }}
        </NButton>
      </NSpace>
    </NCard>
  </div>
</template>
