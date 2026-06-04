import { computed } from 'vue'
import { useLocaleStore } from '../stores/locale'

export type Locale = 'zh-CN' | 'en-US'

export interface TranslationSchema {
  'app.brand': string
  'search.placeholder': string
  'sidebar.inbox': string
  'sidebar.sent': string
  'mail.selectFolder': string
  'reminders.title': string
  'reminders.empty': string
  'settings.common': string
  'settings.common.theme': string
  'settings.common.theme.light': string
  'settings.common.theme.dark': string
  'settings.common.theme.system': string
  'settings.common.openConfig': string
  'settings.accounts': string
  'settings.about': string
  'settings.llm': string
  'settings.llm.title': string
  'settings.llm.add': string
  'settings.llm.provider': string
  'settings.llm.name': string
  'settings.llm.baseURL': string
  'settings.llm.apiKey': string
  'settings.llm.test': string
  'settings.llm.test.success': string
  'settings.llm.test.fail': string
  'settings.llm.test.timeout': string
  'settings.llm.cancel': string
  'settings.llm.save': string
  'settings.llm.delete': string
  'settings.llm.edit': string
  'settings.llm.empty': string
  'settings.llm.testing': string
  'settings.llm.test_connection': string
  'settings.llm.test_chat': string
  'demo.prompt': string
  'demo.greet': string
  'demo.learnMore': string
  'demo.listening': string
}

export function useI18n() {
  const store = useLocaleStore()
  return {
    locale: computed(() => store.current),
    t: computed(() => store.t),
    setLocale: store.setLocale,
  }
}
