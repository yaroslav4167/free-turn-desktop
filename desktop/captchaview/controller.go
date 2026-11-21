// Package captchaview owns a bridge-free native verification window in an
// isolated child process. It never loads profiles or starts a VPN session.
package captchaview

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const ChildFlag = "--captcha-window"

type Labels struct {
	Title    string `json:"title"`
	Reload   string `json:"reload"`
	External string `json:"external"`
}

type childConfig struct {
	URL      string `json:"url"`
	CacheDir string `json:"cacheDir"`
	Labels   Labels `json:"labels"`
}

// Only the local captcha service may be opened. No arbitrary URL supplied by a
// page/UI is accepted, including lookalike hosts, credentials or custom ports.
func ValidURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Port() != "8765" || u.Opaque != "" {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

type runFunc func(context.Context, childConfig, <-chan string, func(string)) error
type task struct {
	url                        string
	cancel                     context.CancelFunc
	commands                   chan string
	ready, fallback, dismissed bool
}

type Controller struct {
	mu       sync.Mutex
	runners  sync.WaitGroup
	active   *task
	closed   bool
	run      runFunc
	timeout  time.Duration
	external func(string)
}

func New(external func(string)) *Controller {
	return &Controller{run: runProcess, timeout: 12 * time.Second, external: external}
}

func (c *Controller) Show(raw string, labels Labels) error {
	if !ValidURL(raw) {
		return errors.New("invalid local captcha URL")
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return errors.New("captcha window controller is closed")
	}
	if c.active != nil && c.active.url == raw && !c.active.dismissed && !c.active.fallback {
		select {
		case c.active.commands <- "focus":
		default:
		}
		c.mu.Unlock()
		return nil
	}
	if c.active != nil {
		c.active.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	t := &task{url: raw, cancel: cancel, commands: make(chan string, 1)}
	c.active = t
	c.runners.Add(1)
	c.mu.Unlock()
	go func() {
		defer c.runners.Done()
		timer := time.AfterFunc(c.timeout, func() { c.fallback(t, true) })
		defer timer.Stop()
		err := c.run(ctx, childConfig{URL: raw, Labels: labels}, t.commands, func(event string) {
			switch event {
			case "READY":
				c.mu.Lock()
				t.ready = true
				c.mu.Unlock()
				timer.Stop()
			case "CLOSED":
				c.mu.Lock()
				t.dismissed = true
				c.mu.Unlock()
				timer.Stop()
			case "EXTERNAL", "FAILED":
				c.fallback(t, false)
			}
		})
		c.mu.Lock()
		shouldFallback := c.active == t && ctx.Err() == nil && !t.dismissed && (!t.ready || err != nil)
		c.mu.Unlock()
		if shouldFallback {
			c.fallback(t, false)
		}
		c.mu.Lock()
		if c.active == t {
			c.active = nil
		}
		c.mu.Unlock()
		cancel()
	}()
	return nil
}

func (c *Controller) fallback(t *task, startup bool) {
	c.mu.Lock()
	if c.closed || c.active != t || t.fallback || t.dismissed || (startup && t.ready) {
		c.mu.Unlock()
		return
	}
	t.fallback = true
	t.cancel()
	open := c.external
	c.mu.Unlock()
	if open != nil {
		open(t.url)
	}
}

func (c *Controller) Hide() {
	c.mu.Lock()
	if c.active != nil {
		c.active.cancel()
		c.active = nil
	}
	c.mu.Unlock()
}

func (c *Controller) Close() {
	c.mu.Lock()
	c.closed = true
	if c.active != nil {
		c.active.cancel()
		c.active = nil
	}
	c.mu.Unlock()
	stopped := make(chan struct{})
	go func() { c.runners.Wait(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
	}
}

func runProcess(ctx context.Context, cfg childConfig, commands <-chan string, emit func(string)) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	cache, err := os.MkdirTemp("", "freeturn-captcha-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(cache) // Only the fresh private directory created above.
	cfg.CacheDir = cache
	cmd := exec.CommandContext(ctx, executable, ChildFlag)
	cmd.Stderr = io.Discard // Native errors may include a private challenge URL.
	cmd.Stdout = &protocolWriter{emit: emit}
	input, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer input.Close()
	// EOF asks the UI thread to close; force termination only if it is stuck.
	cmd.Cancel = func() error { return input.Close() }
	cmd.WaitDelay = time.Second
	if err = cmd.Start(); err != nil {
		return err
	}
	if err = json.NewEncoder(input).Encode(cfg); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case command := <-commands:
				if command == "focus" {
					_, _ = fmt.Fprintln(input, "focus")
				}
			}
		}
	}()
	return cmd.Wait()
}

// exec owns the output-copy goroutine and bounds its lifetime with WaitDelay,
// even when a WebKit/WebView2 subprocess briefly inherits the output pipe.
type protocolWriter struct {
	line    []byte
	discard bool
	emit    func(string)
}

func (w *protocolWriter) Write(data []byte) (int, error) {
	for _, b := range data {
		if b == '\n' {
			if !w.discard {
				switch event := strings.TrimSpace(string(w.line)); event {
				case "READY", "FAILED", "EXTERNAL", "CLOSED":
					w.emit(event)
				}
			}
			w.line = w.line[:0]
			w.discard = false
		} else if len(w.line) < 8192 && !w.discard {
			w.line = append(w.line, b)
		} else {
			w.discard = true
		}
	}
	return len(data), nil
}

// RunChild is handled before normal desktop startup. The private URL travels
// over stdin, not command-line arguments, and no Wails bindings are registered.
func RunChild(input io.Reader) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 64*1024)
	if !scanner.Scan() {
		return errors.New("missing captcha window configuration")
	}
	var cfg childConfig
	if json.Unmarshal(scanner.Bytes(), &cfg) != nil || !ValidURL(cfg.URL) {
		return errors.New("invalid captcha window configuration")
	}
	if cfg.Labels.Title == "" {
		cfg.Labels.Title = "FreeTurn · VK verification"
	}
	if cfg.Labels.Reload == "" {
		cfg.Labels.Reload = "Reload"
	}
	if cfg.Labels.External == "" {
		cfg.Labels.External = "Open in browser"
	}
	go func() {
		for scanner.Scan() {
			if scanner.Text() == "focus" {
				nativeCommand(1)
			}
		}
		nativeCommand(2)
	}()
	return runNative(cfg)
}
