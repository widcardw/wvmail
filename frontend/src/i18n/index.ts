import { computed } from 'vue'
import { createI18n } from 'vue-i18n'
import type { AppConfig } from '~/types/config'
import { ConfigService } from '#/wvmail/services'
import enUSTranslations from './locales/en-US'
import zhCNTranslations from './locales/zh-CN'

export type Locale = 'zh-CN' | 'en-US'

type Widen<T> = T extends string ? string : { readonly [K in keyof T]: Widen<T[K]> }

export type TranslationSchema = Widen<typeof enUSTranslations>

type TranslationPath<T> = T extends string
  ? never
  : {
      [K in keyof T & (string | number)]: T[K] extends string
        ? `${K}`
        : `${K}.${TranslationPath<T[K]>}`
    }[keyof T & (string | number)]

export type TranslationKey = TranslationPath<TranslationSchema>

const messages = {
  'en-US': enUSTranslations,
  'zh-CN': zhCNTranslations,
}

function detectBrowserLocale(): Locale {
  return navigator.language.startsWith('zh') ? 'zh-CN' : 'en-US'
}

export const i18n = createI18n<TranslationSchema, Locale, false>({
  legacy: false,
  locale: detectBrowserLocale(),
  fallbackLocale: 'en-US',
  messages,
})

function loadFromConfig(config: AppConfig) {
  if (config.Language === 'zh-CN' || config.Language === 'en-US') {
    i18n.global.locale.value = config.Language
  }
}

async function setLocale(locale: Locale) {
  i18n.global.locale.value = locale
  try {
    await ConfigService.SaveConfigField('language', locale)
  } catch (error) {
    console.error('Failed to save language setting', error)
  }
}

export function useI18n() {
  const locale = computed(() => i18n.global.locale.value as Locale)

  return {
    locale,
    t: i18n.global.t,
    setLocale,
    loadFromConfig,
  }
}
