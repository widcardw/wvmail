import * as chrono from 'chrono-node'
import type { ParsedResult } from 'chrono-node'
import { type DateMention } from './types.ts'
import { matchZoneSuffix, zoneLabelIn } from './zones.ts'

// chrono-node + the guards that make it usable on real mail: merge the Chinese and
// English parsers, drop matches that name neither a day nor a clock time, and reject
// dates glued to digits/currency or clearly outside the plausible window.

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
export const MAX_MENTIONS = 40

/** Longest sensible match — avoids runaway spans covering half a paragraph. */
export const MAX_MATCH_LENGTH = 64

/** Years beyond the reference are almost always a typo, and nothing to remind about yet. */
const MAX_AHEAD_YEARS = 5

/**
 * Minutes east of UTC for the timezone names chrono-node does not know. Mail uses them
 * constantly ("23:59 AoE", "15:00 CET"), and reading such a clock time as if it were
 * local can move a deadline by most of a day.
 *
 * The list stays with the common, mostly unambiguous names — a wrong guess is visible
 * in the confirmation dialog, but a long tail of ambiguous abbreviations would make
 * that noise constant. An offset written in the text always wins over this table.
 */

const PAST_GRACE_MS = 365 * 24 * 60 * 60 * 1000

/**
 * Junk that means the match is part of a bigger number/payment token. The `.` case
 * is not hypothetical: chrono reads `1.5 小时之后` as `5 小时之后` (a duration), so
 * without it we would highlight the tail of a decimal number.
 */

const GLUED_BEFORE = /[0-9.$¥€£]/

const GLUED_AFTER = /[0-9]/

export const zh = chrono.zh.casual

export const en = chrono.en.casual

export interface Candidate extends DateMention {
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

/**
 * Keep anything from the message's era onwards, drop long-past history and obvious
 * typos. Judging "past" against the *message* rather than the clock is what makes an
 * old mail's deadlines detectable at all.
 */

function isPlausible(at: Date, now: Date): boolean {
  if (at.getTime() < now.getTime() - PAST_GRACE_MS) return false
  return at.getFullYear() <= now.getFullYear() + MAX_AHEAD_YEARS
}

export function toCandidate(
  result: ParsedResult,
  text: string,
  now: Date,
  order: number,
  locale?: string,
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

  // chrono's own timezone parser accepts lowercase two-letter abbreviations, which
  // collide with ordinary words ("15:30 et al" becomes Eastern Time). Zones written
  // that way are dropped, matching the uppercase-only rule for our own table.
  if (result.start.isCertain('timezoneOffset') && /(^|\s)[a-z]{2}$/.test(value)) return null

  const index = result.index + leading
  let length = value.length
  let at = result.start.date()
  let zone: string | null = null

  let zoneApplied = false
  if (hasTime && result.start.isCertain('timezoneOffset')) {
    // chrono applied the offset itself (`UTC+08:00`, `CET`, `(UTC+05:30)`): the instant
    // is already right, we only want the label so the dialog can explain it.
    zone = zoneLabelIn(value)
    zoneApplied = zone !== null
  } else if (hasTime) {
    const suffix = matchZoneSuffix(text, index + length, locale)
    if (suffix) {
      // chrono read the clock time as local; re-read it in the zone the text names.
      const localOffset = -at.getTimezoneOffset()
      at = new Date(at.getTime() + (localOffset - suffix.minutes) * 60_000)
      length += suffix.length
      zone = suffix.label
      zoneApplied = true
    }
  }

  const mentionText = text.slice(index, index + length)
  if (!mentionText || mentionText.length > MAX_MATCH_LENGTH) return null
  if (!isStandalone(text, index, length)) return null
  if (!isPlausible(at, now)) return null

  return {
    index,
    length,
    text: mentionText,
    at,
    epoch: Math.floor(at.getTime() / 1000),
    hasTime,
    hasDate,
    zone,
    zoneApplied,
    score: scoreOf(result),
    order,
  }
}

/**
 * All moments worth offering as reminders, in document order. Overlapping matches
 * from the two languages collapse into the strongest one (most certain fields,
 * then longest text, then the UI language's parser), and neighbouring halves of one
 * expression are glued back together afterwards (see `mergeAdjacent`).
 */
