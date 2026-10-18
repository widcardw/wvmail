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
import { reportFetchFailure, useFeedbackStore } from '~/stores/feedback'
import { useSearchStore } from '~/stores/search'
import { useI18n } from '~/i18n'
import { Events } from '@wailsio/runtime'

export const useAccountsStore = defineStore('accounts', () => {
  const feedback = useFeedbackStore()
  const { t: i18n } = useI18n()

  const accounts = ref<EmailAccount[]>([])
  const avatarRevision = ref<Record<string, number>>({})
  const loading = ref(false)
  const refreshing = ref(false) // force-refresh loading state
  const testingId = ref<string | null>(null)
  const foldersByAccount = ref<Record<string, FolderInfo[]>>({})
  /**
   * Unseen messages per folder, from the server (STATUS UNSEEN), keyed account → folder.
   * Only folders with something to badge are present, so a missing key means zero. (The
   * generated binding types the values as optional, hence `| undefined`: a count is never
   * absent and undefined at once.)
   *
   * It is deliberately not derived from `emailsByFolder`: only the pages the UI has
   * loaded are cached, so that would undercount every mailbox bigger than one page.
   */
  const unreadByFolder = ref<Record<string, Record<string, number | undefined>>>({})
  const emailsByFolder = ref<Record<string, EmailSummary[]>>({})
  const emailTotalByFolder = ref<Record<string, number>>({})
  const currentPageByFolder = ref<Record<string, number>>({})
  const pageSize = 20
  const selectedAccountId = ref<string | null>(null)
  const selectedFolder = ref<string | null>(null)

  // Search lives in its own store (stores/search.ts); the list reads it below.
  const searchStore = useSearchStore()

  // Smart folder (aggregate view across all accounts)
  const smartFolder = ref<string | null>(null)
  const smartEmails = ref<EmailSummary[]>([])
  const smartFolderLoading = ref(false)
  const smartCurrentPage = ref(1)

  const isSmartFolder = computed(() => smartFolder.value !== null)

  // Email detail state. Keep the source mailbox with the selected message so
  // actions also work from search and aggregate views.
  const selectedEmailUid = ref<number | null>(null)
  const selectedEmailAccountId = ref<string | null>(null)
  const selectedEmailFolder = ref<string | null>(null)
  const selectedEmailDetail = ref<EmailDetail | null>(null)
  const loadingDetail = ref(false)
  const detailError = ref<string | null>(null)

  const selectedAccount = computed(() => {
    if (!selectedAccountId.value) return null
    if (smartFolder.value) return null
    return accounts.value.find((a) => a.ID === selectedAccountId.value) ?? null
  })
  const selectedEmailAccount = computed(() =>
    accounts.value.find((a) => a.ID === selectedEmailAccountId.value) ?? null,
  )

  const currentFolders = computed(() => {
    if (!selectedAccountId.value) return []
    return foldersByAccount.value[selectedAccountId.value] ?? []
  })

  const currentEmails = computed(() => {
    if (searchStore.searchActive) return searchStore.searchResults
    if (smartFolder.value) {
      const start = (smartCurrentPage.value - 1) * pageSize
      return smartEmails.value.slice(start, start + pageSize)
    }
    const key = `${selectedAccountId.value}:${selectedFolder.value}`
    return emailsByFolder.value[key] ?? []
  })

  const currentTotal = computed(() => {
    if (searchStore.searchActive) return searchStore.searchTotal
    if (smartFolder.value) return smartEmails.value.length
    const key = `${selectedAccountId.value}:${selectedFolder.value}`
    return emailTotalByFolder.value[key] ?? 0
  })

  const currentPage = computed(() => {
    if (searchStore.searchActive) return searchStore.searchPage
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
    // Seed the badges at startup so the sidebar shows unread counts before any
    // account branch is expanded. The backend STATUSes every cached folder over
    // one connection (FolderUnread); failures are logged there, and the normal
    // triggers (expand, emails-synced) re-fetch later if this fires too early.
    for (const account of accounts.value) void loadFolderUnread(account)
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
        selectedEmailAccountId.value = null
        selectedEmailFolder.value = null
        selectedEmailDetail.value = null
      }
      if (selectedEmailAccountId.value === id) {
        selectedEmailUid.value = null
        selectedEmailAccountId.value = null
        selectedEmailFolder.value = null
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
      // Badges are per folder, so they are only worth fetching once the folder list is
      // known (the backend STATUSes exactly the folders it has cached).
      void loadFolderUnread(account)
    } catch (e) {
      // Keep whatever was already loaded: emptying the list made a transient
      // failure look like "this account has no folders".
      reportFetchFailure('Failed to load folders', e)
    }
  }

  /** Unread count of one folder, 0 when there is nothing to badge. */
  function folderUnread(accountId: string, folder: string): number {
    return unreadByFolder.value[accountId]?.[folder] ?? 0
  }

  /**
   * Total unread across every folder of one account — the badge on the account
   * row. Sums the same server-side counts the folder badges read, so the two
   * always agree; 0 until the account's first per-folder fetch lands (the
   * startup seed or the first expand).
   */
  function accountUnread(accountId: string): number {
    const counts = unreadByFolder.value[accountId]
    if (!counts) return 0
    let total = 0
    for (const n of Object.values(counts)) total += n ?? 0
    return total
  }

  // One IMAP connection per refresh, so sync ticks and folder clicks must not stack up.
  const unreadInflight = new Set<string>()
  const unreadFetchedAt: Record<string, number> = {}
  const UNREAD_MIN_INTERVAL_MS = 30_000

  /**
   * Refresh the per-folder unread counts from the server. Failures are logged only:
   * badges are decoration, and the previous counts are still the best guess.
   */
  async function loadFolderUnread(account: EmailAccount, force = false) {
    if (unreadInflight.has(account.ID)) return
    if (!force && Date.now() - (unreadFetchedAt[account.ID] ?? 0) < UNREAD_MIN_INTERVAL_MS) return
    unreadInflight.add(account.ID)
    try {
      const counts = await EmailService.FolderUnread(account)
      unreadByFolder.value[account.ID] = counts ?? {}
      unreadFetchedAt[account.ID] = Date.now()
    } catch (e) {
      console.error('Failed to load folder unread counts', e)
    } finally {
      unreadInflight.delete(account.ID)
    }
  }

  /** Optimistic ±1 for one folder, clamped at zero (used while flagging read). */
  function adjustUnread(accountId: string, folder: string, delta: number) {
    const counts = unreadByFolder.value[accountId] ?? {}
    const current = counts[folder] ?? 0
    if (current === 0 && delta < 0) return
    unreadByFolder.value[accountId] = { ...counts, [folder]: Math.max(0, current + delta) }
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
      // An explicit refresh with nothing new to show used to look like a no-op.
      feedback.success(i18n.value['mail.refreshed'])
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
      // New mail lands in the folders too: refresh that account's badges (throttled,
      // and one connection for the whole folder list).
      const synced = accounts.value.find((a) => a.ID === syncedAccountId)
      if (synced) void loadFolderUnread(synced)
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
    selectedEmailAccountId.value = account.ID
    selectedEmailFolder.value = folder
    selectedEmailDetail.value = null
    loadingDetail.value = true
    detailError.value = null
    try {
      const detail = await EmailService.FetchEmail(account, folder, uid)
      selectedEmailDetail.value = detail
      // Opening a message reads it: see markEmailRead.
      void markEmailRead(account, folder, uid)
    } catch (e) {
      detailError.value = String(e)
      console.error('Failed to fetch email detail', e)
    } finally {
      loadingDetail.value = false
    }
  }

  /**
   * Opening a message marks it seen — on the server (STORE \Seen) and in the local
   * cache — so the list styling and the tray badge stop counting it. Best effort: a
   * failure only reverts the optimistic update, it never blocks the detail view.
   */
  async function markEmailRead(account: EmailAccount, folder: string, uid: number) {
    const summary = emailsByFolder.value[`${account.ID}:${folder}`]?.find(
      (item) => item.uid === uid,
    )
    if (summary?.is_read) return // already read, nothing to do
    if (summary) summary.is_read = true
    // Adjust the badge only when the message came from the loaded page: a search or
    // aggregate hit is not in `emailsByFolder` at all, so its read state is unknown and
    // guessing would take a count off a folder that may never have had it. The next
    // throttled refresh settles it.
    const knownUnread = summary !== undefined
    if (knownUnread) adjustUnread(account.ID, folder, -1)
    try {
      await EmailService.MarkEmailRead(account, folder, uid)
    } catch (e) {
      if (summary) summary.is_read = false
      if (knownUnread) adjustUnread(account.ID, folder, 1)
      console.error('Failed to mark email as read', e)
    }
  }

  async function refreshEmailBody(account: EmailAccount, folder: string, uid: number) {
    loadingDetail.value = true
    detailError.value = null
    try {
      const detail = await EmailService.RefreshEmailBody(account, folder, uid)
      selectedEmailDetail.value = detail
      feedback.success(i18n.value['mail.refreshed'])
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
    selectedEmailAccountId.value = null
    selectedEmailFolder.value = null
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
    selectedEmailAccountId.value = null
    selectedEmailFolder.value = null
    selectedEmailDetail.value = null
    const account = accounts.value.find((a) => a.ID === accountId)
    if (account) {
      currentPageByFolder.value[`${accountId}:${folderName}`] = 1
      loadEmails(account, folderName)
      // Opening a folder is a good moment to re-read the badges (throttled).
      void loadFolderUnread(account)
    }
  }

  function goToPage(page: number) {
    // A search on screen owns its own paging: the same store the list reads above.
    if (searchStore.searchActive) {
      searchStore.goToPage(page)
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
    selectedEmailAccountId.value = null
    selectedEmailFolder.value = null
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
    unreadByFolder,
    folderUnread,
    accountUnread,
    loadFolderUnread,
    emailsByFolder,
    emailTotalByFolder,
    selectedAccountId,
    selectedFolder,
    selectedEmailUid,
    selectedEmailAccount,
    selectedEmailAccountId,
    selectedEmailFolder,
    selectedEmailDetail,
    loadingDetail,
    detailError,
    selectedAccount,
    currentFolders,
    currentEmails,
    currentTotal,
    currentPage,
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
    forceRefreshEmails,
    goToPage,
    selectEmail,
    refreshEmailBody,
    downloadAttachment,
    selectAccount,
    selectFolder,
  }
})
