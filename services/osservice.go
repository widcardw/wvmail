package services

import (
	"fmt"
	"os"
	"os/exec"
	sysruntime "runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type OsService struct{}

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
