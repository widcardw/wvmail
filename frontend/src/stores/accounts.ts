import { defineStore } from 'pinia'
import { ref, computed, onMounted, onUnmounted } from 'vue'
import type {
  // EmailAccount,
  FolderInfo,
  EmailSummary,
  EmailDetail,
  AttachmentInfo,
  EmailPageResponse,
} from '~/types/email'
import { ConfigService, EmailService, OsService, EmailAccount } from '#/wvmail/services'
import { Events } from '@wailsio/runtime'

export const useAccountsStore = defineStore('accounts', () => {
  const accounts = ref<EmailAccount[]>([])
  const loading = ref(false)
  const refreshing = ref(false) // force-refresh loading state
  const testingId = ref<string | null>(null)
  const foldersByAccount = ref<Record<string, FolderInfo[]>>({})
  const emailsByFolder = ref<Record<string, EmailSummary[]>>({})
  const emailTotalByFolder = ref<Record<string, number>>({})
  const currentPageByFolder = ref<Record<string, number>>({})
  const pageSize = 20
  const selectedAccountId = ref<string | null>(null)
  const selectedFolder = ref<string | null>(null)

  // Email detail state
  const selectedEmailUid = ref<number | null>(null)
  const selectedEmailDetail = ref<EmailDetail | null>(null)
  const loadingDetail = ref(false)
  const detailError = ref<string | null>(null)

  const selectedAccount = computed(() => {
    if (!selectedAccountId.value) return null
    return accounts.value.find((a) => a.ID === selectedAccountId.value) ?? null
  })

  const currentFolders = computed(() => {
    if (!selectedAccountId.value) return []
    return foldersByAccount.value[selectedAccountId.value] ?? []
  })

  const currentEmails = computed(() => {
    const key = `${selectedAccountId.value}:${selectedFolder.value}`
    return emailsByFolder.value[key] ?? []
  })

  const currentTotal = computed(() => {
    const key = `${selectedAccountId.value}:${selectedFolder.value}`
    return emailTotalByFolder.value[key] ?? 0
  })

  const currentPage = computed(() => {
    const key = `${selectedAccountId.value}:${selectedFolder.value}`
    return currentPageByFolder.value[key] ?? 1
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
        selectedEmailUid.value = null
        selectedEmailDetail.value = null
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

  async function testConnection(
    account: EmailAccount,
  ): Promise<{ success: boolean; message: string }> {
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

  async function loadEmails(account: EmailAccount, folder: string, page = 1, ps = pageSize) {
    const key = `${account.ID}:${folder}`
    loading.value = true
    try {
      const resp: EmailPageResponse | null = await EmailService.ListEmails(
        account,
        folder,
        page,
        ps,
      )
      emailsByFolder.value[key] = resp?.emails ?? []
      emailTotalByFolder.value[key] = resp?.total ?? 0
      if (page === 1) {
        currentPageByFolder.value[key] = 1
      }
    } catch (e) {
      console.error('Failed to load emails', e)
      emailsByFolder.value[key] = []
    } finally {
      loading.value = false
    }
  }

  /** Force-refresh the current folder from the IMAP server, bypassing cache. */
  async function forceRefreshEmails() {
    const account = selectedAccount.value
    const folder = selectedFolder.value
    if (!account || !folder) return

    const key = `${account.ID}:${folder}`
    refreshing.value = true
    try {
      const resp: EmailPageResponse | null = await EmailService.RefreshEmails(
        account,
        folder,
        1,
        pageSize,
      )
      emailsByFolder.value[key] = resp?.emails ?? []
      emailTotalByFolder.value[key] = resp?.total ?? 0
      currentPageByFolder.value[key] = 1
    } catch (e) {
      console.error('Failed to refresh emails', e)
    } finally {
      refreshing.value = false
    }
  }

  // Listen for background sync events to refresh the UI automatically
  let unlisten: (() => void) | null = null
  onMounted(() => {
    unlisten = Events.On('emails-synced', (event) => {
      const syncedAccountId = event.data as string
      // If the synced account is currently selected, refresh the current folder
      if (syncedAccountId === selectedAccountId.value && selectedFolder.value) {
        const account = selectedAccount.value
        const folder = selectedFolder.value
        if (account && folder) {
          const key = `${account.ID}:${folder}`
          EmailService.ListEmails(account, folder, 1, pageSize)
            .then((resp: EmailPageResponse | null) => {
              emailsByFolder.value[key] = resp?.emails ?? []
              emailTotalByFolder.value[key] = resp?.total ?? 0
              currentPageByFolder.value[key] = 1
            })
            .catch(() => {})
        }
      }
    })
  })
  onUnmounted(() => {
    unlisten?.()
  })

  async function selectEmail(account: EmailAccount, folder: string, uid: number) {
    selectedEmailUid.value = uid
    selectedEmailDetail.value = null
    loadingDetail.value = true
    detailError.value = null
    try {
      const detail = await EmailService.FetchEmail(account, folder, uid)
      selectedEmailDetail.value = detail
    } catch (e) {
      detailError.value = String(e)
      console.error('Failed to fetch email detail', e)
    } finally {
      loadingDetail.value = false
    }
  }

  async function refreshEmailBody(account: EmailAccount, folder: string, uid: number) {
    loadingDetail.value = true
    detailError.value = null
    try {
      const detail = await EmailService.RefreshEmailBody(account, folder, uid)
      selectedEmailDetail.value = detail
    } catch (e) {
      detailError.value = String(e)
      console.error('Failed to refresh email body', e)
    } finally {
      loadingDetail.value = false
    }
  }

  async function downloadAttachment(attachment: AttachmentInfo): Promise<string | null> {
    const account = selectedAccount.value
    const folder = selectedFolder.value
    const uid = selectedEmailUid.value
    if (!account || !folder || !uid) {
      detailError.value = 'No email selected'
      return null
    }

    try {
      // Show save dialog
      const savePath = await OsService.SaveFileDialog(attachment.filename)
      // Download attachment to chosen path
      await EmailService.DownloadAttachment(account, folder, uid, attachment.part_path, savePath)
      return savePath
    } catch (e) {
      detailError.value = String(e)
      console.error('Failed to download attachment', e)
      return null
    }
  }

  function selectAccount(id: string) {
    selectedAccountId.value = id
    selectedFolder.value = null
    selectedEmailUid.value = null
    selectedEmailDetail.value = null
    const account = accounts.value.find((a) => a.ID === id)
    if (account && !foldersByAccount.value[id]) {
      loadFolders(account)
    }
  }

  function selectFolder(accountId: string, folderName: string) {
    selectedAccountId.value = accountId
    selectedFolder.value = folderName
    selectedEmailUid.value = null
    selectedEmailDetail.value = null
    const account = accounts.value.find((a) => a.ID === accountId)
    if (account) {
      currentPageByFolder.value[`${accountId}:${folderName}`] = 1
      loadEmails(account, folderName)
    }
  }

  function goToPage(page: number) {
    const account = selectedAccount.value
    const folder = selectedFolder.value
    if (!account || !folder) return
    const key = `${account.ID}:${folder}`
    currentPageByFolder.value[key] = page
    loadEmails(account, folder, page)
  }

  return {
    accounts,
    loading,
    refreshing,
    testingId,
    foldersByAccount,
    emailsByFolder,
    selectedAccountId,
    selectedFolder,
    selectedEmailUid,
    selectedEmailDetail,
    loadingDetail,
    detailError,
    selectedAccount,
    currentFolders,
    currentEmails,
    currentTotal,
    currentPage,
    pageSize,
    loadAccounts,
    addAccount,
    removeAccount,
    updateAccount,
    testConnection,
    loadFolders,
    loadEmails,
    forceRefreshEmails,
    goToPage,
    selectEmail,
    refreshEmailBody,
    downloadAttachment,
    selectAccount,
    selectFolder,
  }
})
