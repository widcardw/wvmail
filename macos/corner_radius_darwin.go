//go:build darwin

package macos

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

// AppKit scales window corner rounding with the undocumented
// NSConvolutionOverride1 preference: macOS 26 "Tahoe" uses 26 (points), while
// macOS 15 and earlier used ~10. AppKit reads it once while starting up, which
// is why the value has to be registered before the first window is created.
static void registerWindowCornerRadius(double radius) {
	[[NSUserDefaults standardUserDefaults] registerDefaults:@{
		@"NSConvolutionOverride1": @(radius)
	}];
}
*/
import "C"

import (
	"strconv"
	"strings"
	"syscall"
)

// SetWindowCornerRadiusPreference asks AppKit to round this app's window corners
// with the given radius in points instead of macOS 26 "Tahoe"'s 26pt default.
// Pass 10 for the pre-Tahoe (macOS 15) look.
//
// It must be called before AppKit starts up — i.e. before application.New or
// creating any window — because AppKit reads the value once during startup and
// ignores later changes. Verified on macOS 26.7 by measuring the generated
// window corner mask: 26.0pt by default, 10.2pt after registering 10.
//
// The value is registered in the app's own defaults domain only, so other apps
// keep their own corner rounding, and an explicit value in the app or global
// defaults domain still takes precedence over this call.
//
// The preference is undocumented: on a macOS release that stops honouring it
// this becomes a silent no-op, with windows keeping the system default.
func SetWindowCornerRadiusPreference(radius float64) {
	if !windowCornerRadiusConfigurable() {
		return
	}
	C.registerWindowCornerRadius(C.double(radius))
}

// windowCornerRadiusConfigurable reports whether the running system is macOS 26
// (Tahoe, Darwin 25) or newer. The product version can't be used for this check:
// macOS reports it as 16 or 26 depending on the SDK the binary was built with,
// while the Darwin kernel major is always truthful.
func windowCornerRadiusConfigurable() bool {
	release, err := syscall.Sysctl("kern.osrelease")
	if err != nil {
		return false
	}
	major, err := strconv.Atoi(strings.SplitN(release, ".", 2)[0])
	if err != nil {
		return false
	}
	return major >= 25
}
