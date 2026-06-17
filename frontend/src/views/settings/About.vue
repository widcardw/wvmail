<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  NCard,
  NForm,
  NFormItem,
  NSelect,
  NInputNumber,
  NButton,
  NSpace,
  NDivider,
  useMessage,
} from 'naive-ui'
import { useI18n } from '~/i18n'
import { ConfigService, OsService } from '#/wvmail/services'

const { t } = useI18n()
const message = useMessage()

const logConfig = ref({
  Level: 'info',
  RetentionDays: 30,
  MaxSizeMB: 10,
})

const logDir = ref('')
const saving = ref(false)

const levelOptions = [
  { label: t.value['settings.about.log.level.debug'], value: 'debug' },
  { label: t.value['settings.about.log.level.info'], value: 'info' },
  { label: t.value['settings.about.log.level.warn'], value: 'warn' },
  { label: t.value['settings.about.log.level.error'], value: 'error' },
]

onMounted(async () => {
  try {
    const cfg = await ConfigService.GetConfig()
    if (cfg.LogConfig) {
      logConfig.value = { ...logConfig.value, ...cfg.LogConfig }
    }
  } catch (e) {
    console.error('Failed to load config', e)
  }

  try {
    logDir.value = await ConfigService.GetLogDir()
  } catch (e) {
    console.error('Failed to get log dir', e)
  }
})

async function handleSave() {
  saving.value = true
  try {
    await ConfigService.SaveLogConfig(logConfig.value)
    message.success(t.value['settings.about.log.saved'])
  } catch (e) {
    message.error(String(e))
  } finally {
    saving.value = false
  }
}

async function handleOpenLogFolder() {
  if (!logDir.value) return
  try {
    await OsService.OpenFolder(logDir.value)
  } catch (e) {
    console.error('Failed to open log folder', e)
  }
}
</script>

<template>
  <div style="padding: 16px; max-width: 720px">
    <NCard :title="t['settings.about']" size="small">
      <p style="opacity: 0.6">
        wvmail — Cross-platform desktop email client built with Wails3 + Vue 3
      </p>
      <p style="opacity: 0.4; font-size: 12px; margin-top: 8px">Version 0.0.1</p>

      <NDivider>{{ t['settings.about.log.title'] }}</NDivider>

      <NForm label-placement="left" label-width="140">
        <NFormItem :label="t['settings.about.log.level']">
          <NSelect v-model:value="logConfig.Level" :options="levelOptions" style="width: 160px" />
        </NFormItem>
        <NFormItem :label="t['settings.about.log.retention']">
          <NInputNumber
            v-model:value="logConfig.RetentionDays"
            :min="1"
            :max="365"
            style="width: 120px"
          />
        </NFormItem>
        <NFormItem :label="t['settings.about.log.max_size']">
          <NInputNumber
            v-model:value="logConfig.MaxSizeMB"
            :min="1"
            :max="1024"
            style="width: 120px"
          />
        </NFormItem>
      </NForm>

      <NSpace justify="space-between" style="margin-top: 8px">
        <NButton @click="handleOpenLogFolder">
          {{ t['settings.about.log.open_folder'] }}
        </NButton>
        <NButton type="primary" :loading="saving" @click="handleSave">
          {{ t['settings.llm.save'] }}
        </NButton>
      </NSpace>
    </NCard>
  </div>
</template>
