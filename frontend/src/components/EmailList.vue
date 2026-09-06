<script setup lang="ts">
import { Pagination } from '@ark-ui/vue/pagination'
import { computed } from 'vue'
import { useI18n } from '../i18n'
import { useAccountsStore } from '../stores/accounts'
import { getFolderDisplay } from '../types/email'
import { formatDateRecent, displaySender, truncateSubject } from '../composables/formats'
import AccountAvatar from './AccountAvatar.vue'

const { t, locale } = useI18n()
const accountsStore = useAccountsStore()

const loading = computed(() =>
  accountsStore.isSmartFolder ? accountsStore.smartFolderLoading : accountsStore.loading,
)
const emails = computed(() => accountsStore.currentEmails)
const selectedUid = computed(() => accountsStore.selectedEmailUid)
const pageCount = computed(() => {
  const total = accountsStore.currentTotal
  const ps = accountsStore.pageSize
  return total > 0 ? Math.ceil(total / ps) : 1
})

const folderDisplay = computed(() => {
  if (accountsStore.isSmartFolder) return { label: String(accountsStore.smartFolder) }
  if (accountsStore.selectedFolder)
    return getFolderDisplay(accountsStore.selectedFolder, locale.value)
  return { label: '', icon: '', rank: 99 }
})

const showHeader = computed(() => !!accountsStore.selectedFolder || accountsStore.isSmartFolder)

function handleClick(email: { uid: number; _accountId?: string; _folderName?: string }) {
  const smartF = accountsStore.smartFolder
  if (smartF && email._accountId && email._folderName) {
    const account = accountsStore.accounts.find((a) => a.ID === email._accountId)
    if (account) accountsStore.selectEmail(account, email._folderName, email.uid)
    return
  }
  const account = accountsStore.selectedAccount
  if (account && accountsStore.selectedFolder) {
    accountsStore.selectEmail(account, accountsStore.selectedFolder, email.uid)
  }
}

function smartSenderId(email: { uid: number; _accountId?: string; _folderName?: string }) {
  return email._accountId ?? ''
}

function onPageChange(details: unknown) {
  const page = (details as { page?: number }).page
  if (typeof page === 'number' && page >= 1) accountsStore.goToPage(page)
}
</script>

<template>
  <div class="email-list">
    <div class="list-header" v-if="showHeader">
      <span class="folder-label" :title="accountsStore.selectedFolder || folderDisplay.label">{{
        folderDisplay.label
      }}</span>
      <button
        v-if="!accountsStore.isSmartFolder"
        type="button"
        class="icon-btn"
        :title="t['mail.loading'] === '' ? 'refresh' : 'refresh'"
        :disabled="accountsStore.refreshing"
        @click="accountsStore.forceRefreshEmails()"
      >
        <i
          class="i-ri:refresh-line"
          :class="{ spin: accountsStore.refreshing }"
          aria-hidden="true"
        />
      </button>
      <span class="spacer" />
      <Pagination.Root
        v-if="pageCount > 1"
        :count="accountsStore.currentTotal"
        :page-size="accountsStore.pageSize"
        :page="accountsStore.currentPage"
        :sibling-count="1"
        class="pager-root"
        @page-change="onPageChange"
      >
        <Pagination.PrevTrigger class="pg"
          ><i class="i-ri:arrow-left-s-line"
        /></Pagination.PrevTrigger>
        <Pagination.Context v-slot="pgCtx">
          <template v-for="(pg, i) in pgCtx.pages" :key="i">
            <Pagination.Item v-if="pg.type === 'page'" v-bind="pg" class="pg pg--num">
              {{ pg.value }}
            </Pagination.Item>
            <Pagination.Ellipsis v-else class="pg pg--ell">…</Pagination.Ellipsis>
          </template>
        </Pagination.Context>
        <Pagination.NextTrigger class="pg"
          ><i class="i-ri:arrow-right-s-line"
        /></Pagination.NextTrigger>
      </Pagination.Root>
    </div>

    <div class="body">
      <div v-if="loading" class="spin-row"><span class="spin" /></div>
      <p v-else-if="emails.length === 0" class="empty">{{ t['mail.noEmails'] }}</p>

      <template v-else>
        <div
          v-for="email in emails"
          :key="email.uid"
          class="email-item"
          :class="{ unread: !email.is_read, selected: selectedUid === email.uid }"
          @click="handleClick(email)"
        >
          <div class="email-row">
            <span v-if="accountsStore.isSmartFolder" class="email-avatar"
              ><AccountAvatar v-if="smartSenderId(email)" :account-id="smartSenderId(email)"
            /></span>
            <span class="email-body">
              <span class="email-header">
                <span class="email-from">{{ displaySender(email) }}</span>
                <span class="email-date">{{ formatDateRecent(email.date) }}</span>
              </span>
              <span class="email-subject">{{ truncateSubject(email.subject) }}</span>
            </span>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.email-list {
  height: 100%;
  width: 100%;
  display: flex;
  flex-direction: column;
  min-width: 0;
  background: var(--n-body-color);
}
.list-header {
  flex-shrink: 0;
  padding: 6px 10px;
  border-bottom: 1px solid var(--n-border-color);
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}
.folder-label {
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  white-space: nowrap;
  color: var(--n-text-color);
}
.icon-btn {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: none;
  background: transparent;
  border-radius: 5px;
  cursor: pointer;
  color: var(--n-text-color);
}
.icon-btn:hover {
  background: var(--n-sidebar-hover);
}
.icon-btn:disabled {
  opacity: 50%;
}
.spacer {
  flex: 1;
}
.spacer {
  flex: 1;
}
.pager-root {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.pg {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 22px;
  height: 22px;
  padding: 0;
  border: 1px solid transparent;
  background: transparent;
  border-radius: 5px;
  cursor: pointer;
  font-size: 13px;
  color: var(--n-text-color);
}
.pg:hover {
  background: var(--n-sidebar-hover);
}
.pg[data-selected] {
  background: var(--n-primary-color);
  color: #fff;
}
.pg[data-disabled],
.pg:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}
.pg i {
  font-size: 16px;
}
.pg--ell {
  border: none;
  cursor: default;
  opacity: 0.6;
}
.body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}
.spin-row {
  padding: 32px 0;
  display: flex;
  justify-content: center;
}
.spin {
  width: 18px;
  height: 18px;
  border: 2px solid var(--n-border-color);
  border-top-color: var(--n-primary-color);
  border-radius: 50%;
  animation: rot 0.7s linear infinite;
}
@keyframes rot {
  to {
    transform: rotate(360deg);
  }
}
.rot {
  animation: rot 0.8s linear infinite;
}
.empty {
  padding: 40px 0;
  text-align: center;
  color: var(--color-text-3);
  font-size: 13px;
}
.email-item {
  cursor: pointer;
  border-bottom: 1px solid var(--n-border-color);
  border-left: 2px solid transparent;
  color: var(--n-text-color);
}
.email-item:hover {
  background: var(--n-sidebar-hover);
}
.email-item.selected {
  border-left-color: var(--n-primary-color);
  background: var(--n-sidebar-active);
}
.email-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  overflow: hidden;
}
.email-avatar {
  flex-shrink: 0;
  display: flex;
}
.email-body {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.email-header {
  display: flex;
  align-items: center;
  gap: 8px;
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
  flex-shrink: 0;
}
.email-subject {
  font-size: 12px;
  opacity: 0.75;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-top: 2px;
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
