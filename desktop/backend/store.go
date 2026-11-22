package backend

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/samosvalishe/free-turn-proxy/desktop/backend/routing"
	"github.com/samosvalishe/free-turn-proxy/desktop/backend/serversetup"
)

// Server is a saved connection profile - one VK-call relay + VPS peer setup
// the user can pick from the server list and connect to.
type Server struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Peer     string   `json:"peer"`
	Provider string   `json:"provider"`
	VKLinks  []string `json:"vkLinks"`

	Transport string `json:"transport"`
	Mode      string `json:"mode"`

	ObfProfile  string `json:"obfProfile"`
	ObfKey      string `json:"obfKey"`
	ObfTimingMs int    `json:"obfTimingMs"`

	N              int    `json:"n"`
	StreamsPerCred int    `json:"streamsPerCred"`
	ClientID       string `json:"clientId"`
	Listen         string `json:"listen"`

	DNSMode    string   `json:"dnsMode"`
	DNSServers []string `json:"dnsServers"`

	ManualCaptcha bool `json:"manualCaptcha"`
	Debug         bool `json:"debug"`
	Routes        bool `json:"routes"`

	// DTLSFingerprint is the optional SHA-256 fingerprint for DTLS leaf cert pinning.
	DTLSFingerprint string `json:"dtlsFingerprint,omitempty"`

	// TurnHost/TurnPort override the TURN server address from provider creds; empty uses the creds as-is.
	TurnHost string     `json:"turnHost"`
	TurnPort string     `json:"turnPort"`
	KCP      KCPProfile `json:"kcp"`

	// SSH is set only for servers this app provisioned itself (see the "deploy
	// new VPS" wizard); nil for servers added by link or manual entry.
	SSH *serversetup.SSHConfig `json:"ssh,omitempty"`
	// WgClientConf is the WireGuard config wg-setup generated (Endpoint already
	// pointed at Listen) - import it into a separate WireGuard/AmneziaWG client.
	WgClientConf string `json:"wgClientConf,omitempty"`
	// WgPort is the VPS-local port the managed WireGuard backend listens on -
	// used as the free-turn-proxy server's -connect target on restart.
	WgPort int `json:"wgPort,omitempty"`

	// ConnMode picks how Connect wires this server up: ConnModeRelay (default -
	// listens locally, a separate WireGuard/AmneziaWG client points at it) or
	// ConnModeVPN (embedded tunnel - Connect creates the OS network adapter
	// itself from WgClientConf, no separate client needed). VPN mode requires
	// WgClientConf to be set.
	ConnMode     string         `json:"connMode,omitempty"`
	RoutingLists []routing.List `json:"routingLists"`
	// Missing/false means LAN bypass is enabled, including profiles saved before
	// this option existed. True opts into routing local ranges by normal rules.
	RouteLocalNetworks bool `json:"routeLocalNetworks"`

	CreatedAt time.Time `json:"createdAt"`
}

const (
	ConnModeRelay = "relay"
	ConnModeVPN   = "vpn"
)

// DefaultServer returns a new profile pre-filled with the core's own client defaults.
func DefaultServer() Server {
	return Server{
		Provider:       "vk",
		VKLinks:        []string{},
		Transport:      "tcp",
		Mode:           "udp",
		ObfProfile:     "rtpopus3",
		N:              12,
		StreamsPerCred: 12,
		Listen:         "127.0.0.1:9000",
		DNSMode:        "auto",
		DNSServers:     []string{},
		ClientID:       generateHex(16),
		KCP:            DefaultKCPProfile(),
		ConnMode:       ConnModeRelay,
		RoutingLists:   []routing.List{},
	}
}

type Store struct {
	mu             sync.Mutex
	path           string
	servers        []Server
	recoveryNotice string
}

func NewStore() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve config dir: %w", err)
	}
	appDir := filepath.Join(dir, "FreeTurnDesktop")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	s := &Store{path: filepath.Join(appDir, "servers.json")}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// StateDir is where the core keeps its own state between runs (VK persona, TURN cred cache).
func (s *Store) StateDir() string {
	return filepath.Join(filepath.Dir(s.path), "core-state")
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		if _, backupErr := os.Stat(s.path + ".bak"); backupErr == nil {
			return s.recoverProfiles(nil)
		}
		s.servers = []Server{}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read servers file: %w", err)
	}
	var list []Server
	if err := json.Unmarshal(data, &list); err != nil {
		return s.recoverProfiles(data)
	}
	if list == nil {
		return s.recoverProfiles(data)
	}
	for i := range list {
		normalizeServer(&list[i])
	}
	s.servers = list
	return nil
}

// normalizeServer heals profiles saved before DefaultServer() always set
// VKLinks/DNSServers: a nil slice marshals as JSON null, which the frontend
// (fields like `vkLinks.join(...)`) isn't written to expect. It also backfills
// ConnMode for profiles saved before that field existed: a server with a
// generated WireGuard conf (the "deploy new VPS" wizard) defaults to the
// one-click embedded VPN instead of the older relay-only mode.
func normalizeServer(s *Server) {
	if s.RoutingLists == nil {
		s.RoutingLists = []routing.List{}
	}
	links := make([]string, 0, len(s.VKLinks))
	for _, link := range s.VKLinks {
		if link = strings.TrimSpace(link); link != "" {
			links = append(links, link)
		}
	}
	s.VKLinks = links
	if s.DNSServers == nil {
		s.DNSServers = []string{}
	}
	if s.ConnMode == "" {
		if s.WgClientConf != "" {
			s.ConnMode = ConnModeVPN
		} else {
			s.ConnMode = ConnModeRelay
		}
	}
}

