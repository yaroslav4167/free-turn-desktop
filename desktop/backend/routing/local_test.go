package routing

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"
)

func TestLocalBypassWithoutUserListsPreservesPublicInternet(t *testing.T) {
	routes, err := BuildWithBypass(context.Background(), "0.0.0.0/0,::/0", "", nil, nil, localRanges())
	if err != nil {
		t.Fatal(err)
	}
	for _, ip := range []string{"10.0.0.1", "10.255.255.254", "172.16.1.1", "172.31.255.254", "192.168.1.1", "100.64.1.1", "100.127.255.254", "127.0.0.1", "169.254.1.1", "224.0.0.251", "239.255.255.250", "255.255.255.255", "::1", "fd12::1", "fe80::1", "ff02::fb"} {
		if has(routes, ip) {
			t.Fatalf("local address %s still routed through VPN", ip)
		}
	}
	for _, ip := range []string{"1.1.1.1", "8.8.8.8", "100.63.255.254", "100.128.0.1", "172.15.1.1", "172.32.1.1", "192.167.1.1", "203.0.113.5", "2001:4860:4860::8888", "2606:4700:4700::1111"} {
		if !has(routes, ip) {
			t.Fatalf("public address %s unexpectedly bypassed", ip)
		}
	}
	if len(strings.Split(routes, ",")) > MaxRoutes {
		t.Fatal("automatic exclusions exceeded route budget")
	}
}

func TestLocalBypassWinsOverWhitelistButKeepsVPNDNS(t *testing.T) {
	lists := []List{{Mode: Whitelist, Enabled: true, Entries: []string{"10.0.0.0/8", "203.0.113.0/24"}}}
	routes, err := BuildWithBypass(context.Background(), "0.0.0.0/0", "10.9.0.1", lists, nil, localRanges())
	if err != nil {
		t.Fatal(err)
	}
	if has(routes, "10.2.0.1") || !has(routes, "10.9.0.1") || !has(routes, "203.0.113.8") || has(routes, "8.8.8.8") {
		t.Fatal("LAN/DNS/whitelist priority broken", routes)
	}
	// Opting out keeps the original list/AllowedIPs semantics.
	routes, err = Build(context.Background(), "0.0.0.0/0", "", lists, nil)
	if err != nil || !has(routes, "10.2.0.1") {
		t.Fatal("local VPN access cannot be restored by disabling bypass")
	}
}

func TestLocalBypassKeepsPrivateIPv6VPNDNS(t *testing.T) {
	routes, err := BuildWithBypass(context.Background(), "::/0", "fd12::53", nil, nil, localRanges())
	if err != nil || !has(routes, "fd12::53") || has(routes, "fd12::1") || !has(routes, "2606:4700:4700::1111") {
		t.Fatal("IPv6 DNS service exception broken", routes, err)
	}
}

func TestLocalBypassSupportsConnectedPublicSubnetsAndIPv6(t *testing.T) {
	bypass := append(localRanges(), netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("2001:db8:1234::/64"))
	routes, err := BuildWithBypass(context.Background(), "0.0.0.0/0,::/0", "", nil, nil, bypass)
	if err != nil {
		t.Fatal(err)
	}
	for _, ip := range []string{"198.51.100.5", "2001:db8:1234::1"} {
		if has(routes, ip) {
			t.Fatal("connected network not bypassed", ip)
		}
	}
	for _, ip := range []string{"198.51.101.5", "2001:db8:1235::1"} {
		if !has(routes, ip) {
			t.Fatal("connected exclusion escaped its subnet", ip)
		}
	}
}

func TestLocalBypassDoesNotExpandAllowedIPsAndRejectsEmptyPlan(t *testing.T) {
	if _, err := BuildWithBypass(context.Background(), "10.0.0.0/8", "", nil, nil, localRanges()); err == nil || !strings.Contains(err.Error(), "отключите") {
		t.Fatal("private-only VPN needs an actionable error", err)
	}
	routes, err := BuildWithBypass(context.Background(), "203.0.113.0/24", "10.9.0.1", nil, nil, localRanges())
	if err != nil || has(routes, "10.9.0.1") || has(routes, "8.8.8.8") {
		t.Fatal("automatic bypass expanded cryptographic AllowedIPs", routes, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := BuildWithBypass(ctx, "0.0.0.0/0", "", nil, nil, localRanges()); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled route planning continued", err)
	}
}

func TestLocalInterfaceSnapshotSkipsTunnelsAndDownInterfaces(t *testing.T) {
	for _, test := range []struct {
		name  string
		flags net.Flags
		want  bool
	}{
		{"eth0", net.FlagUp | net.FlagBroadcast, true},
		{"en0", net.FlagUp | net.FlagBroadcast, true},
		{"utun3", net.FlagUp | net.FlagPointToPoint, false},
		{"FreeTurn", net.FlagUp | net.FlagBroadcast, false},
		{"ftun0", net.FlagUp | net.FlagBroadcast, false},
		{"eth0", net.FlagBroadcast, false},
		{"lo", net.FlagUp | net.FlagLoopback, false},
	} {
		if got := localInterface(net.Interface{Name: test.name, Flags: test.flags}); got != test.want {
			t.Fatal(test.name, got)
		}
	}
}

func TestAutomaticLocalExclusionsDoNotConsumeUserListLimit(t *testing.T) {
	lists := make([]List, MaxLists)
	for i := range lists {
		lists[i] = List{Mode: Blacklist, Enabled: true, Entries: []string{"203.0.113.5"}}
	}
	if _, err := BuildWithBypass(context.Background(), "0.0.0.0/0", "", lists, nil, localRanges()); err != nil {
		t.Fatal(err)
	}
}
