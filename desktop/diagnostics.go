package main

import (
	"github.com/samosvalishe/free-turn-proxy/desktop/backend"
	"github.com/samosvalishe/free-turn-proxy/desktop/integration"
	"github.com/samosvalishe/free-turn-proxy/desktop/locale"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) ExportDiagnostics() error {
	status, events := a.engine.DiagnosticSnapshot()
	data, err := backend.Diagnostics(desktopVersion, integration.VPNHelperAvailable(), a.store.List(), status, events)
	if err != nil {
		return err
	}
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{Title: locale.Text("Сохранить диагностику", "Save diagnostics"), DefaultFilename: "FreeTurn-diagnostics.json", Filters: []wailsruntime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}}})
	if err != nil || path == "" {
		return err
	}
	return writePrivateFile(path, data)
}
