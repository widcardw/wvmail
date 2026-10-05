/**
 * Text for a count badge (sidebar unread counts, and anything else that needs one).
 *
 * Returns an empty string for zero and for anything that is not a positive number: a
 * badge reading "0" is noise, and `v-if` on the result is then the whole visibility rule.
 * Counts above `max` collapse to `{max}+` so a mailbox with thousands of unread messages
 * cannot stretch the row it sits in.
 */
function formatBadgeCount(count: number, max = 99): string {
  if (!Number.isFinite(count) || count <= 0) return ''
  const whole = Math.floor(count)
  return whole > max ? `${max}+` : String(whole)
}

export { formatBadgeCount }
