// The shapes the rest of the module trades in. Epoch **seconds** (not millis) is the
// project-wide unit for stored moments, so the detector speaks it too.

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
   * Timezone the mention named (`AoE`, `CET`, `UTC-12`), if any. A date-only mention
   * carries the label too — `September 15, 2026 AOE` names a zone even though there is
   * no clock time to convert.
   */
  zone: string | null
  /**
   * True when `at` was actually re-read in that zone (a clock time was present). The UI
   * only explains a conversion when this is set, so `AOE` on a bare date cannot look
   * like the day was shifted.
   */
  zoneApplied: boolean
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
