<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from '~/i18n'
import { useFeedbackStore } from '~/stores/feedback'
import { ConfigService, OsService } from '#/wvmail/services'
import UiSelect from '~/components/UiSelect.vue'

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

function num(e: Event): number {
  return Number((e.target as HTMLInputElement).value) || 0
}

async function handleSave() {
  saving.value = true
  try {
    await ConfigService.SaveLogConfig(logConfig.value)
    fb.success(t.value['settings.about.log.saved'])
  } catch (e) {
    fb.error(String(e))
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

async function handleSaveSafety() {
  scanExternalSafety.value = !scanExternalSafety.value
  try {
    await ConfigService.SaveConfigField(
      'scan_external_safety',
      scanExternalSafety.value ? 'true' : 'false',
    )
  } catch (e) {
    fb.error(String(e))
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

      <div class="row">
        <label class="row__label">{{ t['settings.about.log.level'] }}</label>
        <UiSelect
          :options="levelOptions"
          :model-value="logConfig.Level"
          @update:model-value="(v: string) => (logConfig.Level = v)"
        />
      </div>
      <div class="row">
        <label class="row__label">{{ t['settings.about.log.retention'] }}</label>
        <input
          class="numsel"
          type="number"
          :value="logConfig.RetentionDays"
          min="1"
          max="365"
          @input="logConfig.RetentionDays = num($event)"
        />
      </div>
      <div class="row">
        <label class="row__label">{{ t['settings.about.log.max_size'] }}</label>
        <input
          class="numsel"
          type="number"
          :value="logConfig.MaxSizeMB"
          min="1"
          max="1024"
          @input="logConfig.MaxSizeMB = num($event)"
        />
      </div>

      <div class="actions actions--split">
        <button type="button" class="btn" @click="handleOpenLogFolder">
          {{ t['settings.about.log.open_folder'] }}
        </button>
        <button type="button" class="btn btn--primary" :disabled="saving" @click="handleSave">
          {{ saving ? '…' : t['settings.llm.save'] }}
        </button>
      </div>

      <div class="divider">⚠️ {{ t['settings.about.security'] }}</div>
      <div class="row">
        <span class="row__label">{{ t['settings.about.scan_safety'] }}</span>
        <button
          type="button"
          class="switch"
          :class="{ on: scanExternalSafety }"
          role="switch"
          :aria-checked="scanExternalSafety"
          @click="handleSaveSafety"
        >
          <span class="switch__thumb" />
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
.about-line {
  margin: 0;
  padding: 2px 18px;
  font-size: 13px;
}
.lead {
  padding: 12px 18px 6px;
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
  font-weight: 600;
  color: var(--color-text-2);
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
  font-size: 13px;
  color: var(--color-text-2);
}
.numsel {
  width: 180px;
  height: 28px;
  padding: 0 8px;
  font-size: 13px;
  color: var(--n-text-color);
  background: var(--n-action-color);
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
}
.numsel:focus {
  border-color: var(--n-primary-color);
  outline: none;
}
.actions {
  display: flex;
  gap: 8px;
  padding: 12px 18px;
}
.actions--split {
  justify-content: space-between;
}
.btn {
  padding: 5px 12px;
  font-size: 13px;
  color: var(--n-text-color);
  background: var(--n-action-color);
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  cursor: pointer;
}
.btn:hover {
  background: var(--n-sidebar-hover);
}
.btn--primary {
  color: #fff;
  background: var(--n-primary-color);
  border-color: transparent;
}
.btn--primary:disabled {
  opacity: 0.6;
}
.switch {
  width: 40px;
  height: 22px;
  border-radius: 11px;
  position: relative;
  border: none;
  background: var(--n-border-color);
  cursor: pointer;
  padding: 0;
}
.switch.on {
  background: var(--n-primary-color);
}
.switch__thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: #fff;
  transition: left 0.15s ease;
}
.switch.on .switch__thumb {
  left: 20px;
}
</style>
