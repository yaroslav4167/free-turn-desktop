package manual

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type resourceTransportFunc func(*http.Request) (*http.Response, error)

func (f resourceTransportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCaptchaResourceDirectoryAndMIME(t *testing.T) {
	t.Parallel()
	entry, _ := url.Parse("https://st.vk.ru/vkid/vkid-bff/dist/projects/notRobotCaptcha-web/entrypoints/entry.demo.js")
	local, _ := url.Parse("http://localhost:8765" + resourceProxyURL(entry))
	for _, path := range []string{"../chunks/js/394.demo.js?cache-bust=true&retry-attempt=1", "../chunks/css/iui_subapp.demo.css", "../chunks/fonts/demo.woff2"} {
		t.Run(path, func(t *testing.T) {
			relative, _ := url.Parse(path)
			requestURL := local.ResolveReference(relative)
			expected := entry.ResolveReference(relative)
			mime := "application/javascript"
			if strings.Contains(path, ".css") {
				mime = "text/css"
			} else if strings.Contains(path, ".woff2") {
				mime = "font/woff2"
			}
			calls := 0
			handler := captchaResourceHandler(resourceTransportFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.String() != expected.String() || r.Host != "st.vk.ru" {
					t.Fatalf("wrong origin/directory/query: %s, want %s", r.URL, expected)
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {mime}, "X-Content-Type-Options": {"nosniff"}}, Body: io.NopCloser(strings.NewReader("demo resource")), ContentLength: 13}, nil
			}), nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, requestURL.String(), nil))
			if calls != 1 || response.Code != 200 || response.Header().Get("Content-Type") != mime || response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Body.String() != "demo resource" {
				t.Fatalf("resource response: %d, %v, %q", response.Code, response.Header(), response.Body.String())
			}
		})
	}
}

func TestCaptchaResourceAllowlistAndOptionalLoader(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		path   string
		status int
	}{
		{"/captcha-resource/https/ad.mail.ru/static/sync-loader.js", 200},
		{"/generic_proxy?proxy_url=https%3A%2F%2Fad.mail.ru%2Fstatic%2Fsync-loader.js", 200},
		{"/captcha-resource/https/ad.mail.ru/anything-else.js", 403},
		{"/captcha-resource/https/ads.vk.ru/blocked.js", 403},
		{"/captcha-resource/https/st.vk.ru.evil.test/blocked.js", 403},
		{"/captcha-resource/http/127.0.0.1/private", 403},
		{"/captcha-resource/file/st.vk.ru/private", 400},
		{"/captcha-resource/https/user@st.vk.ru/private", 400},
		{"/captcha-resource/https/st.vk.ru%2Fevil.test/private", 400},
	} {
		response := httptest.NewRecorder()
		handler := captchaResourceHandler(resourceTransportFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("blocked/optional resource contacted upstream")
			return nil, nil
		}), nil)
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		if response.Code != test.status {
			t.Fatalf("%s: status %d, want %d", test.path, response.Code, test.status)
		}
		if test.status == 200 && (!strings.HasPrefix(response.Header().Get("Content-Type"), "application/javascript") || response.Header().Get("X-Content-Type-Options") != "nosniff") {
			t.Fatal("optional loader stub must be inert JavaScript, not a MIME error")
		}
	}
}

func TestCaptchaResourceRedirectAndTokenCompatibility(t *testing.T) {
	t.Parallel()
	for _, location := range []string{"../chunks/js/demo.js", "https://st.vk.com/assets/demo.js", "https://evil.test/demo.js"} {
		handler := captchaResourceHandler(resourceTransportFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": {location}}, Body: io.NopCloser(strings.NewReader(""))}, nil
		}), nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/captcha-resource/https/st.vk.ru/project/entrypoints/entry.js", nil))
		if strings.Contains(location, "evil.test") {
			if response.Code != http.StatusBadGateway {
				t.Fatal("forbidden redirect forwarded")
			}
			continue
		}
		upstream, _ := url.Parse("https://st.vk.ru/project/entrypoints/entry.js")
		relative, _ := url.Parse(location)
		if response.Header().Get("Location") != resourceProxyURL(upstream.ResolveReference(relative)) {
			t.Fatal("redirect lost resource origin/path")
		}
	}
	for _, path := range []string{"/captcha-resource/https/api.vk.ru/method/captchaNotRobot.check", "/generic_proxy?proxy_url=https%3A%2F%2Fapi.vk.ru%2Fmethod%2FcaptchaNotRobot.check"} {
		keys := make(chan string, 1)
		handler := captchaResourceHandler(resourceTransportFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"response":{"success_token":"synthetic-token"}}`))}, nil
		}), keys)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, strings.NewReader("demo=1")))
		select {
		case token := <-keys:
			if token != "synthetic-token" {
				t.Fatal("wrong token")
			}
		default:
			t.Fatal("success token not captured")
		}
	}
}

func TestCaptchaResourceRefererRestoresActualCDNPath(t *testing.T) {
	t.Parallel()
	target, _ := url.Parse("https://st.vk.ru/project/chunks/js/demo.js")
	input := "http://localhost:8765/captcha-resource/https/st.vk.ru/project/entrypoints/entry.js"
	if got := rewriteProxyHeaderURL(input, target); got != "https://st.vk.ru/project/entrypoints/entry.js" {
		t.Fatalf("incorrect CDN referer: %s", got)
	}
}

func TestCaptchaResourceDoesNotRelabelHTMLAsScript(t *testing.T) {
	t.Parallel()
	handler := captchaResourceHandler(resourceTransportFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/html"}, "X-Content-Type-Options": {"nosniff"}}, Body: io.NopCloser(strings.NewReader("<html>upstream error</html>"))}, nil
	}), nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/captcha-resource/https/st.vk.ru/project/chunks/js/demo.js", nil))
	if response.Header().Get("Content-Type") != "text/html" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("MIME protection was bypassed")
	}
}
