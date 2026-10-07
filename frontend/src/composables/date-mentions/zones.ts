import type { DateMention, FindDateMentionsOptions } from './types.ts'

// The timezone written next to a clock time (`AoE`, `CET`, `(UTC-12)`, `北京时间`) and
// the conversion into the reader's local time. Split out because it is the fiddliest
// part of the parse and shares nothing with the candidate scoring.

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

export function zoneLabelIn(matchText: string): string | null {
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
 * Absorbs a zone that follows a mention chrono stopped short of, so the chip covers the
 * whole expression and the moment is read in the right zone:
 *
 *   `September 15, 2026 AOE`           — chrono matches only the date, the zone dangles
 *   `September 25, 2026 (23:59) (PST)` — the zone sits after the bracket the merge closed
 *
 * A clock time is converted; a bare date keeps its day and only carries the label (there
 * is no instant to move, and shifting noon would silently change the date).
 */

export function absorbTrailingZone(
  text: string,
  mention: DateMention,
  options: FindDateMentionsOptions,
): DateMention {
  if (mention.zone) return mention

  const suffix = matchZoneSuffix(text, mention.index + mention.length, options.locale)
  if (!suffix) return mention

  let at = mention.at
  if (mention.hasTime) {
    const localOffset = -at.getTimezoneOffset()
    at = new Date(at.getTime() + (localOffset - suffix.minutes) * 60_000)
  }
  const length = mention.length + suffix.length
  return {
    ...mention,
    at,
    epoch: Math.floor(at.getTime() / 1000),
    length,
    text: text.slice(mention.index, mention.index + length),
    zone: suffix.label,
    zoneApplied: mention.hasTime,
  }
}

/**
 * Reads the timezone written after a clock time: a colloquial name from the table
 * (`AoE`, `PT`, `CET`) and/or an explicit offset (`(UTC-12)`, `GMT+5:30`). The explicit
 * offset wins; the returned length lets the chip cover the whole expression.
 */

export function matchZoneSuffix(text: string, from: number, locale?: string): ZoneSuffix | null {
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
