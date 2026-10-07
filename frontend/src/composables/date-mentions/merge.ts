import {
  type Candidate,
  MAX_MATCH_LENGTH,
  MAX_MENTIONS,
  en,
  toCandidate,
  zh,
} from './candidates.ts'
import { type DateMention, type FindDateMentionsOptions } from './types.ts'

// Adjacent matches are usually one moment split by the parser (`3月5日` + `下午3点`).
// These merge them back together, and the joined text is reparsed to pick up a
// timezone that only becomes visible once the halves are adjacent.

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

export function mergeAdjacent(
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
export function collectMentions(
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
      zoneApplied: candidate.zoneApplied,
    })
    coveredUntil = candidate.index + candidate.length
    if (picked.length >= MAX_MENTIONS) break
  }
  return picked
}
