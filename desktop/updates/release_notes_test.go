package updates

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestReleaseNotesComeFromCheckedReleaseWithoutExtraRequests(t *testing.T) {
	m, _, _ := fixture(t, "v2.0.0")
	original := m.client.Transport
	requests := 0
	body := "## Changes\n\n- Export profiles"
	m.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.String() != m.apiURL {
			t.Fatal("notes made an extra network request")
		}
		response, err := original.RoundTrip(req)
		if err != nil {
			return nil, err
		}
		var latest release
		if err := json.NewDecoder(response.Body).Decode(&latest); err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		latest.Body = body
		data, _ := json.Marshal(latest)
		response.Body = io.NopCloser(strings.NewReader(string(data)))
		return response, nil
	})
	if err := m.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if m.State().ReleaseNotes != body || requests != 1 {
		t.Fatal("notes missing from checked release")
	}
	body = ""
	if err := m.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if m.State().ReleaseNotes != "" || requests != 2 {
		t.Fatal("empty description retained stale notes")
	}
}

func TestReleaseNotesAreBoundedWithoutBreakingUnicode(t *testing.T) {
	if got := boundedReleaseNotes("  ## Изменения\n\n- Профили  "); got != "## Изменения\n\n- Профили" {
		t.Fatal(got)
	}
	long := boundedReleaseNotes(strings.Repeat("Профили 🟢", 10000))
	if len(long) > (64<<10)+4 || !utf8.ValidString(long) || !strings.HasSuffix(long, "\n…") {
		t.Fatal("unbounded or broken Unicode notes")
	}
}
