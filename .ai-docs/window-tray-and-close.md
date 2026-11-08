# Window Close Behaviour & System Tray

How "closing the window hides the app to the tray" is implemented, why it needs a
Wails *hook* rather than a listener, and how the app quits (tray menu, Settings →
退出应用, or a plain close). The behavior was checked against Wails3
`v3.0.0-beta.25`; the project now uses Wails `v3.1.0`.

## The setting

Settings → 通用 / General → **关闭窗口时动作** (`minimize_when_close`):

| value | behaviour when the main window is closed |
|---|---|
| `false` (default) | the window closes and the app quits |
| `true` | the window is hidden and the app keeps running in the system tray |

`AppConfig.MinimizeWhenClose` (`services/configservice.go`) stores it; the UI lives
in `views/settings/Common.vue` and goes through
`ConfigService.SaveConfigField("minimize_when_close", "true"|"false")`.

## Close interception (main.go)

```go
win.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
    if !cfgSvc.MinimizeWhenClose() { // read per close: Settings applies immediately
        return
    }
    event.Cancel() // stops the built-in listener that destroys the window
    win.Hide()
})
```

Three details make this work:

1. **Hook, not listener.** `WebviewWindow.HandleWindowEvent` runs hooks first and
   returns early if the event was cancelled; only then do listeners run — and
   Wails' built-in `WindowClosing` *listener* (registered in the window
   constructor) is what marks the window destroyed, calls the native close and
   removes it from the window manager. Cancelling in a hook skips all of it.
   The hook is registered on **both** `events.Common.WindowClosing` (1030, what
   `Close()` emits and what the other platforms use) and, on darwin,
   `events.Mac.WindowShouldClose` (1169, a user close): Wails maps the platform
   event onto the common one internally, and hooking the platform event directly
   means the decision does not depend on that mapping — this is what made the red
   traffic light reliable.
2. **macOS never closes the window on its own.** `windowShouldClose`
   (`webview_window_darwin.m`) returns `true` only when Wails' `unconditionallyClose`
   flag is set — which that built-in listener does. For a user-initiated close it
   emits `WindowClosing` and returns `false`. So intercepting the event is the only
   way to keep the window; hiding it afterwards is what the user sees.
3. **`ApplicationShouldTerminateAfterLastWindowClosed` is `false`** (and Windows
   mirrors it with `DisableQuitOnLastWindowClosed`): the app's lifetime must not
   depend on its window existing, so no close path can terminate the process on its
   own. With the flag off the hook asks `requestQuit()` for the quit (see below). If
   a close ever slips past the hook the window is gone but the app stays in the tray,
   and `Show()` re-runs the window — never a dead app.

## Quitting: one path for three callers (main.go `requestQuit`)

```go
var quitOnce atomic.Bool

func requestQuit() {
    if !quitOnce.CompareAndSwap(false, true) { return }
    go func() {                       // off the caller's stack: AppKit ignores a
        application.Get().Quit()      // terminate raised while a close unwinds
        time.Sleep(5 * time.Second)   // still alive => the request was dropped
        os.Exit(0)
    }()
}
```

| Caller | How it gets there |
|---|---|
| tray menu → 退出 wvmail | `menu.Add(labels.quit).OnClick(...)` → `requestQuit()` |
| Settings → 通用 → 退出应用 | button → `ConfirmDialog` → `OsService.QuitApp()` → the injected `requestQuit` |
| close with 隐藏到系统托盘 **off** | `closeRequest` hook → `requestQuit()` |

Two details are load-bearing:

1. **The flag is set before the request.** `app.Quit()` tears the windows down and
   that close comes back through the hook above; unguarded, the hook would read it
   as a *user* close and — with "hide to tray" on — cancel it and hide the window,
   i.e. Quit would silently do nothing but hide. So the hook returns early whenever
   `quitOnce` is set, and `closeRequest` never re-enters the hide branch during a
   teardown. The same flag keeps the two close events (platform + common) from
   starting the teardown twice.
2. **The 5 s `os.Exit(0)` fallback.** `app.Quit()` blocks until AppKit's termination
   sequence finishes; reaching the next line means the request was dropped, and a
   windowless process that ignores "quit" is worse than a slightly abrupt exit.
   Config and cache are written per operation, so nothing is lost.

`OsService` does **not** call `application.Get().Quit()` itself: `NewOsService(quit)`
takes the guarded path as a parameter (`main.go` passes `requestQuit`), so the button
cannot bypass the flag. With no hook injected, `QuitApp()` returns an error instead
of silently doing nothing (`services/osservice_test.go`).

`Cmd+Q` and the macOS app menu never come through here — they call the native
teardown, which the hook cannot block (it can only cancel a *window close*), so a
real quit is always a real quit.

## Settings → 退出应用 (the confirm dialog)

`frontend/src/components/ui/ConfirmDialog.vue` is the shared Ark `Dialog` shell with
`role="alertdialog"` (a destructive confirm, so the destructive button sits on the
right), and `Common.vue` keeps only the `showQuitDialog` ref and the button:

```
button (btn btn--danger) → showQuitDialog = true → dialog → QuitApp()
```

