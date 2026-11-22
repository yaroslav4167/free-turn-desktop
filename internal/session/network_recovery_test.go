package session

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// Synthetic link transitions: no live TURN/VPN and no host network changes.
func TestEstablishedSessionSurvivesNetworkLossAndRecovers(t *testing.T) {
	for _, scenario := range []string{"wifi-change", "internet-loss", "wake-with-reconnect"} {
		t.Run(scenario, func(t *testing.T) {
			s := newTestSession(t, Options{ConnectTimeout: 30 * time.Millisecond, StatusInterval: time.Millisecond}, nil)
			ctx, cancel := context.WithCancel(t.Context())
			done := make(chan error, 1)
			s.connected.Store(1)
			go func() { done <- s.watch(ctx, cancel) }()
			defer func() {
				cancel()
				if err := <-done; err != nil {
					t.Error(err)
				}
			}()
			waitPhase(t, s, PhaseConnected)
			s.connected.Store(0)
			waitPhase(t, s, PhaseConnecting)
			// An established session's reconnect must not inherit its first-connect deadline.
			time.Sleep(60 * time.Millisecond)
			if ctx.Err() != nil {
				t.Fatal("network interruption killed an established session")
			}
			s.connected.Store(1)
			waitPhase(t, s, PhaseConnected)
		})
	}
}

func TestNetworkRecoveryCaptchaKeepsSessionAliveThenReturnsConnected(t *testing.T) {
	var captcha atomic.Bool
	s := newTestSession(t, Options{ConnectTimeout: 30 * time.Millisecond, StatusInterval: time.Millisecond}, captcha.Load)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	s.connected.Store(1)
	go func() { done <- s.watch(ctx, cancel) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	waitPhase(t, s, PhaseConnected)
	captcha.Store(true)
	s.connected.Store(0)
	waitPhase(t, s, PhaseCaptcha)
	time.Sleep(60 * time.Millisecond)
	if ctx.Err() != nil {
		t.Fatal("captcha cancelled recovery")
	}
	s.connected.Store(1)
	captcha.Store(false)
	waitPhase(t, s, PhaseConnected)
}

func TestWakeProbeCancellationDoesNotRequestReconnect(t *testing.T) {
	s := newWakeSession(t)
	s.connected.Store(1)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if s.wakeNeedsRecycle(ctx) {
		t.Fatal("cancelled connection requested a recycle")
	}
}

func waitPhase(t *testing.T, s *Session, phase Phase) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if s.Snapshot().Phase == phase {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("got %s, want %s", s.Snapshot().Phase, phase)
}
