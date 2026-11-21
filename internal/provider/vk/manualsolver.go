package vk

import (
	"context"
	"fmt"
	"net"

	"github.com/samosvalishe/free-turn-proxy/internal/provider/vk/internal/browserprofile"
	"github.com/samosvalishe/free-turn-proxy/internal/provider/vk/internal/captcha"
	manualcaptcha "github.com/samosvalishe/free-turn-proxy/internal/provider/vk/internal/captcha/manual"
)

// The local port has one owner, but cancelled requests must not wait for an
// unrelated three-minute challenge or later present their obsolete URL.
var proxyManualSlot = make(chan struct{}, 1)

// ProxyManualSolver показывает UI после запуска локального captcha-прокси.
func ProxyManualSolver(show func(url string), hide func()) ManualSolverFunc {
	return func(ctx context.Context, e *captcha.Error, dialer net.Dialer, profile browserprofile.Profile) (string, error) {
		if e.RedirectURI == "" {
			return "", fmt.Errorf("manual captcha: no redirect_uri")
		}

		if err := ctx.Err(); err != nil {
			return "", err
		}
		select {
		case proxyManualSlot <- struct{}{}:
			defer func() { <-proxyManualSlot }()
		case <-ctx.Done():
			return "", ctx.Err()
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if hide != nil {
			defer hide()
		}

		return manualcaptcha.SolveViaProxyWithPresenter(ctx, e.RedirectURI, dialer, profile, show)
	}
}
