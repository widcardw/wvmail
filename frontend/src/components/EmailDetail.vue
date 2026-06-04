<script setup lang="ts">
import { NSpin, NEmpty, NScrollbar, NAlert, NButton, NIcon } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { useI18n } from '../i18n'
import { useAccountsStore } from '../stores/accounts'
import { computed } from 'vue'

const { t } = useI18n()
const accountsStore = useAccountsStore()

const detail = computed(() => accountsStore.selectedEmailDetail)
const loading = computed(() => accountsStore.loadingDetail)

function formatDate(dateStr: string): string {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  return date.toLocaleString(undefined, {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function getInitials(name: string, addr: string): string {
  return (name || addr || '?').charAt(0).toUpperCase()
}

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
    await accountsStore.refreshEmailBody(account, accountsStore.selectedFolder, accountsStore.selectedEmailUid)
  }
}

/** Basic HTML sanitizer for email body rendering.
 *  Strips dangerous tags and event handler attributes. */
function sanitizeHtml(html: string): string {
  // Remove dangerous tags
  const dangerous = /<\s*\b(script|iframe|object|embed|frame|form|input|style)\b[^>]*>.*?<\/\s*\1\s*>|<(script|iframe|object|embed|frame|form|input|style)\b[^>]*\/?\s*>/gis
  let clean = html.replace(dangerous, '')
  // Remove on* event handlers
  clean = clean.replace(/\s+on\w+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)/gi, '')
  // Remove javascript: and data: URIs in href/src
  clean = clean.replace(/(?:href|src|action)\s*=\s*(?:"javascript:[^"]*"|'javascript:[^']*'|javascript:[^\s>]+)/gi, '')
  clean = clean.replace(/(?:href|src|action)\s*=\s*(?:"data:[^"]*"|'data:[^']*'|data:[^\s>]+)/gi, '')
  return clean
}
</script>

<template>
  <div class="email-detail">
    <NScrollbar style="height: 100%;">
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
          <NEmpty
            :description="t['mail.selectFolder']"
            style="padding: 4rem 0"
          />
        </template>

        <template v-if="detail">
          <div class="detail-header">
            <div style="display: flex; align-items: flex-start;">
              <div class="detail-avatar-row">
                <div class="detail-avatar">{{ getInitials(detail.from_name, detail.from_addr) }}</div>
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
                style="margin-left: auto; flex-shrink: 0;"
                @click="handleRefresh"
              >
                <template #icon>
                  <NIcon><Icon icon="ri:refresh-line" /></NIcon>
                </template>
              </NButton>
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

          <div class="detail-body">
            <div v-if="detail.body_html" class="body-html" v-html="sanitizeHtml(detail.body_html)" />
            <div v-else-if="detail.body_text" class="body-text">{{ detail.body_text }}</div>
            <div v-else class="body-empty" style="opacity: 0.4; text-align: center; padding: 2rem">
              (No content)
            </div>
          </div>
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
  white-space: pre-wrap;
  word-break: break-word;
}

.body-text {
  font-family: inherit;
}
</style>

<style>
/* Global styles for v-html rendered email body */
.body-html {
  line-height: 1.6;
  word-break: break-word;
}

.body-html img {
  max-width: 100%;
  height: auto;
}

.body-html a {
  color: var(--n-primary-color);
}

.body-html blockquote {
  margin: 8px 0;
  padding: 0 0 0 12px;
  border-left: 3px solid var(--n-border-color);
  opacity: 0.8;
}

.body-html pre {
  overflow-x: auto;
  background: var(--n-action-color);
  padding: 12px;
  border-radius: 4px;
}

.body-html table {
  max-width: 100%;
  overflow-x: auto;
  display: block;
}
</style>
