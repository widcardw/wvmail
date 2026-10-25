import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { EmailService, NetworkService } from '#/wvmail/services'
import {
  ACCOUNT_HEALTH_EVENT_LIMIT,
  HEALTH_FAILURES_BEFORE_DOWN,
  applyAccountHealthEvent,
  enqueueAccountHealthEvents,
  combineHealth,
  healthFromIdle,
  healthFromStatus,
  markerFromHealth,
} from '~/composables/account-health'
import type {
  AccountHealthEvent,
  AccountHealth,
  AccountMarker,
  IdleSignal,
  StatusSignal,
} from '~/composables/account-health'

export type { AccountMarker } from '~/composables/account-health'

function normalizeHealthEvent(event: {
  seq?: unknown
  at?: unknown
  account?: unknown
  source?: unknown
  outcome?: unknown
}): AccountHealthEvent {
  const source = String(event.source ?? '')
  const outcome = String(event.outcome ?? '')
  return {
    seq: Number(event.seq) || 0,
    at: Number(event.at) || 0,
    account: String(event.account ?? ''),
    source: source === 'idle' || source === 'sync' || source === 'status' ? source : 'status',
    outcome:
      outcome === 'ok' || outcome === 'failure' || outcome === 'forgot' ? outcome : 'failure',
  }
}

/**
 * How often the IDLE states are re-read.
 *
 * Matches the watcher's own reconcile tick, and costs nothing: the call reads an
 * in-memory map of the connections that already exist. It is not a probe, so it
 * must not be compared with the title-bar meter's 20s/6s schedule — that one
 * dials a server every time.
 */
const IDLE_POLL_MS = 30_000

interface Signals {
  status: StatusSignal
  idle: IdleSignal
}

function blankSignals(): Signals {
  return {
    status: { measured: false, failures: 0 },
    idle: { present: false, live: false, everLive: false, downPolls: 0 },
  }
}

/**
 * Per-account connection health, for the sidebar marker.
 *
 * Holds the two raw signals (see composables/account-health.ts for what they mean
 * and how they combine) and nothing else: which icon and which colour belong to
 * the component.
 *
 * Deliberately does not import the accounts store: the flow is one-way
 * (accounts → health), so `accounts.loadFolderStatuses` reports the sweep's
 * outcome here instead of this store reaching back for the account list. That is
 * what keeps the sidebar's retry free of a store cycle.
 */
