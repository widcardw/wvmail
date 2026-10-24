/**
 * Regression tests for `src/i18n/format.ts` — the `{name}` interpolation and
 * `singular | plural` selection behind `tf()`.
 *
 * Run with: pnpm test   (Node ≥ 23 strips TypeScript types natively)
 *
 * The cases that matter are the ones the old hand-rolled `.replace('{count}', …)`
 * got wrong: a value containing `$&` (a replacement pattern, not a literal), a
 * placeholder with no parameter (must stay visible, not vanish), and the singular
 * branch of an English plural.
 */
import { formatMessage, formatPlural } from '../src/i18n/format.ts'

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

console.log('pluralisation')
const resources = '{count} resource | {count} resources'
check('one picks the singular branch', formatPlural(resources, 1, 'en-US') === '1 resource')
check('two picks the plural branch', formatPlural(resources, 2, 'en-US') === '2 resources')
check('zero is plural in English', formatPlural(resources, 0, 'en-US') === '0 resources')
check('a fraction is plural in English', formatPlural(resources, 1.5, 'en-US') === '1.5 resources')
check(
  'a single branch is used for every count',
  formatPlural('{count} 个远程资源', 1, 'zh-CN') === '1 个远程资源' &&
    formatPlural('{count} 个远程资源', 7, 'zh-CN') === '7 个远程资源',
)
check(
  'extra parameters reach the branch',
  formatPlural('{n} of {total} | {n} of {total}', 1, 'en-US', { total: 4 }) === '1 of 4',
)
check(
  'a `count` parameter overrides the printed number',
  formatPlural('{count} item | {count} items', 1, 'en-US', { count: 'one' }) === 'one item',
)
check(
  'a message with no pipe is returned as is',
  formatPlural('Body drawn with CSS', 5, 'en-US') === 'Body drawn with CSS',
)

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exit(1)
