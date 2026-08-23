import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { darkTheme, useOsTheme } from 'naive-ui'
import type { AppConfig } from '~/types/config'
import { ConfigService } from '#/wvmail/services'

export type ThemeMode = 'Light' | 'Dark' | 'System'

const DEFAULT_PRIMARY_COLOR = '#18a058'

function normalizeHexColor(value: string): string | null {
  const color = value.trim()
  if (/^#[0-9a-fA-F]{6}$/.test(color)) return color.toLowerCase()
  if (/^#[0-9a-fA-F]{3}$/.test(color)) {
    return `#${color
      .slice(1)
      .split('')
      .map((c) => c + c)
      .join('')
      .toLowerCase()}`
  }
  return null
}

function mixColor(color: string, target: string, amount: number): string {
  const sourceRgb = color
    .slice(1)
    .match(/.{2}/g)
    ?.map((part) => parseInt(part, 16))
  const targetRgb = target
    .slice(1)
    .match(/.{2}/g)
    ?.map((part) => parseInt(part, 16))
  if (!sourceRgb || !targetRgb) return color
  return `#${sourceRgb
    .map((channel, index) =>
      Math.round(channel + (targetRgb[index] - channel) * amount)
        .toString(16)
        .padStart(2, '0'),
    )
    .join('')}`
}

export const useThemeStore = defineStore('theme', () => {
  const mode = ref<ThemeMode>('System')
  const primaryColor = ref(DEFAULT_PRIMARY_COLOR)
  const osTheme = useOsTheme()

  const icon = computed(() => {
    switch (mode.value) {
      case 'Dark':
        return '🌙'
      case 'Light':
        return '☀️'
      default:
        return '🖥'
    }
  })

  const naiveTheme = computed(() => {
    switch (mode.value) {
      case 'Dark':
        return darkTheme
      case 'System':
        return osTheme.value === 'dark' ? darkTheme : null
      default:
        return null
    }
  })

  const themeOverrides = computed(() => ({
    common: {
      primaryColor: primaryColor.value,
      primaryColorHover: mixColor(primaryColor.value, '#ffffff', 0.15),
      primaryColorPressed: mixColor(primaryColor.value, '#000000', 0.2),
      primaryColorSuppl: mixColor(primaryColor.value, '#ffffff', 0.15),
    },
  }))

  function loadFromConfig(config: AppConfig) {
    if (config.Theme === 'Light' || config.Theme === 'Dark' || config.Theme === 'System') {
      mode.value = config.Theme
    }
    const configuredColor = normalizeHexColor(config.PrimaryColor || '')
    primaryColor.value = configuredColor ?? DEFAULT_PRIMARY_COLOR
  }

  async function switchTo(m: ThemeMode) {
    mode.value = m
    try {
      await ConfigService.SaveConfigField('theme', m)
    } catch {
      // Config save failed silently
    }
  }

  let primaryColorSaveTimer: ReturnType<typeof setTimeout> | null = null

  function setPrimaryColor(value: string) {
    const normalized = normalizeHexColor(value)
    if (!normalized) return
    primaryColor.value = normalized

    if (primaryColorSaveTimer) clearTimeout(primaryColorSaveTimer)
    primaryColorSaveTimer = setTimeout(() => {
      void ConfigService.SaveConfigField('primary_color', normalized).catch((error) => {
        console.error('Failed to save primary color', error)
      })
    }, 250)
  }

  function switchMode() {
    const order: Array<ThemeMode> = ['Light', 'Dark', 'System']
    const idx = order.indexOf(mode.value)
    mode.value = order[(idx + 1) % order.length]
  }

  return {
    mode,
    primaryColor,
    icon,
    naiveTheme,
    themeOverrides,
    loadFromConfig,
    switchMode,
    switchTo,
    setPrimaryColor,
  }
})
