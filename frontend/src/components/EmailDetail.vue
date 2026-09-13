<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ConfigService, TranslationService } from '../../bindings/wvmail/services'
import { useI18n } from '../i18n'
import { useAccountsStore } from '../stores/accounts'
import { useThemeStore } from '../stores/theme'
import { useFeedbackStore } from '../stores/feedback'
import { formatDate } from '../composables/formats'
import {
  sanitizeHtmlRehype as sanitizeHtml,
  linkifyPlainText,
  extractTextForTranslation,
  translateHtmlWithLayout,
} from '../composables/html-utils'
import EmailShadowBody from './EmailShadowBody.vue'
import EmailAttachments from './EmailAttachments.vue'
import EmailLinkDialog from './EmailLinkDialog.vue'

const { t } = useI18n()
const accountsStore = useAccountsStore()
const themeStore = useThemeStore()
const fb = useFeedbackStore()

const isDarkMode = computed(() => themeStore.effectiveTheme === 'dark')
const detail = computed(() => accountsStore.selectedEmailDetail)
const loading = computed(() => accountsStore.loadingDetail)

const suspiciousRevealed = ref(false)
const attachmentsOpen = ref(true)
const blockedCount = computed(() => {
  if (!detail.value) return 0
  let count = 0
  if (detail.value.attachments) count += detail.value.attachments.length
  if (detail.value.body_html) count++
  return count
})

const translating = ref(false)
const translatedText = ref<string | null>(null)
const translationError = ref<string | null>(null)
const translatedHtml = ref<string | null>(null)
// bump to invalidate an in-flight translation when cancelled / re-clicked
let translateToken = 0

watch(detail, () => {
  translatedText.value = null
  translatedHtml.value = null
  translationError.value = null
  suspiciousRevealed.value = false
  attachmentsOpen.value = true
})

function senderDisplay(detail: { from_name?: string; from_addr?: string; from?: string }): {
  name: string | null
  addr: string | null
  fallback: string | null
} {
  if (detail.from_name && detail.from_addr)
    return { name: detail.from_name, addr: detail.from_addr, fallback: null }
  if (detail.from_name) return { name: null, addr: null, fallback: detail.from_name }
  if (detail.from_addr) return { name: null, addr: null, fallback: detail.from_addr }
  if (detail.from) return { name: null, addr: null, fallback: detail.from }
  return { name: null, addr: null, fallback: 'Unknown' }
}

async function handleRefresh() {
  // drop stale translation/rstate so a re-fetch is always actionable
  translatedText.value = null
  translatedHtml.value = null
  translationError.value = null
  const account = accountsStore.selectedAccount
  const folder = accountsStore.selectedFolder
  const uid = accountsStore.selectedEmailUid
  if (!account || !folder || uid == null) {
    fb.info('当前邮件来自聚合视图，无法单独重拉正文；请在对应账户文件夹内重试')
    return
  }
  await accountsStore.refreshEmailBody(account, folder, uid)
}

function cancelTranslation() {
  translateToken++
  translating.value = false
  translatedText.value = null
  translatedHtml.value = null
  translationError.value = null
}

async function handleTranslate() {
  if (!detail.value) return
  const gen = ++translateToken
  translating.value = true
  translationError.value = null
  try {
    const cfg = await ConfigService.GetConfig()
    const tCfg = cfg.TranslationConfig || {}
    const targetLang = tCfg.DefaultTargetLang || 'zh-CN'
    const engine = tCfg.DefaultEngine || 'llm'
    const cfgName = tCfg.LLMConfigName
    let llmCfg = null
    if (engine === 'llm' && cfg.LLMConfigs && cfg.LLMConfigs.length > 0) {
      llmCfg = (cfgName && cfg.LLMConfigs.find((c) => c.Name === cfgName)) || cfg.LLMConfigs[0]
    }
    const translateFn = (text: string) =>
      TranslationService.TranslateText(text, targetLang, engine, llmCfg, tCfg.DeepLAuthKey || '')

    if (detail.value.body_html) {
      const htmlResult = await translateHtmlWithLayout(detail.value.body_html, translateFn)
      if (gen !== translateToken) return
      if (htmlResult) {
        translatedText.value = null
        translatedHtml.value = sanitizeHtml(htmlResult)
        return
      }
    }

    const textToTranslate = detail.value.body_text
      ? detail.value.body_text
      : detail.value.body_html
        ? extractTextForTranslation(detail.value.body_html)
        : ''
    if (!textToTranslate) {
      if (gen === translateToken) translationError.value = 'No email content to translate'
      return
    }
    const result = await translateFn(textToTranslate)
    if (gen !== translateToken) return
    translatedText.value = result
    translatedHtml.value = null
  } catch (e) {
    if (gen === translateToken) translationError.value = String(e)
  } finally {
    if (gen === translateToken) translating.value = false
  }
}

