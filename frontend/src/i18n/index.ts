import { computed } from 'vue'
import { useLocaleStore } from '../stores/locale'

export type Locale = 'zh-CN' | 'en-US'

export interface TranslationSchema {
  'app.brand': string
  'search.placeholder': string
  'sidebar.inbox': string
  'sidebar.sent': string
  'mail.selectFolder': string
  'mail.loading': string
  'mail.noEmails': string
  'mail.link.title': string
  'mail.link.body': string
  'mail.link.open': string
  'mail.attachment': string
  'mail.attachments': string
  'notes.title': string
  'notes.empty': string
  'notes.placeholder': string
  'notes.search': string
  'notes.new': string
  'notes.delete': string
  'notes.untitled': string
  'reminders.title': string
  'reminders.empty': string
  'settings.common': string
  'settings.common.theme': string
  'settings.common.theme.light': string
  'settings.common.theme.dark': string
  'settings.common.theme.system': string
  'settings.common.openConfig': string
  'settings.accounts': string
  'settings.accounts.add': string
  'settings.accounts.edit': string
  'settings.accounts.name': string
  'settings.accounts.email': string
  'settings.accounts.provider': string
  'settings.accounts.imap_host': string
  'settings.accounts.imap_port': string
  'settings.accounts.imap_user': string
  'settings.accounts.imap_pass': string
  'settings.accounts.smtp_host': string
  'settings.accounts.smtp_port': string
  'settings.accounts.smtp_user': string
  'settings.accounts.smtp_pass': string
  'settings.accounts.use_tls': string
  'settings.accounts.test': string
  'settings.accounts.testing': string
  'settings.accounts.test_success': string
  'settings.accounts.test_fail': string
  'settings.accounts.delete': string
  'settings.accounts.deleted': string
  'settings.accounts.delete_confirm': string
  'settings.accounts.empty': string
  'settings.accounts.save': string
  'settings.accounts.cancel': string
  'settings.accounts.provider_detected': string
  'settings.accounts.auth_code_hint': string
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
  cancel: string
}

export function useI18n() {
  const store = useLocaleStore()
  return {
    locale: computed(() => store.current),
    t: computed(() => store.t),
    setLocale: store.setLocale,
  }
}
