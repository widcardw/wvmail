<script setup lang="ts">
import { NEmpty, NSpin, NScrollbar, NButton, NIcon, NPagination, NTooltip } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { useI18n } from '../i18n'
import { useAccountsStore } from '../stores/accounts'
import { getFolderDisplay } from '../types/email'
import { computed, h } from 'vue'
import { useThemeVars } from 'naive-ui'
import { formatDateRecent, displaySender, truncateSubject } from '../composables/formats'

const themeVars = useThemeVars()
const { primaryColor, borderColor } = themeVars.value

const { t, locale } = useI18n()
const accountsStore = useAccountsStore()

const loading = computed(() => {
  if (accountsStore.isSmartFolder) return accountsStore.smartFolderLoading
  return accountsStore.emailsByFolder === undefined
})
const emails = computed(() => accountsStore.currentEmails)
const selectedUid = computed(() => accountsStore.selectedEmailUid)
const pageCount = computed(() => {
  if (accountsStore.isSmartFolder) return 1
  const total = accountsStore.currentTotal
  const ps = accountsStore.pageSize
  return total > 0 ? Math.ceil(total / ps) : 1
})

const folderDisplay = computed(() => {
  if (accountsStore.isSmartFolder) {
    return { label: accountsStore.smartFolder, icon: '' }
  }
  if (accountsStore.selectedFolder) {
    return getFolderDisplay(accountsStore.selectedFolder, locale.value)
  }
  return { label: '', icon: '', rank: 99 }
})

function handleClick(email: { uid: number; _accountId?: string; _folderName?: string }) {
  const smartF = accountsStore.smartFolder
  if (smartF && email._accountId && email._folderName) {
    const account = accountsStore.accounts.find((a) => a.ID === email._accountId)
    if (account) {
      accountsStore.selectEmail(account, email._folderName, email.uid)
    }
    return
  }
  const account = accountsStore.selectedAccount
  if (account && accountsStore.selectedFolder) {
    accountsStore.selectEmail(account, accountsStore.selectedFolder, email.uid)
  }
}

function getEmailAccountIcon(email: { _accountId?: string }) {
  let accountName = ''
  if (accountsStore.isSmartFolder && email._accountId) {
    const acct = accountsStore.accounts.find((a) => a.ID === email._accountId)
    if (acct) accountName = acct.Name || acct.Email
  } else if (accountsStore.selectedAccount) {
    accountName = accountsStore.selectedAccount.Name || accountsStore.selectedAccount.Email
  }
  return accountName ? accountName.charAt(0).toUpperCase() : null
}
</script>

<template>
  <div class="email-list">
    <div class="list-header" v-if="accountsStore.selectedFolder || accountsStore.isSmartFolder">
      <NTooltip placement="bottom" trigger="hover">
        <template #trigger>
          <span class="folder-label">{{ folderDisplay.label }}</span>
        </template>
        {{ accountsStore.selectedFolder || (accountsStore.isSmartFolder ? folderDisplay.label + ' (all accounts)' : '') }}
      </NTooltip>
      <NButton
        size="tiny"
        quaternary
        :loading="accountsStore.refreshing"
        class="refresh-btn"
        @click="accountsStore.forceRefreshEmails()"
        v-if="!accountsStore.isSmartFolder"
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
                  <span
                    v-if="accountsStore.isSmartFolder"
                    class="email-avatar"
                  >{{ getEmailAccountIcon(email) }}</span>
                  <span class="email-from">{{ displaySender(email) }}</span>
                  <span class="email-date">{{ formatDateRecent(email.date) }}</span>
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

.email-avatar {
  width: 16px;
  height: 16px;
  min-width: 16px;
  border-radius: 50%;
  background: var(--n-action-color);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 8px;
  font-weight: 700;
  margin-right: 4px;
  color: var(--n-text-color);
  line-height: 1;
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
