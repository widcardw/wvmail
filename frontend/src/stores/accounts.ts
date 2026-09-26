import { defineStore } from 'pinia'
import { ref, computed, onMounted, onUnmounted } from 'vue'
import type {
  // EmailAccount,
  FolderInfo,
  EmailSummary,
  EmailDetail,
  AttachmentInfo,
  EmailPageResponse,
  EmailSearchFilter,
} from '~/types/email'
import { ConfigService, EmailService, OsService, EmailAccount } from '#/wvmail/services'
import { useFeedbackStore } from '~/stores/feedback'
import { useI18n } from '~/i18n'
import { Events } from '@wailsio/runtime'

export const useAccountsStore = defineStore('accounts', () => {
  const feedback = useFeedbackStore()
  const { t: i18n } = useI18n()

  // Fetch failures used to be console.error only, so a timed-out IMAP connect
  // looked like "nothing happened" for a minute and then silently kept stale
  // mail. Surface the reason to the user as well.
  function reportFetchFailure(action: string, e: unknown) {
    console.error(action, e)
    const detail = e instanceof Error ? e.message : String(e)
    feedback.error(`${i18n.value['mail.refreshFailed']}: ${detail}`)
  }

  const accounts = ref<EmailAccount[]>([])
  const avatarRevision = ref<Record<string, number>>({})
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

  // Global cached-email search state. Search results carry account/folder
  // metadata so they can be opened from the aggregate result list.
  const searchActive = ref(false)
  const searchResults = ref<EmailSummary[]>([])
  const searchTotal = ref(0)
  const searchPage = ref(1)
  const searchLoading = ref(false)
  const searchQuery = ref<EmailSearchFilter[]>([])
  const searchAccountId = ref('')
  const searchFolderName = ref('')
  let searchRequestId = 0

  // Smart folder (aggregate view across all accounts)
  const smartFolder = ref<string | null>(null)
  const smartEmails = ref<EmailSummary[]>([])
  const smartFolderLoading = ref(false)
  const smartCurrentPage = ref(1)

  const isSmartFolder = computed(() => smartFolder.value !== null)

  // Email detail state
  const selectedEmailUid = ref<number | null>(null)
  const selectedEmailDetail = ref<EmailDetail | null>(null)
  const loadingDetail = ref(false)
  const detailError = ref<string | null>(null)

  const selectedAccount = computed(() => {
    if (!selectedAccountId.value) return null
    if (smartFolder.value) return null
    return accounts.value.find((a) => a.ID === selectedAccountId.value) ?? null
  })

  const currentFolders = computed(() => {
    if (!selectedAccountId.value) return []
    return foldersByAccount.value[selectedAccountId.value] ?? []
  })

  const currentEmails = computed(() => {
    if (searchActive.value) return searchResults.value
    if (smartFolder.value) {
      const start = (smartCurrentPage.value - 1) * pageSize
      return smartEmails.value.slice(start, start + pageSize)
    }
    const key = `${selectedAccountId.value}:${selectedFolder.value}`
    return emailsByFolder.value[key] ?? []
  })

  const currentTotal = computed(() => {
    if (searchActive.value) return searchTotal.value
    if (smartFolder.value) return smartEmails.value.length
    const key = `${selectedAccountId.value}:${selectedFolder.value}`
    return emailTotalByFolder.value[key] ?? 0
  })

  const currentPage = computed(() => {
    if (searchActive.value) return searchPage.value
    if (smartFolder.value) return smartCurrentPage.value
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

  function markAvatarUpdated(id: string, filename: string) {
    const account = accounts.value.find((item) => item.ID === id)
    if (account) account.Avatar = filename
    avatarRevision.value[id] = (avatarRevision.value[id] ?? 0) + 1
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
      // Keep whatever was already loaded: emptying the list made a transient
      // failure look like "this account has no folders".
      reportFetchFailure('Failed to load folders', e)
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
      // Keep the previously loaded page: the backend deliberately returns an
      // error instead of an empty list when a refresh looks wrong, and blanking
      // the list here threw that protection away.
      reportFetchFailure('Failed to load emails', e)
    } finally {
      loading.value = false
    }
  }

  async function searchEmails(filters: EmailSearchFilter[], accountId = '', folderName = '') {
    const normalized = filters
      .map((filter) => ({ field: filter.field, value: filter.value.trim() }))
      .filter((filter) => filter.value.length > 0)
    if (normalized.length === 0) {
      clearSearch()
      return
    }

    const requestId = ++searchRequestId
    searchActive.value = true
    searchLoading.value = true
    searchPage.value = 1
    searchQuery.value = normalized
    searchAccountId.value = accountId
    searchFolderName.value = folderName
    try {
      const resp: EmailPageResponse | null = await EmailService.SearchEmails(
        { account_id: accountId, folder_name: folderName, filters: normalized },
        1,
        pageSize,
      )
      if (requestId !== searchRequestId) return
      searchResults.value = (resp?.emails ?? []).map((email) => ({
        ...email,
        account_id: email.account_id,
        folder_name: email.folder_name,
      }))
      searchTotal.value = resp?.total ?? 0
    } catch (e) {
      if (requestId !== searchRequestId) return
      // Without feedback an empty result was indistinguishable from a failure.
      reportFetchFailure('Failed to search emails', e)
      searchResults.value = []
      searchTotal.value = 0
    } finally {
      if (requestId === searchRequestId) searchLoading.value = false
    }
  }

  function clearSearch() {
    searchRequestId++
    searchActive.value = false
    searchResults.value = []
    searchTotal.value = 0
    searchPage.value = 1
    searchQuery.value = []
    searchAccountId.value = ''
    searchFolderName.value = ''
    searchLoading.value = false
  }

  /** Force-refresh from the IMAP server, bypassing cache. Keeps your current page
   *  instead of hard-resetting to page 1 (previously: any email you were reading on a
   *  later page appeared to drop away after a refresh / background sync). */
  async function forceRefreshEmails() {
    const account = selectedAccount.value
    const folder = selectedFolder.value
    if (!account || !folder) return

    const key = `${account.ID}:${folder}`
    refreshing.value = true
    try {
      const cur = currentPageByFolder.value[key] || 1
      const resp: EmailPageResponse | null = await EmailService.RefreshEmails(
        account,
        folder,
        cur,
        pageSize,
      )
      const total = resp?.total ?? 0
      const maxPages = total > 0 ? Math.ceil(total / pageSize) : 1
      const next = Math.max(1, Math.min(cur, maxPages))
      emailsByFolder.value[key] = resp?.emails ?? []
      emailTotalByFolder.value[key] = total
      currentPageByFolder.value[key] = next
    } catch (e) {
      reportFetchFailure('Failed to refresh emails', e)
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
          const want = currentPageByFolder.value[key] || 1
          EmailService.ListEmails(account, folder, want, pageSize)
            .then((resp: EmailPageResponse | null) => {
              const total = resp?.total ?? 0
              const maxPages = total > 0 ? Math.ceil(total / pageSize) : 1
              const page = Math.min(Math.max(1, want), maxPages)
              if (page === want) {
                emailsByFolder.value[key] = resp?.emails ?? []
                emailTotalByFolder.value[key] = total
                currentPageByFolder.value[key] = page
              } else {
                // our page is now beyond the last page (list shrank) -> show last page
                return EmailService.ListEmails(account, folder, page, pageSize).then((p2) => {
                  emailsByFolder.value[key] = p2?.emails ?? []
                  emailTotalByFolder.value[key] = p2?.total ?? 0
                  currentPageByFolder.value[key] = page
                })
              }
            })
            // Background refresh: log only. A toast here would fire on every
            // failed sync tick, and the previous list is still shown.
            .catch((e) => console.error('Failed to refresh folder after sync', e))
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
      // Dismissing the save dialog is a normal outcome, not a download failure.
      if (String(e).toLowerCase().includes('cancel')) return null
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
    smartFolder.value = null // clear smart folder selection
    selectedEmailUid.value = null
    selectedEmailDetail.value = null
    const account = accounts.value.find((a) => a.ID === accountId)
    if (account) {
      currentPageByFolder.value[`${accountId}:${folderName}`] = 1
      loadEmails(account, folderName)
    }
  }

  function goToPage(page: number) {
    if (searchActive.value) {
      const requestId = ++searchRequestId
      searchPage.value = page
      searchLoading.value = true
      EmailService.SearchEmails(
        {
          account_id: searchAccountId.value,
          folder_name: searchFolderName.value,
          filters: searchQuery.value,
        },
        page,
        pageSize,
      )
        .then((resp: EmailPageResponse | null) => {
          if (requestId !== searchRequestId) return
          searchResults.value = resp?.emails ?? []
          searchTotal.value = resp?.total ?? 0
        })
        .catch((e) => {
          if (requestId === searchRequestId) console.error('Failed to load search page', e)
        })
        .finally(() => {
          if (requestId === searchRequestId) searchLoading.value = false
        })
      return
    }
    if (smartFolder.value) {
      smartCurrentPage.value = page
      return
    }
    const account = selectedAccount.value
    const folder = selectedFolder.value
    if (!account || !folder) return
    const key = `${account.ID}:${folder}`
    currentPageByFolder.value[key] = page
    loadEmails(account, folder, page)
  }

  /** Smart folder helpers */
  function folderMatchesType(folderName: string, smartType: string): boolean {
    const lower = folderName.toLowerCase().trim()
    const matches = (patterns: string[]) =>
      patterns.some((pattern) => lower === pattern || lower.endsWith(`/${pattern}`))
    switch (smartType) {
      case 'inbox':
        return matches(['inbox', '收件箱'])
      case 'sent':
        return matches(['sent', 'sent mail', 'sent messages', 'sent items', '已发送', '已发送邮件'])
      case 'drafts':
        return lower === 'drafts' || lower.includes('draft')
      default:
        return false
    }
  }

  async function selectSmartFolder(smartType: string) {
    smartFolder.value = smartType
    selectedAccountId.value = null
    selectedFolder.value = null
    selectedEmailUid.value = null
    selectedEmailDetail.value = null
    smartFolderLoading.value = true

    try {
      const all: EmailSummary[] = []
      for (const account of accounts.value) {
        let folders = foldersByAccount.value[account.ID]
        if (!folders) {
          // Load folders first if not yet cached
          try {
            folders = await EmailService.ListFolders(account)
            foldersByAccount.value[account.ID] = folders
          } catch {
            continue
          }
        }
        const match = folders.find((f) => folderMatchesType(f.name, smartType))
        if (!match) continue
        const resp: EmailPageResponse | null = await EmailService.ListEmails(
          account,
          match.name,
          1,
          200,
        )
        if (resp?.emails) {
          // Attach account/folder metadata for opening emails from smart folder
          for (const email of resp.emails) {
            email.account_id = account.ID
            email.folder_name = match.name
          }
          all.push(...resp.emails)
        }
      }
      // Sort by date descending across all accounts
      all.sort((a, b) => new Date(b.date).getTime() - new Date(a.date).getTime())
      smartEmails.value = all
    } catch (e) {
      // Keep any previously loaded aggregate rather than showing an empty list,
      // and say why: an empty smart folder looked like "no mail at all".
      reportFetchFailure('Failed to load smart folder', e)
    } finally {
      smartFolderLoading.value = false
    }
  }

  return {
    accounts,
    avatarRevision,
    loading,
    refreshing,
    testingId,
    foldersByAccount,
    emailsByFolder,
    emailTotalByFolder,
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
    searchActive,
    searchLoading,
    pageSize,
    smartFolder,
    smartEmails,
    smartFolderLoading,
    smartCurrentPage,
    isSmartFolder,
    folderMatchesType,
    selectSmartFolder,
    loadAccounts,
    addAccount,
    removeAccount,
    updateAccount,
    markAvatarUpdated,
    testConnection,
    loadFolders,
    loadEmails,
    searchEmails,
    clearSearch,
    forceRefreshEmails,
    goToPage,
    selectEmail,
    refreshEmailBody,
    downloadAttachment,
    selectAccount,
    selectFolder,
  }
})
