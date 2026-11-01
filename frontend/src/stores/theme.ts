import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { AppConfig } from '~/types/config'
import { ConfigService } from '#/wvmail/services'

export type ThemeMode = 'Light' | 'Dark' | 'System'
export type ResolvedTheme = 'light' | 'dark'

const DEFAULT_PRIMARY_COLOR = '#e11d48'

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

function getOsDarkQuery(): MediaQueryList | null {
  if (typeof window === 'undefined' || !window.matchMedia) return null
  return window.matchMedia('(prefers-color-scheme: dark)')
}

function resolveOsTheme(query: MediaQueryList | null): ResolvedTheme {
  return query?.matches ? 'dark' : 'light'
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
  const osDarkQuery = getOsDarkQuery()
  // Keep the OS result reactive. Reading matchMedia directly inside the computed
  // caches the old value because matchMedia itself is not a Vue dependency.
  const systemTheme = ref<ResolvedTheme>(resolveOsTheme(osDarkQuery))

  /** Effective light/dark after resolving System against the OS preference. */
  const effectiveTheme = computed<ResolvedTheme>(() => {
    switch (mode.value) {
      case 'Dark':
        return 'dark'
      case 'System':
        return systemTheme.value
      default:
        return 'light'
    }
  })

  function apply() {
    const root = document.documentElement
    root.dataset['theme'] = effectiveTheme.value
    // Runtime primary overrides (palette itself lives in styles/global.css).
    root.style.setProperty('--n-primary-color', primaryColor.value)
    root.style.setProperty('--n-primary-hover', mixColor(primaryColor.value, '#ffffff', 0.15))
    root.style.setProperty('--n-primary-pressed', mixColor(primaryColor.value, '#000000', 0.2))
  }

  function loadFromConfig(config: AppConfig) {
    if (config.Theme === 'Light' || config.Theme === 'Dark' || config.Theme === 'System') {
      mode.value = config.Theme
    }
    const configuredColor = normalizeHexColor(config.PrimaryColor || '')
    primaryColor.value = configuredColor ?? DEFAULT_PRIMARY_COLOR
    apply()
  }

  async function switchTo(m: ThemeMode) {
    mode.value = m
    apply()
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
    // Apply synchronously so every accent updates immediately (incl. dark).
    apply()

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
    switchTo(order[(idx + 1) % order.length])
  }

  function syncSystemTheme() {
    systemTheme.value = resolveOsTheme(osDarkQuery)
    if (mode.value === 'System') apply()
  }

  // Keep effective theme in sync when the OS preference changes while on System.
  if (osDarkQuery) {
    osDarkQuery.addEventListener('change', syncSystemTheme)
  }

  return {
    mode,
    primaryColor,
    effectiveTheme,
    apply,
    loadFromConfig,
    switchMode,
    switchTo,
    setPrimaryColor,
  }
})
