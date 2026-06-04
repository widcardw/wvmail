<script setup lang="ts">
import { NSpin, NEmpty, NScrollbar } from 'naive-ui'
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

function getInitials(name: string): string {
  return (name || '?').charAt(0).toUpperCase()
}
</script>

<template>
  <div class="email-detail">
    <NScrollbar style="height: 100%;">
      <NSpin :show="loading">
        <template v-if="!detail && !loading">
          <NEmpty
            :description="t['mail.selectFolder']"
            style="padding: 4rem 0"
          />
        </template>

        <template v-if="detail">
          <div class="detail-header">
            <div class="detail-avatar-row">
              <div class="detail-avatar">{{ getInitials(detail.from) }}</div>
              <div class="detail-sender-info">
                <span class="detail-from">{{ detail.from }}</span>
                <span class="detail-date">{{ formatDate(detail.date) }}</span>
              </div>
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
            <div v-if="detail.body_text" class="body-text">{{ detail.body_text }}</div>
            <div v-else-if="detail.body_html" class="body-html" v-html="detail.body_html" />
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
