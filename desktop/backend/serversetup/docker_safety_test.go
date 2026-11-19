package serversetup

import (
	"strings"
	"testing"
)

func TestDockerInstallerRejectsModifiedBytes(t *testing.T) {
	for _, script := range [][]byte{nil, []byte("#!/bin/bash\necho untrusted\n")} {
		if verifyDockerInstaller(script) == nil {
			t.Fatal("unverified installer accepted")
		}
	}
}

func TestCreateDockerRefusesExistingBeforeDownloading(t *testing.T) {
	srv := newFakeSSHServer(t, func(_ string, stdin []byte) (string, int) {
		s := string(stdin)
		if s == "" {
			return "", 0
		}
		if strings.Contains(s, "docker ps -aq") {
			return testContainer, 0
		}
		if !strings.Contains(s, "cmd_probe()") {
			t.Error("unexpected mutation")
		}
		return `{"proto":2,"result":"ok","data":{"installed":false}}`, 0
	})
	cfg := srv.sshConfig()
	pin, err := exec(cfg, "true", "")
	if err != nil {
		t.Fatal(err)
	}
	cfg.HostFingerprint = pin.fingerprint
	err = CreateDocker(cfg, 56000, 51820, "rtpopus3", "", nil)
	if err == nil || !strings.Contains(err.Error(), "установка уже существует") {
		t.Fatalf("existing container not protected: %v", err)
	}
}

func TestDockerPeerManagementUsesExactKey(t *testing.T) {
	srv := newFakeSSHServer(t, func(_ string, stdin []byte) (string, int) {
		s := string(stdin)
		if strings.Contains(s, "install.sh client-list") {
			return `{"proto":3,"result":"ok","data":{"clients":[{"pub":"pub-a","name":"owner","self":true,"ip":"10.13.13.2"},{"pub":"pub-b","name":"desktop-b","ip":"10.13.13.2"}],"share":{"mode":"udp","obf_profile":"rtpopus3","obf_key":"test"}}}`, 0
		}
		if !strings.Contains(s, "install.sh client-conf --name=desktop-b") {
			t.Error("wrong peer selected")
		}
		return `{"proto":3,"result":"ok","data":{"client":{"pub":"pub-b","client_id":"test-id"},"conf_b64":"W0ludGVyZmFjZV0K"}}`, 0
	})
	cfg := srv.sshConfig()
	cfg.Manager = HackdiazDocker
	issued, err := PeerConf(cfg, "pub-b", "", "")
	if err != nil || issued.PublicKey != "pub-b" {
		t.Fatalf("peer: %+v %v", issued, err)
	}
	if err := PeerRemove(cfg, "pub-a"); err == nil {
		t.Fatal("owner deletion allowed")
	}
	if err := PeerRemove(cfg, "unknown-key"); err == nil {
		t.Fatal("unmatched peer deletion allowed")
	}
}
