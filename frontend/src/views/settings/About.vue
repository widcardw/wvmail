<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Field } from '@ark-ui/vue/field'
import { useI18n } from '~/i18n'
import { useFeedbackStore } from '~/stores/feedback'
import { ConfigService, OsService } from '#/wvmail/services'
import UiSelect from '~/components/ui/UiSelect.vue'
import UiNumberInput from '~/components/ui/UiNumberInput.vue'
import { useAutoPersist } from '~/composables/use-autopersist'

const { t } = useI18n()
const fb = useFeedbackStore()

const logConfig = ref({
  Level: 'info',
  RetentionDays: 0,
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

// Retention options offered in Settings → About. UiSelect is string-valued,
// so days travel as strings and are parsed on change. '0' = never remove
// expired logs, and is the default.
const retentionOptions = computed(() => [
  { label: t.value['settings.about.log.retention.7'], value: '7' },
  { label: t.value['settings.about.log.retention.30'], value: '30' },
  { label: t.value['settings.about.log.retention.60'], value: '60' },
  { label: t.value['settings.about.log.retention.90'], value: '90' },
  { label: t.value['settings.about.log.retention.180'], value: '180' },
  { label: t.value['settings.about.log.retention.365'], value: '365' },
  { label: t.value['settings.about.log.retention.never'], value: '0' },
])
const retentionValue = computed(() => String(logConfig.value.RetentionDays))

onMounted(async () => {
  try {
    const cfg = await ConfigService.GetConfig()
    if (cfg.LogConfig) logConfig.value = { ...logConfig.value, ...cfg.LogConfig }
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
  scheduleLog()
}

function changeRetention(value: string) {
  logConfig.value.RetentionDays = Number(value)
  scheduleLog()
}

function changeMax(value: number) {
  logConfig.value.MaxSizeMB = value
  scheduleLog()
}

async function saveLog() {
  // Auto-persisted: saving silently is the norm, so only failures toast.
  try {
    await ConfigService.SaveLogConfig(logConfig.value)
  } catch (e) {
    fb.error(String(e))
  }
}
const persist = useAutoPersist()
function scheduleLog() {
  persist.schedule('log', () => saveLog())
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
        <UiSelect
          :options="retentionOptions"
          :model-value="retentionValue"
          @update:model-value="changeRetention"
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
