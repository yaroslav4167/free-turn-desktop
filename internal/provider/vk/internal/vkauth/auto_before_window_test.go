package vkauth

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/samosvalishe/free-turn-proxy/internal/provider/vk/internal/browserprofile"
	"github.com/samosvalishe/free-turn-proxy/internal/provider/vk/internal/captcha"
)

func TestSuccessfulAutomaticSolveNeverInvokesManualWindow(t *testing.T) {
	c := New(Config{FingerprintSeed: "auto-before-window"})
	autoCalls := 0
	c.autoSolver = func(context.Context, *captcha.Error, int, tlsclient.HttpClient, browserprofile.Profile) (string, error) {
		autoCalls++
		return "synthetic-auto-token", nil
	}
	c.manualSolve = func(context.Context, *captcha.Error, net.Dialer, browserprofile.Profile) (string, error) {
		t.Error("manual window opened despite automatic success")
		return "", errors.New("unexpected window")
	}
	data, err := c.solveCaptcha(context.Background(), nil, c.currentPersona(), 1, "DEMO", "name", "synthetic-access", testCaptchaErr())
	if err != nil || autoCalls != 1 || !strings.Contains(data, "success_token=synthetic-auto-token") {
		t.Fatalf("auto solve: calls %d, error %v", autoCalls, err)
	}
}

func TestOnlyExplicitManualModeSkipsAutomaticSolve(t *testing.T) {
	c := New(Config{ManualOnly: true, FingerprintSeed: "explicit-manual"})
	c.autoSolver = func(context.Context, *captcha.Error, int, tlsclient.HttpClient, browserprofile.Profile) (string, error) {
		t.Error("explicit manual mode ignored")
		return "", errors.New("unexpected auto attempt")
	}
	manualCalls := 0
	c.manualSolve = func(context.Context, *captcha.Error, net.Dialer, browserprofile.Profile) (string, error) {
		manualCalls++
		return "synthetic-manual-token", nil
	}
	data, err := c.solveCaptcha(context.Background(), nil, c.currentPersona(), 1, "DEMO", "name", "synthetic-access", testCaptchaErr())
	if err != nil || manualCalls != 1 || !strings.Contains(data, "success_token=synthetic-manual-token") {
		t.Fatalf("explicit manual solve: calls %d, error %v", manualCalls, err)
	}
}

func TestCancelledAutomaticSolveNeverOpensManualWindow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Config{FingerprintSeed: "cancel-auto-before-window"})
	c.autoSolver = func(context.Context, *captcha.Error, int, tlsclient.HttpClient, browserprofile.Profile) (string, error) {
		cancel()
		return "", context.Canceled
	}
	c.manualSolve = func(context.Context, *captcha.Error, net.Dialer, browserprofile.Profile) (string, error) {
		t.Error("cancelled solve opened a window")
		return "", errors.New("unexpected window")
	}
	_, err := c.solveCaptcha(ctx, nil, c.currentPersona(), 1, "DEMO", "name", "synthetic-access", testCaptchaErr())
	if !errors.Is(err, context.Canceled) || c.identity.Gen != 0 || c.lockout.Load() != 0 {
		t.Fatalf("cancelled solve changed identity/lockout: %v", err)
	}
}
