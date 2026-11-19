package backend

import "testing"

func TestCoreConfigPreservesAmneziaProfile(t *testing.T) {
	const profile = "[Interface]\nJc = 4\nJmin = 40\nJmax = 70\n"
	s := Server{ConnMode: ConnModeVPN, WgClientConf: profile}
	if got := s.ToCoreConfig().Tunnel; got != (CoreTunnel{Mode: "awg", Config: profile, MTU: DefaultTunnelMTU}) {
		t.Fatalf("unexpected tunnel config: %+v", got)
	}
	s.ConnMode = ""
	if got := s.ToCoreConfig().Tunnel; got != (CoreTunnel{}) {
		t.Fatalf("non-VPN profile produced tunnel config: %+v", got)
	}
}
