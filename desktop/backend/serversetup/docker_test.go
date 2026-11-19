package serversetup

import (
	"strings"
	"testing"
)

const testContainer = `"abc123" "/free-turn-proxy" "ghcr.io/hackdiaz-dev/free-turn-proxy:latest" true` + "\nFT_DOCKER_OK\n"

func TestDockerDiscovery(t *testing.T) {
	cs, err := parseDockerInstances(`"abc" "/free-turn-proxy" "ghcr.io/hackdiaz-dev/free-turn-proxy:latest" true
"def" "/other-turn" "ghcr.io/hackdiaz-dev/free-turn-proxy:4.1.5" false
"ghi" "/db" "postgres:17" true
"jkl" "/free-turn-proxy-copy" "unknown:latest" true`)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 || !cs[0].Supported || cs[1].Supported || cs[1].Running {
		t.Fatalf("instances: %+v", cs)
	}
	if _, err := parseDockerInstances("permission denied"); err == nil {
		t.Fatal("malformed discovery accepted")
	}
}

func TestDockerFailureNeverPermitsInstall(t *testing.T) {
	for _, output := range []string{"", "permission denied", "cannot connect to Docker"} {
		t.Run(output, func(t *testing.T) {
			srv := newFakeSSHServer(t, func(_ string, stdin []byte) (string, int) {
				if !strings.Contains(string(stdin), "docker ps -aq") {
					t.Error("mutation attempted after failed discovery")
				}
				return output, 1
			})
			if _, err := Install(srv.sshConfig(), nil); err == nil {
				t.Fatal("install proceeded")
			}
		})
	}
}

func TestExistingDockerBlocksDeployment(t *testing.T) {
	srv := newFakeSSHServer(t, func(_ string, stdin []byte) (string, int) {
		if !strings.Contains(string(stdin), "docker ps -aq") {
			t.Error("deployment mutation attempted")
		}
		return testContainer, 0
	})
	cfg := srv.sshConfig()
	if _, err := Install(cfg, nil); err == nil {
		t.Fatal("install accepted")
	}
	if _, err := WgSetup(cfg, 51820, "127.0.0.1:9000", nil); err == nil {
		t.Fatal("WG setup accepted")
	}
	if err := Start(cfg, StartOptions{}, nil); err == nil {
		t.Fatal("legacy restart accepted")
	}
	if _, err := Uninstall(cfg, false, false, nil); err == nil {
		t.Fatal("legacy uninstall accepted")
	}
}

func TestAttachDockerOnlyCreatesDevice(t *testing.T) {
	created := false
	adds := 0
	srv := newFakeSSHServer(t, func(_ string, stdin []byte) (string, int) {
		s := string(stdin)
		if s == "" {
			return "", 0
		}
		if strings.Contains(s, "docker ps -aq") {
			return testContainer, 0
		}
		if strings.Contains(s, "systemctl") || strings.Contains(s, "docker restart") || strings.Contains(s, "docker run") || strings.Contains(s, " apply") {
			t.Error("server configuration changed during attach")
		}
		if !strings.Contains(s, "stat -c %u") || !strings.Contains(s, "grep -qx") {
			t.Error("missing controller ownership guard")
		}
		switch {
		case strings.Contains(s, "install.sh probe"):
			return `{"proto":3,"result":"ok","data":{"installed":true,"running":true,"method":"docker","config":{"backend":"new","listen_port":56952,"wg_port":51820}}}`, 0
		case strings.Contains(s, "install.sh client-conf"):
			if !created {
				return `{"proto":3,"result":"err","code":"not_found"}`, 1
			}
			return `{"proto":3,"result":"ok","data":{"link":"freeturn://test-device"}}`, 0
		case strings.Contains(s, "install.sh client-add"):
			created = true
			adds++
			return `{"proto":3,"result":"ok","data":{"link":"freeturn://test-device"}}`, 0
		default:
			t.Errorf("unexpected command: %s", s)
			return "", 1
		}
	})
	for i := 0; i < 2; i++ {
		cfg := srv.sshConfig()
		pin, err := exec(cfg, "true", "")
		if err != nil {
			t.Fatal(err)
		}
		cfg.HostFingerprint = pin.fingerprint
		link, err := AttachDocker(cfg, "abc123", "desktop-test")
		if err != nil || link != "freeturn://test-device" {
			t.Fatalf("attach: %q %v", link, err)
		}
	}
	if adds != 1 {
		t.Fatalf("retries created %d peers", adds)
	}
}

func TestDockerLifecycleNeverTouchesSystemd(t *testing.T) {
	srv := newFakeSSHServer(t, func(_ string, stdin []byte) (string, int) {
		s := string(stdin)
		if !strings.Contains(s, "docker restart free-turn-proxy") || strings.Contains(s, "systemctl") || strings.Contains(s, "install.sh restart") {
			t.Error("wrong restart target")
		}
		return "FT_DOCKER_OK\n", 0
	})
	cfg := srv.sshConfig()
	cfg.Manager = HackdiazDocker
	if err := Start(cfg, StartOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(cfg, false, false, nil); err == nil {
		t.Fatal("shared container uninstall accepted")
	}
}
