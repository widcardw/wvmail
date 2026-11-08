/**
 * The stateful half of the search bar: the tag state machine, the contextual
 * ("locked") filter derived from the folder being read, the contact lookups, and
 * the debounce around them. The syntax itself lives in `search-query.ts`.
 *
 * Split out of `components/header/SearchBar.vue` so the widget is only the widget:
 * it keeps the input element, focus and keyboard navigation, and renders what this
 * returns.
 */
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from '~/i18n'
import { useAccountsStore } from '~/stores/accounts'
import { useContactsStore } from '~/stores/contacts'
import { useSearchStore } from '~/stores/search'
import { useFeedbackStore } from '~/stores/feedback'
import { usePreferencesStore } from '~/stores/preferences'
import type { EmailSearchField, EmailSearchFilter } from '~/types/email'
import {
  SUGGESTION_LIMIT,
  buildDisplayedTags,
  buildFieldSuggestions,
  isCompleteEmail,
  matchAccounts,
  matchSavedContacts,
  mergeAccountSuggestions,
  parseSearchInput,
  toSearchFilters,
} from '~/composables/search-query'
import type { ContactSuggestion, SearchTag } from '~/composables/search-query'
import { EmailService } from '#/wvmail/services'

/** Two-press delete: how long the "press again" warning stays armed. */
const DELETE_ARM_MS = 2500
/** Contact lookup debounce — long enough to skip a burst of keystrokes. */
const CONTACT_DEBOUNCE_MS = 120

export interface MailSearchOptions {
  /** Called after the input is rewritten (choose a field / accept a suggestion). */
  focusInput?: () => void
}

