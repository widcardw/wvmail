<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '~/i18n'
import { useThemeStore } from '~/stores/theme'
import { ConfigService, OsService } from '#/wvmail/services'
import UiSelect from '~/components/UiSelect.vue'

const { t, locale, setLocale } = useI18n()
const theme = useThemeStore()

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

function onLocale(v: string) {
  if (v === 'zh-CN' || v === 'en-US') setLocale(v)
}
function onTheme(v: string) {
  if (v === 'Light' || v === 'Dark' || v === 'System') theme.switchTo(v)
}
function onColor(e: Event) {
  const v = (e.target as HTMLInputElement).value
  if (/^#[0-9a-fA-F]{6}$/.test(v)) theme.setPrimaryColor(v)
}

async function openConfigFolder() {
  try {
    const dir = await ConfigService.GetConfigDir()
    await OsService.OpenFolder(dir)
  } catch (e) {
    console.error('Failed to open config folder', e)
  }
}
</script>

<template>
  <div class="wrap">
    <section class="surface">
      <header class="surface__head">
        <h2>{{ t['settings.common'] }}</h2>
      </header>

      <div class="row">
        <label class="row__label">Language / 语言</label>
        <UiSelect :options="localeOptions" :model-value="locale" @update:model-value="onLocale" />
      </div>

      <div class="row">
        <label class="row__label">{{ t['settings.common.theme'] }}</label>
        <UiSelect :options="themeOptions" :model-value="theme.mode" @update:model-value="onTheme" />
      </div>

      <div class="row">
        <span class="row__label">{{ t['settings.common.primaryColor'] }}</span>
        <div class="color">
          <input
            type="color"
            class="color__native"
            :value="theme.primaryColor"
            @input="onColor"
            aria-label="primary color"
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
      </div>

      <div class="actions">
        <button type="button" class="btn" @click="openConfigFolder">
          {{ t['settings.common.openConfig'] }}
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
.row {
  display: flex;
  align-items: center;
  gap: 16px;
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
  outline: none;
}
.ctl:focus {
  border-color: var(--n-primary-color);
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
</style>
