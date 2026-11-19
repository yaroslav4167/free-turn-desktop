package serversetup

import (
	"strings"
	"testing"
)

func TestProbeExposesExistingInstallation(t *testing.T) {
	shrinkTimings(t)
	srv := newFakeSSHServer(t, func(_ string, stdin []byte) (string, int) {
		if strings.Contains(string(stdin), "echo FT_DOCKER_OK") {
			return "FT_DOCKER_OK\n", 0
		}
		return `{"proto":2,"result":"ok","data":{"installed":true,"running":true,"existing_service":"freeturn-server.service","version":"v1.2.3","wg":{"port":51820}}}`, 0
	})
	probe, err := Probe(srv.sshConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !probe.Installed || !probe.Running || probe.ExistingService != "freeturn-server.service" || probe.Version != "v1.2.3" || probe.WgPort != 51820 {
		t.Fatalf("unexpected probe: %+v", probe)
	}
}
