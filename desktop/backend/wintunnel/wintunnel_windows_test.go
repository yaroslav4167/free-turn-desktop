//go:build windows

package wintunnel

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"testing"

	"github.com/samosvalishe/free-turn-proxy/desktop/locale"
	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

func TestWintunAccessDeniedExplainsElevation(t *testing.T) {
	defer locale.Set("en")
	for _, language := range []struct{ code, hint string }{
		{"ru", "Запуск от имени администратора"},
		{"en", "Run as administrator"},
	} {
		locale.Set(language.code)
		for _, cause := range []error{windows.ERROR_ACCESS_DENIED, windows.ERROR_PRIVILEGE_NOT_HELD, os.ErrPermission} {
			err := wintunAdapterError(fmt.Errorf("Error creating interface: %w", cause))
			if !strings.Contains(err.Error(), language.hint) {
				t.Fatalf("%s: missing elevation hint: %v", language.code, err)
			}
			if !errors.Is(err, cause) || !strings.Contains(err.Error(), "create wintun adapter") {
				t.Fatalf("lost original diagnostic: %v", err)
			}
		}
	}
}

func TestWintunOtherErrorsKeepOriginalDiagnostic(t *testing.T) {
	cause := errors.New("driver initialization failed")
	err := wintunAdapterError(cause)
	if !errors.Is(err, cause) || err.Error() != "create wintun adapter: driver initialization failed" {
		t.Fatalf("unrelated error was rewritten: %v", err)
	}
}

type fakeRouteAdapter struct {
	routes map[netip.Prefix]netip.Addr
	err    error
}

func (a *fakeRouteAdapter) AddRoute(destination netip.Prefix, nextHop netip.Addr, _ uint32) error {
	if a.err != nil {
		return a.err
	}
	if _, exists := a.routes[destination]; exists {
		return windows.ERROR_OBJECT_ALREADY_EXISTS
	}
	a.routes[destination] = nextHop
	return nil
}

func TestAddRoutesPreservesLocalAddress(t *testing.T) {
	local := netip.MustParsePrefix("10.13.13.9/32")
	subnet := netip.MustParsePrefix("10.13.13.0/24")
	a := &fakeRouteAdapter{routes: map[netip.Prefix]netip.Addr{local: netip.IPv4Unspecified(), subnet: netip.IPv4Unspecified()}}
	routes := []*winipcfg.RouteData{
		{Destination: netip.MustParsePrefix("0.0.0.0/0"), NextHop: netip.IPv4Unspecified()},
		{Destination: subnet, NextHop: netip.IPv4Unspecified()},
		{Destination: netip.MustParsePrefix("::/0"), NextHop: netip.IPv6Unspecified()},
	}
	if err := addRoutesPreservingLocal(a, routes); err != nil {
		t.Fatal(err)
	}
	for _, prefix := range []netip.Prefix{local, subnet, routes[0].Destination, routes[2].Destination} {
		if _, ok := a.routes[prefix]; !ok {
			t.Errorf("missing route %s", prefix)
		}
	}
}

func TestAddRoutesReturnsFailure(t *testing.T) {
	a := &fakeRouteAdapter{err: windows.ERROR_ACCESS_DENIED}
	err := addRoutesPreservingLocal(a, []*winipcfg.RouteData{{Destination: netip.MustParsePrefix("0.0.0.0/0"), NextHop: netip.IPv4Unspecified()}})
	if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("got %v", err)
	}
}
