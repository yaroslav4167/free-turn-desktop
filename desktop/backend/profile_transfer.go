package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/samosvalishe/free-turn-proxy/desktop/backend/routing"
	"github.com/samosvalishe/free-turn-proxy/mobile"
)

const MaxProfileImport = 8 << 20

type profileArchive struct {
	Format   string   `json:"format"`
	Version  int      `json:"version"`
	Profiles []Server `json:"profiles"`
}

func (s *Store) ExportProfile(id string) ([]byte, error) {
	profile, ok := s.Get(id)
	if !ok {
		return nil, errors.New("profile not found")
	}
	// Connection keys are necessary to reconnect. SSH credentials and local
	// key paths are deliberately not portable and never leave this machine.
	profile.SSH = nil
	return json.MarshalIndent(profileArchive{"freeturn-profiles", 1, []Server{profile}}, "", "  ")
}

func (s *Store) ImportProfiles(data []byte) ([]Server, error) {
	if len(data) > MaxProfileImport {
		return nil, errors.New("profile archive exceeds 8 MiB")
	}
	var archive profileArchive
	if err := json.Unmarshal(data, &archive); err != nil {
		return nil, errors.New("invalid profile JSON")
	}
	if archive.Format != "freeturn-profiles" || archive.Version != 1 || len(archive.Profiles) == 0 || len(archive.Profiles) > 100 {
		return nil, errors.New("unsupported profile archive (expected 1–100 profiles)")
	}
	for i := range archive.Profiles {
		p := &archive.Profiles[i]
		p.SSH = nil
		if p.Peer == "" {
			return nil, fmt.Errorf("profile %d: peer is required", i+1)
		}
		normalizeServer(p)
		if p.ConnMode != ConnModeVPN && p.ConnMode != ConnModeRelay {
			return nil, errors.New("invalid connection mode")
		}
		cfg, err := p.ToCoreConfig().JSON()
		if err != nil {
			return nil, fmt.Errorf("profile %d: invalid configuration", i+1)
		}
		if mobile.ValidateConfig(cfg) != "" {
			return nil, fmt.Errorf("profile %d: invalid connection configuration", i+1)
		}
		if p.ConnMode == ConnModeVPN {
			if _, err := mobile.ParseTunnelConfig(p.WgClientConf, DefaultTunnelMTU); err != nil {
				return nil, fmt.Errorf("profile %d: invalid VPN configuration", i+1)
			}
		}
		lists, err := routing.Normalize(p.RoutingLists)
		if err != nil {
			return nil, fmt.Errorf("profile %d: invalid routing lists", i+1)
		}
		p.RoutingLists = lists
		p.ID = generateHex(8)
		p.CreatedAt = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.servers
	s.servers = append(append([]Server(nil), previous...), archive.Profiles...)
	if err := s.persist(); err != nil {
		s.servers = previous
		return nil, err
	}
	s.recoveryNotice = ""
	result := make([]Server, len(archive.Profiles))
	for i, profile := range archive.Profiles {
		result[i] = cloneServer(profile)
	}
	return result, nil
}
