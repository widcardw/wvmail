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
 *  - drop matches glued to digits/currency, dates that are clearly history relative
 *    to the message, and dates far in the future;
 *  - read the timezone written next to a clock time (`AoE`, `CET`, `(UTC-12)`) and
 *    convert the moment into the reader's local time;
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
  /**
   * Timezone the clock time was written in, when the text named one (`AoE`, `CET`,
   * `UTC-12`). `at` is already converted to the reader's local time; this is only so
   * the UI can explain why the time differs from the text.
   */
  zone: string | null
}

export interface FindDateMentionsOptions {
  /**
   * Reference moment: **the date of the message**, not "now". It drives both
   * relative expressions (`明天` in a message from last month means the day after
   * *that* day) and the plausibility window. Injectable so tests stay deterministic.
   */
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
const MAX_MATCH_LENGTH = 64

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
const ZONE_OFFSETS: Record<string, number> = {
  aoe: -720, // Anywhere on Earth (UTC-12)
  utc: 0,
  gmt: 0,
  wet: 0,
  west: 60,
  bst: 60,
  cet: 60,
  cest: 120,
  eet: 120,
  eest: 180,
  msk: 180,
  pt: -480,
  pst: -480,
  pdt: -420,
  mt: -420,
  mst: -420,
  mdt: -360,
  et: -300,
  est: -300,
  edt: -240,
  ct: -360,
  cdt: -300,
  jst: 540,
  kst: 540,
  hkt: 480,
  sgt: 480,
  ist: 330,
  aest: 600,
  aedt: 660,
  acst: 570,
  acdt: 630,
  nzst: 720,
  nzdt: 780,
}

/** Names whose meaning depends on the reader: CST is China in a zh UI, US Central otherwise. */
const AMBIGUOUS_ZONES: Record<string, { zh: number; other: number }> = {
  cst: { zh: 480, other: -360 },
}

/** Longest zone expression accepted after a clock time ("AoE (UTC-12)" is 13). */
const MAX_ZONE_LENGTH = 24

/** A zone name, optionally parenthesised: `AoE`, `(CET)`. */
const ZONE_NAME_RE = /^\s{0,3}\(?([A-Za-z]{2,8})\)?/
/** An explicit offset, optionally parenthesised: `(UTC-12)`, `GMT+5:30`. */
const ZONE_OFFSET_RE = /^\s{0,3}\(?\s*(?:UTC|GMT)\s*([+-])\s*(\d{1,2})(?::?(\d{2}))?\s*\)?/

interface ZoneSuffix {
  /** Characters consumed after the clock time. */
  length: number
  /** Offset of that zone, in minutes east of UTC. */
  minutes: number
  /** What to show the user — the name they wrote, plus the offset when both appear. */
  label: string
}

function zoneOffsetFor(name: string, locale?: string): number | null {
  const key = name.toLowerCase()
  const ambiguous = AMBIGUOUS_ZONES[key]
  if (ambiguous) return locale?.toLowerCase().startsWith('zh') ? ambiguous.zh : ambiguous.other
  return key in ZONE_OFFSETS ? ZONE_OFFSETS[key] : null
}

function explicitOffset(
  match: RegExpExecArray,
  consumedBefore: number,
  name: string | null,
): ZoneSuffix | null {
  const minutes = (match[1] === '-' ? -1 : 1) * (Number(match[2]) * 60 + Number(match[3] ?? 0))
  if (Math.abs(minutes) > 14 * 60) return null
  const offset = `UTC${match[1]}${match[2]}${match[3] ? ':' + match[3] : ''}`
  // Noon in UTC-12 is not midnight in UTC+12; a bogus offset is worse than none.
  return {
    length: consumedBefore + match[0].length,
    minutes,
    label: name ? `${name} (${offset})` : offset,
  }
}

/**
 * The timezone expression a match *ends* with, for labelling a moment chrono already
 * converted itself (`CET`, `(UTC+05:30)`, `UTC-12`). Returns null when there is none.
 */
function zoneLabelIn(matchText: string): string | null {
  const offset = /(?:UTC|GMT)\s*([+-])\s*(\d{1,2})(?::?(\d{2}))?/.exec(matchText)
  if (offset) return `UTC${offset[1]}${offset[2]}${offset[3] ? ':' + offset[3] : ''}`
  // Any recognized name inside the match: `3 Oct 23:59 PST` and `23:59 PST on 3 Oct`
  // both carry the zone in a different place.
  for (const token of matchText.match(/[A-Za-z]{2,8}/g) ?? []) {
    if (token.length <= 2 && token !== token.toUpperCase()) continue
    if (/^(utc|gmt)$/i.test(token) || zoneOffsetFor(token) !== null) return token
  }
  return null
}

/**
 * Reads the timezone written after a clock time: a colloquial name from the table
 * (`AoE`, `PT`, `CET`) and/or an explicit offset (`(UTC-12)`, `GMT+5:30`). The explicit
 * offset wins; the returned length lets the chip cover the whole expression.
 */
function matchZoneSuffix(text: string, from: number, locale?: string): ZoneSuffix | null {
  const rest = text.slice(from, from + MAX_ZONE_LENGTH)
  if (!rest) return null

  // An offset on its own ("(UTC-12)") — chrono handles the unparenthesised form itself.
  const direct = ZONE_OFFSET_RE.exec(rest)
  if (direct) return explicitOffset(direct, 0, null)

  const nameMatch = ZONE_NAME_RE.exec(rest)
  if (!nameMatch) return null
  const raw = nameMatch[1]
  // Two-letter abbreviations are only read as zones when written like one ("ET", not "et").
  if (raw.length <= 2 && raw !== raw.toUpperCase()) return null

  const trailing = ZONE_OFFSET_RE.exec(rest.slice(nameMatch[0].length))
  if (trailing) return explicitOffset(trailing, nameMatch[0].length, raw)

  const offset = zoneOffsetFor(raw, locale)
  return offset === null ? null : { length: nameMatch[0].length, minutes: offset, label: raw }
}

/**
 * How far *before* the reference a moment may sit and still be shown. Deadlines are
 * routinely mentioned after the fact ("the review deadline has been extended to
 * September 28, 2026, at 23:59" arrives days later), and reminders exist to be
 * reviewed, so the window is deliberately generous — only genuinely historical
 * context (over a year before the message) is treated as noise. The confirmation
 * dialog flags a moment that is already past.
 *
 * (Measured against the *message* date, so this rarely fires at all: a 2023 message
 * may legitimately talk about 2023.)
 */
const PAST_GRACE_MS = 365 * 24 * 60 * 60 * 1000

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

/**
 * Keep anything from the message's era onwards, drop long-past history and obvious
 * typos. Judging "past" against the *message* rather than the clock is what makes an
 * old mail's deadlines detectable at all.
 */
function isPlausible(at: Date, now: Date): boolean {
  if (at.getTime() < now.getTime() - PAST_GRACE_MS) return false
  return at.getFullYear() <= now.getFullYear() + MAX_AHEAD_YEARS
}

function toCandidate(
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

  if (hasTime && result.start.isCertain('timezoneOffset')) {
    // chrono applied the offset itself (`UTC+08:00`, `CET`, `(UTC+05:30)`): the instant
    // is already right, we only want the label so the dialog can explain it.
    zone = zoneLabelIn(value)
  } else if (hasTime) {
    const suffix = matchZoneSuffix(text, index + length, locale)
    if (suffix) {
      // chrono read the clock time as local; re-read it in the zone the text names.
      const localOffset = -at.getTimezoneOffset()
      at = new Date(at.getTime() + (localOffset - suffix.minutes) * 60_000)
      length += suffix.length
      zone = suffix.label
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
export function findDateMentions(
  text: string,
  options: FindDateMentionsOptions = {},
): DateMention[] {
  return mergeAdjacent(text, collectMentions(text, options), options)
}

/**
 * What may sit between the day and the time of a single expression: punctuation,
 * brackets, whitespace, and the English connectives. Anything else ("会议", "and",
 * a new sentence) keeps two separate mentions.
 */
const MERGE_GLUE_RE = /^(?:[\s,，、:：@~\-–—()（）[\]【】]|at|on){0,6}$/i

function isMergeGlue(gap: string): boolean {
  return MERGE_GLUE_RE.test(gap)
}

/** One half names a day, the other names a clock time — together they are one moment. */
function areComplements(a: DateMention, b: DateMention): boolean {
  return (
    (a.hasDate && !a.hasTime && b.hasTime && !b.hasDate) ||
    (a.hasTime && !a.hasDate && b.hasDate && !b.hasTime)
  )
}

/**
 * `by 3 Oct (23:59 PST)` reaches us as two results — the bracket stops chrono from
 * joining the day to the time, and the second half even swallows the stray `)`. Glue
 * such neighbours back together and let the parsers read the joined expression, which
 * is also what gets the timezone conversion right (`3 Oct 23:59 PST`, not "tomorrow").
 * Declines the merge when the joined text still does not parse as one moment.
 */
function mergeAdjacent(
  text: string,
  mentions: DateMention[],
  options: FindDateMentionsOptions,
): DateMention[] {
  if (mentions.length < 2) return mentions

  const merged: DateMention[] = []
  for (let i = 0; i < mentions.length; i++) {
    const first = mentions[i]
    const second = mentions[i + 1]
    if (second) {
      const span = text.slice(first.index, second.index + second.length)
      const glued =
        areComplements(first, second) &&
        span.length <= MAX_MATCH_LENGTH &&
        isMergeGlue(text.slice(first.index + first.length, second.index))
      if (glued) {
        const combined = reparseJoined(text, span, first.index, options)
        if (combined) {
          merged.push(combined)
          i++
          continue
        }
      }
    }
    merged.push(first)
  }
  return merged
}

/**
 * Re-reads a joined span with the same parsers. Brackets become spaces — same length,
 * so every offset still maps onto the original text — and only a single match covering
 * both halves is accepted.
 */
function reparseJoined(
  text: string,
  span: string,
  offset: number,
  options: FindDateMentionsOptions,
): DateMention | null {
  const normalized = span.replace(/[()（）[\]【】]/g, ' ')
  const inner = collectMentions(normalized, options)
  if (inner.length !== 1) return null
  const only = inner[0]
  if (!only.hasDate || !only.hasTime) return null

  // `3 Oct (23:59 PST)` parses as `3 Oct  23:59 PST`: the closing bracket now sits just
  // past the match, so take it back — a chip must not end mid-bracket.
  const index = offset + only.index
  let length = only.length
  const next = text[index + length]
  if (next && CLOSING_BRACKETS.has(next) && hasUnclosedBracket(text.slice(index, index + length))) {
    length += 1
  }
  return { ...only, index, length, text: text.slice(index, index + length) }
}

const CLOSING_BRACKETS = new Set([')', '）', ']', '】'])
const OPENING_BRACKETS = new Set(['(', '（', '[', '【'])

function hasUnclosedBracket(slice: string): boolean {
  let depth = 0
  for (const char of slice) {
    if (OPENING_BRACKETS.has(char)) depth++
    else if (CLOSING_BRACKETS.has(char)) depth--
  }
  return depth > 0
}

/** Parses and de-duplicates; the caller may glue adjacent halves afterwards. */
function collectMentions(text: string, options: FindDateMentionsOptions = {}): DateMention[] {
  if (!text || text.length < 3) return []

  const now = options.now ?? new Date()
  // Mail mixes languages, so both parsers always run; the UI locale only breaks ties.
  const parsers = options.locale?.toLowerCase().startsWith('zh') ? [zh, en] : [en, zh]

  const candidates: Candidate[] = []
  parsers.forEach((parser, order) => {
    for (const result of parser.parse(text, now, { forwardDate: true })) {
      const candidate = toCandidate(result, text, now, order, options.locale)
      if (candidate) candidates.push(candidate)
    }
  })

  candidates.sort(
    (a, b) => a.index - b.index || b.score - a.score || b.length - a.length || a.order - b.order,
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
      zone: candidate.zone,
    })
    coveredUntil = candidate.index + candidate.length
    if (picked.length >= MAX_MENTIONS) break
  }
  return picked
}
