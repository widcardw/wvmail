package main

import (
	"embed"
	_ "embed"
	"log"
	"time"

	"wvmail/macos"
	"wvmail/services"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// Register a custom event whose associated data type is string.
	// This is not required, but the binding generator will pick up registered events
	// and provide a strongly typed JS/TS API for them.
	application.RegisterEvent[string]("time")
	application.RegisterEvent[string]("emails-synced")
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
			ApplicationShouldTerminateAfterLastWindowClosed: true,
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
	})

	// macOS 26 (Tahoe) drops the traffic lights below the 40px in-page title
	// bar to clear its larger window corner radius. The header (.app__titlebar)
	// is 40px tall, so keep the lights centered at 20pt from the top instead.
	macos.CenterTrafficLights(win, 20)

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
