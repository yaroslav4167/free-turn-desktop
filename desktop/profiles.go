package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/samosvalishe/free-turn-proxy/desktop/backend"
	"github.com/samosvalishe/free-turn-proxy/desktop/locale"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) ExportProfile(id string) error {
	data, err := a.store.ExportProfile(id)
	if err != nil {
		return err
	}
	answer, err := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.WarningDialog,
		Title:         locale.Text("Экспорт профиля", "Export profile"),
		Message:       locale.Text("Файл содержит ключи подключения VPN. Не публикуйте его. SSH-пароли и пути к SSH-ключам не экспортируются.", "The file contains VPN connection keys. Do not publish it. SSH credentials and key paths are excluded."),
		Buttons:       []string{locale.Text("Экспортировать", "Export"), locale.Text("Отмена", "Cancel")},
		DefaultButton: locale.Text("Отмена", "Cancel"), CancelButton: locale.Text("Отмена", "Cancel"),
	})
	if err != nil || answer != locale.Text("Экспортировать", "Export") {
		return err
	}
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{Title: locale.Text("Экспорт профиля", "Export profile"), DefaultFilename: "FreeTurn-profile.json", Filters: []wailsruntime.FileFilter{{DisplayName: "FreeTurn JSON", Pattern: "*.json"}}})
	if err != nil || path == "" {
		return err
	}
	return writePrivateFile(path, data)
}

func (a *App) ImportProfiles() ([]backend.Server, error) {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: locale.Text("Импорт профилей", "Import profiles"), Filters: []wailsruntime.FileFilter{{DisplayName: "FreeTurn JSON", Pattern: "*.json"}}})
	if err != nil || path == "" {
		return []backend.Server{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, backend.MaxProfileImport+1))
	if err != nil {
		return nil, err
	}
	return a.store.ImportProfiles(data)
}

func (a *App) ProfileRecoveryNotice() string { return a.store.RecoveryNotice() }

func (a *App) RestoreProfilesBackup() error {
	state, _, _, _, preparing, _ := a.engine.TraySnapshot()
	if preparing || (state != "idle" && state != "error" && state != "") {
		return errors.New(locale.Text("Сначала отключите соединение", "Disconnect before restoring profiles"))
	}
	return a.store.RestoreBackup()
}

func writePrivateFile(path string, data []byte) error {
	// Do not truncate an existing export on a failed write.
	f, err := os.CreateTemp(filepath.Dir(path), ".freeturn-export-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
