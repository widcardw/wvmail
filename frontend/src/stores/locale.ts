import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { zhCN, enUS, dateZhCN, dateEnUS } from 'naive-ui'
import type { NLocale, NDateLocale } from 'naive-ui'
import type { Locale, TranslationSchema } from '~/i18n'
import type { AppConfig } from '~/types/config'
import enUSTranslations from '~/i18n/locales/en-US'
import zhCNTranslations from '~/i18n/locales/zh-CN'
import { ConfigService } from '#/wvmail/services'

const TRANSLATIONS: Record<Locale, TranslationSchema> = {
  'en-US': enUSTranslations,
  'zh-CN': zhCNTranslations,
}

const NAIVE_LOCALES: Record<Locale, { locale: NLocale; dateLocale: NDateLocale }> = {
  'en-US': { locale: enUS, dateLocale: dateEnUS },
  'zh-CN': { locale: zhCN, dateLocale: dateZhCN },
}

function detectBrowserLocale(): Locale {
  const lang = navigator.language
  if (lang.startsWith('zh')) return 'zh-CN'
  return 'en-US'
}

export const useLocaleStore = defineStore('locale', () => {
  const current = ref<Locale>(detectBrowserLocale())

  const naiveLocale = computed(() => NAIVE_LOCALES[current.value].locale)
  const naiveDateLocale = computed(() => NAIVE_LOCALES[current.value].dateLocale)

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

  return { current, naiveLocale, naiveDateLocale, t, loadFromConfig, setLocale }
})
