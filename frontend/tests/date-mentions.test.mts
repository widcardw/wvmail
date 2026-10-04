/**
 * Table tests for the email date/time detector.
 *
 * Run with: pnpm test   (Node ≥ 23 strips TypeScript types natively)
 *
 * The reference "now" is fixed so relative expressions stay deterministic:
 * 2026-03-01 10:00 local time — a Sunday.
 */
import { findDateMentions } from '../src/composables/date-mentions.ts'
import type { FindDateMentionsOptions } from '../src/composables/date-mentions.ts'

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

const NOW = new Date(2026, 2, 1, 10, 0, 0)

const pad = (n: number) => String(n).padStart(2, '0')
const local = (d: Date) =>
  `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`

function find(text: string, options: FindDateMentionsOptions = {}) {
  return findDateMentions(text, { now: NOW, locale: 'zh-CN', ...options })
}

/** First mention resolved to local time, or '(none)'. */
function first(text: string, options: FindDateMentionsOptions = {}) {
  const mentions = find(text, options)
  return mentions.length > 0 ? local(mentions[0].at) : '(none)'
}

console.log('formats we must support')
check('2026-03-05 15:30', first('会议 2026-03-05 15:30 开始') === '2026-03-05 15:30')
check('ISO date only', first('2026-03-05') === '2026-03-05 12:00')
check('2026年3月5日', first('2026年3月5日交材料') === '2026-03-05 12:00')
check('3月5日下午3点', first('3月5日下午3点复盘') === '2026-03-05 15:00')
check('明天下午两点', first('明天下午两点开会') === '2026-03-02 14:00')
check('周五 15:00', first('会议改到周五 15:00') === '2026-03-06 15:00')
check('Mar 5, 2026 at 3:30 PM', first('Board call Mar 5, 2026 at 3:30 PM') === '2026-03-05 15:30')
check('tomorrow 9am', first('standup tomorrow 9am') === '2026-03-02 09:00')
check('bare clock time rolls forward', first('见 9:05') === '2026-03-02 09:05')
check('deadline 23:59', first('截止 2026-03-05 23:59') === '2026-03-05 23:59')

console.log('match text and offsets')
const deadline = find('截止日期：2026-03-05 23:59 前提交')
check('one mention', deadline.length === 1)
check('offset points at the date', deadline[0]?.index === 5)
check('exact text', deadline[0]?.text === '2026-03-05 23:59')
check(
  'slice reproduces the match',
  deadline[0] !== undefined &&
    '截止日期：2026-03-05 23:59 前提交'.slice(
      deadline[0].index,
      deadline[0].index + deadline[0].length,
    ) === deadline[0].text,
)

const two = find('3月5日 15:30 开会，3月7日 10:00 复盘')
check('two mentions found', two.length === 2)
check('both resolvable', first('3月5日 15:30 开会，3月7日 10:00 复盘') === '2026-03-05 15:30')
check('second mention resolved', two[1] !== undefined && local(two[1].at) === '2026-03-07 10:00')
check(
  'mentions do not overlap',
  two.every((m, i) => i === 0 || m.index >= two[i - 1].index + two[i - 1].length),
)

console.log('flags')
const withTime = find('2026-03-05 15:30')
check('hasTime for a clock time', withTime[0]?.hasTime === true)
check('hasDate for a calendar date', withTime[0]?.hasDate === true)
const dateOnly = find('2026年3月5日')
check('date only has no time', dateOnly[0]?.hasTime === false)
check('date only still has a date', dateOnly[0]?.hasDate === true)
const clockOnly = find('见 9:05')
check('clock only has no date', clockOnly[0]?.hasDate === false)
check(
  'epoch matches the resolved moment',
  withTime[0]?.epoch === Math.floor(withTime[0].at.getTime() / 1000),
)

