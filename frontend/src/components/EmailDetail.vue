<script setup lang="ts">
import {
  NSpin,
  NEmpty,
  NScrollbar,
  NAlert,
  NButton,
  NIcon,
  NCollapse,
  NCollapseItem,
  NList,
  NListItem,
  NTooltip,
  useDialog,
} from 'naive-ui'
import { Icon } from '@iconify/vue'
import { OsService, ConfigService, TranslationService } from '../../bindings/wvmail/services'
import { useI18n } from '../i18n'
import { useAccountsStore } from '../stores/accounts'
import { useThemeStore } from '../stores/theme'
import { computed, ref, h, watch } from 'vue'
import { AttachmentInfo } from '../types/email'
import { getAttachmentIcon } from '../composables/attachment-icons'
import { formatDate, formatFileSize, getInitials } from '../composables/formats'
import { sanitizeHtmlRehype as sanitizeHtml, extractTextForTranslation } from '../composables/html-utils'
import EmailShadowBody from './EmailShadowBody.vue'

const { t } = useI18n()
const accountsStore = useAccountsStore()
const themeStore = useThemeStore()
const dialog = useDialog()

const isDarkMode = computed(() => themeStore.naiveTheme !== null)

const detail = computed(() => accountsStore.selectedEmailDetail)
const loading = computed(() => accountsStore.loadingDetail)

// Translation state
const translating = ref(false)
const translatedText = ref<string | null>(null)
const translationError = ref<string | null>(null)

// Clear translation when switching to a different email
watch(detail, () => {
  translatedText.value = null
  translationError.value = null
})

/**
 * Returns sender display parts. When both name and address are available,
 * renders as bold name + <email>. Otherwise falls back to whichever exists.
 */
function senderDisplay(detail: { from_name?: string; from_addr?: string; from?: string }): {
  name: string | null
  addr: string | null
  fallback: string | null
} {
  if (detail.from_name && detail.from_addr) {
    return { name: detail.from_name, addr: detail.from_addr, fallback: null }
  }
  if (detail.from_name) {
    return { name: null, addr: null, fallback: detail.from_name }
  }
  if (detail.from_addr) {
    return { name: null, addr: null, fallback: detail.from_addr }
  }
  if (detail.from) {
    return { name: null, addr: null, fallback: detail.from }
  }
  return { name: null, addr: null, fallback: 'Unknown' }
}

async function handleRefresh() {
  const account = accountsStore.selectedAccount
  if (account && accountsStore.selectedFolder && accountsStore.selectedEmailUid) {
    await accountsStore.refreshEmailBody(
      account,
      accountsStore.selectedFolder,
      accountsStore.selectedEmailUid,
    )
  }
  translatedText.value = null
  translationError.value = null
}

async function handleTranslate() {
  if (!detail.value) return
  translating.value = true
  translationError.value = null
  try {
    const cfg = await ConfigService.GetConfig()
    const tCfg = cfg.TranslationConfig || {}
    const targetLang = tCfg.DefaultTargetLang || 'zh-CN'
    const engine = tCfg.DefaultEngine || 'llm'

    const textToTranslate = detail.value.body_text
      ? detail.value.body_text
      : detail.value.body_html
        ? extractTextForTranslation(detail.value.body_html)
        : ''
    if (!textToTranslate) {
      translationError.value = 'No email content to translate'
      return
    }

    let llmCfg = null
    if (engine === 'llm' && cfg.LLMConfigs && cfg.LLMConfigs.length > 0) {
      llmCfg = cfg.LLMConfigs[0]
    }

    const result = await TranslationService.TranslateText(
      textToTranslate,
      targetLang,
      engine,
      llmCfg,
      tCfg.DeepLAuthKey || '',
    )
    translatedText.value = result
  } catch (e) {
    translationError.value = String(e)
    console.error('Translation failed', e)
  } finally {
    translating.value = false
  }
}

function clearTranslation() {
  translatedText.value = null
  translationError.value = null
}

async function handleDownloadAttachment(attachment: AttachmentInfo) {
  const path = await accountsStore.downloadAttachment(attachment)
  if (path) {
    dialog.success({
      title: 'Downloaded',
      content: `Saved to ${path}`,
      positiveText: 'OK',
    })
  }
}

