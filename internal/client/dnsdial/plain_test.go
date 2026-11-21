package dnsdial

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestPlainResolverServerSnapshot(t *testing.T) {
	packet, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &dns.Server{PacketConn: packet, Handler: dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		answer := new(dns.Msg)
		answer.SetReply(req)
		if req.Question[0].Qtype == dns.TypeA {
			answer.Answer = []dns.RR{&dns.A{Hdr: dns.RR_Header{Name: req.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET}, A: net.ParseIP("192.0.2.10")}}
		}
		_ = w.WriteMsg(answer)
	})}
	go func() { _ = server.ActivateAndServe() }()
	defer server.Shutdown()
	servers := []string{packet.LocalAddr().String()}
	resolver := NewPlainResolver(servers)
	servers[0] = "192.0.2.53:53"
	old := append([]string(nil), udpDNSServers()...)
	SetUDPDNSServers([]string{"192.0.2.54"})
	defer SetUDPDNSServers(old)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ips, err := resolver.LookupNetIP(ctx, "ip4", "relay.example.test")
	if err != nil || len(ips) != 1 || ips[0].String() != "192.0.2.10" {
		t.Fatalf("resolver inherited mutable/global servers: %v, %v", ips, err)
	}
}
