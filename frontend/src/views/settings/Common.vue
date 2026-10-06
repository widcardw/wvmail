<script setup lang="ts">
import { computed, ref } from 'vue'
import { Field } from '@ark-ui/vue/field'
import { useI18n } from '~/i18n'
import { useThemeStore } from '~/stores/theme'
import { usePreferencesStore } from '~/stores/preferences'
import { ConfigService, OsService } from '#/wvmail/services'
import UiSelect from '~/components/UiSelect.vue'
import UiSwitch from '~/components/UiSwitch.vue'
import QuitConfirmDialog from '~/components/settings/QuitConfirmDialog.vue'

const { t, locale, setLocale } = useI18n()
const theme = useThemeStore()
const preferences = usePreferencesStore()

/** Quitting ends the process, so the button only opens the confirmation. */
const showQuitDialog = ref(false)

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

function onAllowRemoteChange(checked: boolean) {
  void preferences.setAllowRemoteContent(checked)
}

function onDetectDatesChange(checked: boolean) {
  void preferences.setDetectDatesInMail(checked)
}

function onRemoveSender(sender: string) {
  void preferences.removeAllowedRemoteSender(sender)
}
</script>

<template>
  <div class="wrap">
    <section class="surface">
      <header class="surface__head">
        <h2>{{ t['settings.common'] }}</h2>
      </header>

      <Field.Root class="row">
        <Field.Label class="row__label">{{ t['settings.common.language'] }}</Field.Label>
        <UiSelect :options="localeOptions" :model-value="locale" @update:model-value="onLocale" />
      </Field.Root>

      <Field.Root class="row">
        <Field.Label class="row__label">{{ t['settings.common.theme'] }}</Field.Label>
        <UiSelect :options="themeOptions" :model-value="theme.mode" @update:model-value="onTheme" />
      </Field.Root>

      <Field.Root class="row">
        <Field.Label class="row__label">{{ t['settings.common.tagDelete'] }}</Field.Label>
        <UiSelect
          :options="tagDeleteOptions"
          :model-value="String(preferences.tagDeletePresses)"
          @update:model-value="onTagDeletePresses"
        />
      </Field.Root>

      <!-- Native color input + preset swatches: a bespoke composite, so it keeps the
           raw control. The explicit Field id still links the label to it. -->
      <Field.Root class="row" id="primary-color">
        <Field.Label class="row__label">{{ t['settings.common.primaryColor'] }}</Field.Label>
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
      </Field.Root>

      <Field.Root class="row">
        <Field.Label class="row__label">{{ t['settings.common.actionOnClose'] }}</Field.Label>
        <UiSelect
          :options="minimizeOrCloseOptions"
          :model-value="String(Number(preferences.minimizeWhenClose))"
          @update:model-value="onActionOnClose"
        />
      </Field.Root>

      <UiSwitch
        class="row"
        label-class="row__label"
        :checked="preferences.detectDatesInMail"
        @update:checked="onDetectDatesChange"
      >
        {{ t['settings.common.detectDates'] }}
      </UiSwitch>
      <p class="row-hint">{{ t['settings.common.detectDatesHint'] }}</p>

      <div class="actions">
        <button type="button" class="btn" @click="openConfigFolder">
          {{ t['settings.common.openConfig'] }}
        </button>
        <button type="button" class="btn btn--danger" @click="showQuitDialog = true">
          {{ t['quitApp'] }}
        </button>
      </div>
    </section>

    <section class="surface surface--spaced">
      <header class="surface__head">
        <h2>{{ t['settings.remote.title'] }}</h2>
      </header>

      <UiSwitch
        class="row remote-row"
        label-class="row__label"
        :checked="preferences.allowRemoteContent"
        @update:checked="onAllowRemoteChange"
      >
        {{ t['settings.remote.allow'] }}
      </UiSwitch>

      <div class="row row--top">
        <span class="row__label">{{ t['settings.remote.hint'] }}</span>
        <div class="senders">
          <p v-if="preferences.allowedRemoteSenders.length === 0" class="senders__empty">
            {{ t['settings.remote.empty'] }}
          </p>
          <ul v-else class="senders__list">
            <li
              v-for="sender in preferences.allowedRemoteSenders"
              :key="sender"
              class="senders__item"
            >
              <span>{{ sender }}</span>
              <button type="button" class="btn btn--sm" @click="onRemoveSender(sender)">
                {{ t['settings.remote.remove'] }}
              </button>
            </li>
          </ul>
        </div>
      </div>
    </section>

    <QuitConfirmDialog v-model:open="showQuitDialog" />
  </div>
</template>

<style scoped>
.wrap {
  padding: 16px;
  max-width: 800px;
  color: var(--n-text-color);
}
.row {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 10px 18px;
}
.row__label {
  flex-shrink: 0;
}
.row-hint {
  margin: -4px 0 8px;
  padding: 0 18px;
  font-size: 12px;
  color: var(--color-text-3);
}
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
.surface--spaced {
  margin-top: 14px;
}
/* Keep stacked content inside a ROW: the shared `.row__label` rule sets
   `flex: 0 0 240px`, which in a column container becomes a 200px-tall label box
   (a large blank gap). Use `.row--top` + a flex:1 content column instead. */
.row--top {
  align-items: flex-start;
}
/* Ark Switch: the root is a <label> for the hidden input, so the whole row is
   clickable. Part visuals live in styles/ui.css. */
.remote-row {
  cursor: pointer;
}
.senders {
  flex: 1;
  min-width: 0;
}
.senders__empty {
  margin: 0;
  font-size: 12px;
  opacity: 0.65;
}
.senders__list {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.senders__item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 13px;
}
.senders__item {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