function handleLinkClick(event: MouseEvent) {
  // Use composedPath() for shadow DOM support
  const path = event.composedPath?.() ?? []
  const target = (path[0] ?? event.target) as HTMLElement | null
  if (!target) return
  // Walk up to find the anchor element
  let el: HTMLElement | null = target
  while (el && el.tagName !== 'A') {
    el = el.parentElement
  }
  if (!el || !(el instanceof HTMLAnchorElement)) return

  const href = el.getAttribute('href')
  if (!href) return

  // Ignore mailto: links (handled by system)
  if (href.startsWith('mailto:')) return

  // Ignore anchor-only links
  if (href.startsWith('#') && !href.startsWith('http')) return

  event.preventDefault()

  const url = href.startsWith('http') ? href : el.href

  dialog.warning({
    title: t.value['mail.link.title'],
    content: () =>
      h('div', [
        h('p', { style: 'margin-bottom: 8px; opacity: 0.8' }, t.value['mail.link.body']),
        h(
          'p',
          {
            style:
              'word-break: break-all; font-size: 13px; background: var(--n-action-color); padding: 8px; border-radius: 4px;',
          },
          url,
        ),
      ]),
    positiveText: t.value['mail.link.open'],
    negativeText: t.value['cancel'],
    onPositiveClick: async () => {
      try {
        await OsService.OpenURL(url)
      } catch (e) {
        console.error('Failed to open URL', e)
      }
    },
  })
}
</script>

<template>
  <div class="email-detail">
    <NScrollbar style="height: 100%">
      <NSpin :show="loading">
        <NAlert
          v-if="accountsStore.detailError"
          type="error"
          closable
          style="margin: 12px"
          @close="accountsStore.detailError = null"
        >
          {{ accountsStore.detailError }}
        </NAlert>

        <template v-if="!detail && !loading">
          <NEmpty :description="t['mail.selectFolder']" style="padding: 4rem 0" />
        </template>

        <template v-if="detail">
          <div class="detail-header">
            <div style="display: flex; align-items: flex-start">
              <div class="detail-avatar-row">
                <div class="detail-avatar">
                  {{ getInitials(detail.from_name, detail.from_addr) }}
                </div>
                <div class="detail-sender-info">
                  <span class="detail-from" v-if="senderDisplay(detail).name">
                    <strong>{{ senderDisplay(detail).name }}</strong>
                    <span class="detail-email">&lt;{{ senderDisplay(detail).addr }}&gt;</span>
                  </span>
                  <span class="detail-from" v-else>{{ senderDisplay(detail).fallback }}</span>
                  <span class="detail-date">{{ formatDate(detail.date) }}</span>
                </div>
              </div>
              <NButton
                size="tiny"
                quaternary
                :loading="loading"
                style="margin-left: auto; flex-shrink: 0"
                @click="handleRefresh"
              >
                <template #icon>
                  <NIcon><Icon icon="ri:refresh-line" /></NIcon>
                </template>
              </NButton>
              <NTooltip trigger="hover">
                <template #trigger>
                  <NButton
                    size="tiny"
                    quaternary
                    :loading="translating"
                    style="flex-shrink: 0"
                    @click="translatedText ? clearTranslation() : handleTranslate()"
                  >
                    <template #icon>
                      <NIcon
                        ><Icon :icon="translatedText ? 'ri:close-line' : 'ri:translate-2'"
                      /></NIcon>
                    </template>
                  </NButton>
                </template>
                {{ translatedText ? t['mail.translation.close'] : t['mail.translation.translate'] }}
              </NTooltip>
            </div>

            <h2 class="detail-subject">{{ detail.subject || '(No Subject)' }}</h2>

            <div class="detail-meta">
              <div class="meta-row">
                <span class="meta-label">To:</span>
                <span class="meta-value">{{ (detail.to || []).join(', ') }}</span>
              </div>
              <div class="meta-row" v-if="detail.cc && detail.cc.length > 0">
                <span class="meta-label">Cc:</span>
                <span class="meta-value">{{ detail.cc.join(', ') }}</span>
              </div>
            </div>
          </div>

          <!-- Attachments -->
          <NCollapse
            v-if="detail.attachments && detail.attachments.length > 0"
            class="detail-attachments"
          >
            <NCollapseItem
              :title="`${detail.attachments.length} ${detail.attachments.length === 1 ? t['mail.attachment'] : t['mail.attachments']}`"
              name="attachments"
            >
              <NList :bordered="false" :hoverable="false" class="attachment-list">
                <NListItem v-for="att in detail.attachments" :key="att.filename + '-' + att.size">
                  <template #prefix>
                    <NIcon size="20" style="flex-shrink: 0">
                      <Icon :icon="getAttachmentIcon(att.mime_type, att.filename)" />
                    </NIcon>
                  </template>
                  <div class="attachment-details">
                    <span class="attachment-filename">{{ att.filename }}</span>
                    <span class="attachment-size">{{ formatFileSize(att.size) }}</span>
                  </div>
                  <template #suffix>
                    <NButton size="tiny" quaternary @click="handleDownloadAttachment(att)">
                      <template #icon>
                        <NIcon size="16"><Icon icon="ri:download-2-line" /></NIcon>
                      </template>
                    </NButton>
                  </template>
                </NListItem>
              </NList>
            </NCollapseItem>
          </NCollapse>

          <div class="detail-body" @click="handleLinkClick">
            <EmailShadowBody
              v-if="detail.body_html"
              :html="sanitizeHtml(detail.body_html)"
              :dark="isDarkMode"
            />
            <div v-else-if="detail.body_text" class="body-text">{{ detail.body_text }}</div>
            <div v-else class="body-empty" style="opacity: 0.4; text-align: center; padding: 2rem">
              (No content)
            </div>
          </div>

          <!-- Translation result -->
          <div v-if="translatedText" class="detail-body translation-result">
            <div class="translation-divider">{{ t['mail.translation.label'] }}</div>
            <div class="body-text">{{ translatedText }}</div>
          </div>
          <NAlert
            v-if="translationError"
            type="error"
            closable
            style="margin: 12px"
            @close="translationError = null"
          >
            {{ translationError }}
          </NAlert>
        </template>
      </NSpin>
    </NScrollbar>
  </div>