console.log('false positives we must not highlight')
const noise = [
  '版本 2.10.2 已发布',
  '服务器 192.168.1.1 不通',
  '报价 $1,234.56 元',
  '电话 138 0013 8000',
  '见 第3章',
  '2月30日 不存在',
  '2026年13月45日 不存在',
  '提交 abc13/55 号文件',
  '1.5 小时之后',
  'RFC 3339 格式',
]
for (const text of noise) {
  check(`no mention in ${JSON.stringify(text)}`, find(text).length === 0)
}

console.log('plausibility window (relative to the message, not the clock)')
check('over a year back ignored', find('2024-12-01 已发货').length === 0)
check('months back kept', first('2026-02-01 已发货') === '2026-02-01 12:00')
check('far future ignored', find('2099-01-01 到期').length === 0)
check('near future kept', first('2029-01-01') === '2029-01-01 12:00')
check('same-day earlier time kept', first('2026-03-01 08:00 开始') === '2026-03-01 08:00')
check('previous day kept', first('2026-02-28 22:00 发出') === '2026-02-28 22:00')
// A deadline announced after the fact is the whole point of the feature.
check(
  'deadline already past is kept',
  first('deadline was September 28, 2026, at 23:59', { now: new Date(2026, 9, 4, 12, 0) }) ===
    '2026-09-28 23:59',
)
check('bare weekday ignored', find('周四前给我').length === 0)
check('""', find('').length === 0)
check('no digits', find('这是一封普通邮件').length === 0)

console.log('timestamps with an explicit timezone')
// The Chinese parser stops at "UTC" (and then reads the offset as a clock time);
// the English one takes the whole thing. Merging must keep the longer match, and the
// moment must resolve to the offset, not to the local reading of "+08:00".
const tzRef = new Date(2026, 8, 25, 9, 0, 0)
const tz = find('截止时间：2026-09-25 11:28:04 UTC+08:00', { now: tzRef })
check('timezone stamp is one mention', tz.length === 1)
check('timezone stamp covers the whole value', tz[0]?.text === '2026-09-25 11:28:04 UTC+08:00')
check('offset fragment is not a separate mention', !tz.some((m) => m.text === '08:00'))
check(
  'offset applied to the instant',
  tz[0]?.epoch === Math.floor(Date.UTC(2026, 8, 25, 3, 28, 4) / 1000),
)
const tzIso = find('2026-09-25T11:28:04+08:00', { now: tzRef })
check('ISO offset form', tzIso.length === 1 && tzIso[0].text === '2026-09-25T11:28:04+08:00')
const tzMailLate = find('Sep 25, 2026, 11:28:04 AM UTC+08:00', { now: tzRef })
check('english offset form', tzMailLate.length === 1 && tzMailLate[0].hasTime)

console.log('timezone names written in the text')
// Epoch assertions only: the test must pass in any developer timezone.
const utc = (y: number, mo: number, d: number, h = 0, mi = 0) =>
  Math.floor(Date.UTC(y, mo - 1, d, h, mi) / 1000)

const aoeOnly = find('deadline 2026-09-28 23:59 AoE', { locale: 'en-US' })
check('AoE name recognised', aoeOnly.length === 1)
check('AoE is UTC-12', aoeOnly[0]?.epoch === utc(2026, 9, 29, 11, 59))
check('AoE label kept for the UI', aoeOnly[0]?.zone === 'AoE')

const parenOffset = find('call 2026-03-05 15:30 (UTC+05:30)', { locale: 'en-US' })
check('parenthesised offset applied', parenOffset[0]?.epoch === utc(2026, 3, 5, 10, 0))
check('parenthesised offset labelled', parenOffset[0]?.zone === 'UTC+05:30')
check(
  'parenthesised offset stays one chip',
  parenOffset[0]?.text === '2026-03-05 15:30 (UTC+05:30)',
)

const named = find('call 2026-03-05 15:30 CET', { locale: 'en-US' })
check('CET applied (chrono knows it natively)', named[0]?.epoch === utc(2026, 3, 5, 14, 30))
check('CET named for the dialog', named[0]?.zone === 'CET')

