package updates

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestHistoryFiltersSortsBoundsAndCachesPublicReleases(t *testing.T) {
	m := New("v3.0.7", "amd64", "example/fork")
	calls := 0
	m.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.String() != "https://api.github.com/repos/example/fork/releases?per_page=30" {
			t.Fatal("wrong history source", req.URL)
		}
		data, _ := json.Marshal([]release{
			{Tag: "v3.0.3", Body: "Old"},
			{Tag: "v3.0.8", Body: "Private draft", Draft: true},
			{Tag: "v3.1.0", Body: "Preview", Prerelease: true},
			{Tag: "nightly", Body: "Invalid version"},
			{Tag: "v3.0.7", Body: strings.Repeat("Профили", 20000), PublishedAt: "2026-11-22T12:00:00Z"},
			{Tag: "v3.0.6", Body: "Previous"},
			{Tag: "v3.0.7", Body: "Duplicate"},
		})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data))), Header: make(http.Header)}, nil
	})
	entries, err := m.History(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 || entries[0].Version != "v3.0.7" || entries[1].Version != "v3.0.6" || entries[2].Version != "v3.0.3" {
		t.Fatal("invalid history", entries)
	}
	if len(entries[0].Notes) > (64<<10)+4 || entries[0].PublishedAt != "2026-11-22T12:00:00Z" {
		t.Fatal("history notes unbounded or date missing")
	}
	entries[0].Notes = "mutated"
	cached, err := m.History(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || cached[0].Notes == "mutated" {
		t.Fatal("history cache missing or aliased")
	}
	if state := m.State(); state.Available || state.Busy != "" || state.LatestVersion != "" {
		t.Fatal("reading history changed update availability")
	}
	m.mu.Lock()
	m.state.LatestVersion = "v3.0.8"
	m.mu.Unlock()
	if _, err := m.History(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("new latest version did not invalidate history cache")
	}
}

func TestHistoryHandlesMissingRepositoryCancelledAndInvalidResponses(t *testing.T) {
	if _, err := New("dev", "amd64", "").History(context.Background()); err == nil {
		t.Fatal("missing source accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New("dev", "amd64", "example/fork").History(ctx); err == nil {
		t.Fatal("cancelled history fetched")
	}
	for _, body := range []string{"broken", "null", `{"message":"not an array"}`} {
		m := New("dev", "amd64", "example/fork")
		m.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})
		if _, err := m.History(context.Background()); err == nil {
			t.Fatal("invalid response accepted")
		}
	}
}