// persist writes to a temp file and renames over the target so a crash mid-write
// never leaves servers.json truncated.
func (s *Store) persist() error {
	data, err := json.MarshalIndent(s.servers, "", "  ")
	if err != nil {
		return err
	}
	// Never replace the last good backup with malformed input.
	previous, readErr := os.ReadFile(s.path)
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	var previousProfiles []Server
	if len(previous) > 0 && json.Unmarshal(previous, &previousProfiles) == nil && previousProfiles != nil {
		if err := atomicProfileWrite(s.path+".bak", previous); err != nil {
			return err
		}
	} else if _, backupErr := os.Stat(s.path + ".bak"); os.IsNotExist(backupErr) {
		if err := atomicProfileWrite(s.path+".bak", data); err != nil {
			return err
		}
	}
	return atomicProfileWrite(s.path, data)
}

func (s *Store) List() []Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Server, len(s.servers))
	copy(out, s.servers)
	for i := range out {
		out[i] = cloneServer(out[i])
	}
	return out
}

func (s *Store) Get(id string) (Server, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, srv := range s.servers {
		if srv.ID == id {
			return cloneServer(srv), true
		}
	}
	return Server{}, false
}

// Save inserts or updates a profile by ID; a blank ID is treated as a new profile.
func (s *Store) Save(srv Server) (Server, error) {
	srv = cloneServer(srv)
	lists, err := routing.Normalize(srv.RoutingLists)
	if err != nil {
		return Server{}, err
	}
	srv.RoutingLists = lists
	s.mu.Lock()
	defer s.mu.Unlock()

	previous := s.servers
	s.servers = append([]Server(nil), previous...)
	normalizeServer(&srv)
	if srv.ID == "" {
		srv.ID = generateHex(8)
		srv.CreatedAt = time.Now()
		s.servers = append(s.servers, srv)
	} else {
		found := false
		for i, existing := range s.servers {
			if existing.ID == srv.ID {
				srv.CreatedAt = existing.CreatedAt
				s.servers[i] = srv
				found = true
				break
			}
		}
		if !found {
			srv.CreatedAt = time.Now()
			s.servers = append(s.servers, srv)
		}
	}
	if err := s.persist(); err != nil {
		s.servers = previous
		return Server{}, err
	}
	return cloneServer(srv), nil
}

func cloneServer(s Server) Server {
	s.VKLinks = append([]string{}, s.VKLinks...)
	s.DNSServers = append([]string{}, s.DNSServers...)
	s.RoutingLists = routing.Clone(s.RoutingLists)
	if s.SSH != nil {
		ssh := *s.SSH
		s.SSH = &ssh
	}
	return s
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.servers
	out := make([]Server, 0, len(previous))
	for _, srv := range s.servers {
		if srv.ID != id {
			out = append(out, srv)
		}
	}
	s.servers = out
	if err := s.persist(); err != nil {
		s.servers = previous
		return err
	}
	return nil
}

func atomicProfileWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".profiles-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
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

// Preserve damaged bytes before recovering; never silently discard user data.
func (s *Store) recoverProfiles(damaged []byte) error {
	backup, err := os.ReadFile(s.path + ".bak")
	var profiles []Server
	valid := err == nil && json.Unmarshal(backup, &profiles) == nil && profiles != nil
	if damaged != nil {
		archive, err := os.CreateTemp(filepath.Dir(s.path), "servers-damaged-*.json")
		if err != nil {
			return err
		}
		_, writeErr := archive.Write(damaged)
		closeErr := archive.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if valid {
		if err := atomicProfileWrite(s.path, backup); err != nil {
			return err
		}
		s.servers = profiles
		for i := range s.servers {
			normalizeServer(&s.servers[i])
		}
		s.recoveryNotice = "recovered"
	} else {
		// Leave a corrupt primary in place until an explicit save/import.
		s.servers = []Server{}
		s.recoveryNotice = "damaged"
	}
	return nil
}

func (s *Store) RecoveryNotice() string { s.mu.Lock(); defer s.mu.Unlock(); return s.recoveryNotice }

func (s *Store) RestoreBackup() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path + ".bak")
	if err != nil {
		return err
	}
	var profiles []Server
	if err := json.Unmarshal(data, &profiles); err != nil {
		return err
	}
	if profiles == nil {
		return fmt.Errorf("invalid profile backup")
	}
	for i := range profiles {
		normalizeServer(&profiles[i])
	}
	previous := s.servers
	s.servers = profiles
	if err := atomicProfileWrite(s.path, data); err != nil {
		s.servers = previous
		return err
	}
	s.recoveryNotice = "restored"
	return nil
}

// GenerateClientID returns a fresh 32-hex-char client ID (16 random bytes),
// matching the core's own client-id format.
func GenerateClientID() string { return generateHex(16) }

func generateHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing means the OS entropy source is broken - a
		// timestamp-derived fallback keeps profile creation from hard-failing.
		for i := range b {
			b[i] = byte(time.Now().UnixNano() >> uint(i))
		}
	}
	return fmt.Sprintf("%x", b)
}
