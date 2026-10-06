/**
 * Connection quality for the title-bar meter (`ConnectionStatus.vue`).
 *
 * The measurement comes from Go (`NetworkService.ProbeConnection`), which times a
 * connection up to the server's IMAP banner — TCP + TLS + one read, i.e. three or
 * four round trips. Measured on a normal home connection against the three
 * configured servers (imap.qq.com, imap.163.com, imap.exmail.qq.com) the probe
 * lands at 134–260 ms, so "good" has to start well above a bare RTT; the gap to
 * "fair" leaves room for a phone hotspot or a busy hotel network before the meter
 * calls the link bad.
 *
 * All the judgement lives here rather than in the Go service so it is one
 * testable function (see tests/connection.test.mts) instead of thresholds spread
 * across the binding boundary.
 */

export type ConnectionQuality = 'High' | 'Medium' | 'Low' | 'Off'

/** At or below this, the link counts as good (green, three bars). */
export const CONNECTION_GOOD_MS = 600
/** At or below this (but above good) the link counts as fair (amber, two bars). */
export const CONNECTION_FAIR_MS = 1500

/** How many of the meter's three bars a quality fills. */
export function connectionBars(quality: ConnectionQuality): number {
  switch (quality) {
    case 'High':
      return 3
    case 'Medium':
      return 2
    case 'Low':
      return 1
    case 'Off':
      return 0
  }
}

/**
 * Turn one probe result into a quality. `ok` decides reachability on its own: a
 * failed probe carries latency -1, and a negative or missing latency is never
 * treated as "fast".
 */
export function classifyConnection(
  ok: boolean,
  latencyMs: number,
  goodMs: number = CONNECTION_GOOD_MS,
  fairMs: number = CONNECTION_FAIR_MS,
): ConnectionQuality {
  if (!ok || !Number.isFinite(latencyMs) || latencyMs < 0) return 'Off'
  if (latencyMs <= goodMs) return 'High'
  if (latencyMs <= fairMs) return 'Medium'
  return 'Low'
}

/**
 * Latency for the tooltip: milliseconds while they are readable, seconds once a
 * stalled link is better described that way. Empty for "no measurement" so the
 * caller can drop the line instead of printing `-1 ms`.
 */
export function formatLatency(latencyMs: number): string {
  if (!Number.isFinite(latencyMs) || latencyMs < 0) return ''
  if (latencyMs < 1000) return `${Math.round(latencyMs)} ms`
  return `${(latencyMs / 1000).toFixed(1)} s`
}
