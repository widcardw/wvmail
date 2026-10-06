<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Field } from '@ark-ui/vue/field'
import { useI18n } from '~/i18n'
import { useFeedbackStore } from '~/stores/feedback'
import { ConfigService, OsService } from '#/wvmail/services'
import UiSelect from '~/components/ui/UiSelect.vue'
import UiNumberInput from '~/components/ui/UiNumberInput.vue'
import UiSwitch from '~/components/ui/UiSwitch.vue'
import { useAutoPersist } from '~/composables/use-autopersist'

const { t } = useI18n()
const fb = useFeedbackStore()

const logConfig = ref({
  Level: 'info',
  RetentionDays: 30,
  MaxSizeMB: 10,
})
const logDir = ref('')
const saving = ref(false)
const scanExternalSafety = ref(true)

const levelOptions = [
  { label: t.value['settings.about.log.level.debug'], value: 'debug' },
  { label: t.value['settings.about.log.level.info'], value: 'info' },
  { label: t.value['settings.about.log.level.warn'], value: 'warn' },
  { label: t.value['settings.about.log.level.error'], value: 'error' },
]

onMounted(async () => {
  try {
    const cfg = await ConfigService.GetConfig()
    if (cfg.LogConfig) logConfig.value = { ...logConfig.value, ...cfg.LogConfig }
    if (cfg.ScanExternalSafety !== undefined) scanExternalSafety.value = cfg.ScanExternalSafety
  } catch (e) {
    console.error('Failed to load config', e)
  }
  try {
    logDir.value = await ConfigService.GetLogDir()
  } catch (e) {
    console.error('Failed to get log dir', e)
  }
})

function changeLevel(value: string) {
  logConfig.value.Level = value
  schedule('log')
}

function changeRetention(value: number) {
  logConfig.value.RetentionDays = value
  schedule('log')
}

function changeMax(value: number) {
  logConfig.value.MaxSizeMB = value
  schedule('log')
}

async function saveLog() {
  try {
    await ConfigService.SaveLogConfig(logConfig.value)
    fb.success(t.value['settings.about.log.saved'])
  } catch (e) {
    fb.error(String(e))
  }
}
async function saveSafety() {
  try {
    await ConfigService.SaveConfigField(
      'scan_external_safety',
      scanExternalSafety.value ? 'true' : 'false',
    )
  } catch (e) {
    fb.error(String(e))
  }
}
const persist = useAutoPersist()
function schedule(kind: 'log' | 'safety') {
  const op = kind === 'safety' ? saveSafety : saveLog
  persist.schedule(kind, () => op())
}

function onScanSafetyChange(checked: boolean) {
  scanExternalSafety.value = checked
  schedule('safety')
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
  <div class="wrap">
    <section class="surface">
      <header class="surface__head">
        <h2>{{ t['settings.about'] }}</h2>
      </header>

      <div class="lead">
        <p class="lead__title">wvmail — Cross-platform desktop email client, Wails3 + Vue 3</p>
        <p class="lead__meta">Version 0.0.1</p>
      </div>

      <div class="divider">{{ t['settings.about.log.title'] }}</div>

      <Field.Root class="row">
        <Field.Label class="row__label">{{ t['settings.about.log.level'] }}</Field.Label>
        <UiSelect
          :options="levelOptions"
          :model-value="logConfig.Level"
          @update:model-value="changeLevel"
        />
      </Field.Root>
      <Field.Root class="row">
        <Field.Label class="row__label">{{ t['settings.about.log.retention'] }}</Field.Label>
        <UiNumberInput
          class="numsel"
          :value="logConfig.RetentionDays"
          :min="1"
          :max="365"
          @value="changeRetention($event)"
        />
      </Field.Root>
      <Field.Root class="row">
        <Field.Label class="row__label">{{ t['settings.about.log.max_size'] }}</Field.Label>
        <UiNumberInput
          class="numsel"
          :value="logConfig.MaxSizeMB"
          :min="1"
          :max="1024"
          @value="changeMax($event)"
        />
      </Field.Root>

      <div class="actions">
        <button type="button" class="btn" @click="handleOpenLogFolder">
          {{ t['settings.about.log.open_folder'] }}
        </button>
      </div>

      <div class="divider">{{ t['settings.about.security'] }}</div>
      <UiSwitch
        class="row"
        label-class="row__label"
        :checked="scanExternalSafety"
        @update:checked="onScanSafetyChange"
      >
        {{ t['settings.about.scan_safety'] }}
      </UiSwitch>
    </section>
  </div>
</template>

<style scoped>
.wrap {
  padding: 16px;
  max-width: 720px;
  color: var(--n-text-color);
}
.about-line {
  margin: 0;
  padding: 2px 18px;
  font-size: 13px;
}
.lead {
  padding: 12px 18px;
}
.lead__title {
  margin: 0;
  font-size: 13px;
}
.lead__meta {
  margin: 4px 0 0;
  font-size: 12px;
  opacity: 0.55;
}
.divider {
  padding: 10px 18px 4px;
  font-size: 13px;
  font-weight: 800;
  color: var(--color-text-1);
  border-top: 1px solid var(--n-border-color);
}
.row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 18px;
}
.row__label {
  width: 220px;
  flex-shrink: 0;
}
.numsel {
  /* Width only: UiNumberInput's `.num` control draws the box. */
  width: 220px;
  height: 28px;
}
.actions {
  display: flex;
  gap: 8px;
  padding: 12px 18px;
}
.actions--split {
  justify-content: space-between;
}
</style>
