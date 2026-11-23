package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/samosvalishe/free-turn-proxy/desktop/backend"
	"github.com/samosvalishe/free-turn-proxy/desktop/locale"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) ExportProfile(id string) error {
	data, err := a.store.ExportProfile(id)
	if err != nil {
		return err
	}
	profile, _ := a.store.Get(id)
	filename := profileExportFilename(profile.Name)
	options, affirmative := profileExportConfirmation(runtime.GOOS)
	return exportProfileFile(data, affirmative,
		func() (string, error) { return wailsruntime.MessageDialog(a.ctx, options) },
		func() (string, error) {
			return wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{Title: locale.Text("Экспорт профиля", "Export profile"), DefaultFilename: filename, CanCreateDirectories: true, Filters: []wailsruntime.FileFilter{{DisplayName: "FreeTurn JSON", Pattern: "*.json"}}})
		},
		writePrivateFile)
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

// UpdateGuestShareVKLink edits the outgoing link without changing the owner's
// saved profile or re-issuing the recipient's WireGuard credentials.
func (a *App) UpdateGuestShareVKLink(raw, vkLink string) (*backend.GuestShare, error) {
	return backend.UpdateGuestShareVKLink(raw, vkLink)
}
