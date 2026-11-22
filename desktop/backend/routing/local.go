package routing

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strings"
)

// LocalBypassPrefixes snapshots local directly-connected networks before a VPN
// changes the route table. Never treat a current FreeTurn/point-to-point tunnel
// as a physical LAN, especially while switching from an existing VPN session.
func LocalBypassPrefixes(ctx context.Context) ([]netip.Prefix, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("локальные интерфейсы: %w", err)
	}
	prefixes := localRanges()
	for _, iface := range interfaces {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !localInterface(iface) {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			return nil, fmt.Errorf("адреса локального интерфейса: %w", err)
		}
		for _, address := range addresses {
			prefix, err := netip.ParsePrefix(address.String())
			if err != nil {
				continue
			}
			if !prefix.IsValid() || prefix.Addr().Is4In6() {
				continue
			}
			// Reject unusably broad/default masks rather than bypassing arbitrary
			// Internet space due to a malformed interface configuration.
			if (prefix.Addr().Is4() && prefix.Bits() < 8) || (prefix.Addr().Is6() && prefix.Bits() < 32) {
				continue
			}
			prefixes = append(prefixes, prefix.Masked())
		}
	}
	return compact(prefixes), nil
}

func localInterface(iface net.Interface) bool {
	name := strings.ToLower(iface.Name)
	return iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagBroadcast != 0 && iface.Flags&(net.FlagLoopback|net.FlagPointToPoint) == 0 && name != "freeturn" && !strings.HasPrefix(name, "ftun")
}

func localRanges() []netip.Prefix {
	var prefixes []netip.Prefix
	for _, cidr := range []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8",
		"169.254.0.0/16", "172.16.0.0/12", "192.168.0.0/16", "224.0.0.0/4", "255.255.255.255/32",
		"::/128", "::1/128", "fc00::/7", "fe80::/10", "fec0::/10", "ff00::/8",
	} {
		prefixes = append(prefixes, netip.MustParsePrefix(cidr))
	}
	return prefixes
}
