import { computed } from 'vue'
import { useLocaleStore } from '../stores/locale'
import enUSTranslations from './locales/en-US'

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

export function useI18n() {
  const store = useLocaleStore()
  return {
    locale: computed(() => store.current),
    /** The raw dictionary: `t['key.path']`. No parameters, no formatting. */
    t: computed(() => store.t),
    /** `tf('key.path', { name })` — interpolation, or `tf('key.path', count)` for plurals. */
    tf: store.tf,
    setLocale: store.setLocale,
  }
}
