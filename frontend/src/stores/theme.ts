import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { darkTheme, useOsTheme } from 'naive-ui'
import type { AppConfig } from '~/types/config'
import { ConfigService } from '#/wvmail/services'

export type ThemeMode = 'Light' | 'Dark' | 'System'

export const useThemeStore = defineStore('theme', () => {
  const mode = ref<ThemeMode>('System')
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

  function loadFromConfig(config: AppConfig) {
    if (config.Theme === 'Light' || config.Theme === 'Dark' || config.Theme === 'System') {
      mode.value = config.Theme
    }
  }

  async function switchTo(m: ThemeMode) {
    mode.value = m
    try {
      await ConfigService.SaveConfigField('theme', m)
    } catch {
      // Config save failed silently
    }
  }

  function switchMode() {
    const order: Array<ThemeMode> = ['Light', 'Dark', 'System']
    const idx = order.indexOf(mode.value)
    mode.value = order[(idx + 1) % order.length]
  }

  return { mode, icon, naiveTheme, loadFromConfig, switchMode, switchTo }
})
