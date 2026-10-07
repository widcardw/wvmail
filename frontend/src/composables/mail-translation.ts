/**
 * Translating the message being read, with the engine and model from Settings.
 *
 * Extracted from `EmailDetail.vue`: the state machine (in-flight token, cancel,
 * clear) has nothing to do with the reading pane's layout, and the pane only needs
 * to ask "translate / cancel / clear" and render what comes back.
 */
import { computed, ref } from 'vue'
import { AppConfig, ConfigService, TranslationService } from '#/wvmail/services'
import { useI18n } from '~/i18n'
import {
  extractTextForTranslation,
  getTranslationMaxChars,
  translateHtmlWithLayout,
  translatePlainText,
} from '~/composables/html'
import type { EmailDetail } from '~/types/email'
import type { Ref } from 'vue'

export interface MailTranslationOptions {
  /** The message being read; the source of the text to translate. */
  detail: Ref<EmailDetail | null>
  /**
   * Re-runs the app's sanitizer over translated HTML, so the result obeys the same
   * remote-content / theme / date-mention policy as the original body.
   */
  sanitize: (html: string) => string
  /** True while the body is still behind the spam gate (translation must not open it). */
  blocked: () => boolean
}

export function useMailTranslation(options: MailTranslationOptions) {
  const { t } = useI18n()

  const translating = ref(false)
  const translatedText = ref<string | null>(null)
  const translatedHtml = ref<string | null>(null)
  const error = ref<string | null>(null)
  /** Bumped to invalidate an in-flight translation when cancelled or re-clicked. */
  let token = 0

  const hasTranslation = computed(() => Boolean(translatedHtml.value || translatedText.value))
  /** True when there is anything to render (a translation, or its error). */
  const hasOutput = computed(() =>
    Boolean(translatedHtml.value || translatedText.value || error.value),
  )

  function clear() {
    translatedText.value = null
    translatedHtml.value = null
    error.value = null
  }

  function cancel() {
    token++
    translating.value = false
    clear()
  }

  /** Translate / cancel / clear, i.e. what the header's translate button does. */
  function toggle() {
    if (translating.value) {
      cancel()
      return
    }
    if (hasTranslation.value) {
      clear()
      return
    }
    void start()
  }

  async function start() {
    const mail = options.detail.value
    if (!mail) return
    // A suspicious (spam/Junk) email stays behind the reveal gate: translating it
    // must not render a body the user has not chosen to reveal.
    if (options.blocked()) return

    const gen = ++token
    translating.value = true
    error.value = null
    try {
      const cfg: AppConfig = await ConfigService.GetConfig()
      const tCfg = cfg.TranslationConfig || {}
      const targetLang = tCfg.DefaultTargetLang || 'zh-CN'
      const engine = tCfg.DefaultEngine || 'llm'
      const cfgName = tCfg.LLMConfigName
      let llmCfg = null
      if (engine === 'llm' && cfg.LLMConfigs && cfg.LLMConfigs.length > 0) {
        const picked = cfgName ? cfg.LLMConfigs.find((c) => c.Name === cfgName) : undefined
        const chosen = picked ?? cfg.LLMConfigs[0]
        // Translate with the model picked in the translation settings; a config that
        // never picked one keeps its own default. Mutating the fetched config keeps
        // it a real LLMConfig: the typed helpers below take a narrow shape and
        // reject object literals that carry extra keys.
        if (tCfg.LLMModel) chosen.Model = tCfg.LLMModel
        llmCfg = chosen
      }
      const maxTranslationChars = engine === 'llm' ? getTranslationMaxChars(llmCfg) : undefined
      const translateFn = (text: string) =>
        TranslationService.TranslateText(text, targetLang, engine, llmCfg, tCfg.DeepLAuthKey || '')

      if (mail.body_html) {
        const htmlResult = await translateHtmlWithLayout(
          mail.body_html,
          translateFn,
          maxTranslationChars,
        )
        if (gen !== token) return
        if (htmlResult) {
          translatedText.value = null
          translatedHtml.value = options.sanitize(htmlResult)
          return
        }
      }

      const textToTranslate = mail.body_text
        ? mail.body_text
        : mail.body_html
          ? extractTextForTranslation(mail.body_html)
          : ''
      if (!textToTranslate) {
        if (gen === token) error.value = 'No email content to translate'
        return
      }
      const result = await translatePlainText(textToTranslate, translateFn, maxTranslationChars)
      if (gen !== token) return
      if (!result) {
        // A separator/paragraph-count mismatch makes the translator return
        // nothing: report it instead of leaving the UI as if the click was lost.
        translatedText.value = null
        translatedHtml.value = null
        error.value = t.value['mail.translation.empty']
        return
      }
      translatedText.value = result
      translatedHtml.value = null
    } catch (e) {
      if (gen === token) error.value = String(e)
    } finally {
      if (gen === token) translating.value = false
    }
  }

  return {
    translating,
    translatedText,
    translatedHtml,
    error,
    hasTranslation,
    hasOutput,
    start,
    cancel,
    clear,
    toggle,
  }
}
