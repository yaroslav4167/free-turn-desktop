package updates

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestUnifiedReleaseMigrationSelectsNativeArchive(t *testing.T) {
	const repo = "yaroslav4167/free-turn-desktop"
	const tag = "v3.0.0"
	for _, test := range []struct{ platform, arch, bridge, asset string }{
		{"linux", "amd64", "v2.6.6", "FreeTurn-Linux-amd64-v3.0.0.tar.gz"},
		{"windows", "amd64", "v1.1.4", "FreeTurn-Windows-amd64-v3.0.0.zip"},
		{"darwin", "arm64", "v0.2.3", "FreeTurn-macOS-arm64-v3.0.0.zip"},
		{"darwin", "amd64", "v0.2.3", "FreeTurn-macOS-amd64-v3.0.0.zip"},
	} {
		t.Run(test.platform+"-"+test.arch, func(t *testing.T) {
			m := NewForPlatform(test.bridge, test.arch, repo, test.platform)
			m.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != "https://api.github.com/repos/"+repo+"/releases/latest" {
					t.Fatalf("wrong repository: %s", r.URL)
				}
				assets := []asset{}
				for _, name := range []string{"FreeTurn-Linux-amd64-v3.0.0.tar.gz", "FreeTurn-Windows-amd64-v3.0.0.zip", "FreeTurn-macOS-arm64-v3.0.0.zip", "FreeTurn-macOS-amd64-v3.0.0.zip", "checksums.txt"} {
					assets = append(assets, asset{Name: name, URL: "https://github.com/" + repo + "/releases/download/" + tag + "/" + name, Size: 1024})
				}
				data, _ := json.Marshal(release{Tag: tag, Assets: assets})
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(data)))}, nil
			})
			if err := m.Check(context.Background()); err != nil {
				t.Fatal(err)
			}
			if got := m.State(); !got.Available || got.AssetName != test.asset || got.Repository != repo {
				t.Fatalf("wrong migration: %+v", got)
			}
			if m.trustedAsset("https://github.com/yaroslav4167/free-turn-linux/releases/download/v3.0.0/" + test.asset) {
				t.Fatal("cross-repository asset accepted")
			}
		})
	}
}

func TestUnifiedVersionOutranksEveryBridge(t *testing.T) {
	for _, bridge := range []string{"v2.6.6", "v1.1.4", "v0.2.3"} {
		if !newer("v3.0.0", bridge) || newer(bridge, "v3.0.0") {
			t.Fatalf("unsafe migration ordering: %s", bridge)
		}
	}
}
