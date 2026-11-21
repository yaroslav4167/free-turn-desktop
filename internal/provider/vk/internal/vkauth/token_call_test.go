package vkauth

import (
	"context"
	"errors"
	"net"
	"net/url"
	"testing"

	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/samosvalishe/free-turn-proxy/internal/provider/vk/internal/browserprofile"
	"github.com/samosvalishe/free-turn-proxy/internal/provider/vk/internal/captcha"
)

func testCaptchaResponse(sid string) map[string]any {
	return map[string]any{"error": map[string]any{
		"error_code": float64(14), "error_msg": "Captcha required", "captcha_sid": sid,
		"redirect_uri": "https://id.vk.ru/captcha?session_token=demo-" + sid,
	}}
}

func TestCallTokenCaptchaRecheck(t *testing.T) {
	for _, scenario := range []string{"not-needed", "transient", "fresh-challenge", "persistent", "cancelled", "new-request-error"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls, solves := 0, 0
			client := New(Config{ManualOnly: true, FingerprintSeed: "recheck-demo"})
			client.manualSolve = func(_ context.Context, challenge *captcha.Error, _ net.Dialer, _ browserprofile.Profile) (string, error) {
				solves++
				if challenge.CaptchaSid != "fresh" {
					t.Fatalf("obsolete challenge presented: %s", challenge.CaptchaSid)
				}
				return "demo-solution", nil
			}
			client.requestFn = func(_ context.Context, _ tlsclient.HttpClient, _ browserprofile.Profile, data, _ string) (map[string]any, error) {
				calls++
				if calls > 3 {
					t.Fatal("unbounded captcha rechecks")
				}
				if scenario == "not-needed" || (scenario == "transient" && calls == 2) {
					return map[string]any{"response": map[string]any{"token": "demo-call-token"}}, nil
				}
				if calls == 1 {
					if scenario == "cancelled" {
						cancel()
					}
					return testCaptchaResponse("obsolete"), nil
				}
				if scenario == "new-request-error" {
					return nil, errors.New("demo network failure")
				}
				form, _ := url.ParseQuery(data)
				if calls == 2 {
					if form.Get("captcha_sid") != "" || form.Get("success_token") != "" {
						t.Fatal("ordinary recheck contains stale captcha data")
					}
					return testCaptchaResponse("fresh"), nil
				}
				if form.Get("captcha_sid") != "fresh" || form.Get("success_token") != "demo-solution" {
					t.Fatalf("solution applied to wrong request: %v", form)
				}
				if scenario == "persistent" {
					return testCaptchaResponse("still-needed"), nil
				}
				return map[string]any{"response": map[string]any{"token": "demo-call-token"}}, nil
			}
			token, err := client.fetchCallToken(ctx, nil, client.currentPersona(), 1, "DEMO", "name", "demo-access", DefaultCredentials[0])
			switch scenario {
			case "not-needed", "transient", "fresh-challenge":
				if err != nil || token != "demo-call-token" {
					t.Fatalf("token %q, error %v", token, err)
				}
			case "cancelled":
				if !errors.Is(err, context.Canceled) || calls != 1 {
					t.Fatalf("cancelled attempt continued: calls %d, %v", calls, err)
				}
			default:
				if err == nil {
					t.Fatal("challenge/network failure ignored")
				}
			}
			wantSolves := 0
			if scenario == "fresh-challenge" || scenario == "persistent" {
				wantSolves = 1
			}
			if solves != wantSolves {
				t.Fatalf("solver calls %d, want %d", solves, wantSolves)
			}
			wantCalls := map[string]int{"not-needed": 1, "transient": 2, "fresh-challenge": 3, "persistent": 3, "cancelled": 1, "new-request-error": 2}[scenario]
			if calls != wantCalls {
				t.Fatalf("request count %d, want %d", calls, wantCalls)
			}
			if (scenario == "not-needed" || scenario == "transient") && (client.captchaAttempt != 0 || client.identity.Gen != 0 || client.lockout.Load() != 0) {
				t.Fatal("unnecessary captcha changed solve ladder, persona or lockout")
			}
		})
	}
}
