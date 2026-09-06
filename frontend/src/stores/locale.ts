import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { Locale, TranslationSchema } from '~/i18n'
import type { AppConfig } from '~/types/config'
import enUSTranslations from '~/i18n/locales/en-US'
import zhCNTranslations from '~/i18n/locales/zh-CN'
import { ConfigService } from '#/wvmail/services'

const TRANSLATIONS: Record<Locale, TranslationSchema> = {
  'en-US': enUSTranslations,
  'zh-CN': zhCNTranslations,
}

function detectBrowserLocale(): Locale {
  const lang = navigator.language
  if (lang.startsWith('zh')) return 'zh-CN'
  return 'en-US'
}

export const useLocaleStore = defineStore('locale', () => {
  const current = ref<Locale>(detectBrowserLocale())

  const t = computed(() => TRANSLATIONS[current.value])

  function loadFromConfig(config: AppConfig) {
    if (config.Language === 'zh-CN' || config.Language === 'en-US') {
      current.value = config.Language
    }
  }

  async function setLocale(locale: Locale) {
    current.value = locale
    try {
      await ConfigService.SaveConfigField('language', locale)
    } catch {
      // Config save failed silently — the UI is already updated
    }
  }

  return { current, t, loadFromConfig, setLocale }
})
