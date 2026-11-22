package updates

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"time"
)

type ReleaseEntry struct {
	Version     string `json:"version"`
	PublishedAt string `json:"publishedAt"`
	Notes       string `json:"notes"`
}

// History is read-only and independent of downloading/installing an update.
// Bound both the response and individual descriptions; cache repeated opens.
func (m *Manager) History(ctx context.Context) ([]ReleaseEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	m.historyMu.Lock()
	defer m.historyMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m.repository == "" {
		return nil, errors.New("репозиторий обновлений не задан при сборке")
	}
	latest := m.State().LatestVersion
	if time.Now().Before(m.historyExpires) && latest == m.historyLatest {
		return append([]ReleaseEntry{}, m.history...), nil
	}
	body, err := m.get(ctx, strings.TrimSuffix(m.apiURL, "/latest")+"?per_page=30")
	if err != nil {
		return nil, err
	}
	defer body.Close()
	var releases []release
	if err := json.NewDecoder(io.LimitReader(body, 8<<20)).Decode(&releases); err != nil {
		return nil, errors.New("некорректная история релизов GitHub")
	}
	if releases == nil {
		return nil, errors.New("некорректная история релизов GitHub")
	}
	entries := make([]ReleaseEntry, 0, min(30, len(releases)))
	seen := make(map[string]bool)
	for _, r := range releases {
		if _, valid := parseVersion(r.Tag); !valid || r.Draft || r.Prerelease || seen[r.Tag] {
			continue
		}
		seen[r.Tag] = true
		entries = append(entries, ReleaseEntry{r.Tag, r.PublishedAt, boundedReleaseNotes(r.Body)})
		if len(entries) == 30 {
			break
		}
	}
	sort.SliceStable(entries, func(i, j int) bool { return newer(entries[i].Version, entries[j].Version) })
	m.history, m.historyLatest, m.historyExpires = entries, latest, time.Now().Add(10*time.Minute)
	return append([]ReleaseEntry{}, entries...), nil
}
