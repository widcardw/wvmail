package main

import (
	"embed"
	_ "embed"
	"log"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"wvmail/macos"
	"wvmail/services"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

// Tray icons: the plain envelope while every inbox is read, the wax-sealed one
// while any account still has unseen mail.
//
//go:embed assets/tray-empty.png
var trayIconEmpty []byte

//go:embed assets/wvmail-tray.png
var trayIconUnread []byte

func init() {
	// Register a custom event whose associated data type is string.
	// This is not required, but the binding generator will pick up registered events
	// and provide a strongly typed JS/TS API for them.
	application.RegisterEvent[string]("time")
	application.RegisterEvent[string]("emails-synced")
	// Emitted by ConfigService when the UI language changes, so the system tray can
	// rebuild its labels without a restart.
	application.RegisterEvent[string]("language-changed")
}

// main function serves as the application's entry point. It initializes the application, creates a window,
// and starts a goroutine that emits a time-based event every second. It subsequently runs the application and
// logs any error that might occur.
func main() {

	// macOS 26 "Tahoe" rounds window corners far more aggressively than earlier
	// releases (26pt vs ~10pt). Ask AppKit for the macOS 15-era radius instead.
	// This must happen before application.New because AppKit reads the
	// preference once while starting up. It is a no-op on other platforms and on
	// Sequoia (Darwin 24) or older.
	macos.SetWindowCornerRadiusPreference(10)

	// Create shared service instances. We keep references so the background
	// sync service can use ConfigService and EmailService.
	cfgSvc := &services.ConfigService{}

	// Initialize file logging from config
	appCfg, cfgErr := cfgSvc.GetConfig()
	if cfgErr == nil {
		if err := services.InitLogger(&appCfg.LogConfig); err != nil {
			log.Printf("failed to init logger: %v", err)
		}
	}

	emailSvc := services.NewEmailService()
	syncSvc := services.NewEmailSyncService(cfgSvc, emailSvc)

	app := application.New(application.Options{
		Name:        "wvmail",
		Description: "A demo of using raw HTML & CSS",
		Services: []application.Service{
			application.NewService(&services.OsService{}),
			application.NewService(&services.GreetService{}),
			application.NewService(cfgSvc),
			application.NewService(emailSvc),
			application.NewService(services.NewNotesService()),
			application.NewService(&services.TranslationService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			// The app's lifetime must not depend on its window: closing the window
			// is decided by the hook below, never by AppKit. With this off, no close
			// path can terminate the process, which is what makes "hide to tray"
			// hold even if a close slips past the hook (the window is then gone but
			// the tray keeps the app alive, and Show() re-runs the window).
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		Windows: application.WindowsOptions{
			// Same on Windows: the hook owns the decision.
			DisableQuitOnLastWindowClosed: true,
		},
	})

	// Create a new window with the necessary options.
	// 'Title' is the title of the window.
	// 'Mac' options tailor the window when running on macOS.
	// 'BackgroundColour' is the background colour of the window.
	// 'URL' is the URL that will be loaded into the webview.
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "WVMail",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropNormal,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(27, 38, 54),
		URL:              "/",
		Width:            1280,
		Height:           768,
		MinWidth:         800,
		MinHeight:        600,
	})

	// macOS 26 (Tahoe) drops the traffic lights below the 40px in-page title
	// bar to clear its larger window corner radius. The header (.app__titlebar)
	// is 40px tall, so keep the lights centered at 20pt from the top instead.
	macos.CenterTrafficLights(win, 20)

	// Show the window again from the tray: a hidden window is not minimised, so
	// restoring has to cover both states.
	showMainWindow := func() {
		if win.IsMinimised() {
			win.UnMinimise()
		}
		win.Show()
		win.Focus()
	}
	hideMainWindow := func() {
		win.Hide()
	}

	// System Tray. Its menu is the only way back once the window is hidden, so it
	// is built before the close hook below can hide anything.
	tray := app.SystemTray.New()
	tray.SetIcon(trayIconEmpty)
	tray.SetTooltip("wvmail")
	// Left click restores the window (right click keeps opening the menu via Wails'
	// smart defaults), so a hidden-to-tray app is one click away.
	tray.OnClick(func() {
		showMainWindow()
	})

	// The badge follows unread mail. EmailSyncService owns the count (an IMAP
	// STATUS UNSEEN per account, refreshed with the background sync), so the icon
	// stays correct while the window is hidden and nothing in the UI has to
	// report anything.
	syncSvc.SetUnreadListener(func(hasUnread bool) {
		if hasUnread {
			tray.SetIcon(trayIconUnread)
			return
		}
		tray.SetIcon(trayIconEmpty)
	})

	// The tray exists before the webview, so it cannot use the frontend's i18n: its
	// labels come from the persisted language and are rebuilt whenever the UI saves
	// a new one (ConfigService emits "language-changed").
	buildTrayMenu := func(language string) *application.Menu {
		labels := trayStrings(language)
		menu := app.NewMenu()
		menu.Add(labels.show).OnClick(func(ctx *application.Context) {
			showMainWindow()
		})
		menu.Add(labels.hide).OnClick(func(ctx *application.Context) {
			hideMainWindow()
		})
		menu.AddSeparator()
		// Same event the Cmd+, key binding emits; the frontend router listens for it.
		menu.Add(labels.settings).OnClick(func(ctx *application.Context) {
			showMainWindow()
			win.EmitEvent("navigate:settings", nil)
		})
		menu.AddSeparator()
		menu.Add(labels.quit).OnClick(func(ctx *application.Context) {
			app.Quit()
		})
		return menu
	}
	tray.SetMenu(buildTrayMenu(appCfg.Language))
	app.Event.On("language-changed", func(event *application.CustomEvent) {
		language, ok := event.Data.(string)
		if !ok || language == "" {
			return
		}
		tray.SetMenu(buildTrayMenu(language))
	})

	// "Hide to tray on close" (Settings → General) is decided by this hook:
	// flag on → hide the window and stay in the tray; flag off → quit the app.
	// AppKit never terminates on its own (ApplicationShouldTerminateAfterLastWindowClosed
	// is off), so this is the only place a window close is interpreted.
	//
	// The decision must happen in a *hook*: hooks run before the built-in
	// WindowClosing listener, which is what destroys the window and removes it from
	// the manager. Cancelling the event skips that listener, and hiding the window
	// leaves the session (sync, tray, webview) alive. The flag is read on every
	// close, so flipping the switch in Settings applies to the next close.
	//
	// quitting guards the teardown started by app.Quit: window closes arriving while
	// the app is going away must not re-enter the branch below.
	var quitting atomic.Bool
	closeRequest := func(event *application.WindowEvent) {
		if cfgSvc.MinimizeWhenClose() {
			// Flag on: cancel the close (which skips Wails' built-in WindowClosing
			// listener that destroys the window) and hide it instead, so the
			// session — background sync, tray, webview — survives.
			event.Cancel()
			hideMainWindow()
			return
		}
		// Flag off: "closing the window quits the app". The close itself is left to
		// Wails, and the terminate request is raised off this event's call stack:
		// AppKit ignores a terminate while it is still handling the window's close,
		// which would leave the app running with its window gone. Both the platform
		// and the common event can arrive for one user close, hence the CAS.
		if quitting.CompareAndSwap(false, true) {
			go func() {
				app.Quit()
				// app.Quit blocks until AppKit's termination sequence finishes, so
				// reaching this line means the request was ignored (AppKit drops a
				// terminate raised while a window close is still unwinding). Never
				// leave the user with a windowless process that ignores "closing
				// quits": exit for real a moment later. Config and cache are written
				// per operation, so nothing is lost by skipping the last teardown steps.
				time.Sleep(5 * time.Second)
				os.Exit(0)
			}()
		}
	}
	// Common.WindowClosing covers programmatic Close() and the other platforms.
	win.RegisterHook(events.Common.WindowClosing, closeRequest)
	if runtime.GOOS == "darwin" {
		// A user close on macOS (red traffic light, Cmd-W) is delivered as
		// Mac.WindowShouldClose and only reaches Common.WindowClosing through
		// Wails' platform→common mapping, so hook the platform event too.
		win.RegisterHook(events.Mac.WindowShouldClose, closeRequest)
	}

	// Start the background email sync service
	syncSvc.Start()
	defer syncSvc.Stop()

	// Register keyboard shortcuts
	// Cmd+, (macOS) / Ctrl+, (other) — navigate to settings
	app.KeyBinding.Add("Cmd+,", func(window application.Window) {
		window.EmitEvent("navigate:settings", nil)
	})
	app.KeyBinding.Add("Ctrl+,", func(window application.Window) {
		window.EmitEvent("navigate:settings", nil)
	})

	// Create a goroutine that emits an event containing the current time every second.
	// The frontend can listen to this event and update the UI accordingly.
	go func() {
		for {
			now := time.Now().Format(time.RFC1123)
			app.Event.Emit("time", now)
			time.Sleep(time.Second)
		}
	}()

	// Run the application. This blocks until the application has been exited.
	err := app.Run()

	// If an error occurred while running the application, log it and exit.
	if err != nil {
		log.Fatal(err)
	}
}

// trayMenuStrings holds the few labels the tray menu needs. The frontend owns the
// rest of the UI language, but the tray is built before the webview exists, so it
// follows the persisted setting (a language change shows up on the next launch).
type trayMenuStrings struct {
	show     string
	hide     string
	settings string
	quit     string
}

func trayStrings(language string) trayMenuStrings {
	if language == "zh-CN" {
		return trayMenuStrings{
			show:     "显示主窗口",
			hide:     "隐藏主窗口",
			settings: "设置…",
			quit:     "退出 wvmail",
		}
	}
	return trayMenuStrings{
		show:     "Show main window",
		hide:     "Hide main window",
		settings: "Settings…",
		quit:     "Quit wvmail",
	}
}
