package main

import (
	"context"
	"time"
)

// Called in a background goroutine. The first check has no startup delay;
// later checks retain their interval and respect preferences and shutdown.
func runAutoUpdateChecks(ctx context.Context, interval time.Duration, enabled func() bool, check func(context.Context) error) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if ctx.Err() != nil {
				return
			}
			if enabled() {
				_ = check(ctx)
			}
			timer.Reset(interval)
		}
	}
}