export function useMailSearch(options: MailSearchOptions = {}) {
  const { t } = useI18n()
  const accountsStore = useAccountsStore()
  const contactsStore = useContactsStore()
  const searchStore = useSearchStore()
  const feedback = useFeedbackStore()
  const preferences = usePreferencesStore()

  const input = ref('')
  const tags = ref<SearchTag[]>([])
  /** Set when the user removes the contextual filter, so it does not come back. */
  const defaultFilterDismissed = ref(false)
  const pendingDeleteKey = ref<string | null>(null)
  const activeSuggestionIndex = ref(-1)
  const historyContactSuggestions = ref<ContactSuggestion[]>([])

  void contactsStore.loadContacts()

  let contactRequestId = 0
  let contactTimer: ReturnType<typeof setTimeout> | undefined
  let pendingDeleteTimer: ReturnType<typeof setTimeout> | undefined

  const fieldLabels = computed<Record<EmailSearchField, string>>(() => ({
    from: t('search.from'),
    to: t('search.to'),
    content: t('search.content'),
  }))

  /**
   * Reading Sent means every query is really "from me"; reading Inbox means "to me".
   * Shown as a locked tag the user can dismiss, and never written back to `tags`.
   */
  const contextualDefaultFilter = computed<SearchTag | null>(() => {
    const account = accountsStore.selectedAccount
    const folder = accountsStore.selectedFolder
    const email = account?.Email?.trim() || ''
    if (accountsStore.isSmartFolder || !email || !folder) return null

    const field: EmailSearchField | null = accountsStore.folderMatchesType(folder, 'sent')
      ? 'from'
      : accountsStore.folderMatchesType(folder, 'inbox')
        ? 'to'
        : null
    if (field && tags.value.some((tag) => tag.field === field)) return null
    return field ? { field, value: email, locked: true } : null
  })

  const defaultFilter = computed<SearchTag | null>(() =>
    defaultFilterDismissed.value ? null : contextualDefaultFilter.value,
  )

  const displayedTags = computed<SearchTag[]>(() =>
    buildDisplayedTags(tags.value, defaultFilter.value),
  )

  const parsedInput = computed(() => parseSearchInput(input.value))

  const accountSuggestions = computed(() => {
    const { field, value } = parsedInput.value
    if ((field !== 'from' && field !== 'to') || input.value.trim() === '') return []
    if (defaultFilter.value?.field === field) return []
    return matchAccounts(accountsStore.accounts, value)
  })

  const savedContactSuggestions = computed(() => {
    const { field, value } = parsedInput.value
    if ((field !== 'from' && field !== 'to') || input.value.trim() === '') return []
    if (defaultFilter.value?.field === field) return []
    return matchSavedContacts(contactsStore.sortedContacts, value)
  })

  const suggestionAccounts = computed(() =>
    mergeAccountSuggestions(
      accountSuggestions.value,
      savedContactSuggestions.value,
      historyContactSuggestions.value,
      t('contacts.title'),
      t('search.history'),
    ),
  )

  const fieldSuggestions = computed(() =>
    buildFieldSuggestions(input.value, fieldLabels.value, defaultFilter.value?.field ?? null),
  )

  /** The filters a search runs with — the locked one included on purpose. */
  function effectiveFilters(): EmailSearchFilter[] {
    return toSearchFilters(displayedTags.value)
  }

  function scopeAccountId() {
    return accountsStore.isSmartFolder ? '' : accountsStore.selectedAccount?.ID || ''
  }

  function scopeFolderName() {
    return accountsStore.isSmartFolder ? '' : accountsStore.selectedFolder || ''
  }

  async function runSearch() {
    if (tags.value.length === 0) {
      searchStore.clearSearch()
      return
    }
    await searchStore.searchEmails(effectiveFilters(), scopeAccountId(), scopeFolderName())
  }

  function clearPendingDelete() {
    pendingDeleteKey.value = null
    if (pendingDeleteTimer) {
      clearTimeout(pendingDeleteTimer)
      pendingDeleteTimer = undefined
    }
  }

  async function addTag(value: string = parsedInput.value.value) {
    const field = parsedInput.value.field
    const normalized = value.trim()
    if (!normalized) return

    // One sender per query: a new `from:` replaces the previous one.
    if (field === 'from') {
      tags.value = tags.value.filter((tag) => tag.field !== 'from')
    }
    if (
      field === 'to' &&
      tags.value.some((tag) => tag.field === 'to' && tag.value === normalized)
    ) {
      input.value = ''
      return
    }
    tags.value.push({ field, value: normalized })
    clearPendingDelete()
    input.value = ''
    activeSuggestionIndex.value = -1
    await runSearch()
  }

  function fillSuggestion(email: string) {
    input.value = `${parsedInput.value.field}: ${email}`
    activeSuggestionIndex.value = -1
    options.focusInput?.()
  }

  function chooseField(field: EmailSearchField) {
    // The query syntax stays English in every locale; the labels shown to the user
    // are translated, the text that goes into the box is not.
    input.value = `${field[0].toUpperCase()}${field.slice(1)}: `
    activeSuggestionIndex.value = -1
    options.focusInput?.()
  }

  function removeTag(index: number) {
    if (displayedTags.value[index]?.locked) {
      // Dismissing the contextual filter only hides it: same query, fewer tags.
      defaultFilterDismissed.value = true
      clearPendingDelete()
      void runSearch()
      return
    }
    const removableIndex = index - (defaultFilter.value ? 1 : 0)
    if (removableIndex >= 0) {
      tags.value.splice(removableIndex, 1)
      clearPendingDelete()
      void runSearch()
    }
  }

  function removeLastTag() {
    const lastTag = tags.value.length > 0 ? tags.value[tags.value.length - 1] : defaultFilter.value
    const deleteKey = lastTag ? `${lastTag.field}:${lastTag.value}` : null
    if (!deleteKey) return

    // Backspace twice (Settings → tag delete): the first press arms, the second acts.
    if (preferences.tagDeletePresses === 2 && pendingDeleteKey.value !== deleteKey) {
      pendingDeleteKey.value = deleteKey
      if (pendingDeleteTimer) clearTimeout(pendingDeleteTimer)
      pendingDeleteTimer = setTimeout(() => {
        clearPendingDelete()
      }, DELETE_ARM_MS)
      feedback.warn(t('search.deleteAgain'))
      return
    }

    clearPendingDelete()
    if (lastTag?.locked) {
      defaultFilterDismissed.value = true
    } else {
      tags.value.pop()
    }
    void runSearch()
  }

  async function loadContactSuggestions() {
    const requestId = ++contactRequestId
    const { field, value } = parsedInput.value
    historyContactSuggestions.value = []
    if (
      (field !== 'from' && field !== 'to') ||
      !value.trim() ||
      defaultFilter.value?.field === field
    ) {
      return
    }

    try {
      const result = await EmailService.SuggestEmailContacts(
        field,
        value.trim(),
        scopeAccountId(),
        SUGGESTION_LIMIT,
      )
      if (requestId === contactRequestId) {
        historyContactSuggestions.value = (result || []) as ContactSuggestion[]
      }
    } catch (error) {
      if (requestId === contactRequestId) {
        console.error('Failed to load email contact suggestions', error)
      }
    }
  }

  function scheduleContactSuggestions() {
    if (contactTimer) clearTimeout(contactTimer)
    contactTimer = setTimeout(() => void loadContactSuggestions(), CONTACT_DEBOUNCE_MS)
  }

  /** Switching account/folder: the contextual filter changes, so re-run or reset. */
  function onScopeChange() {
    defaultFilterDismissed.value = false
    clearPendingDelete()
    if (accountsStore.isSmartFolder) {
      tags.value = []
      input.value = ''
      activeSuggestionIndex.value = -1
      searchStore.clearSearch()
      return
    }
    if (tags.value.length > 0) void runSearch()
  }

  watch(
    () => [
      accountsStore.selectedAccount?.ID,
      accountsStore.selectedFolder,
      accountsStore.isSmartFolder,
    ],
    onScopeChange,
  )

  watch(() => input.value, clearPendingDelete)

  watch(
    () => [
      input.value,
      accountsStore.selectedAccount?.ID,
      accountsStore.selectedFolder,
      accountsStore.isSmartFolder,
      defaultFilter.value?.field,
    ],
    scheduleContactSuggestions,
  )

  onBeforeUnmount(() => {
    if (contactTimer) clearTimeout(contactTimer)
    clearPendingDelete()
  })

  return {
    input,
    tags,
    displayedTags,
    parsedInput,
    fieldLabels,
    accountSuggestions,
    suggestionAccounts,
    fieldSuggestions,
    activeSuggestionIndex,
    defaultFilter,
    effectiveFilters,
    addTag,
    removeTag,
    removeLastTag,
    chooseField,
    fillSuggestion,
    isCompleteEmail,
    clearPendingDelete,
  }
}
