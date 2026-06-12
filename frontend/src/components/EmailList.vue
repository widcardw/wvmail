<script setup lang="ts">
import { NEmpty, NSpin, NScrollbar, NButton, NIcon, NPagination, NTooltip } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { useI18n } from '../i18n'
import { useAccountsStore } from '../stores/accounts'
import { getFolderDisplay } from '../types/email'
import { computed } from 'vue'
import { useThemeVars } from 'naive-ui'

const themeVars = useThemeVars()
const { primaryColor, primaryColorHover, borderColor } = themeVars.value

const { t, locale } = useI18n()
const accountsStore = useAccountsStore()

const loading = computed(() => accountsStore.emailsByFolder === undefined)
const emails = computed(() => accountsStore.currentEmails)
const selectedUid = computed(() => accountsStore.selectedEmailUid)
const pageCount = computed(() => {
  const total = accountsStore.currentTotal
  const ps = accountsStore.pageSize
  return total > 0 ? Math.ceil(total / ps) : 1
})

const folderDisplay = computed(() =>
  accountsStore.selectedFolder
    ? getFolderDisplay(accountsStore.selectedFolder, locale.value)
    : { label: '', icon: '', rank: 99 },
)

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

function displaySender(email: { from_name?: string; from_addr?: string; from?: string }): string {
  if (email.from_name) return email.from_name
  if (email.from_addr) return email.from_addr.split('@')[0]
  if (email.from) return email.from
  return 'Unknown'
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
      <NTooltip placement="bottom" trigger="hover">
        <template #trigger>
          <span class="folder-label">{{ folderDisplay.label }}</span>
        </template>
        {{ accountsStore.selectedFolder }}
      </NTooltip>
      <NButton
        size="tiny"
        quaternary
        :loading="accountsStore.refreshing"
        class="refresh-btn"
        @click="accountsStore.forceRefreshEmails()"
      >
        <template #icon>
          <NIcon><Icon icon="ri:refresh-line" /></NIcon>
        </template>
      </NButton>
      <div class="list-header-spacer"></div>
      <NPagination
        v-if="pageCount > 1"
        v-model:page="accountsStore.currentPage"
        :page-count="pageCount"
        :disabled="accountsStore.loading"
        simple
        @update:page="accountsStore.goToPage"
      />
    </div>

    <NScrollbar style="height: 100%">
      <NSpin :show="accountsStore.loading">
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
              <div class="sel-marker"></div>
              <div class="unread-marker" :class="{ visible: !email.is_read }"></div>
              <div class="email-body">
                <div class="email-header">
                  <span class="email-from">{{ displaySender(email) }}</span>
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
  padding: 6px 16px;
  border-bottom: 1px solid v-bind(borderColor);
  display: flex;
  align-items: center;
  gap: 6px;
}

.list-header-spacer {
  flex: 1;
}

.folder-label {
  text-transform: uppercase;
  letter-spacing: 0.5px;
  white-space: nowrap;
}

.refresh-btn {
  flex-shrink: 0;
}

.email-item {
  cursor: pointer;
  transition: border-color 0.1s;
  border-bottom: 1px solid v-bind(borderColor);
  border-left: 2px solid transparent;
}

.email-item:hover {
  border-left-color: v-bind(borderColor);
}

.email-item.selected {
  border-left-color: v-bind(primaryColor);
}

.email-row {
  display: flex;
  align-items: center;
  gap: 5px;
  padding-top: 12px;
  padding-bottom: 12px;
  padding-right: 8px;
  overflow: hidden;
}

.unread-marker {
  width: 3px;
  height: 24px;
  border-radius: 2px;
  flex-shrink: 0;
  background: transparent;
  transition: background-color 0.15s;
}

.unread-marker.visible {
  background: var(--primary-color);
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

.email-item:not(.unread) .email-from {
  opacity: 0.85;
}
</style>
