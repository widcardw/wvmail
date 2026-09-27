# Window Close Behaviour & System Tray

How "closing the window hides the app to the tray" is implemented, why it needs a
Wails *hook* rather than a listener, and what the tray menu does. Applies to Wails3
`v3.0.0-beta.25`.

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
3. **`ApplicationShouldTerminateAfterLastWindowClosed` stays `true`.** Nothing
   actually closes while the option is on, so it never fires; and when the option
   is off it is exactly the behaviour we want (closing the window quits). Setting
   it to `false` would instead leave a windowless, tray-less app running.

`App.Quit()` (tray → Quit, Cmd+Q, the macOS app menu) does **not** go through
`WindowClosing` — it calls the native app teardown — so the hook can never block a
real quit.

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
- Left-clicking the tray icon opens the menu (`applySmartDefaults` falls back to
  `OpenMenu()` when no window is attached). `AttachWindow()` is deliberately *not*
  used: it positions the window next to the tray icon, which is wrong for a main
  window.
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
- **Known gap:** the app never marks mail as read on the server (there is no IMAP
  `STORE \Seen` anywhere), so reading a message in wvmail does not clear the badge;
  it clears when the mail is read elsewhere, or on the next refresh after the
  server flags change. Adding mark-as-read on open would fix that and is the
  natural follow-up.

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

## Cross-platform notes

The event, hook ordering and `Hide()` exist on every platform, so the same code
covers Windows and Linux. On Windows the auto-quit behaviour is
`WindowsOptions.DisableQuitOnLastWindowClosed`; because we cancel the close before
it completes, no window is "last closed" and that flag is not needed. macOS is the
only platform where the native close decision (`windowShouldClose`) has to be
resolved by Wails, which is why the comment in `main.go` mentions it.
