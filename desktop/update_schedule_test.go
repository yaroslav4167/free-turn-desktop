package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestAutoUpdateChecksImmediatelyAndStopsOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		runAutoUpdateChecks(ctx, time.Hour, func() bool { return true }, func(ctx context.Context) error { close(started); <-ctx.Done(); return ctx.Err() })
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first update check was delayed")
	}
	cancel()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("update checks ignored shutdown")
	}
}

func TestDisabledAutoUpdatesNeverCheck(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Millisecond)
	defer cancel()
	var calls atomic.Int32
	runAutoUpdateChecks(ctx, 5*time.Millisecond, func() bool { return false }, func(context.Context) error { calls.Add(1); return nil })
	if calls.Load() != 0 {
		t.Fatal("disabled auto updates performed a request")
	}
}

func TestCancelledStartupDoesNotCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	runAutoUpdateChecks(ctx, time.Hour, func() bool { return true }, func(context.Context) error { t.Fatal("cancelled startup checked updates"); return nil })
}

func TestAutoUpdatesKeepPeriodicChecksAfterAnError(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	calls := 0
	runAutoUpdateChecks(ctx, 5*time.Millisecond, func() bool { return true }, func(context.Context) error {
		calls++
		if calls == 2 {
			cancel()
		}
		return errors.New("synthetic offline check")
	})
	if calls != 2 {
		t.Fatalf("periodic checking stopped after an error: %d calls", calls)
	}
}
