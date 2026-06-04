import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { EmailAccount, FolderInfo, EmailSummary } from '../types/email'
import { ConfigService, EmailService } from '../../bindings/wvmail/services'

export const useAccountsStore = defineStore('accounts', () => {
  const accounts = ref<EmailAccount[]>([])
  const loading = ref(false)
  const testingId = ref<string | null>(null)
  const foldersByAccount = ref<Record<string, FolderInfo[]>>({})
  const emailsByFolder = ref<Record<string, EmailSummary[]>>({})
  const selectedAccountId = ref<string | null>(null)
  const selectedFolder = ref<string | null>(null)

  const selectedAccount = computed(() => {
    if (!selectedAccountId.value) return null
    return accounts.value.find(a => a.ID === selectedAccountId.value) ?? null
  })

  const currentFolders = computed(() => {
    if (!selectedAccountId.value) return []
    return foldersByAccount.value[selectedAccountId.value] ?? []
  })

  const currentEmails = computed(() => {
    const key = `${selectedAccountId.value}:${selectedFolder.value}`
    return emailsByFolder.value[key] ?? []
  })

  async function loadAccounts() {
    try {
      accounts.value = await ConfigService.GetAccounts()
    } catch (e) {
      console.error('Failed to load accounts', e)
      accounts.value = []
    }
  }

  async function addAccount(account: EmailAccount) {
    try {
      await ConfigService.AddAccount(account)
      await loadAccounts()
    } catch (e) {
      console.error('Failed to add account', e)
      throw e
    }
  }

  async function removeAccount(id: string) {
    try {
      await ConfigService.RemoveAccount(id)
      if (selectedAccountId.value === id) {
        selectedAccountId.value = null
        selectedFolder.value = null
      }
      delete foldersByAccount.value[id]
      await loadAccounts()
    } catch (e) {
      console.error('Failed to remove account', e)
      throw e
    }
  }

  async function updateAccount(id: string, account: EmailAccount) {
    try {
      await ConfigService.UpdateAccount(id, account)
      await loadAccounts()
    } catch (e) {
      console.error('Failed to update account', e)
      throw e
    }
  }

  async function testConnection(account: EmailAccount): Promise<{ success: boolean; message: string }> {
    testingId.value = account.ID
    try {
      const [success, message] = await EmailService.TestConnection(account)
      return { success, message }
    } catch (e) {
      return { success: false, message: String(e) }
    } finally {
      testingId.value = null
    }
  }

  async function loadFolders(account: EmailAccount) {
    try {
      const folders = await EmailService.ListFolders(account)
      foldersByAccount.value[account.ID] = folders
    } catch (e) {
      console.error('Failed to load folders', e)
      foldersByAccount.value[account.ID] = []
    }
  }

  async function loadEmails(account: EmailAccount, folder: string, page = 1, pageSize = 20) {
    const key = `${account.ID}:${folder}`
    try {
      const emails = await EmailService.ListEmails(account, folder, page, pageSize)
      emailsByFolder.value[key] = emails
    } catch (e) {
      console.error('Failed to load emails', e)
      emailsByFolder.value[key] = []
    }
  }

  function selectAccount(id: string) {
    selectedAccountId.value = id
    selectedFolder.value = null
    const account = accounts.value.find(a => a.ID === id)
    if (account && !foldersByAccount.value[id]) {
      loadFolders(account)
    }
  }

  function selectFolder(accountId: string, folderName: string) {
    selectedAccountId.value = accountId
    selectedFolder.value = folderName
    const account = accounts.value.find(a => a.ID === accountId)
    if (account) {
      loadEmails(account, folderName)
    }
  }

  return {
    accounts,
    loading,
    testingId,
    foldersByAccount,
    emailsByFolder,
    selectedAccountId,
    selectedFolder,
    selectedAccount,
    currentFolders,
    currentEmails,
    loadAccounts,
    addAccount,
    removeAccount,
    updateAccount,
    testConnection,
    loadFolders,
    loadEmails,
    selectAccount,
    selectFolder,
  }
})
