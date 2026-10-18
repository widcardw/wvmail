package services

import (
	"fmt"
	"os"
	"os/exec"
	sysruntime "runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// OsService exposes OS-level actions to the frontend: opening files, folders and
// URLs, the native save dialog, and quitting the app.
type OsService struct {
	// quit is the app's guarded quit path (main.requestQuit). Injecting it keeps a
	// quit from the UI on exactly the same route as the tray item and a window
	// close: the flag inside it must be set *before* the request, otherwise the
	// window-close hook reads the teardown as a user close and hides the window
	// instead of letting the app go.
	quit func()
}

func NewOsService(quit func()) *OsService {
	return &OsService{quit: quit}
}

// QuitApp quits the process, after the frontend's confirmation dialog.
//
// In practice this call does not come back: the process ends inside the teardown
// that quit starts, so the UI treats it as a one-way action and leaves the dialog
// up while the app goes away. It returns an error only when nothing was wired to
// quit, which would otherwise be a silent no-op for a button labelled "Quit".
func (o *OsService) QuitApp() error {
	if o.quit == nil {
		return fmt.Errorf("quit is not available")
	}
	o.quit()
	return nil
}

func (o *OsService) GetOs() string {
	return sysruntime.GOOS
}

func (o *OsService) OpenFile(path string) error {
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %w", err)
	}

	var cmd *exec.Cmd

	switch o.GetOs() {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("explorer", path)
	case "linux":
		cmd = exec.Command("xdg-open", path)
	default:
		return fmt.Errorf("unsupported OS")
	}
	return cmd.Run()
}

func (o *OsService) OpenFolder(path string) error {
	if path == "" {
		return fmt.Errorf("path is empty")
	}

	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %w", err)
	}

	app := application.Get()
	return app.Env.OpenFileManager(path, true)
}

func (o *OsService) PathStat(path string) string {
	if path == "" {
		return "empty"
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return "file"
	} else if err == nil {
		return "dir"
	} else if os.IsNotExist(err) {
		return "non-exist"
	} else {
		return err.Error()
	}
}

func (o *OsService) OpenURL(url string) error {
	if url == "" {
		return fmt.Errorf("url is empty")
	}
	app := application.Get()
	return app.Browser.OpenURL(url)
}

// SaveFileDialog shows a native save-file dialog with the given suggested
// filename and returns the absolute path chosen by the user, or an error if
// the user cancelled.
func (o *OsService) SaveFileDialog(suggestedFilename string) (string, error) {
	app := application.Get()
	path, err := app.Dialog.SaveFile().
		SetFilename(suggestedFilename).
		AttachToWindow(app.Window.Current()).
		PromptForSingleSelection()
	if err != nil {
		return "", fmt.Errorf("save dialog failed: %w", err)
	}
	if path == "" {
		return "", fmt.Errorf("user cancelled")
	}
	return path, nil
}

// SaveEmailDialog prompts for a destination path for an original RFC 822 email.
func (o *OsService) SaveEmailDialog(title, suggestedFilename string) (string, error) {
	app := application.Get()
	path, err := app.Dialog.SaveFile().
		SetMessage(title).
		SetFilename(originalEmailFilename(suggestedFilename)).
		AddFilter("Email message", "*.eml").
		AttachToWindow(app.Window.Current()).
		PromptForSingleSelection()
	if err != nil {
		return "", fmt.Errorf("save dialog failed: %w", err)
	}
	if path == "" {
		return "", fmt.Errorf("user cancelled")
	}
	return path, nil
}
