package main

import (
	"errors"
	"strings"
	"unicode"

	"github.com/samosvalishe/free-turn-proxy/desktop/locale"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Use a portable basename, never a path or an endpoint. Leave room for the
// extension under common filesystem limits, including multibyte Cyrillic names.
func profileExportFilename(name string) string {
	var out strings.Builder
	for _, r := range strings.TrimSpace(name) {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			r = '-'
		}
		if out.Len()+len(string(r)) > 180 {
			break
		}
		out.WriteRune(r)
	}
	base := strings.Trim(out.String(), " .-")
	if strings.HasSuffix(strings.ToLower(base), ".json") {
		base = strings.TrimRight(base[:len(base)-5], " .")
	}
	if base == "" {
		return "FreeTurn-profile.json"
	}
	// The fixed prefix also prevents reserved Windows device basenames.
	return "FreeTurn-profile-" + base + ".json"
}

func profileExportConfirmation(platform string) (wailsruntime.MessageDialogOptions, string) {
	yes, no := locale.Text("Экспортировать", "Export"), locale.Text("Отмена", "Cancel")
	// Both Wails MessageBox (Windows) and GtkMessageDialog (Linux) ignore
	// Buttons. Their QuestionDialog returns "Yes"/"No", independently of the
	// OS display language. Only AppKit returns the custom button labels.
	if platform == "windows" || platform == "linux" {
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
	if err != nil {
		return err
	}
	if answer != affirmative {
		if answer == "" || answer == "No" || answer == "Cancel" || answer == locale.Text("Отмена", "Cancel") {
			return nil
		}
		return errors.New(locale.Text("Не удалось подтвердить экспорт профиля", "Could not confirm profile export"))
	}
	path, err := choose()
	if err != nil || path == "" {
		return err
	}
	return write(path, data)
}
