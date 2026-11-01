import { defineStore } from 'pinia'
import { ref, computed, onMounted, onUnmounted } from 'vue'
import type {
  // EmailAccount,
  FolderInfo,
  FolderStatus,
  EmailSummary,
  EmailDetail,
  AttachmentInfo,
  EmailPageResponse,
} from '~/types/email'
import { ConfigService, EmailService, OsService, TagService, EmailAccount } from '#/wvmail/services'
import type { AccountProxy } from '#/wvmail/services'
import { reportFetchFailure, useFeedbackStore } from '~/stores/feedback'
import { useSearchStore } from '~/stores/search'
import { useTagsStore } from '~/stores/tags'
import { useAccountHealthStore } from '~/stores/accountHealth'
import { useI18n } from '~/i18n'
import { Events } from '@wailsio/runtime'
import { isFlagged, withMailFlag } from '~/composables/mail-flags'
import { accountUnreadCount, adjustUnreadCounts } from '~/composables/folder-semantics'

export const useAccountsStore = defineStore('accounts', () => {
  const feedback = useFeedbackStore()
  const health = useAccountHealthStore()
  const { t: i18n } = useI18n()

  const accounts = ref<EmailAccount[]>([])
  const avatarRevision = ref<Record<string, number>>({})
  const loading = ref(false)
  /**
   * Folder fetches running right now, keyed `account:folder`.
   *
   * Per folder rather than one global flag: an open-folder revalidation of an
   * account you are not looking at must not spin the refresh icon of the folder
   * on screen, which is what a global counter did.
   */
  const refreshingKeys = ref<Record<string, boolean>>({})
  const testingId = ref<string | null>(null)
  const foldersByAccount = ref<Record<string, FolderInfo[]>>({})
  /**
   * Per-folder counters as the server last reported them (STATUS MESSAGES UIDNEXT
   * UNSEEN), keyed account → folder. Two readers: the sidebar badges read
   * `unseen`, and opening a folder compares `messages`/`uid_next` against what the
   * last fetch saw to decide whether anything needs fetching at all (see
   * revalidateOpenFolder). A folder the server would not answer for is absent.
   *
   * It is deliberately not derived from `emailsByFolder`: only the pages the UI has
   * loaded are cached, so that would undercount every mailbox bigger than one page.
   */
  const folderStatusByAccount = ref<Record<string, Record<string, FolderStatus | undefined>>>({})
  const emailsByFolder = ref<Record<string, EmailSummary[]>>({})
  const emailTotalByFolder = ref<Record<string, number>>({})
  const currentPageByFolder = ref<Record<string, number>>({})
  const pageSize = 20
  const selectedAccountId = ref<string | null>(null)
  const selectedFolder = ref<string | null>(null)

  // Search lives in its own store (stores/search.ts); the list reads it below.
  const searchStore = useSearchStore()
  const tagsStore = useTagsStore()

  // Smart folder (aggregate view across all accounts)
  const smartFolder = ref<string | null>(null)
  const smartEmails = ref<EmailSummary[]>([])
  const smartFolderLoading = ref(false)
  const smartFolderRefreshing = ref(false)
  const smartCurrentPage = ref(1)
  const mailTagFilterIDs = ref<string[]>([])
  const smartTagFilterEmails = ref<EmailSummary[]>([])
  const smartTagFilterTotal = ref(0)
  /** Latest aggregate-view request; switching views invalidates older results. */
  let smartFolderRequestId = 0
  let smartTagFilterRequestId = 0

  function invalidateSmartFolderRequests() {
    smartFolderRequestId++
    smartTagFilterRequestId++
    smartFolderLoading.value = false
    smartFolderRefreshing.value = false
  }

  function smartFolderRequestIsCurrent(requestId: number, smartType: string): boolean {
    return requestId === smartFolderRequestId && smartFolder.value === smartType
  }

  const flagging = ref<Record<string, boolean>>({})
  const deletingSelectedEmail = ref(false)
  const flaggedInflight = new Map<string, Promise<EmailSummary[]>>()

  const isSmartFolder = computed(() => smartFolder.value !== null)
  const tagFilterActive = computed(() => mailTagFilterIDs.value.length > 0)

  /** True while the folder on screen is being fetched — the list header's spinner. */
  const refreshingOpenFolder = computed(() => {
    if (!selectedAccountId.value || !selectedFolder.value) return false
    return refreshingKeys.value[`${selectedAccountId.value}:${selectedFolder.value}`] === true
  })

  // Email detail state. Keep the source mailbox with the selected message so
  // actions also work from search and aggregate views.
  const selectedEmailUid = ref<number | null>(null)
  const selectedEmailAccountId = ref<string | null>(null)
  const selectedEmailFolder = ref<string | null>(null)
  const selectedEmailDetail = ref<EmailDetail | null>(null)
  const loadingDetail = ref(false)
  const detailError = ref<string | null>(null)
  /**
   * Latest detail request. Selecting another message, or refreshing the body,
   * invalidates every request that started before it so a slow IMAP fetch cannot
   * paint its result over the message now on screen.
   */
  let detailRequestId = 0

  function detailRequestIsCurrent(
    requestId: number,
    accountId: string,
    folder: string,
    uid: number,
  ): boolean {
    return (
      requestId === detailRequestId &&
      selectedEmailAccountId.value === accountId &&
      selectedEmailFolder.value === folder &&
      selectedEmailUid.value === uid
    )
  }

  const selectedAccount = computed(() => {
    if (!selectedAccountId.value) return null
    if (smartFolder.value) return null
    return accounts.value.find((a) => a.ID === selectedAccountId.value) ?? null
  })
  const selectedEmailAccount = computed(
    () => accounts.value.find((a) => a.ID === selectedEmailAccountId.value) ?? null,
  )

  const currentFolders = computed(() => {
    if (!selectedAccountId.value) return []
    return foldersByAccount.value[selectedAccountId.value] ?? []
  })

  const currentEmails = computed(() => {
    if (searchStore.searchActive) return searchStore.searchResults
    if (smartFolder.value) {
      if (tagFilterActive.value) return smartTagFilterEmails.value
      const start = (smartCurrentPage.value - 1) * pageSize
      return smartEmails.value.slice(start, start + pageSize)
    }
    const key = `${selectedAccountId.value}:${selectedFolder.value}`
    return emailsByFolder.value[key] ?? []
  })

  const currentTotal = computed(() => {
    if (searchStore.searchActive) return searchStore.searchTotal
    if (smartFolder.value) {
      return tagFilterActive.value ? smartTagFilterTotal.value : smartEmails.value.length
    }
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
    // one connection (FolderStatuses); failures are logged there, and the normal
    // triggers (expand, emails-synced) re-fetch later if this fires too early.
    void sweepStatusesInTurn([...accounts.value])
  }

  /**
   * The startup badge sweep, one account at a time.
   *
   * Serial on purpose. Each `FolderStatuses` is a real IMAP connection, and the
   * IDLE watchers are connecting at the same moment; firing four dials in one
   * tick is what a provider with a low concurrent-connection limit (Gmail, 163)
   * answers with a refusal — and a refusal is indistinguishable from "the account
   * is broken", so the sidebar would paint healthy accounts as degraded on every
   * launch. One dial at a time costs a couple of seconds of missing badges, and
   * an account whose turn has not come yet simply draws none.
   *
   * The queue is shared across calls, so a second `loadAccounts` (add / edit /
   * reorder) waits its turn instead of racing the sweep in flight; accounts
   * already swept are then skipped by the 30 s freshness check. A hung server
   * holds the queue for `dialIMAP`'s 60 s timeout — that is the trade: a slow
   * badge instead of a false "disconnected". `loadFolderStatuses` swallows its
   * own failures (it reports them to the badge and the health store), so a dead
   * server cannot break the loop.
   */
  let statusSweep: Promise<void> = Promise.resolve()
  function sweepStatusesInTurn(list: EmailAccount[]): Promise<void> {
    const next = statusSweep.then(async () => {
      for (const account of list) await loadFolderStatuses(account)
    })
    // Nothing here rejects today, but a poisoned queue would silently stop
    // sweeping for the rest of the session — so the chain keeps a clean copy.
    statusSweep = next.catch(() => {})
    return next
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

  /**
   * Delete one account, and optionally the mail cached for it.
   *
   * `purgeCache` is the delete dialog's tick box. It runs after the config write,
   * because the cache must never be thrown away for an account that is still
   * configured (a failed write leaves it in place), and its failure is reported
   * but not rethrown: by then the account is gone, so telling the caller the
   * delete failed would be a lie. What is left is a few unreachable rows, which is
   * what the account would have cost had the box never been ticked.
   */
  async function removeAccount(id: string, { purgeCache = false }: { purgeCache?: boolean } = {}) {
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
      health.forget(id)
      await loadAccounts()
    } catch (e) {
      console.error('Failed to remove account', e)
      throw e
    }

    if (purgeCache) {
      try {
        await EmailService.PurgeAccountCache(id)
      } catch (e) {
        console.error('Failed to purge the account cache', e)
        feedback.error(i18n.value['settings.accounts.cache_purge_failed'])
      }
    }
  }

  /**
   * Change one account's proxy override from the Proxy settings page.
   *
   * The list is patched in place first so the row's summary updates without a
   * round trip; a failed write puts the old value back, because the page's
   * select would otherwise show a setting that is not in config.yaml. No
   * `loadAccounts()` afterwards: nothing else about the account changed, and it
   * would kick off another STATUS sweep for every account.
   */
  async function setAccountProxy(id: string, proxy: AccountProxy) {
    const account = accounts.value.find((item) => item.ID === id)
    if (!account) return

    const previous = account.Proxy
    account.Proxy = proxy
    try {
      await ConfigService.UpdateAccount(id, account)
    } catch (e) {
      account.Proxy = previous
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

  /**
   * Move one account by one slot (-1 up, +1 down) in the order the sidebar and
   * the settings list render.
   *
   * The array is reordered locally first so the list does not jump a round trip
   * later — a menu that has to be reopened to see the result feels broken — and
   * the backend only persists the slot. A failed write re-reads the config, which
   * puts the old order back and is the only honest state to show.
   */
  async function moveAccount(id: string, offset: -1 | 1) {
    const from = accounts.value.findIndex((item) => item.ID === id)
    const to = from + offset
    if (from === -1 || to < 0 || to >= accounts.value.length) return

    const next = accounts.value.slice()
    const [moved] = next.splice(from, 1)
    next.splice(to, 0, moved)
    accounts.value = next

    try {
      await ConfigService.MoveAccount(id, offset)
    } catch (e) {
      console.error('Failed to reorder account', e)
      await loadAccounts()
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
      // It logs in for real, so the sidebar's marker follows the answer: a test
      // that fails from the settings page must not leave a healthy-looking row.
      health.noteTestResult(account.ID, success)
      return { success, message }
    } catch (e) {
      health.noteTestResult(account.ID, false)
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
      void loadFolderStatuses(account)
    } catch (e) {
      // Keep whatever was already loaded: emptying the list made a transient
      // failure look like "this account has no folders".
      reportFetchFailure('Failed to load folders', e)
    }
  }

  /** Unread count of one folder, 0 when there is nothing to badge. */
  function folderUnread(accountId: string, folder: string): number {
    return folderStatusByAccount.value[accountId]?.[folder]?.unseen ?? 0
  }

  /**
   * Unique unread count for one account. Most providers can sum every folder,
   * but Gmail's system views overlap; All Mail is its canonical superset.
   */
  function accountUnread(accountId: string): number {
    return accountUnreadCount(folderStatusByAccount.value[accountId])
  }

  // One IMAP connection per STATUS sweep, so sync ticks and folder clicks must not
  // stack up. The sweep answers for every folder of an account at once, so it is
  // shared by two callers with different tolerances: badges are happy with numbers
  // half a minute old, while the open-folder check wants something recent before it
  // trusts "nothing changed".
  const statusInflight = new Map<string, Promise<void>>()
  const statusFetchedAt: Record<string, number> = {}
  const STATUS_BADGE_MAX_AGE_MS = 30_000
  const STATUS_FRESH_MAX_AGE_MS = 5_000

  // At most one list fetch per folder at a time, and a folder fetched a moment ago
  // is not fetched again just because it was clicked. Each fetch costs a connect +
  // login + SELECT + FETCH, so clicking back and forth between two folders must not
  // dial IMAP on every click.
  const fetchInflight = new Set<string>()
  /**
   * Latest list mutation per account:folder. Both cache loads and server
   * refreshes use it, so a slow page-1 response cannot overwrite page 2 when the
   * user has already paged or refreshed.
   */
  let folderRequestSeq = 0
  const folderRequestIds = new Map<string, number>()
  let listRequestsInFlight = 0

  function beginFolderRequest(key: string): number {
    const id = ++folderRequestSeq
    folderRequestIds.set(key, id)
    return id
  }

  function folderRequestIsCurrent(key: string, id: number): boolean {
    return folderRequestIds.get(key) === id
  }

  /** account:folder → when its rows last came off the server. */
  const folderFetchedAt: Record<string, number> = {}
  const FOLDER_OPEN_REFRESH_MIN_MS = 20_000

  /**
   * account:folder → the mailbox's counters as of the last fetch that actually
   * talked to the server. Comparing it with a fresh STATUS is how opening a folder
   * avoids fetching a mailbox that has not moved (see revalidateOpenFolder).
   *
   * Seeded from `EmailPageResponse.uid_next`, which the backend also fills in for a
   * cached page (it is persisted with the folder meta), so a restart does not cost
   * one pointless fetch per folder.
   */
  const fetchSnapshot: Record<string, { uidNext: number; messages: number }> = {}

  /** Remember what the server said about a folder, when it actually said anything. */
  function recordSnapshot(key: string, resp: EmailPageResponse | null) {
    const uidNext = resp?.uid_next ?? 0
    if (!resp || uidNext === 0) return
    fetchSnapshot[key] = { uidNext, messages: resp.total ?? 0 }
  }

  /**
   * Refresh the per-folder counters from the server (STATUS … MESSAGES UIDNEXT
   * UNSEEN, one connection for the whole folder list). Failures are logged only:
   * badges are decoration, and the previous counts are still the best guess.
   *
   * `maxAgeMs` is how stale the caller will accept without a round trip; a sweep
   * already running is joined instead of started twice, so callers that need the
   * answer (the open-folder check) can simply await it.
   */
  function loadFolderStatuses(
    account: EmailAccount,
    maxAgeMs = STATUS_BADGE_MAX_AGE_MS,
  ): Promise<void> {
    const id = account.ID
    const running = statusInflight.get(id)
    if (running) return running
    if (Date.now() - (statusFetchedAt[id] ?? 0) < maxAgeMs) return Promise.resolve()

    const job = (async () => {
      try {
        const statuses = await EmailService.FolderStatuses(account)
        if (statuses) {
          folderStatusByAccount.value[id] = statuses
          statusFetchedAt[id] = Date.now()
          // This sweep is the only per-account reachability test the app runs on
          // its own, so it doubles as the sidebar marker's status signal.
          health.noteStatusOk(id)
        }
      } catch (e) {
        // Logged only: badges are decoration and the previous counts are still the
        // best guess. The failure is not lost, though — it is what lights up the
        // account's "disconnected" icon.
        health.noteStatusFailure(id)
        console.error('Failed to load folder statuses', e)
      }
    })()
    statusInflight.set(id, job)
    void job.then(() => {
      if (statusInflight.get(id) === job) statusInflight.delete(id)
    })
    return job
  }

  /** Optimistic ±1, including Gmail's All Mail aggregate, clamped at zero. */
  function adjustUnread(accountId: string, folder: string, delta: number) {
    adjustUnreadCounts(folderStatusByAccount.value[accountId], folder, delta)
  }

  /**
   * Load one page of a folder. Serves the local cache when it can, and reports
   * whether it did: the caller uses `false` as "this page is already fresh from the
   * server" so opening a folder does not fetch the same mailbox twice.
   */
  async function loadEmails(
    account: EmailAccount,
    folder: string,
    page = 1,
    ps = pageSize,
  ): Promise<boolean> {
    const key = `${account.ID}:${folder}`
    const requestId = beginFolderRequest(key)
    listRequestsInFlight++
    loading.value = true
    try {
      const activeTagIDs = [...mailTagFilterIDs.value]
      const resp: EmailPageResponse | null = activeTagIDs.length
        ? await TagService.ListTagFilteredEmails({
            scopes: [{ account_id: account.ID, folder_name: folder }],
            tag_ids: activeTagIDs,
            flagged_only: false,
            page,
            page_size: ps,
          })
        : await EmailService.ListEmails(account, folder, page, ps)
      if (!folderRequestIsCurrent(key, requestId)) return false
      const total = resp?.total ?? 0
      const maxPages = total > 0 ? Math.ceil(total / ps) : 1
      if (page > maxPages) {
        await loadEmails(account, folder, maxPages, ps)
        return true
      }
      emailsByFolder.value[key] = resp?.emails ?? []
      emailTotalByFolder.value[key] = total
      currentPageByFolder.value[key] = page
      const fromCache = resp?.from_cache ?? false
      // A page the backend had to fetch is fresh; remember when, so the
      // open-folder revalidation below does not dial the same mailbox again.
      if (!activeTagIDs.length) {
        if (!fromCache) folderFetchedAt[key] = Date.now()
        recordSnapshot(key, resp)
      }
      return fromCache
    } catch (e) {
      if (!folderRequestIsCurrent(key, requestId)) return false
      // Keep the previously loaded page: the backend deliberately returns an
      // error instead of an empty list when a refresh looks wrong, and blanking
      // the list here threw that protection away.
      reportFetchFailure('Failed to load emails', e)
      return false
    } finally {
      listRequestsInFlight--
      if (listRequestsInFlight === 0) loading.value = false
    }
  }

  /**
   * Fetch one folder from the server, keeping the page the user is on instead of
   * hard-resetting to page 1 (previously: any email you were reading on a later
   * page appeared to drop away after a refresh / background sync).
   *
   * `announce` is what separates the callers: an explicit refresh reports success
   * and failure to the user, the open-folder revalidation stays quiet — a toast on
   * every folder click would be noise, and the cached list it painted is still on
   * screen.
   *
   * The header spinner is raised only for the folder on screen (refreshingKeys),
   * so a revalidation running for another folder no longer animates it.
   */
  async function refreshFolder(
    account: EmailAccount,
    folder: string,
    { announce }: { announce: boolean },
  ) {
    const key = `${account.ID}:${folder}`
    // One connection per folder at a time: a click landing while a background
    // fetch is running must not open a second IMAP session on the same mailbox.
    if (fetchInflight.has(key)) return
    fetchInflight.add(key)
    const requestId = beginFolderRequest(key)
    refreshingKeys.value[key] = true
    try {
      const cur = currentPageByFolder.value[key] || 1
      const tagFiltered = mailTagFilterIDs.value.length > 0
      const resp: EmailPageResponse | null = await EmailService.RefreshEmails(
        account,
        folder,
        tagFiltered ? 1 : cur,
        pageSize,
      )
      if (!folderRequestIsCurrent(key, requestId)) return
      const total = resp?.total ?? 0
      const maxPages = total > 0 ? Math.ceil(total / pageSize) : 1
      const next = Math.max(1, Math.min(cur, maxPages))
      folderFetchedAt[key] = Date.now()
      recordSnapshot(key, resp)
      if (tagFiltered) {
        await loadEmails(account, folder, cur)
      } else {
        emailsByFolder.value[key] = resp?.emails ?? []
        emailTotalByFolder.value[key] = total
        currentPageByFolder.value[key] = next
      }
      if (announce) feedback.success(i18n.value['mail.refreshed'])
      // New mail moves the sidebar badges too. loadFolderStatuses is throttled
      // (30s), so the background path normally costs nothing extra.
      void loadFolderStatuses(account)
    } catch (e) {
      if (!folderRequestIsCurrent(key, requestId)) return
      if (announce) reportFetchFailure('Failed to refresh emails', e)
      else console.error('Failed to refresh folder in the background', e)
    } finally {
      fetchInflight.delete(key)
      delete refreshingKeys.value[key]
    }
  }

  /** Force-refresh the open folder from the IMAP server, bypassing the cache. */
  async function forceRefreshEmails() {
    const account = selectedAccount.value
    const folder = selectedFolder.value
    if (!account || !folder) return
    await refreshFolder(account, folder, { announce: true })
  }

  /**
   * Revalidate a folder the user just opened — but only if it actually moved.
   *
   * Opening a folder used to stop at whatever the cache held, so new mail only
   * appeared after pressing the refresh button in the list header — the one
   * gesture a mail client should not require. The cached page is still painted
   * first (that is what makes switching folders feel instant) and the check runs
   * behind it, so the list never blanks out.
   *
   * The check is a STATUS, not a FETCH: the backend answers for every folder over
   * one connection, and UIDNEXT only grows, so a folder whose UIDNEXT and message
   * count still match what the last fetch saw cannot hold anything the list is
   * missing. Fetching it anyway would re-download 20 envelopes to paint the same
   * rows and spin the refresh icon for nothing — which is what clicking through
   * folders used to do on every single click.
   *
   * When there is no reference point (a folder never fetched in this profile, or a
   * server that omits UIDNEXT) the old age window still applies: refresh if the
   * rows are older than FOLDER_OPEN_REFRESH_MIN_MS.
   */
  async function revalidateOpenFolder(account: EmailAccount, folder: string) {
    const key = `${account.ID}:${folder}`
    const seen = fetchSnapshot[key]
    if (!seen) {
      if (Date.now() - (folderFetchedAt[key] ?? 0) < FOLDER_OPEN_REFRESH_MIN_MS) return
      await refreshFolder(account, folder, { announce: false })
      return
    }

    await loadFolderStatuses(account, STATUS_FRESH_MAX_AGE_MS)
    const now = folderStatusByAccount.value[account.ID]?.[folder]
    // `now` is absent when the server refused the STATUS: we cannot prove the list
    // is current, so fall back to fetching it.
    if (now && now.uid_next === seen.uidNext && now.messages === seen.messages) return
    await refreshFolder(account, folder, { announce: false })
  }

  // Listen for background sync events to refresh the UI automatically
  let unlisten: (() => void) | null = null
  onMounted(() => {
    unlisten = Events.On('emails-synced', (event) => {
      const syncedAccountId = event.data as string
      // New mail lands in the folders too: refresh that account's badges (throttled,
      // and one connection for the whole folder list).
      const synced = accounts.value.find((a) => a.ID === syncedAccountId)
      if (synced) void loadFolderStatuses(synced)
      // If the synced account is currently selected, refresh the current folder
      if (syncedAccountId === selectedAccountId.value && selectedFolder.value) {
        const account = selectedAccount.value
        const folder = selectedFolder.value
        if (account && folder) {
          const key = `${account.ID}:${folder}`
          const want = currentPageByFolder.value[key] || 1
          const requestId = beginFolderRequest(key)
          EmailService.ListEmails(account, folder, want, pageSize)
            .then(async (resp: EmailPageResponse | null) => {
              if (!folderRequestIsCurrent(key, requestId)) return
              // The sync just fetched this folder on the server, so a click on it
              // must not trigger another dial straight away.
              folderFetchedAt[key] = Date.now()
              recordSnapshot(key, resp)
              // The tag query reads the cache that the sync just updated. Re-run
              // it instead of painting the unfiltered page over a filtered list.
              if (tagFilterActive.value) {
                await loadEmails(account, folder, want)
                return
              }
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
                  if (!folderRequestIsCurrent(key, requestId)) return
                  emailsByFolder.value[key] = p2?.emails ?? []
                  emailTotalByFolder.value[key] = p2?.total ?? 0
                  currentPageByFolder.value[key] = page
                  recordSnapshot(key, p2)
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
    const requestId = ++detailRequestId
    selectedEmailUid.value = uid
    selectedEmailAccountId.value = account.ID
    selectedEmailFolder.value = folder
    selectedEmailDetail.value = null
    loadingDetail.value = true
    detailError.value = null
    try {
      const detail = await EmailService.FetchEmail(account, folder, uid)
      if (!detailRequestIsCurrent(requestId, account.ID, folder, uid)) return
      selectedEmailDetail.value = detail
      // Opening a message reads it: see markEmailRead.
      void markEmailRead(account, folder, uid)
    } catch (e) {
      if (!detailRequestIsCurrent(requestId, account.ID, folder, uid)) return
      detailError.value = String(e)
      console.error('Failed to fetch email detail', e)
    } finally {
      if (detailRequestIsCurrent(requestId, account.ID, folder, uid)) {
        loadingDetail.value = false
      }
    }
  }

  /**
   * Opening a message marks it seen — on the server (STORE \Seen) and in the local
   * cache — so the list styling and the tray badge stop counting it. Best effort: a
   * failure only reverts the optimistic update, it never blocks the detail view.
   */
  async function markEmailRead(account: EmailAccount, folder: string, uid: number) {
    const folderSummary = emailsByFolder.value[`${account.ID}:${folder}`]?.find(
      (item) => item.uid === uid,
    )
    const touched = [
      folderSummary,
      ...smartEmails.value.filter(
        (email) =>
          email.account_id === account.ID && email.folder_name === folder && email.uid === uid,
      ),
      ...smartTagFilterEmails.value.filter(
        (email) =>
          email.account_id === account.ID && email.folder_name === folder && email.uid === uid,
      ),
    ].filter((summary): summary is EmailSummary => Boolean(summary))
    if (touched.length > 0 && touched.every((summary) => summary.is_read)) return
    const previousRead = touched.map((summary) => summary.is_read)
    for (const summary of touched) summary.is_read = true
    // Adjust the badge only when the message came from the loaded page: a search or
    // aggregate hit is not in `emailsByFolder` at all, so its read state is unknown and
    // guessing would take a count off a folder that may never have had it. The next
    // throttled refresh settles it.
    const knownUnread = folderSummary !== undefined
    if (knownUnread) adjustUnread(account.ID, folder, -1)
    try {
      await EmailService.MarkEmailRead(account, folder, uid)
      tagsStore.setEmailRead(account.ID, folder, uid)
    } catch (e) {
      touched.forEach((summary, index) => {
        summary.is_read = previousRead[index]
      })
      if (knownUnread) adjustUnread(account.ID, folder, 1)
      console.error('Failed to mark email as read', e)
    }
  }

  async function refreshEmailBody(account: EmailAccount, folder: string, uid: number) {
    const requestId = ++detailRequestId
    loadingDetail.value = true
    detailError.value = null
    try {
      const detail = await EmailService.RefreshEmailBody(account, folder, uid)
      if (!detailRequestIsCurrent(requestId, account.ID, folder, uid)) return
      selectedEmailDetail.value = detail
      feedback.success(i18n.value['mail.refreshed'])
    } catch (e) {
      if (!detailRequestIsCurrent(requestId, account.ID, folder, uid)) return
      detailError.value = String(e)
      console.error('Failed to refresh email body', e)
    } finally {
      if (detailRequestIsCurrent(requestId, account.ID, folder, uid)) {
        loadingDetail.value = false
      }
    }
  }

  async function downloadAttachment(attachment: AttachmentInfo): Promise<string | null> {
    const account = selectedAccount.value
    const folder = selectedFolder.value
    const uid = selectedEmailUid.value
    if (!account || !folder || !uid) {
      detailError.value = i18n.value['mail.noEmailSelected']
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
    invalidateSmartFolderRequests()
    smartFolder.value = null
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
    invalidateSmartFolderRequests()
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
      // Paint the cached page first, then revalidate it if the backend served it
      // from cache — opening a folder is the moment the user expects to see new
      // mail, and it used to require pressing the refresh button.
      void loadEmails(account, folderName).then((fromCache) => {
        if (fromCache) revalidateOpenFolder(account, folderName)
      })
      // Opening a folder is a good moment to re-read the badges (throttled).
      void loadFolderStatuses(account)
    }
  }

  /** True while the selected message's star is being written to the server. */
  const selectedEmailFlagged = computed(() => isFlagged(selectedEmailDetail.value?.flags))
  const selectedEmailFlagging = computed(() => {
    const account = selectedEmailAccount.value
    const folder = selectedEmailFolder.value
    const uid = selectedEmailUid.value
    return account && folder && uid != null
      ? flagging.value[emailKey(account.ID, folder, uid)] === true
      : false
  })

  function emailKey(accountId: string, folder: string, uid: number): string {
    return `${accountId}\u200b${folder}\u200b${uid}`
  }

  /**
   * Apply a flag change to every view that can hold the same message: the
   * folder page, aggregate/search results, and the open reading pane. Keyed by
   * account+folder+UID, because the same UID in another folder is another mail.
   */
  function setFlagInMemory(
    accountId: string,
    folder: string,
    uid: number,
    flag: string,
    enabled: boolean,
  ) {
    for (const [key, rows] of Object.entries(emailsByFolder.value)) {
      if (key !== `${accountId}:${folder}`) continue
      const row = rows.find((email) => email.uid === uid)
      if (row) row.flags = withMailFlag(row.flags, flag, enabled)
    }
    for (const email of smartEmails.value) {
      if (email.account_id === accountId && email.folder_name === folder && email.uid === uid) {
        email.flags = withMailFlag(email.flags, flag, enabled)
      }
    }
    for (const email of smartTagFilterEmails.value) {
      if (email.account_id === accountId && email.folder_name === folder && email.uid === uid) {
        email.flags = withMailFlag(email.flags, flag, enabled)
      }
    }
    searchStore.setFlag(accountId, folder, uid, flag, enabled)
    const detail = selectedEmailDetail.value
    if (
      selectedEmailAccountId.value === accountId &&
      selectedEmailFolder.value === folder &&
      selectedEmailUid.value === uid &&
      detail
    ) {
      detail.flags = withMailFlag(detail.flags, flag, enabled)
    }
  }

  /**
   * Toggle the IMAP \Flagged system flag. The UI flips immediately, but a failed
   * STORE rolls every copy back: a star that only exists on screen is worse than
   * a short delay.
   */
  async function toggleSelectedEmailFlagged() {
    const account = selectedEmailAccount.value
    const folder = selectedEmailFolder.value
    const uid = selectedEmailUid.value
    const detail = selectedEmailDetail.value
    if (!account || !folder || uid == null || !detail) {
      feedback.info(i18n.value['mail.flag.aggregate'])
      return
    }
    const key = emailKey(account.ID, folder, uid)
    if (flagging.value[key]) return

    const previous = isFlagged(detail.flags)
    const next = !previous
    flagging.value[key] = true
    setFlagInMemory(account.ID, folder, uid, '\\Flagged', next)
    try {
      await EmailService.SetEmailFlagged(account, folder, uid, next)
      feedback.success(i18n.value[next ? 'mail.flag.added' : 'mail.flag.removed'])
      if (smartFolder.value === 'starred' && !next) {
        smartEmails.value = smartEmails.value.filter(
          (email) =>
            !(email.account_id === account.ID && email.folder_name === folder && email.uid === uid),
        )
        const filteredBefore = smartTagFilterEmails.value.length
        smartTagFilterEmails.value = smartTagFilterEmails.value.filter(
          (email) =>
            !(email.account_id === account.ID && email.folder_name === folder && email.uid === uid),
        )
        if (smartTagFilterEmails.value.length !== filteredBefore) {
          smartTagFilterTotal.value = Math.max(0, smartTagFilterTotal.value - 1)
        }
        const smartTotal = tagFilterActive.value
          ? smartTagFilterTotal.value
          : smartEmails.value.length
        const lastPage = Math.max(1, Math.ceil(smartTotal / pageSize))
        smartCurrentPage.value = Math.min(smartCurrentPage.value, lastPage)
      }
    } catch (e) {
      setFlagInMemory(account.ID, folder, uid, '\\Flagged', previous)
      feedback.error(i18n.value['mail.flag.failed'])
      console.error('Failed to update the star', e)
    } finally {
      delete flagging.value[key]
    }
  }

  /**
   * Delete the message currently open in the reading pane. The backend moves it
   * to Trash (or expunges it when it is already there); the UI removes the local
   * row only after the server accepts the operation and leaves the reading pane
   * empty, rather than opening an adjacent message the user did not choose.
   */
  async function deleteSelectedEmail(): Promise<boolean> {
    const account = selectedEmailAccount.value
    const folder = selectedEmailFolder.value
    const uid = selectedEmailUid.value
    const detail = selectedEmailDetail.value
    if (!account || !folder || uid == null || !detail) {
      feedback.info(i18n.value['mail.noEmailSelected'])
      return false
    }
    if (deletingSelectedEmail.value) return false

    const row = currentEmails.value.find(
      (email) =>
        email.uid === uid &&
        (email.account_id == null || email.account_id === account.ID) &&
        (email.folder_name == null || email.folder_name === folder),
    )
    // `markEmailRead` updates the folder-list row before deleting. Use that row
    // instead of the detail flags so an already-seen message does not decrement
    // the unread badge a second time. Search/aggregate rows are not in
    // `emailsByFolder`, but their `is_read` value is still the best local signal.
    const wasUnread = row ? !row.is_read : false
    const folderKey = `${account.ID}:${folder}`
    const selectionStillCurrent = () =>
      selectedEmailAccountId.value === account.ID &&
      selectedEmailFolder.value === folder &&
      selectedEmailUid.value === uid

    deletingSelectedEmail.value = true
    try {
      await EmailService.DeleteEmail(account, folder, uid)
      // The delete can outlive a folder/email switch. Only cancel the detail
      // request and clear the pane if it still shows the message we deleted.
      const stillSelected = selectionStillCurrent()
      if (stillSelected) detailRequestId++

      const folderRows = emailsByFolder.value[folderKey]
      if (folderRows) {
        emailsByFolder.value[folderKey] = folderRows.filter((email) => email.uid !== uid)
      }
      if (emailTotalByFolder.value[folderKey] != null) {
        emailTotalByFolder.value[folderKey] = Math.max(0, emailTotalByFolder.value[folderKey] - 1)
      }
      smartEmails.value = smartEmails.value.filter(
        (email) =>
          !(email.account_id === account.ID && email.folder_name === folder && email.uid === uid),
      )
      const filteredBefore = smartTagFilterEmails.value.length
      smartTagFilterEmails.value = smartTagFilterEmails.value.filter(
        (email) =>
          !(email.account_id === account.ID && email.folder_name === folder && email.uid === uid),
      )
      if (smartTagFilterEmails.value.length !== filteredBefore) {
        smartTagFilterTotal.value = Math.max(0, smartTagFilterTotal.value - 1)
      }
      searchStore.removeEmail(account.ID, folder, uid)
      tagsStore.removeEmail(account.ID, folder, uid)

      const status = folderStatusByAccount.value[account.ID]?.[folder]
      if (status?.messages != null && status.messages > 0) status.messages--
      if (wasUnread) adjustUnread(account.ID, folder, -1)

      if (stillSelected) {
        selectedEmailUid.value = null
        selectedEmailAccountId.value = null
        selectedEmailFolder.value = null
        selectedEmailDetail.value = null
        detailError.value = null
      }

      feedback.success(i18n.value['mail.deleted'])
      if (!searchStore.searchActive && !smartFolder.value) {
        void loadEmails(account, folder, currentPageByFolder.value[folderKey] || 1)
      }
      // The move changes both source and destination counts (especially Trash's
      // unseen count), so do not accept a cached status here.
      void loadFolderStatuses(account, 0)
      return true
    } catch (e) {
      feedback.error(i18n.value['mail.delete.failed'])
      console.error('Failed to delete email', e)
      return false
    } finally {
      deletingSelectedEmail.value = false
    }
  }

  function goToPage(page: number) {
    // A search on screen owns its own paging: the same store the list reads above.
    if (searchStore.searchActive) {
      searchStore.goToPage(page)
      return
    }
    if (smartFolder.value) {
      if (tagFilterActive.value) {
        void loadSmartTagFilteredPage(page)
        return
      }
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
        return matches([
          'sent',
          'sent mail',
          'sent messages',
          'sent items',
          '已发送',
          '已发送邮件',
          '已发邮件',
        ])
      case 'drafts':
        return lower === 'drafts' || lower.includes('draft')
      default:
        return false
    }
  }

  async function smartTagFilterScopes(
    smartType: string,
  ): Promise<Array<{ account_id: string; folder_name: string }>> {
    if (smartType === 'starred') return []
    const scopes: Array<{ account_id: string; folder_name: string }> = []
    for (const account of accounts.value) {
      let folders = foldersByAccount.value[account.ID]
      if (!folders) {
        try {
          folders = await EmailService.ListFolders(account)
          foldersByAccount.value[account.ID] = folders
        } catch {
          continue
        }
      }
      const match = folders.find((folder) => folderMatchesType(folder.name, smartType))
      if (match) scopes.push({ account_id: account.ID, folder_name: match.name })
    }
    return scopes
  }

  async function loadSmartTagFilteredPage(
    page = smartCurrentPage.value,
    requestId = smartFolderRequestId,
  ) {
    const smartType = smartFolder.value
    if (!tagFilterActive.value || !smartType || smartType === 'classified') return
    const id = ++smartTagFilterRequestId
    smartFolderLoading.value = true
    try {
      const scopes = await smartTagFilterScopes(smartType)
      if (
        id !== smartTagFilterRequestId ||
        smartFolder.value !== smartType ||
        requestId !== smartFolderRequestId
      ) {
        return
      }
      if (scopes.length === 0 && smartType !== 'starred') {
        smartTagFilterEmails.value = []
        smartTagFilterTotal.value = 0
        smartCurrentPage.value = 1
        return
      }
      const resp: EmailPageResponse | null = await TagService.ListTagFilteredEmails({
        scopes,
        tag_ids: [...mailTagFilterIDs.value],
        flagged_only: smartType === 'starred',
        page,
        page_size: pageSize,
      })
      if (
        id !== smartTagFilterRequestId ||
        smartFolder.value !== smartType ||
        requestId !== smartFolderRequestId
      ) {
        return
      }
      const total = resp?.total ?? 0
      const pages = Math.max(1, Math.ceil(total / pageSize))
      const nextPage = Math.min(Math.max(1, page), pages)
      if (nextPage !== page) {
        await loadSmartTagFilteredPage(nextPage, requestId)
        return
      }
      smartCurrentPage.value = nextPage
      smartTagFilterEmails.value = (resp?.emails ?? []) as EmailSummary[]
      smartTagFilterTotal.value = total
    } catch (e) {
      if (id === smartTagFilterRequestId && smartFolder.value === smartType) {
        reportFetchFailure('Failed to apply the tag filter to the aggregate view', e)
      }
    } finally {
      if (id === smartTagFilterRequestId && smartFolder.value === smartType) {
        smartFolderLoading.value = false
      }
    }
  }

  async function setMailTagFilterIDs(ids: string[]) {
    const enabled = new Set(tagsStore.enabledTags.map((tag) => tag.id))
    mailTagFilterIDs.value = [...new Set(ids)].filter((id) => enabled.has(id))
    smartCurrentPage.value = 1

    if (searchStore.searchActive || smartFolder.value === 'classified') return
    if (smartFolder.value) {
      if (tagFilterActive.value) {
        await loadSmartTagFilteredPage(1)
      } else {
        smartTagFilterEmails.value = []
        smartTagFilterTotal.value = 0
      }
      return
    }
    const account = selectedAccount.value
    const folder = selectedFolder.value
    if (account && folder) {
      currentPageByFolder.value[`${account.ID}:${folder}`] = 1
      await loadEmails(account, folder, 1)
    }
  }

  async function selectSmartFolder(smartType: string) {
    const requestId = ++smartFolderRequestId
    smartTagFilterRequestId++
    const previousSmartFolder = smartFolder.value
    smartFolder.value = smartType
    selectedAccountId.value = null
    selectedFolder.value = null
    selectedEmailUid.value = null
    selectedEmailAccountId.value = null
    selectedEmailFolder.value = null
    selectedEmailDetail.value = null
    // Never leave rows from the previous smart folder under the new heading.
    if (previousSmartFolder !== smartType) smartEmails.value = []
    smartTagFilterEmails.value = []
    smartTagFilterTotal.value = 0
    smartFolderLoading.value = true
    smartFolderRefreshing.value = false
    smartCurrentPage.value = 1

    // The category view is a local tag query, not an IMAP aggregate. Its list
    // component owns loading/paging; this shared selection only clears the
    // previous folder/detail state.
    if (smartType === 'classified') {
      smartFolderLoading.value = false
      return
    }

    try {
      if (smartType === 'starred') {
        const snapshots = await Promise.all(
          accounts.value.map(async (account) => {
            try {
              const snapshot = await EmailService.ListFlaggedEmails(account, false)
              return { account, snapshot }
            } catch (e) {
              console.error('Failed to load the flagged snapshot', e)
              return null
            }
          }),
        )
        if (!smartFolderRequestIsCurrent(requestId, smartType)) return
        const all = snapshots.flatMap((entry) => {
          if (!entry?.snapshot) return []
          for (const email of entry.snapshot.emails) email.account_id = entry.account.ID
          return entry.snapshot.emails
        })
        all.sort((a, b) => new Date(b.date).getTime() - new Date(a.date).getTime())
        smartEmails.value = all

        const stale = snapshots
          .filter((entry) => entry?.snapshot?.stale)
          .map((entry) => entry!.account)
        if (stale.length) void refreshFlaggedAccounts(stale, requestId)
      } else {
        const all: EmailSummary[] = []
        for (const account of accounts.value) {
          let folders = foldersByAccount.value[account.ID]
          if (!folders) {
            // Load folders first if not yet cached
            try {
              folders = await EmailService.ListFolders(account)
              if (!smartFolderRequestIsCurrent(requestId, smartType)) return
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
          if (!smartFolderRequestIsCurrent(requestId, smartType)) return
          if (resp?.emails) {
            // Attach account/folder metadata for opening emails from smart folder
            for (const email of resp.emails) {
              email.account_id = account.ID
              email.folder_name = match.name
            }
            all.push(...resp.emails)
          }
        }
        if (!smartFolderRequestIsCurrent(requestId, smartType)) return
        // Sort by date descending across all accounts
        all.sort((a, b) => new Date(b.date).getTime() - new Date(a.date).getTime())
        smartEmails.value = all
      }
      if (tagFilterActive.value) await loadSmartTagFilteredPage(1, requestId)
    } catch (e) {
      if (!smartFolderRequestIsCurrent(requestId, smartType)) return
      // Keep any previously loaded aggregate rather than showing an empty list,
      // and say why: an empty smart folder looked like "no mail at all".
      reportFetchFailure('Failed to load smart folder', e)
    } finally {
      if (smartFolderRequestIsCurrent(requestId, smartType)) {
        smartFolderLoading.value = false
      }
    }
  }

  /** One server search per account at a time, shared by every caller. */
  function refreshFlaggedAccount(account: EmailAccount): Promise<EmailSummary[]> {
    const running = flaggedInflight.get(account.ID)
    if (running) return running
    const job = EmailService.ListFlaggedEmails(account, true)
      .then((snapshot) => {
        const emails = snapshot?.emails ?? []
        for (const email of emails) email.account_id = account.ID
        return emails
      })
      .finally(() => {
        if (flaggedInflight.get(account.ID) === job) flaggedInflight.delete(account.ID)
      })
    flaggedInflight.set(account.ID, job)
    return job
  }

  /**
   * Refresh stale account snapshots one at a time. Gmail and Outlook often cap
   * concurrent IMAP sessions, and firing every account at once made the whole
   * Starred tab feel slower than the slowest server.
   */
  async function refreshFlaggedAccounts(list: EmailAccount[], requestId = smartFolderRequestId) {
    if (!smartFolderRequestIsCurrent(requestId, 'starred') || smartFolderRefreshing.value) return
    smartFolderRefreshing.value = true
    try {
      for (const account of list) {
        try {
          const emails = await refreshFlaggedAccount(account)
          if (!smartFolderRequestIsCurrent(requestId, 'starred')) return
          // A fresh result replaces this account's rows wholesale. Matching
          // folder+UID across accounts would drop unrelated mail because IMAP
          // UIDs are unique only inside one mailbox.
          const others = smartEmails.value.filter((email) => email.account_id !== account.ID)
          smartEmails.value = [...others, ...emails].sort(
            (a, b) => new Date(b.date).getTime() - new Date(a.date).getTime(),
          )
        } catch (e) {
          console.error('Failed to refresh flagged mail', e)
        }
      }
    } finally {
      if (smartFolderRequestIsCurrent(requestId, 'starred')) {
        smartFolderRefreshing.value = false
        if (tagFilterActive.value) {
          void loadSmartTagFilteredPage(smartCurrentPage.value, requestId)
        }
      }
    }
  }

  /** Force a server refresh of All Starred, keeping the current rows on screen. */
  async function refreshStarred() {
    if (smartFolder.value !== 'starred') return
    await refreshFlaggedAccounts([...accounts.value])
  }

  return {
    accounts,
    avatarRevision,
    loading,
    refreshingOpenFolder,
    testingId,
    foldersByAccount,
    folderUnread,
    accountUnread,
    loadFolderStatuses,
    emailsByFolder,
    emailTotalByFolder,
    selectedAccountId,
    selectedFolder,
    selectedEmailUid,
    selectedEmailAccount,
    selectedEmailAccountId,
    selectedEmailFolder,
    selectedEmailFlagged,
    selectedEmailFlagging,
    deletingSelectedEmail,
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
    smartFolderRefreshing,
    smartCurrentPage,
    mailTagFilterIDs,
    tagFilterActive,
    setMailTagFilterIDs,
    isSmartFolder,
    folderMatchesType,
    selectSmartFolder,
    refreshStarred,
    toggleSelectedEmailFlagged,
    deleteSelectedEmail,
    loadAccounts,
    addAccount,
    removeAccount,
    setAccountProxy,
    updateAccount,
    moveAccount,
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
