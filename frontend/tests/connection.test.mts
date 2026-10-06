/**
 * Regression tests for the connection meter's judgement (`ConnectionStatus.vue` +
 * `stores/connection.ts` render `classifyConnection` / `connectionBars` /
 * `formatLatency`).
 *
 * Run with: pnpm test   (Node ≥ 23 strips TypeScript types natively)
 *
 * The interesting cases are the ones a probe can actually produce: a failure
 * carries latency -1 (never "0 ms, very fast"), and the thresholds are
 * inclusive at the boundary so the buckets do not have holes.
 */
import {
  classifyConnection,
  connectionBars,
  formatLatency,
  CONNECTION_FAIR_MS,
  CONNECTION_GOOD_MS,
} from '../src/composables/connection.ts'

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

console.log('connection quality')

// Real measurements from imap.qq.com / imap.163.com / imap.exmail.qq.com sit at
// 134-260 ms and must read as good.
check('a measured 134 ms link is good', classifyConnection(true, 134) === 'High')
check('a measured 260 ms link is good', classifyConnection(true, 260) === 'High')

check('the good boundary itself is good', classifyConnection(true, CONNECTION_GOOD_MS) === 'High')
check(
  'one millisecond past good is fair',
  classifyConnection(true, CONNECTION_GOOD_MS + 1) === 'Medium',
)
check('the fair boundary itself is fair', classifyConnection(true, CONNECTION_FAIR_MS) === 'Medium')
check(
  'one millisecond past fair is bad',
  classifyConnection(true, CONNECTION_FAIR_MS + 1) === 'Low',
)
check('custom thresholds are honoured', classifyConnection(true, 100, 50, 80) === 'Low')

check('a failed probe is off', classifyConnection(false, -1) === 'Off')
check('a failure with a stale latency is still off', classifyConnection(false, 42) === 'Off')
check('a negative latency is off even when ok', classifyConnection(true, -1) === 'Off')
check('NaN is off', classifyConnection(true, Number.NaN) === 'Off')
check(
  'Infinity does not read as good',
  classifyConnection(true, Number.POSITIVE_INFINITY) !== 'High',
)

check('good fills three bars', connectionBars('High') === 3)
check('fair fills two bars', connectionBars('Medium') === 2)
check('bad fills one bar', connectionBars('Low') === 1)
check('off fills none', connectionBars('Off') === 0)

check('milliseconds under a second are shown as ms', formatLatency(134) === '134 ms')
check('sub-millisecond readings round down', formatLatency(0.4) === '0 ms')
check('999 ms stays in milliseconds', formatLatency(999) === '999 ms')
check('a second and up is shown in seconds', formatLatency(1500) === '1.5 s')
check('no measurement formats to nothing', formatLatency(-1) === '')
check('NaN formats to nothing', formatLatency(Number.NaN) === '')

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exit(1)
