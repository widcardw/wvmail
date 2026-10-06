import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import { NetworkService } from '#/wvmail/services'
import { useAccountsStore } from '~/stores/accounts'
import { classifyConnection, connectionBars, formatLatency } from '~/composables/connection'
import type { ConnectionQuality } from '~/composables/connection'

/**
 * How often the meter re-probes while the window is visible.
 *
 * The interval follows the state: a healthy link is only re-checked every 20s
 * (nothing useful changes that fast, and each probe is a real connection), while a
 * bad or failed one is re-checked every 6s so recovery shows up almost at once —
 * which is the moment the user is actually staring at the icon.
 */
const HEALTHY_INTERVAL_MS = 20_000
const DEGRADED_INTERVAL_MS = 6_000

/**
 * Live connection quality for the title-bar meter.
 *
 * The probe itself is Go (`NetworkService.ProbeConnection`, see
 * services/networkservice.go); this store owns *when* to probe, what the last
 * answer means, and which server it described. It measures the active account —
 * the mailbox the user is looking at — and falls back to the first configured one
 * so the icon is meaningful before anything is selected.
 */
export const useConnectionStore = defineStore('connection', () => {
  const accounts = useAccountsStore()

  /**
   * `null` means "not measured yet": the icon stays hidden rather than showing a
   * grey "offline" that nobody has verified.
   */
  const quality = ref<ConnectionQuality | null>(null)
  /** Last measured latency in ms, -1 when the last probe failed. */
  const latencyMs = ref(-1)
  /** How many attempts the last successful probe needed (2 = the retry saved it). */
  const attempts = ref(0)
  const host = ref('')
  const error = ref('')
  const probing = ref(false)
  const lastCheckedAt = ref(0)

  const bars = computed(() => connectionBars(quality.value ?? 'Off'))
  const latencyText = computed(() => formatLatency(latencyMs.value))

  let timer: number | null = null
  let started = false
  let stopped = true
  let stopTargetWatch: (() => void) | null = null

  /** The account the meter describes: the selected one, else the first configured. */
  function target() {
    return accounts.selectedAccount ?? accounts.accounts[0] ?? null
  }

  async function probe() {
    if (probing.value) return

    const account = target()
    if (!account) {
      // No account configured: there is nothing to measure and no icon to show.
      quality.value = null
      host.value = ''
      return
    }

    probing.value = true
    try {
      const res = await NetworkService.ProbeConnection(
        account.IMAPHost,
        account.IMAPPort,
        account.UseTLS,
      )
      host.value = account.IMAPHost
      attempts.value = res.Attempts
      error.value = res.Error
      latencyMs.value = res.Ok ? res.LatencyMS : -1
      quality.value = classifyConnection(res.Ok, res.LatencyMS)
      lastCheckedAt.value = Date.now()
    } catch (e) {
      // The binding failing is not the network failing, but showing the previous
      // state forever would be worse: the meter goes to "off" and the tooltip
      // carries the real reason.
      host.value = account.IMAPHost
      attempts.value = 0
      latencyMs.value = -1
      error.value = e instanceof Error ? e.message : String(e)
      quality.value = 'Off'
      lastCheckedAt.value = Date.now()
    } finally {
      probing.value = false
    }
  }

  function nextDelay() {
    return quality.value === 'High' ? HEALTHY_INTERVAL_MS : DEGRADED_INTERVAL_MS
  }

  function schedule(delay: number) {
    if (stopped) return
    timer = window.setTimeout(() => {
      void tick()
    }, delay)
  }

  async function tick() {
    // Hidden window (minimised, or closed to the tray): keep the timer alive but
    // stop spending connections on an icon nobody can see.
    if (!document.hidden) await probe()
    schedule(nextDelay())
  }

  /** Called when the OS/app regains the network or the window comes back. */
  function probeNow() {
    void probe()
  }

  function start() {
    // An explicit flag rather than "is a timer pending": right after start() the
    // first probe is in flight and the timer does not exist yet, so a second mount
    // would otherwise install a second poller.
    if (started) return
    started = true
    stopped = false

    // Re-probe when the meter's subject changes (accounts finishing their load,
    // or the user picking another account) instead of waiting up to 20s: the
    // tooltip names the host it measured, so a stale host would be a lie.
    stopTargetWatch = watch(
      () => target()?.ID ?? '',
      () => probeNow(),
    )

    window.addEventListener('online', probeNow)
    window.addEventListener('offline', probeNow)
    window.addEventListener('focus', probeNow)
    document.addEventListener('visibilitychange', onVisibilityChange)

    void tick()
  }

  function stop() {
    started = false
    stopped = true
    if (timer !== null) {
      window.clearTimeout(timer)
      timer = null
    }
    stopTargetWatch?.()
    stopTargetWatch = null
    window.removeEventListener('online', probeNow)
    window.removeEventListener('offline', probeNow)
    window.removeEventListener('focus', probeNow)
    document.removeEventListener('visibilitychange', onVisibilityChange)
  }

  function onVisibilityChange() {
    if (!document.hidden) probeNow()
  }

  return {
    quality,
    bars,
    latencyMs,
    latencyText,
    attempts,
    host,
    error,
    probing,
    lastCheckedAt,
    probe,
    start,
    stop,
  }
})
