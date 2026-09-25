//go:build !darwin

package macos

import "github.com/wailsapp/wails/v3/pkg/application"

// CenterTrafficLights is a no-op on platforms other than macOS.
func CenterTrafficLights(_ *application.WebviewWindow, _ float64) {}
