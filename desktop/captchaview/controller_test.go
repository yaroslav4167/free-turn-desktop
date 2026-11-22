package captchaview

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const demoURL = "http://localhost:8765/not_robot_captcha?session_token=synthetic"

func TestValidURL(t *testing.T) {
	for _, url := range []string{demoURL, "http://127.0.0.1:8765/test", "http://[::1]:8765/test"} {
		if !ValidURL(url) {
			t.Fatalf("local URL rejected: %s", url)
		}
	}
	for _, url := range []string{"https://id.vk.ru/captcha", "file:///etc/passwd", "http://localhost:8766/", "http://localhost.evil.test:8765/", "http://localhost:8765@evil.test/", "http://user@localhost:8765/", "javascript:alert(1)", "http://127.1:8765/", "http://localhost:8765\\@evil.test/", "about:blank"} {
		if ValidURL(url) {
			t.Fatalf("non-captcha URL accepted: %s", url)
		}
	}
}

func waitSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("window lifecycle timed out")
	}
}

func TestControllerOpensNativeFirstAndFocusesWithoutDuplicates(t *testing.T) {
	var launches, external atomic.Int32
	ready, focused, stopped := make(chan struct{}), make(chan struct{}), make(chan struct{})
	c := New(func(string) { external.Add(1) })
	c.run = func(ctx context.Context, cfg childConfig, commands <-chan string, emit func(string)) error {
		launches.Add(1)
		if cfg.URL != demoURL {
			t.Error("wrong challenge")
		}
		emit("READY")
		close(ready)
		select {
		case <-commands:
			close(focused)
		case <-ctx.Done():
			return ctx.Err()
		}
		<-ctx.Done()
		close(stopped)
		return ctx.Err()
	}
	defer c.Close()
	if err := c.Show(demoURL, Labels{}); err != nil {
		t.Fatal(err)
	}
	waitSignal(t, ready)
	if err := c.Show(demoURL, Labels{}); err != nil {
		t.Fatal(err)
	}
	waitSignal(t, focused)
	c.Hide()
	waitSignal(t, stopped)
	if launches.Load() != 1 || external.Load() != 0 {
		t.Fatalf("launches %d, browser %d", launches.Load(), external.Load())
	}
}

func TestReadyWindowIsNotClosedWhileChallengeLoadsSlowly(t *testing.T) {
	var external atomic.Int32
	ready := make(chan struct{})
	c := New(func(string) { external.Add(1) })
	c.timeout = 10 * time.Millisecond
	c.run = func(ctx context.Context, _ childConfig, _ <-chan string, emit func(string)) error {
		emit("READY")
		close(ready)
		<-ctx.Done()
		return ctx.Err()
	}
	defer c.Close()
	if err := c.Show(demoURL, Labels{}); err != nil {
		t.Fatal(err)
	}
	waitSignal(t, ready)
	time.Sleep(40 * time.Millisecond)
	c.mu.Lock()
	active := c.active != nil && !c.active.fallback
	c.mu.Unlock()
	if !active || external.Load() != 0 {
		t.Fatal("slow challenge closed a ready native window")
	}
}

func TestControllerFallsBackOnceWithoutAsking(t *testing.T) {
	for _, mode := range []string{"startup-error", "runtime-failure", "external-button", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			opened := make(chan string, 4)
			c := New(func(url string) { opened <- url })
			c.timeout = 20 * time.Millisecond
			c.run = func(ctx context.Context, _ childConfig, _ <-chan string, emit func(string)) error {
				switch mode {
				case "startup-error":
					return errors.New("no WebView")
				case "runtime-failure":
					emit("READY")
					emit("FAILED")
					emit("FAILED")
				case "external-button":
					emit("READY")
					emit("EXTERNAL")
					emit("FAILED")
				case "timeout":
					<-ctx.Done()
					emit("FAILED")
				}
				return ctx.Err()
			}
			defer c.Close()
			if err := c.Show(demoURL, Labels{}); err != nil {
				t.Fatal(err)
			}
			select {
			case url := <-opened:
				if url != demoURL {
					t.Fatal("wrong fallback URL")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("no automatic fallback")
			}
			c.Close()
			select {
			case <-opened:
				t.Fatal("browser opened repeatedly")
			default:
			}
		})
	}
}

func TestControllerDoesNotReopenCancelledOrDismissedChallenges(t *testing.T) {
	for _, mode := range []string{"cancelled", "dismissed"} {
		t.Run(mode, func(t *testing.T) {
			var external atomic.Int32
			started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
			c := New(func(string) { external.Add(1) })
			c.run = func(ctx context.Context, _ childConfig, _ <-chan string, emit func(string)) error {
				if mode == "dismissed" {
					emit("CLOSED")
				}
				close(started)
				<-release
				if mode == "cancelled" {
					emit("FAILED")
				} // Delayed result from an old process.
				close(finished)
				return nil
			}
			defer c.Close()
			if err := c.Show(demoURL, Labels{}); err != nil {
				t.Fatal(err)
			}
			waitSignal(t, started)
			c.mu.Lock()
			old := c.active
			c.mu.Unlock()
			if mode == "cancelled" {
				c.Hide()
			}
			c.fallback(old, true)
			close(release)
			waitSignal(t, finished)
			c.Close()
			if external.Load() != 0 {
				t.Fatal("obsolete/dismissed verification opened browser")
			}
		})
	}
}

func TestChildRejectsUntrustedConfigurationBeforeNativeStartup(t *testing.T) {
	for _, raw := range []string{"", `{"url":"file:///etc/passwd"}`, `{"url":"http://evil.test:8765/"}`, strings.Repeat("x", 70000)} {
		if err := RunChild(strings.NewReader(raw)); err == nil {
			t.Fatal("invalid configuration accepted")
		}
	}
}

func TestProtocolAcceptsOnlyLifecycleMarkers(t *testing.T) {
	var events []string
	w := &protocolWriter{emit: func(event string) { events = append(events, event) }}
	for _, data := range []string{"REA", "DY\nhttp://localhost:8765/?session_token=private\n", strings.Repeat("x", 9000), "READY\nFAILED\nCLOSED\n"} {
		if n, err := w.Write([]byte(data)); n != len(data) || err != nil {
			t.Fatal("output parser failed")
		}
	}
	if strings.Join(events, ",") != "READY,FAILED,CLOSED" {
		t.Fatalf("untrusted output forwarded: %v", events)
	}
}
