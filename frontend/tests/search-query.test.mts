/**
 * Regression tests for the mail search syntax (`composables/search-query.ts`, used
 * by the title-bar search box through `use-mail-search.ts`).
 *
 * Run with: pnpm test   (Node ≥ 23 strips TypeScript types natively)
 *
 * These rules never had a test before they were extracted from SearchBar.vue; what
 * matters is the shape of what the user typed — a bare word is a content search, a
 * known prefix is a field, anything else stays whole — and that the suggestion lists
 * never repeat the contextual ("locked") filter.
 */
import {
  buildDisplayedTags,
  buildFieldSuggestions,
  fieldLabel,
  isCompleteEmail,
  matchAccounts,
  matchSavedContacts,
  mergeAccountSuggestions,
  parseSearchInput,
  toSearchFilters,
} from '../src/composables/search-query.ts'
import type { EmailAccount, EmailSearchField } from '../src/composables/search-query.ts'

let pass = 0
let fail = 0

function check(name: string, cond: boolean) {
  if (cond) {
    pass++
    console.log(`  PASS  ${name}`)
  } else {
    fail++
    console.error(`  FAIL  ${name}`)
  }
}

function acct(email: string, name = '', imapUser = ''): EmailAccount {
  return { Email: email, Name: name, IMAPUser: imapUser } as unknown as EmailAccount
}

const LABELS: Record<EmailSearchField, string> = {
  from: '发件人',
  to: '收件人',
  content: '内容',
}

console.log('search query syntax')

check('a bare word is a content search', parseSearchInput('发票').field === 'content')
check('…and keeps its text', parseSearchInput('  发票  ').value === '发票')
check('from: switches the field', parseSearchInput('from: alice@x.com').field === 'from')
check(
  '…and drops the space after the colon',
  parseSearchInput('from: alice@x.com').value === 'alice@x.com',
)
// Only *leading* space is part of the syntax; `addTag` trims the rest before the
// value reaches the backend.
check('trailing space stays in the value', parseSearchInput('from:   x  ').value === 'x  ')
check('prefixes are case-insensitive', parseSearchInput('TO: bob@x.com').field === 'to')
check('spaces around the colon are allowed', parseSearchInput(' to : bob@x.com').field === 'to')
check('an unknown prefix stays content', parseSearchInput('subject: hi').field === 'content')
check('…and is part of the query text', parseSearchInput('subject: hi').value === 'subject: hi')
check('a colon inside the value survives', parseSearchInput('content: a:b').value === 'a:b')
check('empty input is an empty content search', parseSearchInput('').value === '')

const mine = { field: 'to' as const, value: 'me@x.com', locked: true }
const theirs = { field: 'from' as const, value: 'alice@x.com' }
check('no default filter renders just the tags', buildDisplayedTags([theirs], null).length === 1)
check('the default filter comes first', buildDisplayedTags([theirs], mine)[0].value === 'me@x.com')
check('…and is not duplicated', buildDisplayedTags([theirs], mine).length === 2)

check(
  'locked is dropped on the way to the backend',
  toSearchFilters([mine, theirs]).every((tag) => !('locked' in tag)),
)
check('…but the value survives', toSearchFilters([mine])[0].value === 'me@x.com')

check('field labels are the capitalised prefix', fieldLabel('from') === 'From:')

const accounts = [
  acct('alice@x.com', 'Alice', 'alice'),
  acct('bob@y.com', 'Bob', 'bob'),
  acct('carol@z.com', 'Carol', 'carol'),
]
check('an empty needle lists accounts', matchAccounts(accounts, '').length === 3)
check('matching is by address', matchAccounts(accounts, 'bob@').length === 1)
check('matching is by display name', matchAccounts(accounts, 'carol').length === 1)
check('matching is by IMAP user', matchAccounts(accounts, 'alice').length === 1)
check('matching ignores case', matchAccounts(accounts, 'ALICE').length === 1)
check('no match is no suggestion', matchAccounts(accounts, 'nobody').length === 0)
check('the limit is honoured', matchAccounts(accounts, '', 2).length === 2)

check('an empty input offers every field', buildFieldSuggestions('', LABELS, null).length === 3)
check(
  'a typed prefix narrows to the matching field',
  buildFieldSuggestions('fr', LABELS, null).every((s) => s.field === 'from'),
)
check(
  'the localised label can be typed too',
  buildFieldSuggestions('发', LABELS, null)
    .map((s) => s.field)
    .join() === 'from',
)
check(
  'a field already covered by the locked filter is not offered',
  !buildFieldSuggestions('', LABELS, 'to').some((s) => s.field === 'to'),
)
check(
  'once a prefix is written no field is offered',
  buildFieldSuggestions('from: a', LABELS, null).length === 0,
)
check(
  'the suggestion label matches the tag label',
  buildFieldSuggestions('', LABELS, null)[0].label === 'From:',
)

const merged = mergeAccountSuggestions(
  [acct('alice@x.com', 'Alice')],
  [
    { ID: 'saved:1', address: 'dave@w.com', name: 'Dave' },
    { ID: 'saved:2', address: 'ALICE@x.com', name: 'duplicate' },
  ],
  [
    { address: 'dave@w.com', name: 'duplicate history' },
    { address: 'erin@z.com', name: 'Erin' },
  ],
  '联系人',
  '历史联系人',
)
check('a saved contact that is also an account is skipped', merged.length === 3)
check('saved contacts come after the accounts', merged[1].Email === 'dave@w.com')
check('…and are labelled as contacts', merged[1].Provider === '联系人')
check('history contacts come after saved contacts', merged[2].Email === 'erin@z.com')
check('…and are labelled as history', merged[2].Provider === '历史联系人')
check('suggestion ids are stable', merged[2].ID === 'cached:erin@z.com')
check(
  'the merged list is capped',
  mergeAccountSuggestions([], [], [], '联系人', '历史联系人', 0).length === 0,
)

const savedContacts = [
  { id: 'one', name: 'Alice', email: 'alice@x.com' },
  { id: 'two', name: '财务部', email: 'finance@example.com' },
] as const
check(
  'saved contacts match by name',
  matchSavedContacts([...savedContacts], 'ali')[0]?.address === 'alice@x.com',
)
check(
  'saved contacts match by address',
  matchSavedContacts([...savedContacts], 'finance@')[0]?.name === '财务部',
)
check('an empty saved-contact query returns nothing', matchSavedContacts([...savedContacts], '').length === 0)

check('a whole address completes', isCompleteEmail('alice@x.com'))
check('…even with padding', isCompleteEmail('  alice@x.com '))
check('a partial address does not', !isCompleteEmail('alice@'))
check('a display name does not', !isCompleteEmail('Alice <alice@x.com>'))

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exit(1)
