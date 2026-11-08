<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from '~/i18n'
import { useFeedbackStore } from '~/stores/feedback'
import { ConfigService, OsService } from '#/wvmail/services'
import UiSelect from '~/components/ui/UiSelect.vue'
import UiNumberInput from '~/components/ui/UiNumberInput.vue'
import UiFieldRow from '~/components/ui/UiFieldRow.vue'
import AppIcon from '~/components/icon/AppIcon.vue'
import { useAutoPersist } from '~/composables/use-autopersist'

const { t } = useI18n()
const fb = useFeedbackStore()
const repositoryURL = 'https://github.com/widcardw/wvmail'
const version = '0.0.1'

const logConfig = ref({
  Level: 'info',
  RetentionDays: 0,
  MaxSizeMB: 10,
})
const logDir = ref('')
const saving = ref(false)

const levelOptions = computed(() => [
  { label: t('settings.about.log.level.debug'), value: 'debug' },
  { label: t('settings.about.log.level.info'), value: 'info' },
  { label: t('settings.about.log.level.warn'), value: 'warn' },
  { label: t('settings.about.log.level.error'), value: 'error' },
])

// Retention options offered in Settings → About. UiSelect is string-valued,
// so days travel as strings and are parsed on change. '0' = never remove
// expired logs, and is the default.
const retentionOptions = computed(() => [
  { label: t('settings.about.log.retention.7'), value: '7' },
  { label: t('settings.about.log.retention.30'), value: '30' },
  { label: t('settings.about.log.retention.60'), value: '60' },
  { label: t('settings.about.log.retention.90'), value: '90' },
  { label: t('settings.about.log.retention.180'), value: '180' },
  { label: t('settings.about.log.retention.365'), value: '365' },
  { label: t('settings.about.log.retention.never'), value: '0' },
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

async function handleOpenRepository() {
  try {
    await OsService.OpenURL(repositoryURL)
  } catch (e) {
    console.error('Failed to open repository', e)
  }
}
</script>

<template>
  <div class="wrap wrap--narrow">
    <section class="surface">
      <header class="surface__head">
        <h2>{{ t('settings.about.label') }}</h2>
      </header>

      <div class="lead">
        <p class="lead__title">{{ t('settings.about.tagline') }}</p>
        <div class="lead__meta-row">
          <p class="lead__meta">{{ t('settings.about.version', { version }) }}</p>
          <a
            class="repo-link"
            :href="repositoryURL"
            :aria-label="t('settings.about.github_open')"
            @click.prevent="handleOpenRepository"
          >
            <AppIcon icon="ri:github-fill" :size="14" />
            <span>github.com/widcardw/wvmail</span>
            <AppIcon icon="ri:external-link-line" :size="12" />
          </a>
        </div>
      </div>

      <div class="divider">{{ t('settings.about.log.title') }}</div>

      <UiFieldRow :label="t('settings.about.log.level.label')">
        <UiSelect
          :options="levelOptions"
          :model-value="logConfig.Level"
          @update:model-value="changeLevel"
        />
      </UiFieldRow>
      <UiFieldRow :label="t('settings.about.log.retention.label')">
        <UiSelect
          :options="retentionOptions"
          :model-value="retentionValue"
          @update:model-value="changeRetention"
        />
      </UiFieldRow>
      <UiFieldRow :label="t('settings.about.log.max_size')">
        <UiNumberInput
          :value="logConfig.MaxSizeMB"
          :min="1"
          :max="1024"
          width="220px"
          @value="changeMax($event)"
        />
      </UiFieldRow>

      <div class="actions">
        <button type="button" class="btn" @click="handleOpenLogFolder">
          {{ t('settings.about.log.open_folder') }}
        </button>
      </div>
    </section>
  </div>
</template>

<style scoped>
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
  margin: 0;
  font-size: 12px;
  opacity: 0.55;
}
.lead__meta-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
}
.repo-link {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 7px;
  font-size: 12px;
  color: var(--n-primary-color);
  text-decoration: none;
  border-radius: var(--radius-sm);
}
.repo-link:hover {
  text-decoration: underline;
  background: var(--n-sidebar-hover);
}
.repo-link:focus-visible {
  outline: 2px solid var(--n-primary-color);
  outline-offset: 1px;
}
.divider {
  padding: 10px 18px 4px;
  font-size: 13px;
  font-weight: 800;
  color: var(--color-text-2);
  border-top: 1px solid var(--n-border-color);
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
