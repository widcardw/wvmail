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

import { collectMentions, mergeAdjacent } from './merge.ts'
import { type DateMention, type FindDateMentionsOptions } from './types.ts'
import { absorbTrailingZone } from './zones.ts'

// Public entry: `findDateMentions(text)` plus the types callers need. The stages live in
// zones.ts / candidates.ts / merge.ts; this file owns nothing but the orchestration.

export function findDateMentions(
  text: string,
  options: FindDateMentionsOptions = {},
): DateMention[] {
  const merged = mergeAdjacent(text, collectMentions(text, options), options)
  return merged.map((mention) => absorbTrailingZone(text, mention, options))
}

/**
 * What may sit between the day and the time of a single expression: punctuation,
 * brackets, whitespace, and the English connectives. Anything else ("会议", "and",
 * a new sentence) keeps two separate mentions.
 */

// The types the entry promises callers (`~/composables/date-mentions`).
export type { DateMention, FindDateMentionsOptions } from './types.ts'