The button latches while the request is in flight; the dialog deliberately stays up
because the process ends inside the call, so there is nothing to return to. If the
binding rejects (unwired service), the latch resets and a toast (`quitApp.failed`)
explains why nothing happened.

## Tray

```go
tray := app.SystemTray.New()
tray.SetIcon(trayIcon)      // assets/tray-empty.png (colour, 64×64)
tray.SetTooltip("wvmail")
menu: 显示主窗口 / 隐藏主窗口 / 设置… / 退出 wvmail
```

- **Hidden ≠ minimised.** `win.Hide()` leaves `IsMinimised() == false`, so "show
  main window" must do `UnMinimise()` *if* minimised, then `Show()` + `Focus()`.
  The first version of this menu only called `Show()` when `IsMinimised()` was
  true, which made the window impossible to restore after a hide-to-tray close.
- Left-clicking the tray icon shows/focuses the window (`tray.OnClick`); the menu
  stays on right click. `AttachWindow()` is deliberately *not* used: it positions
  the window next to the tray icon, which is wrong for a main window.
- Clicking the Dock icon while the window is hidden shows it again — Wails
  registers `ApplicationShouldHandleReopen` itself and shows every hidden window.
- Menu labels follow the persisted `language`. They are built once at startup and
  **rebuilt at runtime** when the UI saves a new language: `SaveConfigField("language", …)`
  emits the `language-changed` app event and `main.go` calls `tray.SetMenu(buildTrayMenu(lang))`
  again. The frontend owns the rest of the UI language, but the tray exists before
  the webview does, so it cannot read the frontend's i18n.
- `assets/wvmail-tray.png` is the **unread badge**: the tray shows it whenever any
  inbox still has unseen mail, and `assets/tray-empty.png` otherwise. See below.

## Unread badge

The icon is decided in Go, not by the UI, because it has to stay correct while the
window is hidden:

```
EmailService.RefreshEmails(INBOX)
  └─ STATUS INBOX (UNSEEN) on the connection that is already open
       └─ SetUnseenListener(accountID, unseen)        services/emailservice.go
            └─ unreadTracker.observe → aggregate      services/emailsyncservice.go
                 └─ publishUnread (only on change)
                      └─ tray.SetIcon(unread|empty)    main.go
```

- **Source of truth: `STATUS … (UNSEEN)` on INBOX**, i.e. the server's view. The
  count is taken on the *same* connection as the page fetch (one extra command),
  because the providers throttle connections and a second session just for STATUS
  would double the cost.
- **Startup seed.** `EmailSyncService.Start` calls `checkUnreadOnce`, which asks
  each account once (via `UnseenCount`) even when `sync_interval: 0` disables
  background syncing — otherwise a user whose accounts never auto-sync would never
  see the badge. A STATUS carries no message bodies and all three providers in this
  repo answer in well under a second.
- **Failure keeps the last value.** A failed STATUS/ListFolders is logged at warn
  level and leaves the previous count, so a flaky network cannot silently turn the
  badge off. Accounts that disappear from the config are dropped by
  `unreadTracker.retain`, so deleting an account does not leave the icon lit.
- **Only flips cross threads.** `publishUnread` calls `SetIcon` (which hops to the
  UI thread) only when the aggregate changes, and logs
  `tray: unread badge updated has_unread=…` when it does.
- **Reading a message clears the badge.** Opening one calls
  `EmailService.MarkEmailRead` (from `stores/accounts.ts` `markEmailRead`), which
  flips the cached `is_read`, stores `\Seen` on the server (UID `STORE
  +FLAGS.SILENT`, on a read-write `SELECT`) and then reads a fresh `STATUS UNSEEN`
  on that same connection so the icon flips immediately. The list item is updated
  optimistically and reverted if the server refuses; a cache write failure is
  logged and does not fail the call, because the server flag is what other clients
  and the provider's own count follow.

Manual check: `grep 'tray:' ~/Library/Application\ Support/wvmail/logs/app.log`
shows one `inbox unseen count` line per account at launch, plus a
`unread badge updated` line whenever the aggregate flips.

## Manual verification

1. Settings → 通用 → 关闭窗口时动作: pick 隐藏到系统托盘, close the window with the
   red traffic light.
2. The window disappears, the app stays in the tray and background sync keeps
   running (`~/Library/Application Support/wvmail/logs`).
3. Bring it back via the tray menu **显示主窗口**, or by clicking the Dock icon.
4. Switch the setting back to 退出应用, close the window → the app quits on the
   next close (no restart needed, the flag is read per close).
5. Tray → 退出 wvmail quits while the window is hidden.
6. **With 隐藏到系统托盘 selected**, Settings → 通用 → 退出应用 → confirm: the process
   must actually exit (no window, no tray icon left, no `wvmail` in `ps`). This is the
   case the `quitOnce` guard in the hook exists for — without it the teardown's window
   close is re-read as a user close and the app hides instead.

## Cross-platform notes

The event, hook ordering and `Hide()` exist on every platform, so the same code
covers Windows and Linux. On Windows the auto-quit behaviour is
`WindowsOptions.DisableQuitOnLastWindowClosed`; because we cancel the close before
it completes, no window is "last closed" and that flag is not needed. macOS is the
only platform where the native close decision (`windowShouldClose`) has to be
resolved by Wails, which is why the comment in `main.go` mentions it.
