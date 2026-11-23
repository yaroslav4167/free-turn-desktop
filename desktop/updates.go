package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/samosvalishe/free-turn-proxy/desktop/integration"
	"github.com/samosvalishe/free-turn-proxy/desktop/locale"
	"github.com/samosvalishe/free-turn-proxy/desktop/updates"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Set by release builds, independently of the proxy core's version.
var desktopVersion = "dev"
var updateRepository = "yaroslav4167/free-turn-desktop"

func (a *App) startUpdateChecks(ctx context.Context) {
	ctx, a.updateCancel = context.WithCancel(ctx)
	a.updateContext = ctx
	go runAutoUpdateChecks(ctx, 6*time.Hour, func() bool {
		a.desktopMu.Lock()
		enabled := a.preferences.AutoCheckUpdates
		a.desktopMu.Unlock()
		return enabled && a.updater.State().Repository != ""
	}, a.checkForUpdates)
}

func (a *App) UpdateState() updates.State {
	state := a.updater.State()
	state.CanInstallUpdates = integration.CanInstallUpdates
	state.InstallationMessage, state.BackupPath = a.updateResult.Message, a.updateResult.BackupPath
	// Success notices are transient in settings, but failures/rollback must remain visible.
	if state.Error == "" && a.updateResult.Message != "" && !strings.HasPrefix(a.updateResult.Message, "Обновление установлено.") {
		state.Error = a.updateResult.Message
	}
	return state
}
func (a *App) CheckForUpdates() error { return a.checkForUpdates(a.updateContext) }

func (a *App) ReleaseHistory() ([]updates.ReleaseEntry, error) {
	return a.updater.History(a.updateContext)
}

func (a *App) checkForUpdates(ctx context.Context) error {
	err := a.updater.Check(ctx)
	if ctx.Err() == nil {
		wailsruntime.EventsEmit(ctx, "desktop:update", a.UpdateState())
	}
	return err
}

func (a *App) SetAutoCheckUpdates(enabled bool) error {
	a.desktopMu.Lock()
	defer a.desktopMu.Unlock()
	prefs := a.preferences
	prefs.AutoCheckUpdates = enabled
	if err := integration.SavePreferences(prefs); err != nil {
		return err
	}
	a.preferences = prefs
	return nil
}

func (a *App) DownloadUpdate() error {
	state := a.updater.State()
	if !state.Available || state.Busy != "" {
		return errors.New("сначала проверьте доступное обновление")
	}
	filter := wailsruntime.FileFilter{DisplayName: locale.Text("Архив Linux (*.tar.gz)", "Linux archive (*.tar.gz)"), Pattern: "*.tar.gz"}
	if runtime.GOOS == "windows" {
		filter = wailsruntime.FileFilter{DisplayName: "Windows ZIP (*.zip)", Pattern: "*.zip"}
	}
	if runtime.GOOS == "darwin" {
		filter = wailsruntime.FileFilter{DisplayName: "macOS ZIP (*.zip)", Pattern: "*.zip"}
	}
	destination, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title: locale.Text("Сохранить обновление FreeTurn", "Save FreeTurn update"), DefaultFilename: state.AssetName,
		Filters: []wailsruntime.FileFilter{filter},
	})
	if err != nil || destination == "" {
		return err
	}
	return a.updater.Download(a.updateContext, destination, state.LatestVersion)
}

func (a *App) InstallUpdate() error {
	if handled, err := a.installPlatformUpdate(); handled {
		return err
	}
	if !integration.CanInstallUpdates {
		return errors.New("скачайте обновление и замените FreeTurn.app через Finder")
	}
	if os.Geteuid() == 0 {
		return errors.New("обновление клиента нельзя запускать от root")
	}
	if err := integration.CheckUpdatePermissions(); err != nil {
		return err
	}
	if !a.updateInstallMu.TryLock() {
		return errors.New("установка уже выполняется")
	}
	defer a.updateInstallMu.Unlock()
	state := a.updater.State()
	if !state.Available || state.Busy != "" {
		return errors.New("сначала проверьте доступное обновление")
	}
	target, err := integration.InstalledPath()
	if err != nil {
		return err
	}
	message := locale.Text("Скачать и проверить "+state.LatestVersion+", отключить VPN и перезапустить клиент?\nБинарники будут установлены в "+filepath.Dir(target)+".\nПредыдущая пара сохранится для отката. Профили и настройки сохранятся. Системный VPN-helper автоматически не заменяется.", "Download and verify "+state.LatestVersion+", disconnect VPN and restart the client?\nBinaries will be installed in "+filepath.Dir(target)+".\nThe previous pair will be kept for rollback. Profiles and settings are preserved. The system VPN helper is not replaced automatically.")
	if runtime.GOOS == "windows" {
		message = locale.Text("Скачать и проверить "+state.LatestVersion+", отключить VPN и перезапустить клиент?\nПриложение будет установлено в "+filepath.Dir(target)+". Предыдущий EXE сохранится для отката. Профили и настройки сохранятся.", "Download and verify "+state.LatestVersion+", disconnect VPN and restart the client?\nThe application will be installed in "+filepath.Dir(target)+". The previous EXE is kept for rollback. Profiles and settings are preserved.")
	}
	answer, err := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type: wailsruntime.QuestionDialog, Title: locale.Text("Установить обновление FreeTurn?", "Install FreeTurn update?"),
		Message: message,
		// Wails on Linux uses GTK's Yes/No buttons and returns "Yes"/"No"
		// regardless of the displayed locale or custom button labels.
		Buttons: []string{"Yes", "No"}, DefaultButton: "No", CancelButton: "No",
	})
	if err != nil {
		return err
	}
	confirmed, err := updateInstallationConfirmed(answer)
	if err != nil || !confirmed {
		return err
	}
	stage, err := a.updater.PrepareInstall(a.updateContext, filepath.Dir(target))
	if err != nil {
		return err
	}
	var gate *os.File
	err = a.updater.Restart(func() error {
		var startErr error
		gate, startErr = integration.StartUpdateWorker(stage)
		return startErr
	})
	if err != nil {
		_ = os.RemoveAll(stage)
		return err
	}
	a.desktopMu.Lock()
	a.updateInstallGate = gate
	a.desktopMu.Unlock()
	a.Quit()
	return nil
}

func updateInstallationConfirmed(answer string) (bool, error) {
	switch answer {
	case "Yes", "Установить и перезапустить":
		return true, nil
	case "No", "Cancel", "Отмена", "":
		return false, nil
	default:
		return false, fmt.Errorf("неизвестный ответ диалога подтверждения обновления: %q", answer)
	}
}
