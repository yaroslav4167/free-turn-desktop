package manual

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCaptchaCapturePolicyCannotBeOverridden(t *testing.T) {
	for _, explicitHeader := range []bool{true, false} {
		response := httptest.NewRecorder()
		handler := denyCapture(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Permissions-Policy", "camera=*, microphone=*, display-capture=*")
			w.Header().Set("Feature-Policy", "camera *; microphone *")
			if explicitHeader {
				w.WriteHeader(http.StatusOK)
			}
			_, _ = w.Write([]byte("<audio src='/captcha-sound'></audio>"))
		}))
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://localhost:8765/not_robot_captcha?session_token=synthetic", nil))
		if response.Result().Header.Get("Permissions-Policy") != captchaPermissionsPolicy || response.Result().Header.Get("Feature-Policy") != captchaLegacyFeaturePolicy {
			t.Fatal("upstream response enabled media capture")
		}
		if response.Header().Get("Content-Security-Policy") != "" || response.Body.String() != "<audio src='/captcha-sound'></audio>" {
			t.Fatal("capture policy must not block captcha audio playback")
		}
	}
}
