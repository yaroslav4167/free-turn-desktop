package backend

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/samosvalishe/free-turn-proxy/desktop/backend/routing"
)

func TestPreflightDNSFallback(t *testing.T) {
	for _, kind := range []string{"success", "timeout", "negative", "cancelled", "both-fail"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			want := netip.MustParseAddr("192.0.2.10")
			r := preflightResolver{
				primaryBudget: 5 * time.Millisecond,
				primary: exclusionResolver(func(ctx context.Context, network, host string) ([]netip.Addr, error) {
					if host != "relay.example.test" || network != "ip" {
						t.Fatalf("unexpected lookup %s %s", network, host)
					}
					switch kind {
					case "success":
						return []netip.Addr{want}, nil
					case "negative":
						return nil, &net.DNSError{Err: "no such host", IsNotFound: true}
					case "cancelled":
						cancel()
						return nil, context.Canceled
					default:
						<-ctx.Done()
						return nil, ctx.Err()
					}
				}),
				fallback: exclusionResolver(func(ctx context.Context, network, host string) ([]netip.Addr, error) {
					calls++
					if ctx.Err() != nil {
						t.Fatal("primary timeout consumed fallback context")
					}
					if kind == "both-fail" {
						return nil, errors.New("HTTPS unavailable")
					}
					return []netip.Addr{want}, nil
				}),
			}
			ips, err := r.LookupNetIP(ctx, "ip", "relay.example.test")
			if kind == "success" || kind == "timeout" {
				if err != nil || len(ips) != 1 || ips[0] != want {
					t.Fatalf("%v, %v", ips, err)
				}
			} else if err == nil {
				t.Fatal("DNS failure must abort preparation")
			}
			if kind == "both-fail" && !strings.Contains(err.Error(), "HTTPS unavailable") {
				t.Fatal(err)
			}
			wantCalls := 0
			if kind == "timeout" || kind == "both-fail" {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatalf("fallback calls %d, want %d", calls, wantCalls)
			}
		})
	}
}

func TestPreflightDNSDoesNotInheritSessionResolver(t *testing.T) {
	old := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("previous session resolver used")
		return nil, errors.New("previous session")
	}}
	defer func() { net.DefaultResolver = old }()
	for _, mode := range []string{"", "auto", "plain", "doh"} {
		resolver := newPreflightResolver(Server{DNSMode: mode})
		if resolver == net.DefaultResolver {
			t.Fatalf("%s inherited global resolver", mode)
		}
		if mode == "auto" || mode == "" {
			r := resolver.(preflightResolver)
			primary := r.primary.(*net.Resolver)
			if primary == net.DefaultResolver || primary.Dial != nil {
				t.Fatal("Auto must start with a fresh system resolver")
			}
		}
	}
	r := newPreflightResolver(Server{DNSMode: "auto", DNSServers: []string{"192.0.2.53"}}).(preflightResolver)
	if r.primary.(*net.Resolver).Dial == nil {
		t.Fatal("explicit profile DNS servers ignored")
	}
}

func TestPreflightDNSCancellationBeforeLookup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fail := exclusionResolver(func(context.Context, string, string) ([]netip.Addr, error) {
		t.Fatal("lookup after cancellation")
		return nil, nil
	})
	_, err := (preflightResolver{primary: fail, fallback: fail}).LookupNetIP(ctx, "ip", "relay.example.test")
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestPreflightDNSFallbackForEndpointsAndRoutingAfterSuccess(t *testing.T) {
	primaryCalls, fallbackCalls := 0, 0
	ip := netip.MustParseAddr("192.0.2.10")
	r := preflightResolver{
		primaryBudget: time.Second,
		primary: exclusionResolver(func(context.Context, string, string) ([]netip.Addr, error) {
			primaryCalls++
			if primaryCalls == 1 {
				return []netip.Addr{ip}, nil
			}
			return nil, &net.DNSError{Err: "i/o timeout", IsTimeout: true}
		}),
		fallback: exclusionResolver(func(context.Context, string, string) ([]netip.Addr, error) {
			fallbackCalls++
			return []netip.Addr{ip}, nil
		}),
	}
	// A successful query must not sticky-disable fallback for later queries or
	// reconnects, unlike the core's one-time UDP reachability probe.
	for i := 0; i < 2; i++ {
		if _, err := resolveExclusions(context.Background(), Server{Peer: "relay.example.test:56952"}, r); err != nil {
			t.Fatal(err)
		}
		plan, err := routing.Build(context.Background(), "0.0.0.0/0", "", []routing.List{{Enabled: true, Mode: routing.Whitelist, Entries: []string{"route.example.test"}}}, r)
		if err != nil || plan != "192.0.2.10/32" {
			t.Fatalf("incomplete route plan %q, %v", plan, err)
		}
	}
	if primaryCalls != 4 || fallbackCalls != 3 {
		t.Fatalf("primary %d, fallback %d", primaryCalls, fallbackCalls)
	}
}
