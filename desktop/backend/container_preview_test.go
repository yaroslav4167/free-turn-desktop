package backend

import (
	"strings"
	"testing"

	"github.com/samosvalishe/free-turn-proxy/desktop/backend/serversetup"
)

func TestContainerPreviewRequiresExplicitEndpointChoice(t *testing.T) {
	const wg = "[Interface]\nJc = 4\n[Peer]\n"
	link := ShareLink{Provider: "vk", Peer: "192.0.2.112:56952", ClientID: "test-device",
		ObfProfile: "rtpopus3", ObfKey: strings.Repeat("ab", 32), WgConf: wg, Fingerprint: strings.Repeat("cd", 32)}
	encoded, err := link.Encode()
	if err != nil {
		t.Fatal(err)
	}
	cfg := serversetup.SSHConfig{IP: "vpn.example", HostFingerprint: "test-ssh-pin"}
	p, err := BuildContainerPreview(cfg, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if p.AdvertisedPeer != link.Peer || p.Server.Peer != link.Peer || p.SSHPeer != "vpn.example:56952" {
		t.Fatalf("wrong endpoint choices: advertised=%q selected=%q ssh=%q", p.AdvertisedPeer, p.Server.Peer, p.SSHPeer)
	}
	if p.Server.ClientID != link.ClientID || p.Server.ObfKey != link.ObfKey || p.Server.WgClientConf != strings.TrimSpace(wg) || p.Server.DTLSFingerprint != link.Fingerprint {
		t.Fatal("preview changed credentials, obfuscation, profile or DTLS pin")
	}
	if p.Server.SSH.Manager != serversetup.HackdiazDocker || p.Server.SSH.HostFingerprint != cfg.HostFingerprint || cfg.Manager != "" {
		t.Fatal("preview lost SSH identity or changed input config")
	}
	// Selecting SSH is a local profile edit, not a rewrite of server data.
	p.Server.Peer = p.SSHPeer
	if p.AdvertisedPeer != link.Peer || p.Server.ObfKey != link.ObfKey {
		t.Fatal("endpoint edit altered imported data")
	}
}

func TestContainerPreviewPreservesCustomEndpointsAndIPv6(t *testing.T) {
	for _, test := range []struct{ ssh, peer, want string }{
		{"bastion.example", "vpn.public.example:56952", "bastion.example:56952"},
		{"2001:db8::1", "[2001:db8::2]:56952", "[2001:db8::1]:56952"},
	} {
		encoded, err := (ShareLink{Provider: "vk", Peer: test.peer, ClientID: "test-device", WgConf: "[Interface]"}).Encode()
		if err != nil {
			t.Fatal(err)
		}
		p, err := BuildContainerPreview(serversetup.SSHConfig{IP: test.ssh}, encoded)
		if err != nil {
			t.Fatal(err)
		}
		if p.Server.Peer != test.peer || p.SSHPeer != test.want {
			t.Fatalf("custom endpoint silently changed: %+v", test)
		}
	}
}

func TestContainerPreviewRejectsInvalidEndpointWithoutLeakingLink(t *testing.T) {
	for _, peer := range []string{"invalid", "host:0", "host:70000", "host:port", ":56952"} {
		encoded, _ := (ShareLink{Provider: "vk", Peer: peer, ClientID: "secret-test-id", WgConf: "private-test-config"}).Encode()
		_, err := BuildContainerPreview(serversetup.SSHConfig{IP: "vpn.example"}, encoded)
		if err == nil {
			t.Fatalf("accepted invalid endpoint %q", peer)
		}
		if strings.Contains(err.Error(), "private-test-config") || strings.Contains(err.Error(), "secret-test-id") {
			t.Fatal("credentials leaked in error")
		}
	}
}
