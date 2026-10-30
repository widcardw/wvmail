<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '~/i18n'
import { useAccountsStore } from '~/stores/accounts'
import { useSearchStore } from '~/stores/search'
import { getFolderDisplay } from '~/types/email'
import EmailListHeader from './EmailListHeader.vue'
import EmailListItem from './EmailListItem.vue'

const { t, locale } = useI18n()
const accountsStore = useAccountsStore()
const searchStore = useSearchStore()

const loading = computed(() => {
  if (searchStore.searchActive) return searchStore.searchLoading
  if (
    accountsStore.smartFolder === 'starred' &&
    accountsStore.currentTotal === 0 &&
    accountsStore.smartFolderRefreshing
  ) {
    // The first visit has no snapshot to paint, so keep the empty state from
    // flashing while the initial per-account searches run in the background.
    return true
  }
  return accountsStore.isSmartFolder ? accountsStore.smartFolderLoading : accountsStore.loading
})
const refreshing = computed(() =>
  accountsStore.isSmartFolder
    ? accountsStore.smartFolderRefreshing
    : accountsStore.refreshingOpenFolder,
)
const emails = computed(() => accountsStore.currentEmails)
/**
 * Selection is account+folder+UID, not UID alone: aggregate and search views can
 * contain the same UID in several mailboxes, and comparing only the number marked
 * unrelated rows as selected.
 */
const selectedEmailKey = computed(() => {
  const uid = accountsStore.selectedEmailUid
  const accountId = accountsStore.selectedEmailAccountId
  const folder = accountsStore.selectedEmailFolder
  return uid != null && accountId && folder ? `${accountId}\u200b${folder}\u200b${uid}` : ''
})

function emailKey(email: { uid: number; account_id?: string; folder_name?: string }): string {
  const accountId = email.account_id || accountsStore.selectedAccountId
  const folder = email.folder_name || accountsStore.selectedFolder
  return accountId && folder ? `${accountId}\u200b${folder}\u200b${email.uid}` : ''
}

function isSelected(email: { uid: number; account_id?: string; folder_name?: string }): boolean {
  return selectedEmailKey.value !== '' && selectedEmailKey.value === emailKey(email)
}

const pageCount = computed(() => {
  const total = accountsStore.currentTotal
  const ps = accountsStore.pageSize
  return total > 0 ? Math.ceil(total / ps) : 1
})

const folderDisplay = computed(() => {
  if (accountsStore.isSmartFolder) {
    const labels: Record<string, string> = {
      inbox: t.value['sidebar.all_inbox'],
      sent: t.value['sidebar.all_sent'],
      drafts: t.value['sidebar.all_drafts'],
      starred: t.value['sidebar.all_starred'],
    }
    return { label: labels[String(accountsStore.smartFolder)] ?? String(accountsStore.smartFolder) }
  }
  if (accountsStore.selectedFolder)
    return getFolderDisplay(accountsStore.selectedFolder, locale.value)
  if (searchStore.searchActive) return { label: t.value['search.results'] }
  return { label: '', icon: '', rank: 99 }
})

const showHeader = computed(
  () => !!accountsStore.selectedFolder || accountsStore.isSmartFolder || searchStore.searchActive,
)

function handleClick(email: { uid: number; account_id?: string; folder_name?: string }) {
  const smartF = accountsStore.smartFolder
  if ((smartF || searchStore.searchActive) && email.account_id && email.folder_name) {
    const account = accountsStore.accounts.find((a) => a.ID === email.account_id)
    if (account) accountsStore.selectEmail(account, email.folder_name, email.uid)
    return
  }
  const account = accountsStore.selectedAccount
  if (account && accountsStore.selectedFolder) {
    accountsStore.selectEmail(account, accountsStore.selectedFolder, email.uid)
  }
}
</script>

<template>
  <div class="email-list">
    <EmailListHeader
      v-if="showHeader"
      :label="folderDisplay.label"
      :title="accountsStore.selectedFolder || folderDisplay.label"
      :show-refresh="
        accountsStore.smartFolder === 'starred' ||
        (!accountsStore.isSmartFolder && !searchStore.searchActive)
      "
      :refresh-title="
        accountsStore.smartFolder === 'starred' ? t['mail.refresh.starred'] : undefined
      "
      :refreshing="refreshing"
      :page-count="pageCount"
      :total="accountsStore.currentTotal"
      :page-size="accountsStore.pageSize"
      :page="accountsStore.currentPage"
      @refresh="
        accountsStore.smartFolder === 'starred'
          ? accountsStore.refreshStarred()
          : accountsStore.forceRefreshEmails()
      "
      @page-change="accountsStore.goToPage"
    />

    <div class="body">
      <div v-if="loading" class="spin-row"><span class="spinner" /></div>
      <p v-else-if="emails.length === 0" class="empty">{{ t['mail.noEmails'] }}</p>

      <template v-else>
        <EmailListItem
          v-for="email in emails"
          :key="`${email.account_id || accountsStore.selectedAccountId}:${email.folder_name || accountsStore.selectedFolder}:${email.uid}`"
          :email="email"
          :selected="isSelected(email)"
          :smart="accountsStore.isSmartFolder"
          @select="handleClick"
        />
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
.empty {
  padding: 40px 0;
  text-align: center;
  color: var(--color-text-3);
  font-size: 13px;
}
</style>
