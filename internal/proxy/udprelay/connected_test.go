package udprelay

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/pion/dtls/v3"
	"github.com/samosvalishe/free-turn-proxy/internal/clientsdb"
	"github.com/samosvalishe/free-turn-proxy/internal/transport/dtlsdial"
)

func TestDTLSStreamReadinessAndTraffic(t *testing.T) {
	deps, listen := newPairDeps(t)
	endpoint, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer endpoint.Close()
	deps.ActiveLocalPeer.Store(endpoint.LocalAddr())
	cert, err := dtlsdial.GenerateSelfSignedCert()
	if err != nil {
		t.Fatal(err)
	}
	deps.DTLSDialer.ExpectedFingerprint = dtlsdial.CertificateFingerprint(cert)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pairs := make(chan streamPair)
	ok := make(chan struct{}, 1)
	packets := make(chan *Packet, 1)
	done := make(chan error, 1)
	const clientID = "local-test"
	peer := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9}
	go func() {
		done <- oneDTLS(ctx, deps, &Params{ClientID: clientID}, peer, listen, packets, pairs, ok, 1)
	}()
	var pair streamPair
	select {
	case pair = <-pairs:
	case <-ctx.Done():
		t.Fatal("missing relay pair")
	}
	defer pair.pipe.Close()
	close(pair.ready)
	if got := deps.ConnectedStreams.Load(); got != 0 {
		t.Fatalf("allocation alone counted as connected: %d", got)
	}
	server := make(chan error, 1)
	go func() {
		conn, err := dtls.ServerWithOptions(pair.pipe, peer,
			dtls.WithCertificates(cert), dtls.WithExtendedMasterSecret(dtls.RequireExtendedMasterSecret),
			dtls.WithCipherSuites(dtls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256))
		if err == nil {
			defer conn.Close()
			err = conn.HandshakeContext(ctx)
		}
		if err == nil {
			var id string
			var mode byte
			id, mode, err = clientsdb.ReadClientID(conn)
			if err == nil && (id != clientID || mode != clientsdb.ModeUDP) {
				err = fmt.Errorf("unexpected legacy Client ID record: %q, %d", id, mode)
			}
		}
		if err == nil {
			var buf [maxDatagramLen]byte
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			var n int
			n, err = conn.Read(buf[:])
			if err == nil {
				_, err = conn.Write(buf[:n])
			}
		}
		server <- err
		<-ctx.Done()
	}()
	select {
	case <-ok:
	case err := <-done:
		t.Fatalf("DTLS failed: %v", err)
	case err := <-server:
		t.Fatalf("server failed before traffic: %v", err)
	case <-ctx.Done():
		t.Fatal("DTLS did not become ready")
	}
	if got := deps.ConnectedStreams.Load(); got != 1 {
		t.Fatalf("expected one DTLS stream, got %d", got)
	}
	payload := bytes.Repeat([]byte{0x42}, 1800)
	packets <- &Packet{Data: bytes.Clone(payload), N: len(payload)}
	_ = endpoint.SetReadDeadline(time.Now().Add(3 * time.Second))
	var buf [maxDatagramLen]byte
	n, _, err := endpoint.ReadFromUDP(buf[:])
	if err != nil || !bytes.Equal(buf[:n], payload) {
		t.Fatalf("echo: got %d bytes, err %v", n, err)
	}
	if err := <-server; err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-done:
		if got := deps.ConnectedStreams.Load(); got != 0 {
			t.Fatalf("closed stream still counted: %d", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("DTLS did not stop")
	}
}
