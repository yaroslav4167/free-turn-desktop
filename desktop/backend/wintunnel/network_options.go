package wintunnel

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/samosvalishe/free-turn-proxy/mobile"
)

type ConnectionOptions struct {
	Routes              string
	BypassLocalNetworks bool
}

func applyConnectionOptions(params *mobile.TunnelParams, options ConnectionOptions) error {
	if options.Routes != "" {
		params.AllowedIPs = options.Routes
	}
	if !options.BypassLocalNetworks {
		return nil
	}
	// Broad interface masks create implicit OS connected routes. They would
	// capture a local/corporate subnet even after excluding it from VPN routes.
	// Assign the same tunnel IPs as host addresses; cryptographic AllowedIPs
	// and the original WireGuard configuration are not changed.
	var addresses []string
	prefixes, err := parseInterfacePrefixes(params.Addresses)
	if err != nil {
		return err
	}
	for _, prefix := range prefixes {
		ip := prefix.Addr()
		addresses = append(addresses, netip.PrefixFrom(ip, ip.BitLen()).String())
	}
	params.Addresses = strings.Join(addresses, ",")
	return nil
}

func parseInterfacePrefixes(raw string) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	if strings.TrimSpace(raw) == "" {
		return prefixes, nil
	}
	for _, value := range strings.Split(raw, ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("адрес VPN-интерфейса: %w", err)
		}
		// Unlike route prefixes, interface addresses must keep their host bits.
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}
