//go:build !darwin

package macos

// SetWindowCornerRadiusPreference is a no-op on platforms other than macOS.
func SetWindowCornerRadiusPreference(_ float64) {}
