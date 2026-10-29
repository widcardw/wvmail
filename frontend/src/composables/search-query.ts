/**
 * Search-query rules for the mail search bar: the `from:` / `to:` / `content:`
 * syntax, the tag list, and the suggestion lists built from it.
 *
 * Everything here is a pure function of its arguments (no stores, no Vue, no DOM),
 * so the syntax is testable on bare Node — see tests/search-query.test.mts. The
 * stateful half (tag state machine, contact fetching, store calls) is
 * `use-mail-search.ts`, which is the only consumer of these helpers.
 */
import type { EmailAccount, EmailSearchField, EmailSearchFilter } from '~/types/email'
import type { Contact } from '~/types/contact'

/** A tag in the query bar. `locked` marks the contextual filter ("sent → from me"). */
export type SearchTag = EmailSearchFilter & { locked?: boolean }

/** One row of the suggestion dropdown. */
export interface AccountSuggestion {
  ID: string
  Email: string
  Name: string
  Provider: string
}

/** One `From:` / `To:` / `Content:` row of the suggestion dropdown. */
export interface FieldSuggestion {
  field: EmailSearchField
  label: string
}

/** A contact remembered from earlier mail, as returned by the backend. */
export interface ContactSuggestion {
  ID?: string
  address: string
  name: string
}

/** Without a prefix, a query is a content search. */
export const DEFAULT_SEARCH_FIELD: EmailSearchField = 'content'

export const COMPLETE_EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

/** How many rows the dropdown shows, and how many the backend is asked for. */
export const SUGGESTION_LIMIT = 6

/** `field: value` → its parts; anything without a known prefix is content. */
export function parseSearchInput(input: string): { field: EmailSearchField; value: string } {
  const match = input.match(/^\s*(from|to|content)\s*:\s*(.*)$/i)
  if (!match) return { field: DEFAULT_SEARCH_FIELD, value: input.trim() }
  return { field: match[1].toLowerCase() as EmailSearchField, value: match[2] }
}

/** The tags a query bar renders: the contextual filter (if any) first, then the user's. */
export function buildDisplayedTags(
  tags: SearchTag[],
  defaultFilter: SearchTag | null,
): SearchTag[] {
  return defaultFilter ? [defaultFilter, ...tags] : tags
}

/** The wire form of a tag list — `locked` is UI state and never leaves the bar. */
export function toSearchFilters(tags: SearchTag[]): EmailSearchFilter[] {
  return tags.map(({ field, value }) => ({ field, value }))
}

/** The suggestion label for a field, e.g. `From:` — locale-independent by design. */
export function fieldLabel(field: EmailSearchField): string {
  return `${field[0].toUpperCase()}${field.slice(1)}:`
}

/** Configured accounts matching what has been typed in a `from:`/`to:` query. */
export function matchAccounts(
  accounts: EmailAccount[],
  value: string,
  limit: number = SUGGESTION_LIMIT,
): EmailAccount[] {
  const needle = value.trim().toLowerCase()
  return accounts
    .filter((account) => {
      if (!needle) return true
      return [account.Email, account.Name, account.IMAPUser].some((part) =>
        String(part || '')
          .toLowerCase()
          .includes(needle),
      )
    })
    .slice(0, limit)
}

/** Locally saved contacts matching what has been typed in a from:/to: query. */
export function matchSavedContacts(
  contacts: Contact[],
  value: string,
  limit: number = SUGGESTION_LIMIT,
): ContactSuggestion[] {
  const needle = value.trim().toLowerCase()
  if (!needle) return []
  return contacts
    .filter((contact) =>
      [contact.email, contact.name].some((part) =>
        String(part || '')
          .toLowerCase()
          .includes(needle),
      ),
    )
    .slice(0, limit)
    .map((contact) => ({
      ID: `saved:${contact.id}`,
      address: contact.email,
      name: contact.name,
    }))
}

/**
 * The field rows: offered only while nothing is typed, or while what is typed still
 * prefixes a field name. The field the contextual filter already covers is not
 * offered again.
 */
export function buildFieldSuggestions(
  input: string,
  labels: Record<EmailSearchField, string>,
  takenField: EmailSearchField | null,
): FieldSuggestion[] {
  if (/^\s*(from|to|content)\s*:/i.test(input)) return []
  const needle = input.trim().toLowerCase()
  const fields = (Object.keys(labels) as EmailSearchField[]).filter((field) => {
    if (takenField === field) return false
    if (!needle) return true
    return field.startsWith(needle) || labels[field].toLowerCase().startsWith(needle)
  })
  return fields.map((field) => ({ field, label: fieldLabel(field) }))
}

/**
 * Configured accounts first, then locally saved contacts, then contacts
 * remembered from earlier mail. Each tier is de-duplicated by address and the
 * result is capped to `limit`.
 */
export function mergeAccountSuggestions(
  accounts: EmailAccount[],
  savedContacts: ContactSuggestion[],
  historyContacts: ContactSuggestion[],
  contactsLabel: string,
  historyLabel: string,
  limit: number = SUGGESTION_LIMIT,
): AccountSuggestion[] {
  const configured = new Set(accounts.map((account) => account.Email.toLowerCase()))
  const seen = new Set(configured)
  const saved: AccountSuggestion[] = savedContacts
    .filter((contact) => !configured.has(contact.address.toLowerCase()))
    .map((contact) => ({
      ID: contact.ID || `saved:${contact.address}`,
      Email: contact.address,
      Name: contact.name,
      Provider: contactsLabel,
    }))
  for (const contact of saved) seen.add(contact.Email.toLowerCase())

  const remembered: AccountSuggestion[] = historyContacts
    .filter((contact) => !seen.has(contact.address.toLowerCase()))
    .map((contact) => ({
      ID: `cached:${contact.address}`,
      Email: contact.address,
      Name: contact.name,
      Provider: historyLabel,
    }))
  return [...accounts, ...saved, ...remembered].slice(0, limit)
}

/** True when the typed value is a whole address (used to commit a `to:` tag). */
export function isCompleteEmail(value: string): boolean {
  return COMPLETE_EMAIL_PATTERN.test(value.trim())
}
