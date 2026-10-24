---
description: The connection indicators — the title-bar meter (what it measures, why the probe is stdlib rather than dialIMAP, the latency thresholds, the polling schedule) and the sidebar's per-account "disconnected" marker
---

# Title-bar connection meter

The title bar shows a three-bar signal meter immediately left of the theme switch.
It answers "is the mailbox reachable right now, and how fast does it answer?" for the
**active account**, and turns that into four states.

```
store/connection.ts (polling + state)          components/icon/ConnectionStatus.vue
  start() ──▶ probe()                            ┌─ 3 CSS bars, filled per state
              └─▶ NetworkService.ProbeAccount    │  (green / amber / red / grey)
                    (Go, ≤3s per attempt)        └─ Ark Tooltip: status, host · latency,
                    TCP + TLS + IMAP banner          retry note, last-check time, hint
```

| State | Bars | When |
|---|---|---|
| `High` | 3 green | probe answered in ≤ 600 ms |
| `Medium` | 2 amber | ≤ 1500 ms |
| `Low` | 1 red | slower than 1500 ms (the probe gives up at 3 s) |
| `Off` | grey + red ✗ | both attempts failed (or no host configured) |

## What exactly is measured

`services/networkservice.go` connects to the account's `IMAPHost:IMAPPort` and reads
the server's IMAP banner. That is **TCP + TLS + one read — three or four round
trips** — so the numbers are larger than a ping: measured from this machine,
`imap.qq.com`, `imap.163.com` and `imap.exmail.qq.com` all land at **134–260 ms**,
which is why "good" starts at 600 ms rather than at a bare RTT. The thresholds live
in `frontend/src/composables/connection.ts` as one testable function
(`classifyConnection`), not in Go, so the judgement is not split across the binding.

Two things the probe deliberately does **not** do:

- **It never logs in.** A login every few seconds would open a session and spend a
  credential attempt each time; a rate limit or a rejected password is not a network
  problem, and the sync path reports those on its own.
- **It skips STARTTLS on plaintext accounts.** `dialIMAP` falls back to an insecure
  connection when STARTTLS is refused, so a plaintext account that answers its banner
  is one the mail path can actually use.

A non-IMAP answer (captive portal, HTTP port) is treated as *unreachable*, because
"reachable but unusable" must not read as a healthy connection.

## Why the probe does not use `dialIMAP` / imapclient

It cannot outlive its own timeout, and imapclient applies its own 30 s response
timeout inside `DialStartTLS`/`WaitGreeting`. **Measured while writing this: a silent
server made a "3 second" probe take 60 s** (`go-imap/v2 v2.0.0-beta.8`, no
`CmdTimeout` in `Options`). A raw socket with one `SetDeadline` — plus its own
`readBanner` — bounds the entire exchange, which is what a UI tick needs. The
regression test is `TestProbeIMAPReachableBannerTimeout` (must finish in well under a
second with a 250 ms budget).

## Schedule

| Trigger | Why |
|---|---|
| ±20 s while healthy, ±6 s while `Medium`/`Low`/`Off` | recovery is what the user is staring at; a good link does not need re-checking often |
| window `focus`, `online`, `offline`, tab visible again | the moment the answer can have changed |
| account switch / accounts finishing their load | the tooltip names the host it measured, so a stale host would be a lie |
| click on the icon | manual re-check; the bars pulse while a probe is in flight |
| hidden window (minimised / closed to tray) | timer keeps running, probes are skipped |

The store probes the **selected** account and falls back to the first configured one.
Before the first answer the icon is hidden entirely rather than showing a grey
"offline" nobody verified. If no account is configured at all it never appears.

It passes the account **ID**, not host/port, so Go resolves the route exactly as
`dialIMAP` will — direct, that account's own proxy, or the global one
(`.ai-docs/proxy.md`). Measuring a different path than the mail takes would read
"Off" for a mailbox that works through its proxy, and the sidebar's retry refuses
to dial while the meter reads Off.

## Deliberate limits

- It measures the **mail server**, not general internet quality — the tooltip names the
  host, so the claim is never more than it looks.
- A failed first attempt is retried once after 250 ms; `Attempts > 1` shows as
  "succeeded on retry" in the tooltip, which is the difference between slow and flaky.
- Nothing here is persisted: the meter is live state only, and it resets on restart.
- It describes **one account** (the selected one). Which *other* account is
  disconnected is the sidebar marker's job — see the next section.

## Per-account markers (sidebar)

An account row shows a small connection icon immediately left of its unread badge
(`components/sidebar/SideBar.vue`). This is the half of the question the meter
cannot answer: with several mailboxes configured, "the network is fine" says
nothing about which one stopped working.

