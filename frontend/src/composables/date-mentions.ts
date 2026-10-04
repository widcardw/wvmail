/**
 * Date/time detection for email bodies ("3月5日下午3点", "Mar 5, 2026 at 3:30 PM",
 * "明天 15:30" …), used to highlight those moments inside a message and offer to
 * turn them into reminders.
 *
 * We do NOT hand-roll the grammar: `chrono-node` already carries the locale tables
 * (month names, day periods, weekday words) and its own sanity checks — it rejects
 * `v1.2.3`, `192.168.1.1`, `$1,234.56`, `138 0013 8000`, `2月30日` and `第3章`
 * without any special-casing on our side. What this module adds on top:
 *
 *  - merge the Chinese and English parsers (mail mixes them regardless of UI
 *    language) and keep the strongest match when they overlap;
 *  - drop matches that carry neither a calendar day nor a clock time (a bare `周四`
 *    or `Thursday` is a vague reference, not a moment);
 *  - drop matches glued to digits/currency and matches resolving into the past or
 *    into the far future;
 *  - normalise everything into epoch seconds, the unit reminders are stored in.
 */
import * as chrono from 'chrono-node'
import type { ParsedResult } from 'chrono-node'

export interface DateMention {
  /** Offset of the mention inside the scanned text. */
  index: number
  /** Length of the matched text (already trimmed). */
  length: number
  /** The matched text exactly as it appeared in the message. */
  text: string
  /** Resolved moment, in the user's local timezone. */
  at: Date
  /** Epoch seconds (`at`), matching the reminder storage unit. */
  epoch: number
  /** The mention carried a clock time (`15:30`, `下午3点`). */
  hasTime: boolean
  /** The mention carried an explicit calendar day (`3月5日`, `tomorrow`). */
  hasDate: boolean
}

export interface FindDateMentionsOptions {
  /** Reference "now"; injectable so tests stay deterministic. */
  now?: Date
  /** App locale — decides which chrono language we try first. */
  locale?: string
}

type CertainField = 'year' | 'month' | 'day' | 'hour' | 'minute'

/** Calendar fields are worth more than clock fields when two matches overlap. */
const FIELD_WEIGHTS: Array<[CertainField, number]> = [
  ['year', 2],
  ['month', 2],
  ['day', 2],
  ['hour', 1],
  ['minute', 1],
]

/** A single moment is one reminder; anything past this is a pathological body. */
const MAX_MENTIONS = 40

/** Longest sensible match — avoids runaway spans covering half a paragraph. */
const MAX_MATCH_LENGTH = 40

/** Years beyond this are almost always a typo, and nothing to remind about yet. */
const MAX_AHEAD_YEARS = 5

/**
 * Junk that means the match is part of a bigger number/payment token. The `.` case
 * is not hypothetical: chrono reads `1.5 小时之后` as `5 小时之后` (a duration), so
 * without it we would highlight the tail of a decimal number.
 */
const GLUED_BEFORE = /[0-9.$¥€£]/
const GLUED_AFTER = /[0-9]/

const zh = chrono.zh.casual
const en = chrono.en.casual

interface Candidate extends DateMention {
  score: number
  order: number
}

function scoreOf(result: ParsedResult): number {
  let score = 0
  for (const [field, weight] of FIELD_WEIGHTS) {
    if (result.start.isCertain(field)) score += weight
  }
  return score
}

/** Reject matches that touch digits (`3/5` inside `13/55`) or a currency amount. */
function isStandalone(text: string, index: number, length: number): boolean {
  const before = index > 0 ? text[index - 1] : ''
  const after = index + length < text.length ? text[index + length] : ''
  if (before && GLUED_BEFORE.test(before)) return false
  if (after && GLUED_AFTER.test(after)) return false
  return true
}

/** A reminder in the past is useless; a reminder 20 years out is a typo. */
function isPlausible(at: Date, now: Date): boolean {
  if (at.getTime() < now.getTime() - 60_000) return false
  return at.getFullYear() <= now.getFullYear() + MAX_AHEAD_YEARS
}

function toCandidate(
  result: ParsedResult,
  text: string,
  now: Date,
  order: number,
): Candidate | null {
  // chrono sometimes includes surrounding spaces in `text` (e.g. " 3月5日").
  const leading = result.text.length - result.text.trimStart().length
  const value = result.text.trim()
  if (!value || value.length > MAX_MATCH_LENGTH) return null

  const hasTime = result.start.isCertain('hour') || result.start.isCertain('minute')
  const hasDate = result.start.isCertain('day') || result.start.isCertain('month')
  // A weekday word alone ("周四", "Thursday") resolves to *some* date, but the message
  // is usually talking about a recurring or implied appointment — not a moment.
  if (!hasTime && !hasDate) return null

  const index = result.index + leading
  if (!isStandalone(text, index, value.length)) return null

  const at = result.start.date()
  if (!isPlausible(at, now)) return null

  return {
    index,
    length: value.length,
    text: value,
    at,
    epoch: Math.floor(at.getTime() / 1000),
    hasTime,
    hasDate,
    score: scoreOf(result),
    order,
  }
}

/**
 * All moments worth offering as reminders, in document order. Overlapping matches
 * from the two languages collapse into the strongest one (most certain fields,
 * then longest text, then the UI language's parser).
 */
export function findDateMentions(
  text: string,
  options: FindDateMentionsOptions = {},
): DateMention[] {
  if (!text || text.length < 3) return []

  const now = options.now ?? new Date()
  // Mail mixes languages, so both parsers always run; the UI locale only breaks ties.
  const parsers = options.locale?.toLowerCase().startsWith('zh') ? [zh, en] : [en, zh]

  const candidates: Candidate[] = []
  parsers.forEach((parser, order) => {
    for (const result of parser.parse(text, now, { forwardDate: true })) {
      const candidate = toCandidate(result, text, now, order)
      if (candidate) candidates.push(candidate)
    }
  })

  candidates.sort(
    (a, b) =>
      a.index - b.index ||
      b.score - a.score ||
      b.length - a.length ||
      a.order - b.order,
  )

  const picked: DateMention[] = []
  let coveredUntil = -1
  for (const candidate of candidates) {
    if (candidate.index < coveredUntil) continue
    picked.push({
      index: candidate.index,
      length: candidate.length,
      text: candidate.text,
      at: candidate.at,
      epoch: candidate.epoch,
      hasTime: candidate.hasTime,
      hasDate: candidate.hasDate,
    })
    coveredUntil = candidate.index + candidate.length
    if (picked.length >= MAX_MENTIONS) break
  }
  return picked
}
