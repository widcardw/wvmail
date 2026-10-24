/**
 * Per-account connection health — the little "disconnected" marker the sidebar
 * draws next to an account's unread badge.
 *
 * Two signals feed it, and neither one costs a connection:
 *
 *  - **the STATUS sweep** the badges already run (`EmailService.FolderStatuses`,
 *    one dial for the whole folder list). A failure there means "the account
 *    cannot be reached right now", which is exactly what the marker is about.
 *  - **the IDLE watcher**, for accounts with auto-sync on: `live` says whether
 *    the persistent connection is up, so a drop is visible even for an account
 *    the user never expanded.
 *
 * The title-bar probe (`NetworkService.ProbeAccount`) is deliberately *not* a
 * third source: it measures one host at a time, stops before LOGIN, and is
 * already busy describing the selected account in the title bar. Reusing it per
 * account would mean dialling every server on a timer — with a 60s dial timeout,
 * one dead host would hold a connection slot for a minute while Gmail and 163
 * count concurrent connections.
 *
 * All the judgement lives here rather than in the store so it is testable without
 * Pinia (see tests/account-health.test.mts).
 */

/**
 * How much we trust one account's connection.
 *
 * `null` (no entry) is the fourth state and means "not measured": nothing has
 * said anything about this account yet, and the sidebar draws nothing rather than
 * guessing. That is what the store stores — this type is only for a measurement
 * that actually happened.
 */
export type AccountHealth = 'ok' | 'suspect' | 'down'

/**
 * Consecutive failed sweeps before an account counts as disconnected.
 *
 * One failure is not proof: a server can drop a single connection (concurrency
 * limit, a 5xx, a Wi-Fi roam) and answer the next sweep a moment later. Two in a
 * row is a minute of trying, which is worth a red icon.
 */
export const HEALTH_FAILURES_BEFORE_DOWN = 2

/**
 * Consecutive IDLE polls that must find the watcher disconnected before it shows.
 *
 * The watcher reconnects on its own within 30s of a drop (that is its first
 * backoff step), so a single poll is far too trigger-happy: closing the lid for a
 * second would flash a warning. Two polls is ~a minute of a watcher that has not
 * come back.
 */
export const HEALTH_IDLE_POLLS_BEFORE_SUSPECT = 2

/** What the last STATUS sweep said about an account. */
export interface StatusSignal {
  /** False until the first sweep has answered — "not measured" is not "fine". */
  measured: boolean
  /** Consecutive failures; 0 means the last sweep succeeded. */
  failures: number
}

/** What the IDLE watcher says about an account. */
export interface IdleSignal {
  /** The account has a watcher: auto-sync is on and the server did not refuse IDLE. */
  present: boolean
  /** The watcher's connection is up right now. */
  live: boolean
  /** It has been up at least once since launch. */
  everLive: boolean
  /** Consecutive polls that found it disconnected. */
  downPolls: number
}

/** Health implied by the last STATUS sweep, or null when nothing has measured it. */
export function healthFromStatus(signal: StatusSignal): AccountHealth | null {
  if (!signal.measured) return null
  if (signal.failures <= 0) return 'ok'
  return signal.failures >= HEALTH_FAILURES_BEFORE_DOWN ? 'down' : 'suspect'
}

/**
 * Health implied by the IDLE watcher, or null when it says nothing.
 *
 * A watcher that has never connected is silent on purpose: that is the state of
 * every account during the first seconds after launch (`idleStartDelay`), and of
 * an account added a moment ago. If it truly cannot connect, the STATUS sweep
 * reports it — that signal does not need a warm-up.
 */
export function healthFromIdle(signal: IdleSignal): AccountHealth | null {
  if (!signal.present) return null
  if (signal.live) return 'ok'
  if (!signal.everLive) return null
  return signal.downPolls >= HEALTH_IDLE_POLLS_BEFORE_SUSPECT ? 'suspect' : null
}

/**
 * What an account row draws. Two markers, because there are two different things
 * to say — and "fine" is not one of them:
 *
 *  - `degraded` — the account answers, but the connection is not holding: one
 *    failed sweep, or an IDLE watcher that has been reconnecting for a while.
 *  - `offline` — it cannot be reached at all: two sweeps in a row failed, or the
 *    settings page's connection test did.
 */
export type AccountMarker = 'degraded' | 'offline'

/**
 * The marker for a health state — or nothing.
 *
 * `ok` and "not measured" both draw nothing: the marker is only ever a warning.
 * The difference between them is invisible on screen but matters to
 * `combineHealth`, which is exactly why the two must not be conflated here — an
 * earlier version tested `healthOf(...)` for truthiness in the template, and
 * since `'ok'` is truthy, every healthy account rendered a marker.
 */
export function markerFromHealth(state: AccountHealth | null): AccountMarker | null {
  if (state === 'down') return 'offline'
  if (state === 'suspect') return 'degraded'
  return null
}

/**
 * Combine the two signals.
 *
 * The rules, in order:
 *  - `down` wins over everything. It comes from the operation the user actually
 *    performs — opening a folder dials a *new* connection — so a persistent IDLE
 *    session that happens to still be alive must not paint over it (the classic
 *    case: the server accepts the existing session but refuses new ones). Only a
 *    successful sweep, or the settings page's connection test, clears it.
 *  - a live IDLE connection clears a lone failed sweep: one blip plus proof of
 *    life is not worth a warning.
 *  - otherwise the more pessimistic of the two wins: a reconnecting watcher on a
 *    healthy account still means "the persistent connection is not holding".
 */
export function combineHealth(
  status: AccountHealth | null,
  idle: AccountHealth | null,
): AccountHealth | null {
  if (status === 'down' || idle === 'down') return 'down'
  if (idle === 'ok') return 'ok'
  if (status === 'suspect' || idle === 'suspect') return 'suspect'
  return status ?? idle
}
