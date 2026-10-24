/**
 * Regression tests for the sidebar's per-account connection marker
 * (`SideBar.vue` + `stores/accountHealth.ts` render what these functions decide).
 *
 * Run with: pnpm test   (Node ≥ 23 strips TypeScript types natively)
 *
 * The interesting cases are the quiet ones: "not measured" must never read as
 * "fine" (that would draw a red icon on a healthy account at launch), and a
 * single failure must never read as "disconnected" (that would flash a warning on
 * every Wi-Fi roam).
 */
import {
  HEALTH_FAILURES_BEFORE_DOWN,
  HEALTH_IDLE_POLLS_BEFORE_SUSPECT,
  combineHealth,
  healthFromIdle,
  healthFromStatus,
  markerFromHealth,
} from '../src/composables/account-health.ts'

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

console.log('status signal')

check('nothing measured yet is silent', healthFromStatus({ measured: false, failures: 0 }) === null)
check(
  'a failure before the first answer is still silent',
  healthFromStatus({ measured: false, failures: 3 }) === null,
)
check('a successful sweep is ok', healthFromStatus({ measured: true, failures: 0 }) === 'ok')
check(
  'one failure is only a suspicion',
  healthFromStatus({ measured: true, failures: 1 }) === 'suspect',
)
check(
  'the second failure is a disconnect',
  healthFromStatus({ measured: true, failures: HEALTH_FAILURES_BEFORE_DOWN }) === 'down',
)
check('more failures stay down', healthFromStatus({ measured: true, failures: 7 }) === 'down')

console.log('idle signal')

const idle = (over: Partial<Parameters<typeof healthFromIdle>[0]>) =>
  healthFromIdle({ present: false, live: false, everLive: false, downPolls: 0, ...over })

check('an account with no watcher is silent', idle({}) === null)
check('a live watcher is ok', idle({ present: true, live: true }) === 'ok')
check(
  'a watcher that never connected is silent (startup, not a failure)',
  idle({ present: true, downPolls: 9 }) === null,
)
check(
  'one disconnected poll after being live is still quiet',
  idle({ present: true, everLive: true, downPolls: 1 }) === null,
)
check(
  'a watcher that stays down is a suspicion',
  idle({
    present: true,
    everLive: true,
    downPolls: HEALTH_IDLE_POLLS_BEFORE_SUSPECT,
  }) === 'suspect',
)

console.log('combined')

check('no signal at all draws nothing', combineHealth(null, null) === null)
check('status alone survives', combineHealth('ok', null) === 'ok')
check('idle alone survives', combineHealth(null, 'suspect') === 'suspect')
check('a live connection clears a lone failed sweep', combineHealth('suspect', 'ok') === 'ok')
check(
  'a live connection does not paint over a proven disconnect',
  combineHealth('down', 'ok') === 'down',
)
check(
  'a reconnecting watcher downgrades a healthy account to suspect',
  combineHealth('ok', 'suspect') === 'suspect',
)
check('down wins over suspect', combineHealth('suspect', 'down') === 'down')

console.log('marker')

// The regression this pins: `'ok'` is a truthy string, so a row that decided
// "show a marker if healthOf() is truthy" drew one for every healthy account.
check('a healthy account draws nothing', markerFromHealth('ok') === null)
check('an unmeasured account draws nothing', markerFromHealth(null) === null)
check('a proven disconnect is the offline marker', markerFromHealth('down') === 'offline')
check('a suspicion is the degraded marker', markerFromHealth('suspect') === 'degraded')

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exit(1)