const noZone = find('call 2026-03-05 15:30', { locale: 'en-US' })
check(
  'no zone means local time',
  noZone[0]?.epoch === Math.floor(new Date(2026, 2, 5, 15, 30).getTime() / 1000),
)
check('no zone flag when none written', noZone[0]?.zone === null)

check(
  'lowercase two-letter abbreviation is not a zone',
  find('see you at 15:30 et al').length === 0,
)
const upperAbbrev = find('see you at 15:30 ET', { locale: 'en-US' })
check('uppercase abbreviation still is', upperAbbrev.length === 1)
check('uppercase abbreviation labelled', upperAbbrev[0]?.zone === 'ET')

console.log('deadline wording from real mail')
const aoe = 'The review deadline has been extended to September 28, 2026, at 23:59 AoE (UTC-12).'
const aoePast = find(aoe, { now: new Date(2026, 9, 4, 12, 0), locale: 'en-US' })
check('AOE deadline found at all', aoePast.length === 1)
check(
  'AOE chip covers the whole expression',
  aoePast[0]?.text === 'September 28, 2026, at 23:59 AoE (UTC-12)',
)
// 23:59 in UTC-12 is 11:59Z the next day; the reminder must hold the deadline instant,
// not the local reading of the printed clock time.
check(
  'AOE deadline resolves to the real instant',
  aoePast[0]?.epoch === Date.UTC(2026, 8, 29, 11, 59) / 1000,
)
check('AOE zone is reported for the dialog', aoePast[0]?.zone === 'AoE (UTC-12)')

console.log('a day and its time split by a bracket')
// chrono returns these as two results (`3 Oct` + `23:59 PST)`), which reads as two
// chips with a stray bracket; the detector glues complementary halves back together.
const bracketed = find('by 3 Oct (23:59 PST)', { locale: 'en-US' })
check('bracketed day+time is one mention', bracketed.length === 1)
check('bracketed chip is balanced', bracketed[0]?.text === '3 Oct (23:59 PST)')
check(
  'bracketed moment uses the date, not "tomorrow"',
  bracketed[0]?.epoch === utc(2026, 10, 4, 7, 59),
)
check('bracketed zone labelled', bracketed[0]?.zone === 'PST')

const reversed = find('23:59 PST on 3 Oct', { locale: 'en-US' })
check(
  'order does not matter',
  reversed.length === 1 && reversed[0].epoch === utc(2026, 10, 4, 7, 59),
)

const square = find('3 Oct [23:59 PST]', { locale: 'en-US' })
check('square brackets balanced too', square[0]?.text === '3 Oct [23:59 PST]')

const joined = find('3:30 PM on Mar 5', { locale: 'en-US' })
check('connective joins two halves', joined.length === 1 && joined[0].text === '3:30 PM on Mar 5')

// A word between them means they are separate things, not one expression.
const farApart = find('3 Oct 会议 15:30', { locale: 'zh-CN' })
check('unrelated neighbours stay separate', farApart.length === 2)

console.log('the message date is the reference for relative words')
// "明天" in a message from March must land on March 2nd, not on tomorrow.
check('relative day follows the message', first('明天 15:00 面试') === '2026-03-02 15:00')
check(
  'relative day follows an old message',
  first('明天 15:00 面试', { now: new Date(2026, 8, 25, 9, 0, 0) }) === '2026-09-26 15:00',
)

console.log('locale handling')
check('zh locale', first('3月5日 15:30', { locale: 'zh-CN' }) === '2026-03-05 15:30')
check('en locale', first('Mar 5 15:30', { locale: 'en-US' }) === '2026-03-05 15:30')
check(
  'en locale still reads Chinese mail',
  first('3月5日 15:30', { locale: 'en-US' }) === '2026-03-05 15:30',
)
check(
  'ambiguous 3/5 resolves the same either way',
  first('3/5', { locale: 'zh-CN' }) === first('3/5', { locale: 'en-US' }),
)

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exit(1)
