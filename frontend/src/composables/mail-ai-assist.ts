import { computed, ref } from 'vue'
import { AIAssistService } from '#/wvmail/services'
import { useI18n } from '~/i18n'
import {
  extractTextForTranslation,
  translateHtmlWithLayout,
  translatePlainText,
} from '~/composables/html'
import type { AIAssistSelection } from '~/types/ai-assist'
import type { EmailDetail } from '~/types/email'
import type { Ref } from 'vue'

export interface MailAIAssistOptions {
  /** The message being read; the source of the text to translate or summarize. */
  detail: Ref<EmailDetail | null>
  /**
   * Re-runs the app's sanitizer over translated HTML, so the result obeys the same
   * remote-content / theme / date-mention policy as the original body.
   */
  sanitizeHtml: (html: string) => string
  /** Sanitizes Markdown returned by the summarizer before it is rendered. */
  sanitizeText: (text: string) => string
  /** True while the body is still behind the spam gate. */
  blocked: () => boolean
}

export function useMailAIAssist(options: MailAIAssistOptions) {
  const { t } = useI18n()

  const translating = ref(false)
  const translatedText = ref<string | null>(null)
  const translatedHtml = ref<string | null>(null)
  const translationError = ref<string | null>(null)
  const summarizing = ref(false)
  const summaryHtml = ref<string | null>(null)
  const summaryError = ref<string | null>(null)

  /** Bumped to invalidate an in-flight request when cancelled or re-clicked. */
  let translationToken = 0
  let summaryToken = 0

  const hasTranslation = computed(() => Boolean(translatedHtml.value || translatedText.value))
  const hasTranslationOutput = computed(() =>
    Boolean(translatedHtml.value || translatedText.value || translationError.value),
  )
  const hasSummary = computed(() => Boolean(summaryHtml.value))
  const hasSummaryOutput = computed(() => Boolean(summaryHtml.value || summaryError.value))

  function clearTranslation() {
    translatedText.value = null
    translatedHtml.value = null
    translationError.value = null
  }

  function cancelTranslation() {
    translationToken++
    translating.value = false
    clearTranslation()
  }

  function clearSummary() {
    summaryHtml.value = null
    summaryError.value = null
  }

  function cancelSummary() {
    summaryToken++
    summarizing.value = false
    clearSummary()
  }

  function sourceText(mail: EmailDetail): string {
    if (mail.body_text) return mail.body_text
    return mail.body_html ? extractTextForTranslation(mail.body_html) : ''
  }

  async function startTranslation(selection: AIAssistSelection = {}) {
    const mail = options.detail.value
    if (!mail || options.blocked()) return

    const gen = ++translationToken
    translating.value = true
    translationError.value = null
    try {
      const translateFn = (text: string) =>
        AIAssistService.TranslateText({
          Text: text,
          TargetLang: '',
          Engine: selection.engine ?? '',
          LLMConfigName: selection.llmConfigName ?? '',
          LLMModel: selection.llmModel ?? '',
          Instruction: '',
        })

      if (mail.body_html) {
        const htmlResult = await translateHtmlWithLayout(mail.body_html, translateFn)
        if (gen !== translationToken) return
        if (htmlResult) {
          translatedText.value = null
          translatedHtml.value = options.sanitizeHtml(htmlResult)
          return
        }
      }

      const textToTranslate = sourceText(mail)
      if (!textToTranslate) {
        if (gen === translationToken) {
          translationError.value = t.value['mail.translation.no_content']
        }
        return
      }
      const result = await translatePlainText(textToTranslate, translateFn)
      if (gen !== translationToken) return
      if (!result) {
        translatedText.value = null
        translatedHtml.value = null
        translationError.value = t.value['mail.translation.empty']
        return
      }
      translatedText.value = result
      translatedHtml.value = null
    } catch (error) {
      if (gen === translationToken) translationError.value = String(error)
    } finally {
      if (gen === translationToken) translating.value = false
    }
  }

  async function startSummary(selection: AIAssistSelection = {}) {
    const mail = options.detail.value
    if (!mail || options.blocked()) return

    const text = sourceText(mail)
    if (!text) {
      summaryError.value = t.value['mail.summary.no_content']
      return
    }

    const gen = ++summaryToken
    summarizing.value = true
    summaryHtml.value = null
    summaryError.value = null
    try {
      const result = await AIAssistService.SummarizeText({
        Text: text,
        TargetLang: '',
        Engine: 'llm',
        LLMConfigName: selection.llmConfigName ?? '',
        LLMModel: selection.llmModel ?? '',
        Instruction: '',
      })
      if (gen !== summaryToken) return
      if (!result.trim()) {
        summaryError.value = t.value['mail.summary.empty']
        return
      }
      summaryHtml.value = options.sanitizeText(result)
    } catch (error) {
      if (gen === summaryToken) summaryError.value = String(error)
    } finally {
      if (gen === summaryToken) summarizing.value = false
    }
  }

  return {
    translating,
    translatedText,
    translatedHtml,
    translationError,
    summarizing,
    summaryHtml,
    summaryError,
    hasTranslation,
    hasTranslationOutput,
    hasSummary,
    hasSummaryOutput,
    startTranslation,
    cancelTranslation,
    clearTranslation,
    startSummary,
    cancelSummary,
    clearSummary,
  }
}
