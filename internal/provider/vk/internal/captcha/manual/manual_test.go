package manual

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestRunCaptchaServerPresentsAfterListen(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	keyCh := make(chan string, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ready")
	})
	present := func(captchaURL string) {
		response, err := http.Get(captchaURL) //nolint:gosec,noctx // local test server
		if err != nil {
			t.Errorf("present called before server was ready: %v", err)
			keyCh <- "failed"
			return
		}
		_ = response.Body.Close()
		keyCh <- "token"
	}

	got, err := runCaptchaServerAndWait(ctx, handler, localCaptchaOrigin(), keyCh, "test", present)
	if err != nil {
		t.Fatal(err)
	}
	if got != "token" {
		t.Fatalf("token = %q, want token", got)
	}
}

func TestCancelledCaptchaDoesNotStartServerOrPresent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := runCaptchaServerAndWait(ctx, nil, localCaptchaOrigin(), nil, "test", func(string) {
		t.Fatal("obsolete captcha presented")
	})
	if err != context.Canceled {
		t.Fatal(err)
	}
}

func TestCaptchaHooksPrecedeUpstreamScripts(t *testing.T) {
	target, _ := url.Parse("https://id.vk.ru/captcha")
	for _, head := range []string{"<head>", `<HEAD data-test="1">`, "<head lang='ru'>"} {
		input := "<html>" + head + `<script src="//st.vk.ru/widget.js"></script></head><body><div id="spa_root"></div></body></html>`
		got := rewriteCaptchaHTML(input, target)
		injected := strings.Index(got, "window.__ftpCaptcha=")
		upstream := strings.Index(got, `<script src="/captcha-resource/https/st.vk.ru/widget.js"`)
		if injected < 0 || upstream < 0 || injected > upstream {
			t.Fatalf("hooks must be installed before upstream scripts for %s", head)
		}
		if !strings.Contains(got, `<div id="spa_root"></div>`) {
			t.Fatal("upstream widget root changed")
		}
		if !strings.Contains(got, "background:#0e0f14!important") {
			t.Fatal("captcha page must stay dark independently of app theme")
		}
	}
}

func TestProxyErrorCauseDoesNotExposeChallengeQuery(t *testing.T) {
	cause := context.DeadlineExceeded
	err := &url.Error{Op: "Get", URL: "https://id.vk.ru/captcha?token=private-demo", Err: cause}
	if got := proxyErrorCause(err); got != cause || strings.Contains(got.Error(), "private-demo") {
		t.Fatalf("unsafe error: %v", got)
	}
}

func TestRewriteProxyRedirectLocation(t *testing.T) {
	t.Parallel()

	targetURL, err := url.Parse("https://id.vk.ru/captcha")
	if err != nil {
		t.Fatalf("failed to parse target URL: %v", err)
	}

	testCases := []struct {
		name     string
		location string
		want     string
		ok       bool
	}{
		{
			name:     "keeps safe relative path",
			location: "/captcha?step=2",
			want:     "/captcha?step=2",
			ok:       true,
		},
		{
			name:     "rewrites same-origin absolute URL",
			location: "https://id.vk.ru/captcha?step=2",
			want:     "http://localhost:8765/captcha?step=2",
			ok:       true,
		},
		{
			name:     "blocks scheme-relative redirect",
			location: "//evil.example/captcha",
			ok:       false,
		},
		{
			name:     "blocks slash-backslash redirect",
			location: `/\evil.example/captcha`,
			ok:       false,
		},
		{
			name:     "blocks lookalike absolute host",
			location: "https://id.vk.ru.evil.example/captcha",
			ok:       false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := rewriteProxyRedirectLocation(tc.location, targetURL)
			if ok != tc.ok {
				t.Fatalf("rewriteProxyRedirectLocation() ok = %v, want %v", ok, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("rewriteProxyRedirectLocation() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsAllowedProxyHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		hostname string
		want     bool
	}{
		{"id.vk.ru", true},
		{"api.vk.com", true},
		{"vk.com", true},
		{"sun9-1.userapi.com", true},
		{"ads.vk.com", false},
		{"ads.vk.ru", false},
		{"top-fwz1.mail.ru", false},
		{"sdk-api.apptracer.ru", false},
		{"stats.vk-portal.net", false},
		{"evil.com", false},
	}

	for _, tc := range tests {
		t.Run(tc.hostname, func(t *testing.T) {
			t.Parallel()

			if got := isAllowedProxyHost(tc.hostname); got != tc.want {
				t.Fatalf("isAllowedProxyHost(%q) = %v, want %v", tc.hostname, got, tc.want)
			}
		})
	}
}
