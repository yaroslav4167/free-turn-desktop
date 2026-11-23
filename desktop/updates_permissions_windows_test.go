//go:build windows

package main

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsInstallUpdateChecksElevationBeforeDownloadOrShutdown(t *testing.T) {
	if windows.GetCurrentProcessToken().IsElevated() {
		t.Skip("requires a non-elevated process")
	}
	// No updater, runtime context or engine is initialized. The permission check
	// must return before attempting any download, installation or GUI shutdown.
	err := (&App{}).InstallUpdate()
	if !errors.Is(err, windows.ERROR_ELEVATION_REQUIRED) {
		t.Fatalf("missing preflight elevation error: %v", err)
	}
}