The marker **never dials**. It is built from two signals that already exist, so
showing it costs no connections:

| Signal | Source | Cost |
|---|---|---|
| STATUS sweep failed | `loadFolderStatuses` → `EmailService.FolderStatuses` | the sweep the badges already run (one dial per account, throttled 30 s) |
| IDLE watcher down | `NetworkService.AccountIdleStates` → `EmailIdleService.IdleStates` | none — reads an in-memory map, polled every 30 s |

| Marker | Icon | Colour | When |
|---|---|---|---|
| *(none)* | — | — | nothing has measured it yet, **or it is fine** |
| `degraded` | `streamline:interface-link-broken-…` | red | one failed sweep, or a watcher that has been down for ≥2 polls (≈1 min) |
| `offline` | `streamline:broken-link-2` | grey | two consecutive failed sweeps, or a failed connection test |

The signal → marker mapping lives in `markerOf()` (`stores/accountHealth.ts`), not
in the template. `health` has a third value, `ok`, which is *not* something to
draw; testing `healthOf(...)` for truthiness made every healthy account render a
marker (in the inherited text colour, which read as "all my accounts are greyed
out"). Grey means "no connection at all" — a fact, stated quietly — while red is
reserved for a connection that answers but does not hold.

The icons must come from a monochrome set: `@iconify-json/streamline` draws with
`currentColor`, so UnoCSS emits a `mask` rule and the CSS colour is the state.
`@iconify-json/streamline-color` (also installed) hard-codes `#4147d5` in each
body and cannot be tinted.

Four rules that are easy to get wrong (all pinned in
`frontend/tests/account-health.test.mts`):

- **"Not measured" is not "fine".** A missing entry draws nothing; otherwise every
  account would flash a warning during the first seconds after launch.
- **A watcher that never connected says nothing.** `idleStartDelay` means no watcher
  is live at launch, so only a watcher that *was* live and then dropped counts. If an
  account truly cannot connect, the STATUS sweep reports it — that signal needs no
  warm-up.
- **Absent ≠ false.** `IdleStates()` omits accounts with auto-sync off and servers
  that refused IDLE (`unsupported`): both fall back to interval polling, and reporting
  them would leave a permanent warning on a working account.
- **`offline` outranks a live IDLE session.** A server can keep an existing session
  while refusing new connections (the concurrency limit on Gmail/163). Opening a
  folder is what the user actually does, so the failed dial wins until a sweep
  succeeds.

Clicking the icon retries: `offline` verifies from scratch (bypassing the 30 s badge
cache), `degraded` reuses a sweep from the last 5 s — enough to swallow a double
click, short enough that the answer is still about now. While the meter reads `Off`
nothing is dialled at all: that connection cannot succeed, and `dialIMAP`'s timeout
is 60 s, which the user would spend watching a spinner. The icon is a `<span>`, not
a button: it sits inside `BranchControl` (itself `role="button"`), so a focusable
control there would be a keyboard trap; the same retry is reachable from Settings →
the account's ⋮ menu.

## Files

| Path | Role |
|---|---|
| `services/networkservice.go` | probe + retry + error truncation; also exposes `AccountIdleStates`; registered in `main.go` |
| `services/emailidle.go` | `IdleStates()` — which watchers exist and which are live |
| `frontend/src/stores/connection.ts` | polling schedule, target account, state |
| `frontend/src/stores/accountHealth.ts` | the two per-account signals + the 30 s IDLE poll |
| `frontend/src/composables/connection.ts` | `classifyConnection`, `connectionBars`, `formatLatency` |
| `frontend/src/composables/account-health.ts` | per-account thresholds and the combination rule |
| `frontend/src/components/icon/ConnectionStatus.vue` | the meter (CSS bars, not an icon glyph) + Ark tooltip |
| `frontend/src/components/sidebar/SideBar.vue` | the marker + its retry |
| `frontend/tests/connection.test.mts` | threshold boundaries, failure paths, formatting |
| `frontend/tests/account-health.test.mts` | "not measured" silence, failure counts, combination |
| `services/networkservice_test.go` | local fake/silent/HTTP servers, timeout bound, truncation |
| `services/emailidle_test.go` | `IdleStates` (live / reconnecting / refused), stop-on-delete |

## The icon is drawn, not a font glyph

No preset icon carries a per-bar severity fill (3 green vs. 2 amber vs. 1 red) that
also lines up with a cross for `Off`, so `ConnectionStatus.vue` draws three 4 px bars
stepping 6/10/14 px inside a 16 px box, with unfilled bars left as a
`color-mix(...)` track so the shape never jumps between states. Colours come from the
theme tokens (`--color-success` / `--color-warning` / `--color-danger`), never
hardcoded.
