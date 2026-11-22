package wintunnel

import (
	"testing"

	"github.com/samosvalishe/free-turn-proxy/mobile"
)

func TestLANBypassUsesHostMasksWithoutChangingTunnelIPsOrDNS(t *testing.T) {
	p := &mobile.TunnelParams{Addresses: "10.13.13.3/24,fd12:1234::5/64", AllowedIPs: "0.0.0.0/0,::/0", DNS: "10.13.13.1"}
	if err := applyConnectionOptions(p, ConnectionOptions{Routes: "203.0.113.0/24", BypassLocalNetworks: true}); err != nil {
		t.Fatal(err)
	}
	if p.Addresses != "10.13.13.3/32,fd12:1234::5/128" || p.AllowedIPs != "203.0.113.0/24" || p.DNS != "10.13.13.1" {
		t.Fatal("wrong OS tunnel settings", p)
	}
}

func TestDisabledLANBypassKeepsOriginalInterfaceMasks(t *testing.T) {
	p := &mobile.TunnelParams{Addresses: "10.13.13.3/24,fd12:1234::5/64", AllowedIPs: "0.0.0.0/0,::/0"}
	if err := applyConnectionOptions(p, ConnectionOptions{}); err != nil {
		t.Fatal(err)
	}
	if p.Addresses != "10.13.13.3/24,fd12:1234::5/64" || p.AllowedIPs != "0.0.0.0/0,::/0" {
		t.Fatal("disabled bypass changed settings", p)
	}
}

func TestInterfacePrefixesKeepHostBits(t *testing.T) {
	prefixes, err := parseInterfacePrefixes("10.13.13.3/24,fd12:1234::5/64")
	if err != nil {
		t.Fatal(err)
	}
	if len(prefixes) != 2 || prefixes[0].String() != "10.13.13.3/24" || prefixes[1].String() != "fd12:1234::5/64" {
		t.Fatal("interface address replaced with network address", prefixes)
	}
}