</template>

<style scoped>
.email-detail {
  height: 100%;
}

.detail-header {
  padding: 20px 24px 16px;
  border-bottom: 1px solid var(--n-border-color);
}

.detail-avatar-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.detail-avatar {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 600;
  font-size: 16px;
  color: #fff;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  flex-shrink: 0;
}

.detail-sender-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.detail-from {
  font-size: 14px;
  font-weight: 600;
}

.detail-email {
  font-size: 13px;
  font-weight: 400;
  opacity: 0.65;
  margin-left: 4px;
}

.detail-date {
  font-size: 12px;
  opacity: 0.6;
}

.detail-subject {
  font-size: 20px;
  font-weight: 700;
  margin: 0 0 12px;
  line-height: 1.3;
  word-break: break-word;
}

.detail-meta {
  font-size: 12px;
  opacity: 0.7;
}

.meta-row {
  display: flex;
  gap: 6px;
  margin-bottom: 2px;
}

.meta-label {
  font-weight: 600;
  min-width: 28px;
  opacity: 0.6;
}

.detail-body {
  padding: 20px 24px;
  line-height: 1.6;
  font-size: 14px;
  word-break: break-word;
}

.body-text {
  font-family: inherit;
  white-space: pre-wrap;
}

.translation-result {
  border-top: 1px solid var(--n-border-color);
  margin-top: 0;
}

.translation-divider {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  opacity: 0.5;
  margin-bottom: 8px;
}

/* --- Attachments section --- */
.detail-attachments {
  border-bottom: 1px solid var(--n-border-color);
  background: var(--n-action-color);
}

.detail-attachments :deep(.n-collapse-item__content-wrapper) {
  padding-bottom: 4px;
}

.attachment-list {
  background: transparent !important;
}

.attachment-list :deep(.n-list-item__prefix) {
  padding-left: 1rem;
}

.attachment-list :deep(.n-list-item__suffix) {
  padding-right: 1rem;
}

.attachment-details {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.attachment-filename {
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.attachment-size {
  font-size: 11px;
  opacity: 0.5;
}
</style>
