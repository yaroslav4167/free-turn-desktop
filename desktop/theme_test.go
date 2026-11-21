package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/samosvalishe/free-turn-proxy/desktop/integration"
)

func TestSetThemePersistsWithoutChangingOtherPreferences(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	app := &App{preferences: integration.Preferences{Language: "ru", CloseToTray: true, AutoConnect: true, AutoConnectServerID: "saved", AutoCheckUpdates: true}}
	for _, theme := range []string{"light", "dark", "system"} {
		if err := app.SetTheme(theme); err != nil {
			t.Fatal(err)
		}
		prefs, err := integration.LoadPreferences()
		if err != nil || prefs.Theme != theme || prefs.Language != "ru" || !prefs.CloseToTray || !prefs.AutoConnect || prefs.AutoConnectServerID != "saved" || !prefs.AutoCheckUpdates {
			t.Fatalf("preferences changed: %+v, %v", prefs, err)
		}
	}
	for _, invalid := range []string{"", "auto", "LIGHT"} {
		if err := app.SetTheme(invalid); err == nil {
			t.Fatalf("invalid theme accepted: %q", invalid)
		}
	}
	if app.preferences.Theme != "system" {
		t.Fatal("invalid theme changed preferences")
	}
}

func TestSetThemeDoesNotChangeMemoryOnSaveFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", path)
	t.Setenv("XDG_CONFIG_HOME", path)
	t.Setenv("APPDATA", path)
	app := &App{preferences: integration.Preferences{Theme: "dark"}}
	if err := app.SetTheme("light"); err == nil {
		t.Fatal("expected persistence failure")
	}
	if app.preferences.Theme != "dark" {
		t.Fatal("failed save changed active preference")
	}
}
