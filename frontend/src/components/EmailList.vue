<script setup lang="ts">
import { NEmpty, NSpin, NScrollbar } from 'naive-ui'
import { useI18n } from '../i18n'
import { useAccountsStore } from '../stores/accounts'
import { computed } from 'vue'

const { t } = useI18n()
const accountsStore = useAccountsStore()

const loading = computed(() => accountsStore.emailsByFolder === undefined)
const emails = computed(() => accountsStore.currentEmails)
const selectedUid = computed(() => accountsStore.selectedEmailUid)

function formatDate(dateStr: string): string {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  const now = new Date()
  const isToday = date.toDateString() === now.toDateString()
  if (isToday) {
    return date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
  }
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

function truncateSubject(subject: string, maxLen = 60): string {
  if (!subject) return '(No Subject)'
  return subject.length > maxLen ? subject.slice(0, maxLen) + '…' : subject
}

function handleClick(email: { uid: number }) {
  const account = accountsStore.selectedAccount
  if (account && accountsStore.selectedFolder) {
    accountsStore.selectEmail(account, accountsStore.selectedFolder, email.uid)
  }
}
</script>

<template>
  <div class="email-list">
    <div class="list-header" v-if="accountsStore.selectedFolder">
      <span class="folder-label">{{ accountsStore.selectedFolder }}</span>
    </div>

    <NScrollbar style="height: 100%;">
      <NSpin :show="loading">
        <NEmpty
          v-if="!loading && emails.length === 0"
          :description="t['mail.noEmails']"
          style="padding: 48px 0"
        />

        <div v-else class="email-items">
          <div
            v-for="email in emails"
            :key="email.uid"
            class="email-item"
            :class="{ unread: !email.is_read, selected: selectedUid === email.uid }"
            @click="handleClick(email)"
          >
            <div class="email-row">
              <div class="email-avatar">
                {{ (email.from_name || email.from_addr || email.from || '?').charAt(0).toUpperCase() }}
              </div>
              <div class="email-body">
                <div class="email-header">
                  <span class="email-from">{{ email.from_name || email.from_addr || email.from || 'Unknown' }}</span>
                  <span class="email-date">{{ formatDate(email.date) }}</span>
                </div>
                <div class="email-subject">{{ truncateSubject(email.subject) }}</div>
              </div>
            </div>
          </div>
        </div>
      </NSpin>
    </NScrollbar>
  </div>
</template>

<style scoped>
.email-list {
  height: 100%;
  width: 100%;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.list-header {
  flex-shrink: 0;
  font-size: 13px;
  font-weight: 600;
  opacity: 0.7;
  padding: 8px 16px;
  border-bottom: 1px solid var(--n-border-color);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.email-item {
  cursor: pointer;
  transition: background-color 0.15s;
  border-bottom: 1px solid var(--n-border-color);
}

.email-item:hover {
  background-color: var(--n-action-color);
}

.email-item.selected {
  background-color: var(--n-primary-color-hover);
}

.email-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  overflow: hidden;
}

.email-body {
  flex: 1;
  min-width: 0;
  overflow: hidden;
}

.email-header {
  display: flex;
  align-items: center;
  width: 100%;
}

.email-from {
  font-size: 13px;
  font-weight: 500;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.email-date {
  font-size: 11px;
  opacity: 0.6;
  white-space: nowrap;
  margin-left: auto;
  flex-shrink: 0;
}

.email-subject {
  font-size: 12px;
  opacity: 0.7;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-top: 2px;
  max-width: 100%;
}

.email-item.unread .email-from {
  font-weight: 700;
}

.email-item.unread .email-subject {
  font-weight: 600;
}

.email-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 600;
  font-size: 13px;
  color: #fff;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  flex-shrink: 0;
}
</style>
