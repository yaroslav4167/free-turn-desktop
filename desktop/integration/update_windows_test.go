//go:build windows

package integration

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samosvalishe/free-turn-proxy/desktop/locale"
	"golang.org/x/sys/windows"
)

func TestWindowsUpdateRequiresElevationBeforeReplacingFiles(t *testing.T) {
	defer locale.Set("en")
	for _, language := range []struct{ code, hint string }{
		{"ru", "Запуск от имени администратора"},
		{"en", "Run as administrator"},
	} {
		locale.Set(language.code)
		if err := requireWindowsUpdateElevation(true); err != nil {
			t.Fatalf("elevated updater rejected: %v", err)
		}
		err := requireWindowsUpdateElevation(false)
		if !errors.Is(err, windows.ERROR_ELEVATION_REQUIRED) || !strings.Contains(err.Error(), language.hint) {
			t.Fatalf("missing actionable elevation error: %v", err)
		}
	}
	if windows.GetCurrentProcessToken().IsElevated() {
		return // Exercise the real non-elevated path in ordinary local tests.
	}
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	stage := filepath.Join(dir, "Programs", "FreeTurn", ".freeturn-update-unprepared")
	gate, err := StartUpdateWorker(stage)
	if gate != nil || !errors.Is(err, windows.ERROR_ELEVATION_REQUIRED) {
		t.Fatalf("non-elevated worker started: gate=%v err=%v", gate, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Programs")); !os.IsNotExist(err) {
		t.Fatal("permission check modified the installation")
	}
}

func TestWindowsUpdateRollbackExplainsOutcomeWithoutNil(t *testing.T) {
	defer locale.Set("en")
	locale.Set("ru")
	startErr := errors.New("startup failed")
	message := windowsUpdateRollbackMessage(startErr, nil)
	if !strings.Contains(message, "выполнен откат") || !strings.Contains(message, "startup failed") || strings.Contains(message, "<nil>") {
		t.Fatalf("unclear successful rollback: %s", message)
	}
	message = windowsUpdateRollbackMessage(startErr, errors.New("file locked"))
	if !strings.Contains(message, "Ошибка отката") || !strings.Contains(message, "file locked") || strings.Contains(message, "выполнен откат") {
		t.Fatalf("failed rollback reported as successful: %s", message)
	}
}
