/**
 * How many recipients fit on one line.
 *
 * A message from a large organisation can carry a hundred addresses in To and Cc
 * (mailing lists, ticket systems that add every watcher, "reply all" chains). The
 * reading pane used to join them into one run of text, so a single such message
 * pushed the subject and the body off-screen.
 *
 * The threshold has to be *the first line that is actually full*, not "more than
 * three addresses": one address can be as wide as four short ones, and the same
 * list fits or does not fit depending on the window width. So the component
 * measures the real widths and this function decides the cut — kept pure so the
 * rule is testable without a browser.
 */

/** What to render: how many addresses, and what the trailing chip stands for. */
export interface RecipientLayout {
  /** Addresses shown before the chip. */
  visible: number
  /** Addresses the chip stands for; 0 means "everything fits, no chip". */
  hidden: number
}

/**
 * Greedy first-line fit.
 *
 * `widths` are the measured widths of each address, `available` the width of the
 * value box, `chipWidth` the width of the "等 N 个收件人" chip, `separatorWidth`
 * the width of the ", " between two addresses. Every address that fits together
 * with the chip is kept; the first one that would push the chip onto the next
 * line ends the run.
 */
export function layoutRecipients(
  widths: number[],
  available: number,
  chipWidth: number,
  separatorWidth: number,
): RecipientLayout {
  const total = widths.length
  if (total === 0) return { visible: 0, hidden: 0 }
  // Not measured yet (hidden pane, first frame): show everything rather than an
  // empty row — the component re-runs this as soon as it has a width.
  if (available <= 0) return { visible: total, hidden: 0 }

  let used = 0
  for (let i = 0; i < total; i++) {
    const width = widths[i] + (i > 0 ? separatorWidth : 0)
    const isLast = i === total - 1
    // The last address does not have to leave room for the chip: if it fits, the
    // whole list fits and there is no chip at all.
    const reserved = isLast ? 0 : chipWidth + separatorWidth
    if (used + width + reserved > available) {
      if (i === 0) {
        // A single address wider than the row. Keep it and let the container clip
        // it — an empty row with a "+N" chip reads as a bug.
        return { visible: 1, hidden: total - 1 }
      }
      return { visible: i, hidden: total - i }
    }
    used += width
  }
  return { visible: total, hidden: 0 }
}
