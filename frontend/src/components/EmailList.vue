<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '../i18n'
import { useAccountsStore } from '../stores/accounts'
import { getFolderDisplay } from '../types/email'
import EmailListHeader from './EmailListHeader.vue'
import EmailListItem from './EmailListItem.vue'

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

</script>

<template>
  <div class="email-list">
    <EmailListHeader
      v-if="showHeader"
      :label="folderDisplay.label"
      :title="accountsStore.selectedFolder || folderDisplay.label"
      :show-refresh="!accountsStore.isSmartFolder"
      :refreshing="accountsStore.refreshing"
      :page-count="pageCount"
      :total="accountsStore.currentTotal"
      :page-size="accountsStore.pageSize"
      :page="accountsStore.currentPage"
      @refresh="accountsStore.forceRefreshEmails()"
      @page-change="accountsStore.goToPage"
    />

    <div class="body">
      <div v-if="loading" class="spin-row"><span class="spinner" /></div>
      <p v-else-if="emails.length === 0" class="empty">{{ t['mail.noEmails'] }}</p>

      <template v-else>
        <EmailListItem
          v-for="email in emails"
          :key="email.uid"
          :email="email"
          :selected="selectedUid === email.uid"
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
