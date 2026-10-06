---
description: The title-bar connection meter — what it measures, why the probe is stdlib rather than dialIMAP, the latency thresholds, the polling schedule, and its deliberate limits
---

# Title-bar connection meter

The title bar shows a three-bar signal meter immediately left of the theme switch.
It answers "is the mailbox reachable right now, and how fast does it answer?" for the
**active account**, and turns that into four states.

```
store/connection.ts (polling + state)          components/icon/ConnectionStatus.vue
  start() ──▶ probe()                            ┌─ 3 CSS bars, filled per state
              └─▶ NetworkService.ProbeConnection │  (green / amber / red / grey)
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

## Deliberate limits

- It measures the **mail server**, not general internet quality — the tooltip names the
  host, so the claim is never more than it looks.
- A failed first attempt is retried once after 250 ms; `Attempts > 1` shows as
  "succeeded on retry" in the tooltip, which is the difference between slow and flaky.
- Nothing here is persisted: the meter is live state only, and it resets on restart.

## Files

| Path | Role |
|---|---|
| `services/networkservice.go` | probe + retry + error truncation; registered in `main.go` |
| `frontend/src/stores/connection.ts` | polling schedule, target account, state |
| `frontend/src/composables/connection.ts` | `classifyConnection`, `connectionBars`, `formatLatency` |
| `frontend/src/components/icon/ConnectionStatus.vue` | the meter (CSS bars, not an icon glyph) + Ark tooltip |
| `frontend/tests/connection.test.mts` | threshold boundaries, failure paths, formatting |
| `services/networkservice_test.go` | local fake/silent/HTTP servers, timeout bound, truncation |

## The icon is drawn, not a font glyph

No preset icon carries a per-bar severity fill (3 green vs. 2 amber vs. 1 red) that
also lines up with a cross for `Off`, so `ConnectionStatus.vue` draws three 4 px bars
stepping 6/10/14 px inside a 16 px box, with unfilled bars left as a
`color-mix(...)` track so the shape never jumps between states. Colours come from the
theme tokens (`--color-success` / `--color-warning` / `--color-danger`), never
hardcoded.
