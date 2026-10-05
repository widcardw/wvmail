/**
 * Regression tests for count badges (`UiBadge.vue` renders `formatBadgeCount`).
 *
 * Run with: pnpm test   (Node ≥ 23 strips TypeScript types natively)
 *
 * The interesting cases are the boundaries: the cap itself is not abbreviated, one more
 * than the cap is, and "nothing to badge" has to be an empty string so the component's
 * `v-if` is the whole visibility rule.
 */
import { formatBadgeCount } from '../src/composables/badge.ts'

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

console.log('badge counts')
check('a small count is shown as is', formatBadgeCount(1) === '1')
check('a two-digit count is shown as is', formatBadgeCount(42) === '42')
check('the cap itself is not abbreviated', formatBadgeCount(99) === '99')
check('one above the cap collapses to 99+', formatBadgeCount(100) === '99+')
check('far above the cap stays 99+', formatBadgeCount(123456) === '99+')
check('a custom cap is honoured', formatBadgeCount(1000, 999) === '999+')
check('a custom cap keeps its own boundary', formatBadgeCount(999, 999) === '999')
check('fractional counts are floored', formatBadgeCount(3.7) === '3')

console.log('badge counts that must not render')
// Zero is the common case: every folder without unread mail hits it.
check('zero renders nothing', formatBadgeCount(0) === '')
check('negative renders nothing', formatBadgeCount(-3) === '')
check('NaN renders nothing', formatBadgeCount(Number.NaN) === '')
check('Infinity renders nothing', formatBadgeCount(Number.POSITIVE_INFINITY) === '')

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exit(1)
