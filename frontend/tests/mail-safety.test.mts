/**
 * The spam gate's decision table (`EmailDetail.vue` renders what this decides).
 *
 * Run with: pnpm test   (Node ≥ 23 strips TypeScript types natively)
 *
 * The case worth pinning is the middle column: the Settings switch must actually
 * change the outcome. It did not before — `scan_external_safety` was written to
 * the config and read by nobody, so the gate ignored it.
 */
import { isSpamGated } from '../src/composables/mail-safety.ts'

let pass = 0
let fail = 0

function check(name: string, cond: boolean) {
  if (cond) {
    pass++
    console.log(`  ok   ${name}`)
  } else {
    fail++
    console.log(`  FAIL ${name}`)
  }
}

console.log('spam gate')

check('a flagged message is held while interception is on', isSpamGated(true, true, false))
check('revealing it opens the gate', !isSpamGated(true, true, true))
check('turning interception off opens the gate', !isSpamGated(true, false, false))
check('an ordinary message is never held', !isSpamGated(false, true, false))
check('an ordinary message stays open with interception off', !isSpamGated(false, false, false))

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exitCode = 1
