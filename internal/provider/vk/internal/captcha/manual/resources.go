package manual

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

const resourceProxyPrefix = "/captcha-resource/"

// Preserve the upstream directory: webpack publicPath, module imports and CSS
// url(...) resolve against the resource URL, not the captcha document URL.
func resourceProxyURL(target *url.URL) string {
	path := target.EscapedPath()
	if path == "" {
		path = "/"
	}
	result := resourceProxyPrefix + target.Scheme + "/" + target.Host + path
	if target.RawQuery != "" {
		result += "?" + target.RawQuery
	}
	if target.Fragment != "" {
		result += "#" + target.EscapedFragment()
	}
	return result
}

func resourceProxyTarget(local *url.URL) (*url.URL, error) {
	var target *url.URL
	var err error
	if local.Path == "/generic_proxy" {
		// Keep older tabs/API requests compatible, but new resource links use
		// a path namespace rather than hiding the whole URL inside a query.
		target, err = url.Parse(local.Query().Get("proxy_url"))
	} else {
		raw := strings.TrimPrefix(local.EscapedPath(), resourceProxyPrefix)
		parts := strings.SplitN(raw, "/", 3)
		if !strings.HasPrefix(local.EscapedPath(), resourceProxyPrefix) || len(parts) != 3 {
			return nil, fmt.Errorf("invalid captcha resource path")
		}
		target, err = url.Parse(parts[0] + "://" + parts[1] + "/" + parts[2])
		if err == nil {
			target.RawQuery = local.RawQuery
		}
	}
	if err != nil || target == nil || target.Host == "" || target.User != nil || (target.Scheme != "https" && target.Scheme != "http") {
		return nil, fmt.Errorf("invalid captcha resource URL")
	}
	return target, nil
}

func optionalAdLoader(target *url.URL) bool {
	return strings.EqualFold(target.Hostname(), "ad.mail.ru") && target.Path == "/static/sync-loader.js"
}

func captchaResourceHandler(transport http.RoundTripper, keyCh chan<- string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target, err := resourceProxyTarget(r.URL)
		if err != nil {
			http.Error(w, "Bad resource URL", http.StatusBadRequest)
			return
		}
		if optionalAdLoader(target) && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			// Do not proxy advertising or return a text/plain error as a script.
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Cache-Control", "no-store")
			if r.Method == http.MethodGet {
				_, _ = io.WriteString(w, "/* Optional advertising loader is not proxied. */")
			}
			return
		}
		if !isAllowedProxyHost(strings.ToLower(target.Hostname())) {
			http.Error(w, "Forbidden host", http.StatusForbidden)
			return
		}
		proxy := &httputil.ReverseProxy{
			Transport: transport, ErrorHandler: captchaProxyError,
			Rewrite: func(req *httputil.ProxyRequest) {
				req.Out.URL.Path = target.Path
				req.Out.URL.RawPath = target.RawPath
				req.Out.URL.RawQuery = target.RawQuery
				rewriteProxyRequest(req.Out, target)
			},
			ModifyResponse: func(res *http.Response) error {
				for _, header := range []string{
					"Content-Security-Policy", "Content-Security-Policy-Report-Only",
					"X-Content-Security-Policy", "X-WebKit-CSP", "Cross-Origin-Opener-Policy",
					"Cross-Origin-Embedder-Policy", "Cross-Origin-Resource-Policy",
					"X-Frame-Options", "Strict-Transport-Security",
				} {
					res.Header.Del(header)
				}
				// Keep Content-Type and nosniff intact: HTML is never relabelled
				// as JS/CSS merely to suppress the browser's security check.
				res.Header.Set("Access-Control-Allow-Origin", "*")
				if res.StatusCode >= 300 && res.StatusCode < 400 {
					if location := res.Header.Get("Location"); location != "" {
						next, err := url.Parse(location)
						if err != nil {
							return fmt.Errorf("invalid captcha resource redirect")
						}
						next = target.ResolveReference(next)
						if next.User != nil || (next.Scheme != "https" && next.Scheme != "http") || !isAllowedProxyHost(strings.ToLower(next.Hostname())) {
							return fmt.Errorf("forbidden captcha resource redirect")
						}
						res.Header.Set("Location", resourceProxyURL(next))
					}
				}
				if strings.Contains(target.Path, "captchaNotRobot.check") {
					body, err := io.ReadAll(res.Body)
					if err != nil {
						return err
					}
					_ = res.Body.Close()
					res.Body = io.NopCloser(bytes.NewReader(body))
					res.ContentLength = int64(len(body))
					res.Header.Set("Content-Length", fmt.Sprint(len(body)))
					notifyKey(keyCh, extractSuccessToken(body))
				}
				return nil
			},
		}
		proxy.ServeHTTP(w, r)
	})
}
