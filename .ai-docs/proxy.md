---
description: The proxy setting — one global endpoint plus a per-account override, why the socket is dialled by hand, how CONNECT/SOCKS5 are spoken, and what the cached snapshot is for
---

# Proxy

One optional proxy, shared by everything the app dials: IMAP (per account, with
an override), and the outbound HTTP used by the LLM, translation and OAuth. The
motivation is the ordinary one — a mailbox on a server that is blocked where the
user lives, reachable through the local proxy tool they already run
(`127.0.0.1:7897` in the case that prompted this).

## The model

```
config.yaml
├── proxy: { enabled, url }                 # Settings → Proxy, the default route
└── email_accounts[].proxy: { mode, url }   # per-account override
```

`mode` has three values, and the empty string is the important one:

| `mode` | Meaning |
|---|---|
| `""` (absent) | Follow the global proxy. What every account written before this option existed gets. |
| `"custom"` | Use this account's own `url`, whether or not the global proxy is on. |
| `"off"` | Connect directly even when the global proxy is on — for a mailbox on the local network that must not be sent through a remote tunnel. |

Resolution lives in one place, `proxyForAccount` in `services/proxy.go`, and
`composables/proxy.ts` mirrors it for display only (the settings page describes
where each account's mail goes). A bad address is an **error**, never a silent
fallback to a direct connection: a mailbox that quietly bypassed the tunnel would
fail with a timeout and no explanation.

## Address format

`url` accepts `host:port`, or a full URL:

- `127.0.0.1:7897` → treated as **HTTP** (`defaultProxyScheme`). Every proxy tool
  exposes an HTTP port, and Clash/v2ray "mixed" ports answer both HTTP CONNECT and
  SOCKS5, so this is the guess that works most often.
- `socks5://127.0.0.1:1080` → SOCKS5, no auth.
- `http://user:pass@127.0.0.1:7897` → HTTP with `Proxy-Authorization: Basic`.

A missing port is rejected rather than defaulted: guessing 8080 for a tool on 7897
produces "connection refused", which reads as "the proxy is broken" instead of
"the address is incomplete".

Both mechanisms resolve hostnames **at the proxy** (SOCKS5 sends the name as a
domain; CONNECT carries the name in the request line), which is what a user behind
a censoring network needs — local DNS for `imap.gmail.com` does not answer.

## Why the socket is dialled by hand

`imapclient.Options.Dialer` is a concrete `*net.Dialer`, so a proxy dialer cannot
be plugged into it. `dialIMAPWith` therefore opens the TCP connection itself and
then does what `DialTLS` / `DialStartTLS` / `DialInsecure` do internally: wrap in
TLS where the account uses it, then `imapclient.New`. Two details are easy to lose
there:

- **The banner survives the handshake.** An IMAP server sends its greeting as soon
  as the tunnel opens, so a CONNECT response and the first bytes of the banner can
  arrive in the same packet. `httpConnectDialer` reads the response headers with a
  `bufio.Reader` and returns a `bufferedConn` that drains it first — a plain
  `conn.Read` would lose those bytes and the client would wait for a greeting that
  already arrived.
- **The TLS handshake is now bounded by the caller's timeout.** The old
  `tls.DialWithDialer` path bounded only the TCP connect, so a server that accepted
  the socket and stalled could hold the call indefinitely. A deadline is set before
  the handshake and cleared afterwards, where the IMAP client takes over with its
  own per-command deadlines.

## The HTTP side

`newHTTPClient(timeout)` builds a client whose transport dials through the same
resolver, so LLM, translation and OAuth follow the global proxy without knowing it
exists. `Proxy` on the transport is left nil on purpose: `HTTP_PROXY` /
`HTTPS_PROXY` in the environment are ignored, so the settings page is the single
source of truth. A broken setting surfaces as a request error carrying
`proxy: …` — the LLM test dialog shows exactly that instead of a request that
quietly went out another way.

## The cached snapshot

Dial-time code has no `ConfigService` and must not re-read YAML on every folder
open, so `readConfig` publishes `cfg.Proxy` into an atomic pointer
(`globalProxy`) and `writeConfig` publishes again after a successful write. Reads
happen on every config access, which is what keeps the copy from going stale;
publishing after the write is what makes a settings change take effect on the very
next connection.

## The probe follows the route too

The title-bar meter asks `NetworkService.ProbeAccount(accountID)` — the account ID,
not host/port — so Go resolves the same route `dialIMAP` will. A probe that
measured a direct connection while the mail went through a tunnel would read
"Off" for a mailbox that works, and the sidebar's retry refuses to dial while the
meter reads Off, so the mistake would also disable the manual retry.

`NetworkService.TestProxy(url)` is the settings page's button: it opens a tunnel to
`imap.gmail.com:993`, completes TLS and reads the banner. Reaching a Google IMAP
endpoint proves the tunnel carries IMAP + TLS, which a TCP check against any local
port would not.

## Deliberate limits

- **No PAC, no per-request rules, no bypass list.** One endpoint, on or off.
- **SMTP is not wired.** There is no send path yet; when one is added it must use
  `dialerForAccount` too.
- **The proxy is not used for the webview itself.** Remote content in mail is
  fetched by the embedded browser, which follows the OS proxy settings.
- Credentials are supported but stored in plain text inside `config.yaml` (unlike
  the account passwords, which are encrypted). A proxy password is usually a
  throwaway on `127.0.0.1`; encrypting it would need a migration for a field that
  is empty for almost everyone.

## Files

| Path | Role |
|---|---|
| `services/proxy.go` | config shape, address parsing, resolution, CONNECT + SOCKS5 dialers, `newHTTPClient` |
| `services/imapdial.go` | dials through `dialerForAccount`, keeps TLS/STARTTLS/plaintext fallbacks |
| `services/networkservice.go` | `ProbeAccount`, `TestProxy` |
| `services/configstore.go` | `proxy` field save (JSON, validated), publishes the snapshot |
| `services/configservice.go` | `validateAccountProxy` on add/update |
| `frontend/src/views/settings/Proxy.vue` | global switch + address + test, per-account modes |
| `frontend/src/components/settings/AccountForm.vue` | the same three modes inside the account form |
| `frontend/src/composables/proxy.ts` | `proxyModeOf`, `resolveProxyRoute` (display only) |
| `services/proxy_test.go` | parsing, resolution, a fake CONNECT proxy, a fake SOCKS5 proxy, config round trip |
| `frontend/tests/proxy.test.mts` | the display rules |
