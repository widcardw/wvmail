import { defineStore } from 'pinia'
import { ref } from 'vue'
import { EmailService } from '#/wvmail/services'
import { reportFetchFailure } from '~/stores/feedback'
import { PAGE_SIZE } from '~/types/email'
import { withMailFlag } from '~/composables/mail-flags'
import type { EmailPageResponse, EmailSearchFilter, EmailSummary } from '~/types/email'

/**
 * Cached search results (across accounts and folders).
 *
 * Split out of the accounts store: search is a *view* over the cache that happens to
 * be rendered by the same list, not part of the account/folder state. The mail list
 * asks this store whether a search is active before it falls back to folders, and the
 * search bar is its only writer.
 */
export const useSearchStore = defineStore('search', () => {
  const searchActive = ref(false)
  const searchResults = ref<EmailSummary[]>([])
  const searchTotal = ref(0)
  const searchPage = ref(1)
  const searchLoading = ref(false)
  const searchQuery = ref<EmailSearchFilter[]>([])
  const searchAccountId = ref('')
  const searchFolderName = ref('')
  /** Invalidates an in-flight page/search request when a newer one starts. */
  let requestId = 0

  async function searchEmails(filters: EmailSearchFilter[], accountId = '', folderName = '') {
    const normalized = filters
      .map((filter) => ({ field: filter.field, value: filter.value.trim() }))
      .filter((filter) => filter.value.length > 0)
    if (normalized.length === 0) {
      clearSearch()
      return
    }

    const id = ++requestId
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
        PAGE_SIZE,
      )
      if (id !== requestId) return
      searchResults.value = (resp?.emails ?? []).map((email) => ({
        ...email,
        account_id: email.account_id,
        folder_name: email.folder_name,
      }))
      searchTotal.value = resp?.total ?? 0
    } catch (e) {
      if (id !== requestId) return
      // Without feedback an empty result was indistinguishable from a failure.
      reportFetchFailure('Failed to search emails', e)
      searchResults.value = []
      searchTotal.value = 0
    } finally {
      if (id === requestId) searchLoading.value = false
    }
  }

  function clearSearch() {
    requestId++
    searchActive.value = false
    searchResults.value = []
    searchTotal.value = 0
    searchPage.value = 1
    searchQuery.value = []
    searchAccountId.value = ''
    searchFolderName.value = ''
    searchLoading.value = false
  }

  /** Keep search rows in step with a flag change made from the reading pane. */
  function setFlag(accountId: string, folder: string, uid: number, flag: string, enabled: boolean) {
    const row = searchResults.value.find(
      (email) =>
        email.account_id === accountId && email.folder_name === folder && email.uid === uid,
    )
    if (row) row.flags = withMailFlag(row.flags, flag, enabled)
  }

  /** Drop a deleted message from the current result page and its total. */
  function removeEmail(accountId: string, folder: string, uid: number) {
    const before = searchResults.value.length
    searchResults.value = searchResults.value.filter(
      (email) =>
        !(email.account_id === accountId && email.folder_name === folder && email.uid === uid),
    )
    if (searchResults.value.length !== before) {
      searchTotal.value = Math.max(0, searchTotal.value - 1)
    }
  }

  /** Paging while a search is on screen: re-runs the stored query for that page. */
  function goToPage(page: number) {
    const id = ++requestId
    searchPage.value = page
    searchLoading.value = true
    EmailService.SearchEmails(
      {
        account_id: searchAccountId.value,
        folder_name: searchFolderName.value,
        filters: searchQuery.value,
      },
      page,
      PAGE_SIZE,
    )
      .then((resp: EmailPageResponse | null) => {
        if (id !== requestId) return
        searchResults.value = resp?.emails ?? []
        searchTotal.value = resp?.total ?? 0
      })
      .catch((e) => {
        if (id === requestId) console.error('Failed to load search page', e)
      })
      .finally(() => {
        if (id === requestId) searchLoading.value = false
      })
  }

  return {
    searchActive,
    searchResults,
    searchTotal,
    searchPage,
    searchLoading,
    searchQuery,
    searchAccountId,
    searchFolderName,
    searchEmails,
    clearSearch,
    setFlag,
    removeEmail,
    goToPage,
  }
})
