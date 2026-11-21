package backend

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/samosvalishe/free-turn-proxy/desktop/backend/routing"
	"github.com/samosvalishe/free-turn-proxy/internal/client/dnsdial"
)

// Do not use net.DefaultResolver here: Session.Run replaces it, so the next
// connection would inherit the previous session's mode and UDP server list.
// Auto preserves system/split DNS first, but an unresponsive UDP path must not
// consume the entire preflight deadline before the HTTPS fallback can run.
func newPreflightResolver(srv Server) routing.Resolver {
	doh := dnsdial.AppDialer(dnsdial.DNSModeDoH).Resolver
	plain := dnsdial.NewPlainResolver(srv.DNSServers)
	switch srv.DNSMode {
	case dnsdial.DNSModeDoH:
		return doh
	case dnsdial.DNSModePlain:
		return plain
	default:
		var primary routing.Resolver = &net.Resolver{}
		if len(srv.DNSServers) != 0 {
			primary = plain
		}
		return preflightResolver{primary: primary, fallback: doh, primaryBudget: 2 * time.Second}
	}
}

type preflightResolver struct {
	primary, fallback routing.Resolver
	primaryBudget     time.Duration
}

func (r preflightResolver) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	primaryCtx, cancel := context.WithTimeout(ctx, r.primaryBudget)
	ips, err := r.primary.LookupNetIP(primaryCtx, network, host)
	cancel()
	if err == nil {
		return ips, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	var dnsErr *net.DNSError
	// A negative answer is not a transport failure. Never bypass NXDOMAIN or
	// silently omit a routing entry, and never reuse stale IPs from another run.
	if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
		return nil, err
	}
	ips, fallbackErr := r.fallback.LookupNetIP(ctx, network, host)
	if fallbackErr != nil {
		return nil, fmt.Errorf("primary DNS: %v; DoH fallback: %w", err, fallbackErr)
	}
	return ips, nil
}
