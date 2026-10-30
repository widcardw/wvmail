<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from '~/i18n'
import { useThemeStore } from '~/stores/theme'
import { usePreferencesStore } from '~/stores/preferences'
import { useFeedbackStore } from '~/stores/feedback'
import { ConfigService, OsService } from '#/wvmail/services'
import UiSelect from '~/components/ui/UiSelect.vue'
import UiSwitch from '~/components/ui/UiSwitch.vue'
import UiFieldRow from '~/components/ui/UiFieldRow.vue'
import ConfirmDialog from '~/components/ui/ConfirmDialog.vue'

const { t, locale, setLocale } = useI18n()
const theme = useThemeStore()
const preferences = usePreferencesStore()
const fb = useFeedbackStore()

/** Quitting ends the process, so the button only opens the confirmation. */
const showQuitDialog = ref(false)
const quitting = ref(false)

async function confirmQuit() {
  if (quitting.value) return
  quitting.value = true
  try {
    await OsService.QuitApp()
  } catch (e) {
    console.error('Failed to quit the app', e)
    quitting.value = false
    fb.error(t.value['quitApp.failed'])
  }
}

const PRESETS = ['#18a058', '#2080f0', '#f0a020', '#d03050', '#8a2be2', '#e11d48']
const localeOptions = [
  { label: 'English', value: 'en-US' },
  { label: '中文', value: 'zh-CN' },
]

const themeOptions = computed(() => [
  { label: t.value['settings.common.theme.system'], value: 'System' },
  { label: t.value['settings.common.theme.light'], value: 'Light' },
  { label: t.value['settings.common.theme.dark'], value: 'Dark' },
])
const tagDeleteOptions = computed(() => [
  { label: t.value['settings.common.tagDelete.once'], value: '1' },
  { label: t.value['settings.common.tagDelete.twice'], value: '2' },
])
const minimizeOrCloseOptions = computed(() => [
  { label: t.value['settings.common.minimizeWhenClose'], value: '1' },
  { label: t.value['settings.common.quitWhenClose'], value: '0' },
])

function onLocale(v: string) {
  if (v === 'zh-CN' || v === 'en-US') setLocale(v)
}
function onTheme(v: string) {
  if (v === 'Light' || v === 'Dark' || v === 'System') theme.switchTo(v)
}
function onTagDeletePresses(v: string) {
  if (v === '1' || v === '2') void preferences.setTagDeletePresses(Number(v) as 1 | 2)
}
function onColor(e: Event) {
  const v = (e.target as HTMLInputElement).value
  if (/^#[0-9a-fA-F]{6}$/.test(v)) theme.setPrimaryColor(v)
}
function onActionOnClose(v: string) {
  preferences.setMinimizeWhenClose(Boolean(Number(v)))
}

async function openConfigFolder() {
  try {
    const dir = await ConfigService.GetConfigDir()
    await OsService.OpenFolder(dir)
  } catch (e) {
    console.error('Failed to open config folder', e)
  }
}

function onDetectDatesChange(checked: boolean) {
  void preferences.setDetectDatesInMail(checked)
}

function onCollapseSmartFoldersChange(checked: boolean) {
  void preferences.setCollapseSidebarSmartFolders(checked)
}

function onCollapseUtilitiesChange(checked: boolean) {
  void preferences.setCollapseSidebarUtilities(checked)
}
</script>

<template>
  <div class="wrap">
    <section class="surface">
      <header class="surface__head">
        <h2>{{ t['settings.common'] }}</h2>
      </header>

      <UiFieldRow :label="t['settings.common.language']">
        <UiSelect :options="localeOptions" :model-value="locale" @update:model-value="onLocale" />
      </UiFieldRow>
      <UiFieldRow :label="t['settings.common.theme']">
        <UiSelect :options="themeOptions" :model-value="theme.mode" @update:model-value="onTheme" />
      </UiFieldRow>
      <UiFieldRow :label="t['settings.common.tagDelete']">
        <UiSelect
          :options="tagDeleteOptions"
          :model-value="String(preferences.tagDeletePresses)"
          @update:model-value="onTagDeletePresses"
        />
      </UiFieldRow>

      <!-- Native color input + preset swatches: a bespoke composite, so it keeps the
           raw control. The explicit Field id still links the label to it. -->
      <UiFieldRow :label="t['settings.common.primaryColor']">
        <div class="color">
          <input
            id="primary-color"
            type="color"
            class="color__native"
            :value="theme.primaryColor"
            @input="onColor"
            :aria-label="t['settings.common.primaryColorPicker']"
          />
          <button
            v-for="c in PRESETS"
            :key="c"
            type="button"
            class="swatch"
            :style="{ background: c }"
            :class="{ active: theme.primaryColor.toLowerCase() === c }"
            :aria-label="`preset ${c}`"
            @click="theme.setPrimaryColor(c)"
          />
          <code class="color__hex">{{ theme.primaryColor }}</code>
        </div>
      </UiFieldRow>
      <UiFieldRow :label="t['settings.common.actionOnClose']">
        <UiSelect
          :options="minimizeOrCloseOptions"
          :model-value="String(Number(preferences.minimizeWhenClose))"
          @update:model-value="onActionOnClose"
        />
      </UiFieldRow>
      <UiFieldRow
        :label="t['settings.common.detectDates']"
        :hint="t['settings.common.detectDatesHint']"
      >
        <UiSwitch :checked="preferences.detectDatesInMail" @update:checked="onDetectDatesChange" />
      </UiFieldRow>
      <UiFieldRow
        :label="t['settings.common.collapseSmartFolders']"
        :hint="t['settings.common.collapseSmartFoldersHint']"
      >
        <UiSwitch
          :checked="preferences.collapseSidebarSmartFolders"
          @update:checked="onCollapseSmartFoldersChange"
        />
      </UiFieldRow>
      <UiFieldRow
        :label="t['settings.common.collapseUtilities']"
        :hint="t['settings.common.collapseUtilitiesHint']"
      >
        <UiSwitch
          :checked="preferences.collapseSidebarUtilities"
          @update:checked="onCollapseUtilitiesChange"
        />
      </UiFieldRow>

      <div class="actions">
        <button type="button" class="btn" @click="openConfigFolder">
          {{ t['settings.common.openConfig'] }}
        </button>
        <button type="button" class="btn btn--danger" @click="showQuitDialog = true">
          {{ t['quitApp'] }}
        </button>
      </div>
    </section>

    <ConfirmDialog
      :open="showQuitDialog"
      :title="t['quitApp.title']"
      :description="t['quitApp.body']"
      :confirm-label="t['quitApp.confirm']"
      :busy="quitting"
      danger
      @update:open="showQuitDialog = $event"
      @confirm="confirmQuit"
    />
  </div>
</template>

<style scoped>
.color {
  display: flex;
  align-items: center;
  gap: 6px;
}
.color__native {
  width: 34px;
  height: 28px;
  padding: 0;
  border: none;
  background: none;
  cursor: pointer;
}
.swatch {
  width: 20px;
  height: 20px;
  border-radius: 5px;
  border: 1px solid rgba(128, 128, 128, 0.4);
  cursor: pointer;
  padding: 0;
}
.swatch.active {
  outline: 2px solid var(--n-primary-color);
  outline-offset: 1px;
}
.color__hex {
  font-size: 12px;
  opacity: 0.7;
}
.actions {
  padding: 6px 18px 16px;
  --at-apply: space-x-2;
}
</style>
