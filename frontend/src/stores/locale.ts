import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { Locale, TranslationSchema } from '~/i18n'
import type { AppConfig } from '~/types/config'
import enUSTranslations from '~/i18n/locales/en-US'
import zhCNTranslations from '~/i18n/locales/zh-CN'
import { formatMessage, formatPlural } from '~/i18n/format'
import type { MessageParams } from '~/i18n/format'
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

  /**
   * Translate *and* format one message. `t['key']` stays the plain dictionary
   * (zero-cost, compile-time-checked); reach for `tf` whenever the message has
   * `{placeholders}`, and pass a number to select a `singular | plural` branch.
   *
   * A plain function rather than a computed: it reads `current` on every call, so
   * whatever render or computed calls it re-runs on a language switch.
   */
  function tf(key: keyof TranslationSchema, params: MessageParams | number = {}): string {
    const message = TRANSLATIONS[current.value][key]
    return typeof params === 'number'
      ? formatPlural(message, params, current.value)
      : formatMessage(message, params)
  }

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

  return { current, t, tf, loadFromConfig, setLocale }
})
