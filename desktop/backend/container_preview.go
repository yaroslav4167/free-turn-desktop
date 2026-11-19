package backend

import (
	"errors"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"github.com/samosvalishe/free-turn-proxy/desktop/backend/serversetup"
)

// ContainerPreview is deliberately not saved. The advertised host can be stale,
// or intentionally different from SSH (NAT, bastion, separate public address).
// Offer a choice rather than silently trusting or overwriting either address.
type ContainerPreview struct {
	Server         Server `json:"server"`
	AdvertisedPeer string `json:"advertisedPeer"`
	SSHPeer        string `json:"sshPeer"`
	NeedsReview    bool   `json:"needsReview"`
}

func BuildContainerPreview(cfg serversetup.SSHConfig, link string) (*ContainerPreview, error) {
	parsed, err := ParseShareLink(link)
	if err != nil {
		return nil, errors.New("контейнер вернул некорректную ссылку клиента")
	}
	host, port, err := net.SplitHostPort(parsed.Peer)
	if err != nil || host == "" {
		return nil, errors.New("контейнер вернул некорректный адрес реле")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 || cfg.IP == "" {
		return nil, errors.New("контейнер вернул некорректный адрес реле")
	}
	srv := parsed.ApplyTo(DefaultServer())
	if srv.WgClientConf == "" || srv.ClientID == "" {
		return nil, errors.New("контейнер не вернул полный VPN-профиль")
	}
	cfg.Manager = serversetup.HackdiazDocker
	srv.SSH = &cfg
	srv.Name = cfg.IP
	return &ContainerPreview{
		Server: srv, AdvertisedPeer: srv.Peer, SSHPeer: net.JoinHostPort(cfg.IP, port),
		NeedsReview: normalizedRelayHost(host) != normalizedRelayHost(cfg.IP),
	}, nil
}

// Compare names/IP literals without DNS lookups. A separate public name or
// bastion still requires confirmation, even if DNS currently returns one IP.
func normalizedRelayHost(host string) string {
	host = strings.TrimSpace(host)
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.Unmap().String()
	}
	return strings.ToLower(strings.TrimSuffix(host, "."))
}
