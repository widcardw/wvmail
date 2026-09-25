//go:build darwin

// Package macos contains small helpers that talk to AppKit directly for
// things Wails does not expose as window options.
package macos

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

// centerTrafficLights raises the three standard window buttons so their
// vertical center sits at desiredCenterY points below the top edge of the
// window. Buttons already at or above that height are left alone, so on
// macOS versions whose default inset is already fine this is a no-op.
static void centerTrafficLights(void *nsWindowPtr, double desiredCenterY) {
	NSWindow *window = (NSWindow *)nsWindowPtr;
	if (window == nil || ![window isKindOfClass:[NSWindow class]]) {
		return;
	}
	// In fullscreen AppKit floats the buttons in its own overlay; leave them.
	if (([window styleMask] & NSWindowStyleMaskFullScreen) != 0) {
		return;
	}

	NSButton *closeButton = [window standardWindowButton:NSWindowCloseButton];
	if (closeButton == nil || closeButton.superview == nil) {
		return;
	}

	// Window coordinates start at the bottom-left, so convert the button's
	// frame and measure its vertical center from the top edge of the window.
	NSRect frameInWindow = [[closeButton superview] convertRect:closeButton.frame toView:nil];
	double currentCenterFromTop = window.frame.size.height - (frameInWindow.origin.y + frameInWindow.size.height / 2.0);
	double delta = currentCenterFromTop - desiredCenterY; // > 0: buttons sit too low
	if (delta < 0.5) {
		return;
	}

	NSButton *buttons[3] = {
		closeButton,
		[window standardWindowButton:NSWindowMiniaturizeButton],
		[window standardWindowButton:NSWindowZoomButton],
	};
	for (int i = 0; i < 3; i++) {
		NSButton *button = buttons[i];
		if (button == nil || button.superview == nil) {
			continue;
		}
		NSRect frame = button.frame;
		// Title bar container views are not flipped: larger y is higher.
		frame.origin.y += button.superview.isFlipped ? -delta : delta;
		[button setFrameOrigin:frame.origin];
	}
}

// dispatchCenterTrafficLightsAt runs centerTrafficLights on the main thread,
// where all AppKit window work must happen.
static void dispatchCenterTrafficLightsAt(void *nsWindowPtr, double desiredCenterY) {
	dispatch_async(dispatch_get_main_queue(), ^{
		centerTrafficLights(nsWindowPtr, desiredCenterY);
	});
}
*/
import "C"

import (
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// CenterTrafficLights keeps the native traffic-light buttons vertically
// centered on centerY points below the top edge of win.
//
// macOS 26 (Tahoe) lowers the standard window buttons to clear its larger
// window corner radius, which makes them hang below the middle of a custom
// in-page title bar (Wails' MacTitleBarHiddenInset gives us no way to tune
// that). This helper measures where the buttons actually are and nudges them
// up when they sit below centerY. AppKit re-lays out the title bar on resize,
// refocus and after leaving fullscreen — resetting any manually set frame —
// so the correction is re-applied on those events.
func CenterTrafficLights(win *application.WebviewWindow, centerY float64) {
	if win == nil {
		return
	}

	apply := func() {
		nsWindow := win.NativeWindow()
		if nsWindow == nil {
			return
		}
		C.dispatchCenterTrafficLightsAt(nsWindow, C.double(centerY))
	}

	// events.Mac.* matches the raw event ids fired by Wails' darwin backend;
	// the Common.* aliases are registered too in case a remapped id is
	// dispatched first. Applying twice is harmless (the second pass is a
	// no-op once the buttons are in place).
	for _, eventType := range []events.WindowEventType{
		events.Mac.WindowDidResize,
		events.Mac.WindowDidBecomeKey,
		events.Mac.WindowDidExitFullScreen,
		events.Common.WindowDidResize,
		events.Common.WindowFocus,
		events.Common.WindowUnFullscreen,
	} {
		win.OnWindowEvent(eventType, func(*application.WindowEvent) { apply() })
	}

	// The title bar receives its final layout shortly after the window is
	// first shown; retry a few times to beat that initial layout.
	for _, delay := range []time.Duration{
		100 * time.Millisecond,
		500 * time.Millisecond,
		1500 * time.Millisecond,
		3000 * time.Millisecond,
	} {
		time.AfterFunc(delay, apply)
	}
}