export const useAccountHealthStore = defineStore('accountHealth', () => {
  const signals = ref<Record<string, Signals>>({})
  /** Accounts with a retry in flight — the row shows a spinner instead of the icon. */
  const checking = ref<Record<string, boolean>>({})
  const pendingEvents = ref<AccountHealthEvent[]>([])
  let lastEventSeq = 0
  let draining = false
  let unlistenHealthEvents: (() => void) | null = null

  /**
   * The published state, derived rather than assigned: every note* call only
   * updates a signal, so there is exactly one place where "what does this account
   * look like" is decided.
   */
  const health = computed<Record<string, AccountHealth>>(() => {
    const out: Record<string, AccountHealth> = {}
    for (const [id, entry] of Object.entries(signals.value)) {
      const state = combineHealth(healthFromStatus(entry.status), healthFromIdle(entry.idle))
      if (state) out[id] = state
    }
    return out
  })

  /** `null` while nothing has measured the account — the row draws no icon. */
  function healthOf(accountId: string): AccountHealth | null {
    return health.value[accountId] ?? null
  }

  /**
   * What the row should draw, or null for "nothing".
   *
   * A getter rather than a template expression: `health` carries a third value
   * (`ok`) that must not be drawn, and "is this truthy" is the wrong question to
   * ask of it — see `markerFromHealth`.
   */
  function markerOf(accountId: string): AccountMarker | null {
    return markerFromHealth(health.value[accountId] ?? null)
  }

  function ensure(accountId: string): Signals {
    let entry = signals.value[accountId]
    if (!entry) {
      entry = blankSignals()
      signals.value[accountId] = entry
    }
    return entry
  }

  /** The STATUS sweep answered: the account is reachable, whatever it said before. */
  function noteStatusOk(accountId: string) {
    const entry = ensure(accountId)
    entry.status.measured = true
    entry.status.failures = 0
  }

  /** The STATUS sweep failed (one dial for the whole folder list). */
  function noteStatusFailure(accountId: string) {
    const entry = ensure(accountId)
    entry.status.measured = true
    entry.status.failures += 1
  }

  /**
   * The settings page's "test connection" ran. It logs in for real, so its answer
   * is stronger evidence than a sweep and it settles the state either way — a
   * failure jumps straight to `down` instead of waiting for a second one.
   */
  function noteTestResult(accountId: string, ok: boolean) {
    if (ok) {
      noteStatusOk(accountId)
      return
    }
    const entry = ensure(accountId)
    entry.status.measured = true
    entry.status.failures = Math.max(entry.status.failures, HEALTH_FAILURES_BEFORE_DOWN)
  }

  function setChecking(accountId: string, value: boolean) {
    if (value) checking.value[accountId] = true
    else delete checking.value[accountId]
  }

  /** Drop every trace of an account — it was removed, so nothing may linger. */
  function forget(accountId: string) {
    delete signals.value[accountId]
    delete checking.value[accountId]
  }

  function enqueueEvents(events: AccountHealthEvent[]) {
    const result = enqueueAccountHealthEvents(
      pendingEvents.value,
      events,
      lastEventSeq,
      ACCOUNT_HEALTH_EVENT_LIMIT,
    )
    pendingEvents.value = result.queue
    lastEventSeq = result.lastSeq
    void drainEvents()
  }

  async function drainEvents() {
    if (draining) return
    draining = true
    try {
      while (pendingEvents.value.length > 0) {
        const [event, ...rest] = pendingEvents.value
        pendingEvents.value = rest
        if (event.outcome === 'forgot') {
          forget(event.account)
          continue
        }
        applyAccountHealthEvent(ensure(event.account), event)
      }
    } finally {
      draining = false
    }
  }

  /**
   * Re-read the IDLE watcher states.
   *
   * An account absent from the answer has no watcher (auto-sync off, or the
   * server refused IDLE) and therefore contributes no signal; a present-false one
   * is in its reconnect backoff, and only becomes visible once it has stayed
   * there for a couple of polls (see healthFromIdle).
   */
  async function refreshIdle() {
    let states: Record<string, boolean | undefined>
    try {
      states = (await NetworkService.AccountIdleStates()) ?? {}
    } catch (e) {
      // A failed read is not evidence about any account: keep the last answer.
      console.error('Failed to read the IDLE states', e)
      return
    }

    for (const [id, entry] of Object.entries(signals.value)) {
      if (id in states) continue
      entry.idle.present = false
      entry.idle.live = false
      entry.idle.downPolls = 0
    }

    for (const [id, live] of Object.entries(states)) {
      const entry = ensure(id)
      entry.idle.present = true
      entry.idle.live = live === true
      if (entry.idle.live) {
        entry.idle.everLive = true
        entry.idle.downPolls = 0
      } else {
        entry.idle.downPolls += 1
      }
    }
  }

  let timer: number | null = null
  let started = false

  function tick() {
    // A hidden window (minimised, or closed to the tray) has no icon to update,
    // and the states it would read cannot have been acted on by anyone.
    if (!document.hidden) void refreshIdle()
    timer = window.setTimeout(tick, IDLE_POLL_MS)
  }

  function onWake() {
    void refreshIdle()
  }

  function onVisibilityChange() {
    if (!document.hidden) void refreshIdle()
  }

  function start() {
    // An explicit flag rather than "is a timer pending": the first refresh is in
    // flight before the timer exists, so a second mount would install a second
    // poller (the same reason stores/connection.ts has one).
    if (started) return
    started = true
    unlistenHealthEvents = Events.On('account-health', (event) => {
      enqueueEvents([normalizeHealthEvent(event.data)])
    })
    void catchUpEvents()
    window.addEventListener('focus', onWake)
    document.addEventListener('visibilitychange', onVisibilityChange)
    void refreshIdle()
    timer = window.setTimeout(tick, IDLE_POLL_MS)
  }

  function stop() {
    started = false
    if (timer !== null) {
      window.clearTimeout(timer)
      timer = null
    }
    window.removeEventListener('focus', onWake)
    document.removeEventListener('visibilitychange', onVisibilityChange)
    unlistenHealthEvents?.()
    unlistenHealthEvents = null
    pendingEvents.value = []
  }

  async function catchUpEvents() {
    try {
      const events = (await EmailService.AccountHealthEvents()) ?? []
      enqueueEvents(events.map(normalizeHealthEvent))
    } catch (e) {
      console.error('Failed to read account health events', e)
    }
  }

  return {
    health,
    checking,
    healthOf,
    markerOf,
    noteStatusOk,
    noteStatusFailure,
    noteTestResult,
    setChecking,
    forget,
    refreshIdle,
    start,
    stop,
  }
})
