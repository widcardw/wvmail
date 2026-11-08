/**
 * Regression tests for standalone placeholder formatting and vue-i18n messages.
 *
 * Run with: pnpm test   (Node ≥ 23 strips TypeScript types natively)
 *
 * The cases that matter are the ones the old hand-rolled `.replace('{count}', …)`
 * got wrong: a value containing `$&` (a replacement pattern, not a literal), a
 * placeholder with no parameter (must stay visible, not vanish), and the singular
 * branch of an English plural.
 */
import { createI18n } from 'vue-i18n'
import { formatMessage } from '../src/i18n/format.ts'
import enUS from '../src/i18n/locales/en-US.ts'
import zhCN from '../src/i18n/locales/zh-CN.ts'

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

console.log('named interpolation')
check(
  'a single placeholder is filled',
  formatMessage('{count} unread', { count: 3 }) === '3 unread',
)
check(
  'several placeholders are filled',
  formatMessage('{n} of {total} selected', { n: 2, total: 9 }) === '2 of 9 selected',
)
check(
  'a repeated placeholder is filled everywhere',
  formatMessage('{x} and {x}', { x: 'a' }) === 'a and a',
)
check('numbers are stringified', formatMessage('checked at {time}', { time: 0 }) === 'checked at 0')
check(
  'hyphenated names are supported',
  formatMessage('signed by {user-name}', { 'user-name': 'lee' }) === 'signed by lee',
)

console.log('interpolation edge cases')
// The old `.replace()` treated `$&` in the *replacement* as "the matched text".
check(
  'a dollar sign in the value stays literal',
  formatMessage('{code}', { code: '$& $1 $`' }) === '$& $1 $`',
)
// Silently blanking a missing value hides typos; leaving the braces makes them visible.
check(
  'an unknown placeholder is left standing',
  formatMessage('{count} of {total}', { count: 1 }) === '1 of {total}',
)
check('an empty template is untouched', formatMessage('', { a: 1 }) === '')
check(
  'a template without placeholders is untouched',
  formatMessage('no params here', { a: 1 }) === 'no params here',
)

console.log('vue-i18n messages')
const i18n = createI18n({
  legacy: false,
  locale: 'en-US',
  fallbackLocale: 'en-US',
  messages: { 'en-US': enUS, 'zh-CN': zhCN },
})
check(
  'nested messages are resolved',
  i18n.global.t('settings.accounts.folders.title') === 'Sidebar Folders',
)
check(
  'email addresses and domain patterns keep literal at signs',
  i18n.global.t('settings.remote.add_placeholder') ===
    'user@example.com, @example.com, or news@*.example.com',
)
check(
  'named interpolation works with plural selection',
  i18n.global.t('mail.recipients.more', { count: 1, n: 1 }, 1) === '1 more recipient',
)
check(
  'plural selection chooses the other branch',
  i18n.global.t('mail.recipients.more', { count: 2, n: 2 }, 2) === '2 more recipients',
)
i18n.global.locale.value = 'zh-CN'
check(
  'Chinese domain patterns keep literal at signs',
  i18n.global.t('settings.remote.add_placeholder') ===
    'user@example.com、@example.com 或 news@*.example.com',
)
check(
  'locale switching uses nested messages',
  i18n.global.t('mail.thread.count', { count: 3, n: 3 }, 3) === '这个会话共有 3 封邮件',
)

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exit(1)
