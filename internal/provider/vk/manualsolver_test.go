package vk

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/samosvalishe/free-turn-proxy/internal/provider/vk/internal/browserprofile"
	"github.com/samosvalishe/free-turn-proxy/internal/provider/vk/internal/captcha"
)

func TestCancelledQueuedCaptchaNeverPresentsOrHidesAnotherChallenge(t *testing.T) {
	proxyManualSlot <- struct{}{}
	defer func() { <-proxyManualSlot }()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	solver := ProxyManualSolver(func(string) { t.Error("cancelled captcha shown") }, func() { t.Error("another captcha hidden") })
	done := make(chan error, 1)
	go func() {
		_, err := solver(ctx, &captcha.Error{RedirectURI: "https://id.vk.ru/demo"}, net.Dialer{}, browserprofile.Profile{})
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled captcha blocked behind another challenge")
	}
}

func TestAlreadyCancelledCaptchaDoesNotTakeSlot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	solver := ProxyManualSolver(func(string) { t.Error("cancelled captcha shown") }, func() { t.Error("cancelled captcha changed UI") })
	_, err := solver(ctx, &captcha.Error{RedirectURI: "https://id.vk.ru/demo"}, net.Dialer{}, browserprofile.Profile{})
	if !errors.Is(err, context.Canceled) || len(proxyManualSlot) != 0 {
		t.Fatalf("cancelled attempt took slot: %v", err)
	}
}