function clearTranslation() {
  translatedText.value = null
  translatedHtml.value = null
  translationError.value = null
}

const linkOpen = ref(false)
const pendingUrl = ref('')

async function handleLinkClick(event: MouseEvent) {
  const path: EventTarget[] = (event as any).composedPath?.() ?? []
  const target = (path[0] ?? event.target) as HTMLElement | null
  if (!target) return
  let el: HTMLElement | null = target
  while (el && el.tagName !== 'A') el = el.parentElement
  if (!el || !(el instanceof HTMLAnchorElement)) return
  const href = el.getAttribute('href')
  if (!href) return
  if (href.startsWith('mailto:')) return
  if (href.startsWith('#') && !href.startsWith('http')) return

  event.preventDefault()
  pendingUrl.value = href.startsWith('http') ? href : el.href
  linkOpen.value = true
}

function closeError() {
  accountsStore.detailError = null
}

const showAttachments = computed(
  () =>
    detail.value?.attachments &&
    detail.value.attachments.length > 0 &&
    (!detail.value.is_suspicious || suspiciousRevealed.value),
)
</script>

<template>
  <div class="email-detail">
    <div class="scroll">
      <div v-if="loading" class="centerpad"><span class="spinner spinner--detail" /></div>

      <!-- error / security / translation alerts -->
      <div v-if="accountsStore.detailError" class="alert alert--err with-x">
        <span>{{ accountsStore.detailError }}</span>
        <button type="button" class="x" aria-label="close" @click="closeError()">
          <i class="i-ri:close-line" />
        </button>
      </div>

      <div v-if="detail?.is_suspicious && !suspiciousRevealed" class="alert alert--warn">
        <p style="margin: 0">{{ t['mail.security.title'] }}</p>
        <p style="margin: 0">
          {{ t['mail.security.body'].replace('{count}', String(blockedCount)) }}
        </p>
        <button type="button" class="btn btn--sm btn--subtle" @click="suspiciousRevealed = true">
          {{ t['mail.security.show_all'] }}
        </button>
      </div>

      <div v-if="!detail && !loading" class="centerpad empty">{{ t['mail.selectFolder'] }}</div>

      <template v-if="detail">
        <div class="detail-header">
          <div class="head-row">
            <div class="sender">
              <span v-if="senderDisplay(detail).name" class="from">
                <strong>{{ senderDisplay(detail).name }}</strong>
                <span class="addr">&lt;{{ senderDisplay(detail).addr }}&gt;</span>
              </span>
              <span v-else class="from">{{ senderDisplay(detail).fallback }}</span>
              <span class="ddate">{{ formatDate(detail.date) }}</span>
            </div>
            <div class="head-actions">
              <button
                type="button"
                class="icon-btn"
                title="refresh"
                :disabled="loading"
                @click="handleRefresh"
              >
                <i class="i-ri:refresh-line" :class="{ rot: loading }" aria-hidden="true" />
              </button>
              <button
                type="button"
                class="icon-btn"
                :title="
                  translating
                    ? '取消翻译'
                    : translatedText || translatedHtml
                      ? t['mail.translation.close']
                      : t['mail.translation.translate']
                "
                @click="
                  translating
                    ? cancelTranslation()
                    : translatedText || translatedHtml
                      ? clearTranslation()
                      : handleTranslate()
                "
              >
                <span v-if="translating" class="ic-spin" />
                <i
                  v-else
                  :class="translatedText || translatedHtml ? 'i-ri:close-line' : 'i-ri:translate-2'"
                  aria-hidden="true"
                />
              </button>
            </div>
          </div>

          <h2 class="subject">{{ detail.subject || '(No Subject)' }}</h2>

          <div class="meta">
            <div class="mrow">
              <span class="mlabel">To:</span>
              <span>{{ (detail.to || []).join(', ') }}</span>
            </div>
            <div v-if="detail.cc && detail.cc.length > 0" class="mrow">
              <span class="mlabel">Cc:</span>
              <span>{{ detail.cc.join(', ') }}</span>
            </div>
          </div>
        </div>

        <EmailAttachments
          v-if="showAttachments"
          :attachments="detail.attachments!"
          :open="attachmentsOpen"
          @update:open="attachmentsOpen = $event"
        />

        <div class="detail-body" @click="handleLinkClick">
          <EmailShadowBody
            v-if="detail.body_html && (!detail.is_suspicious || suspiciousRevealed)"
            :html="sanitizeHtml(detail.body_html)"
            :dark="isDarkMode"
          />
          <EmailShadowBody
            v-else-if="detail.body_text"
            :html="linkifyPlainText(detail.body_text)"
            :dark="isDarkMode"
          />
          <div v-else class="centerpad-empty">(No content)</div>
        </div>

        <div v-if="translatedHtml" class="detail-body">
          <div class="td">{{ t['mail.translation.label'] }}</div>
          <EmailShadowBody :html="translatedHtml" :dark="isDarkMode" />
        </div>
        <div v-if="translatedText" class="detail-body">
          <div class="td">{{ t['mail.translation.label'] }}</div>
          <div class="body-text">{{ translatedText }}</div>
        </div>
        <div v-if="translationError" class="alert alert--err with-x" style="margin: 12px">
          <span>{{ translationError }}</span>
          <button type="button" class="x" aria-label="close" @click="translationError = null">
            <i class="i-ri:close-line" />
          </button>
        </div>
      </template>
    </div>
  </div>
  <Teleport to="body">
    <EmailLinkDialog v-model:open="linkOpen" :url="pendingUrl" />
  </Teleport>
