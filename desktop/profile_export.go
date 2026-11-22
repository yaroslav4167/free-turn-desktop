package main

import (
	"github.com/samosvalishe/free-turn-proxy/desktop/locale"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func profileExportConfirmation(platform string) (wailsruntime.MessageDialogOptions, string) {
	yes, no := locale.Text("Экспортировать", "Export"), locale.Text("Отмена", "Cancel")
	// Wails Windows uses MessageBox: WarningDialog is OK-only and ignores
	// Buttons. QuestionDialog returns the stable ABI tokens "Yes" and "No",
	// independently of the OS display language. Do not compare them to labels.
	if platform == "windows" {
		yes, no = "Yes", "No"
	}
	return wailsruntime.MessageDialogOptions{
		Type:    wailsruntime.QuestionDialog,
		Title:   locale.Text("Экспорт профиля", "Export profile"),
		Message: locale.Text("Файл содержит ключи подключения VPN. Не публикуйте его. SSH-пароли и пути к SSH-ключам не экспортируются.", "The file contains VPN connection keys. Do not publish it. SSH credentials and key paths are excluded."),
		Buttons: []string{yes, no}, DefaultButton: no, CancelButton: no,
	}, yes
}

// Keep the entire decision chain testable: confirmation must lead to a save
// chooser, and nothing is written until the user selects a destination.
func exportProfileFile(data []byte, affirmative string, confirm func() (string, error), choose func() (string, error), write func(string, []byte) error) error {
	answer, err := confirm()
	if err != nil || answer != affirmative {
		return err
	}
	path, err := choose()
	if err != nil || path == "" {
		return err
	}
	return write(path, data)
}
