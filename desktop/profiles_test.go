package main

import (
	"errors"
	"testing"

	"github.com/samosvalishe/free-turn-proxy/desktop/locale"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestWindowsProfileConfirmationOpensSaveChooser(t *testing.T) {
	defer locale.Set("en")
	for _, language := range []string{"ru", "en"} {
		locale.Set(language)
		options, affirmative := profileExportConfirmation("windows")
		if options.Type != wailsruntime.QuestionDialog || options.DefaultButton != "No" || options.CancelButton != "No" || affirmative != "Yes" {
			t.Fatal("Windows confirmation does not match Wails MessageBox ABI")
		}
		chosen, written := false, false
		err := exportProfileFile([]byte("synthetic-profile"), affirmative,
			func() (string, error) { return "Yes", nil },
			func() (string, error) { chosen = true; return "chosen-profile.json", nil },
			func(path string, data []byte) error {
				written = true
				if path != "chosen-profile.json" || string(data) != "synthetic-profile" {
					t.Fatal("selection ignored")
				}
				return nil
			})
		if err != nil || !chosen || !written {
			t.Fatalf("affirmative Windows response did not reach save chooser: %v", err)
		}
	}
}

func TestProfileExportNeverWritesOnCancellationOrDialogFailure(t *testing.T) {
	failure := errors.New("dialog unavailable")
	for _, test := range []struct {
		name, answer, path    string
		confirmErr, chooseErr error
		wantChooser           bool
	}{
		{name: "declined", answer: "No"},
		{name: "old OK-only response", answer: "Ok"},
		{name: "confirmation error", confirmErr: failure},
		{name: "save cancelled", answer: "Yes", wantChooser: true},
		{name: "save error", answer: "Yes", chooseErr: failure, wantChooser: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			chosen := false
			err := exportProfileFile(nil, "Yes", func() (string, error) { return test.answer, test.confirmErr }, func() (string, error) { chosen = true; return test.path, test.chooseErr }, func(string, []byte) error { t.Fatal("export wrote without a selected destination"); return nil })
			if chosen != test.wantChooser {
				t.Fatal("unexpected save chooser invocation")
			}
			if (test.confirmErr != nil || test.chooseErr != nil) && !errors.Is(err, failure) {
				t.Fatal("dialog error swallowed")
			}
		})
	}
}

func TestNonWindowsExportUsesLocalizedConfirmation(t *testing.T) {
	defer locale.Set("en")
	locale.Set("ru")
	for _, platform := range []string{"linux", "darwin"} {
		options, affirmative := profileExportConfirmation(platform)
		if affirmative != "Экспортировать" || options.CancelButton != "Отмена" {
			t.Fatal("localized dialog labels lost")
		}
	}
}
