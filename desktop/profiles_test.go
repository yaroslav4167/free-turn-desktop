package main

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/samosvalishe/free-turn-proxy/desktop/locale"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestProfileExportFilenameUsesServerNameSafely(t *testing.T) {
	for _, test := range []struct{ name, want string }{
		{"Германия", "FreeTurn-profile-Германия.json"},
		{"  My server  ", "FreeTurn-profile-My server.json"},
		{"London.json", "FreeTurn-profile-London.json"},
		{"London.JSON", "FreeTurn-profile-London.json"},
		{"EU / Berlin: 1", "FreeTurn-profile-EU - Berlin- 1.json"},
		{"../../server", "FreeTurn-profile-server.json"},
		{"CON", "FreeTurn-profile-CON.json"},
		{"LPT1.notes", "FreeTurn-profile-LPT1.notes.json"},
		{"", "FreeTurn-profile.json"},
		{"...", "FreeTurn-profile.json"},
		{"bad\nname", "FreeTurn-profile-bad-name.json"},
	} {
		if got := profileExportFilename(test.name); got != test.want {
			t.Fatalf("%q: got %q, want %q", test.name, got, test.want)
		}
	}
	long := profileExportFilename(strings.Repeat("Я", 200))
	if len(long) > 255 || !utf8.ValidString(long) || !strings.HasSuffix(long, ".json") {
		t.Fatal("invalid long Unicode filename")
	}
}

func TestSystemProfileConfirmationOpensSaveChooser(t *testing.T) {
	defer locale.Set("en")
	for _, platform := range []string{"windows", "linux"} {
		for _, language := range []string{"ru", "en"} {
			locale.Set(language)
			options, affirmative := profileExportConfirmation(platform)
			if options.Type != wailsruntime.QuestionDialog || options.DefaultButton != "No" || options.CancelButton != "No" || affirmative != "Yes" {
				t.Fatalf("%s/%s: confirmation does not match Wails native response", platform, language)
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
				t.Fatalf("%s/%s: affirmative response did not reach save chooser: %v", platform, language, err)
			}
		}
	}
}

func TestProfileExportNeverWritesOnCancellationOrDialogFailure(t *testing.T) {
	failure := errors.New("dialog unavailable")
	for _, test := range []struct {
		name, answer, path    string
		confirmErr, chooseErr error
		wantChooser           bool
		wantError             bool
	}{
		{name: "declined", answer: "No"},
		{name: "old OK-only response", answer: "Ok", wantError: true},
		{name: "old GTK OK-only response", answer: "OK", wantError: true},
		{name: "GTK dialog closed", answer: ""},
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
			if test.wantError && err == nil {
				t.Fatal("unexpected native response silently cancelled export")
			}
			if (test.confirmErr != nil || test.chooseErr != nil) && !errors.Is(err, failure) {
				t.Fatal("dialog error swallowed")
			}
		})
	}
}

func TestMacExportUsesLocalizedConfirmation(t *testing.T) {
	defer locale.Set("en")
	locale.Set("ru")
	options, affirmative := profileExportConfirmation("darwin")
	if affirmative != "Экспортировать" || options.CancelButton != "Отмена" {
		t.Fatal("localized dialog labels lost")
	}
}
