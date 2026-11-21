package manual

import "net/http"

// Captcha needs neither capture devices nor screen sharing. Do not use
// media-src 'none': playback of the audio challenge must remain available.
const captchaPermissionsPolicy = "camera=(), microphone=(), display-capture=()"
const captchaLegacyFeaturePolicy = "camera 'none'; microphone 'none'; display-capture 'none'"

type capturePolicyWriter struct{ http.ResponseWriter }

func (w capturePolicyWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w capturePolicyWriter) denyCapture() {
	w.Header().Set("Permissions-Policy", captchaPermissionsPolicy)
	w.Header().Set("Feature-Policy", captchaLegacyFeaturePolicy)
}

func (w capturePolicyWriter) WriteHeader(status int) {
	// Apply after ReverseProxy has copied upstream headers. VK/redirect/error
	// responses cannot override the local no-capture policy.
	w.denyCapture()
	w.ResponseWriter.WriteHeader(status)
}

func (w capturePolicyWriter) Write(data []byte) (int, error) {
	w.denyCapture()
	return w.ResponseWriter.Write(data)
}

func denyCapture(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := capturePolicyWriter{w}
		writer.denyCapture()
		handler.ServeHTTP(writer, r)
	})
}
