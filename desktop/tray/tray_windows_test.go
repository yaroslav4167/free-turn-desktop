//go:build windows

package tray

import (
	"testing"
	"time"
)

func TestTrayLifecycle(t *testing.T) {
	showCalled := false
	quitCalled := false

	tray, err := Start("FreeTurn Desktop Test", func() {
		showCalled = true
	}, func() {
		quitCalled = true
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if tray.hwnd == 0 {
		t.Fatalf("expected valid hwnd, got 0")
	}

	time.Sleep(100 * time.Millisecond)
	tray.UpdateStatus(Status{State: "connected", Streams: 3, Total: 12, HasCaptcha: true}, 0)
	tray.UpdateStatus(Status{State: "connecting"}, 1)

	tray.Close()

	// Ensure multiple Close calls are safe
	tray.Close()

	_ = showCalled
	_ = quitCalled
}

func TestNativeStatusTrayIcons(t *testing.T) {
	for _, light := range []bool{false, true} {
		for phase := Idle; phase <= Failed; phase++ {
			handle, err := createWindowsStatusIcon(iconKey{phase, light, 0, 20})
			if err != nil || handle == 0 {
				t.Fatalf("status %d: %v", phase, err)
			}
			pDestroyIcon.Call(uintptr(handle))
		}
	}
}

func TestFailedStatusRequestRemainsRetryable(t *testing.T) {
	tip := (Status{State: "connected"}).Tooltip()
	target := &Tray{status: Status{State: "connected"}, requestedTip: tip}
	target.needsRetry.Store(true)
	// A completion for an old request must not acknowledge a new status.
	target.markStatusApplied(Status{State: "connecting"}, 0, tip)
	if !target.needsRetry.Load() {
		t.Fatal("stale completion acknowledged pending request")
	}
	target.markStatusApplied(target.status, 0, tip)
	if target.needsRetry.Load() {
		t.Fatal("successful current status remains pending")
	}
}

func TestDenseNativeTrayIcons(t *testing.T) {
	for _, light := range []bool{false, true} {
		handle, err := loadWindowsTrayIcon(light)
		if err != nil || handle == 0 {
			t.Fatalf("load native tray icon: %v", err)
		}
		pDestroyIcon.Call(uintptr(handle))
	}
}