</template>

<style scoped>
.email-detail {
  height: 100%;
  color: var(--n-text-color);
  background: var(--n-body-color);
}
.scroll {
  height: 100%;
  overflow-y: auto;
}
.centerpad {
  display: flex;
  justify-content: center;
  padding: 48px 0;
}
.centerpad-empty {
  opacity: 0.4;
  text-align: center;
  padding: 2rem;
}
.empty {
  color: var(--color-text-3);
  font-size: 13px;
}
.spinner--detail {
  width: 20px;
  height: 20px;
}
.rot {
  animation: rot360 0.8s linear infinite;
}
@keyframes rot360 {
  to {
    transform: rotate(360deg);
  }
}

.alert {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  margin: 12px;
  padding: 9px 12px;
  font-size: 13px;
  border-radius: 6px;
  border: 1px solid var(--n-border-color);
}
.alert--err {
  background: rgba(208, 48, 80, 0.14);
  border-color: #d03050;
}
.alert--warn {
  background: rgba(240, 160, 32, 0.14);
  border-color: #f0a020;
}
.x {
  border: none;
  background: none;
  cursor: pointer;
  color: inherit;
  opacity: 0.6;
}
.x i {
  font-size: 15px;
}
.with-x {
  align-items: center;
}

.detail-header {
  padding: 18px 22px 14px;
  border-bottom: 1px solid var(--n-border-color);
}
.head-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}
.sender {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.from {
  font-size: 14px;
  font-weight: 600;
}
.addr {
  font-size: 13px;
  font-weight: 400;
  opacity: 0.65;
  margin-left: 4px;
}
.ddate {
  font-size: 12px;
  opacity: 0.6;
}
.head-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

/* Top-of-detail actions become themed, visible buttons (not faint icons) */
.detail-header .ic-spin {
  display: inline-block;
  width: 14px;
  height: 14px;
  border: 2px solid var(--n-border-color);
  border-top-color: var(--n-primary-color);
  border-radius: 50%;
  animation: rot360 0.8s linear infinite;
}
.subject {
  font-size: 20px;
  font-weight: 700;
  margin: 12px 0 8px;
  line-height: 1.3;
  word-break: break-word;
}
.meta {
  font-size: 12px;
  opacity: 0.75;
}
.mrow {
  display: flex;
  gap: 6px;
  margin-bottom: 2px;
}
.mlabel {
  font-weight: 600;
  min-width: 26px;
  opacity: 0.6;
}

.detail-body {
  padding: 18px 22px;
  line-height: 1.6;
  font-size: 14px;
  word-break: break-word;
}
.body-text {
  font-family: inherit;
  white-space: pre-wrap;
}
.td {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  opacity: 0.5;
  margin-bottom: 8px;
}
</style>
