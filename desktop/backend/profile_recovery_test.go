package backend

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samosvalishe/free-turn-proxy/desktop/backend/serversetup"
)

func testProfileStore(t *testing.T) *Store {
	t.Helper()
	return &Store{path: filepath.Join(t.TempDir(), "servers.json"), servers: []Server{}}
}

func TestFailedProfileWritesDoNotMutateMemoryOrDisk(t *testing.T) {
	s := testProfileStore(t)
	p, err := s.Save(DefaultServer())
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(s.path)
	if err := os.Remove(s.path + ".bak"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(s.path+".bak", 0o700); err != nil {
		t.Fatal(err)
	}
	p.Name = "must not persist"
	if _, err := s.Save(p); err == nil {
		t.Fatal("write should fail")
	}
	if got, _ := s.Get(p.ID); got.Name == p.Name {
		t.Fatal("failed save mutated memory")
	}
	if err := s.Delete(p.ID); err == nil {
		t.Fatal("delete should fail")
	}
	if len(s.List()) != 1 {
		t.Fatal("failed delete mutated memory")
	}
	after, _ := os.ReadFile(s.path)
	if !bytes.Equal(before, after) {
		t.Fatal("failed transaction changed primary file")
	}
}

func TestDamagedProfilesRecoverLastGoodBackupAndPreserveBytes(t *testing.T) {
	s := testProfileStore(t)
	p, err := s.Save(DefaultServer())
	if err != nil {
		t.Fatal(err)
	}
	p.Name = "second revision"
	if _, err := s.Save(p); err != nil {
		t.Fatal(err)
	}
	damaged := []byte(`{"truncated":`)
	if err := os.WriteFile(s.path, damaged, 0o600); err != nil {
		t.Fatal(err)
	}
	reopened := &Store{path: s.path}
	if err := reopened.load(); err != nil {
		t.Fatal(err)
	}
	if reopened.RecoveryNotice() != "recovered" || len(reopened.List()) != 1 || reopened.List()[0].Name != "" {
		t.Fatal("last good revision not recovered")
	}
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(s.path), "servers-damaged-*.json"))
	if len(files) != 1 {
		t.Fatal("damaged file lost")
	}
	got, _ := os.ReadFile(files[0])
	if !bytes.Equal(got, damaged) {
		t.Fatal("original damaged bytes changed")
	}
	if err := reopened.load(); err != nil {
		t.Fatal("recovery did not produce readable primary", err)
	}
}

func TestDamagedProfilesWithoutBackupAllowImportWithoutDiscardingOriginal(t *testing.T) {
	s := testProfileStore(t)
	damaged := []byte("null")
	if err := os.WriteFile(s.path, damaged, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.load(); err != nil {
		t.Fatal(err)
	}
	if s.RecoveryNotice() != "damaged" || len(s.List()) != 0 {
		t.Fatal("missing recovery warning")
	}
	got, _ := os.ReadFile(s.path)
	if !bytes.Equal(got, damaged) {
		t.Fatal("damaged primary overwritten")
	}
}

func TestProfileExportImportRoundtripAndValidation(t *testing.T) {
	s := testProfileStore(t)
	p := DefaultServer()
	p.Peer = "127.0.0.1:9001"
	p.Name = "roundtrip"
	p.ObfProfile = "none"
	p.VKLinks = []string{"SYNTHETIC-CODE"}
	p.SSH = &serversetup.SSHConfig{Password: "synthetic-secret"}
	saved, err := s.Save(p)
	if err != nil {
		t.Fatal(err)
	}
	data, err := s.ExportProfile(saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "synthetic-secret") {
		t.Fatal("SSH password exported")
	}
	imported, err := s.ImportProfiles(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(imported) != 1 || imported[0].ID == saved.ID || imported[0].Peer != p.Peer || imported[0].Name != p.Name {
		t.Fatal("roundtrip failed")
	}
	if len(s.List()) != 2 {
		t.Fatal("import overwrote original")
	}
	imported[0].VKLinks[0] = "must not mutate store"
	if got, _ := s.Get(imported[0].ID); got.VKLinks[0] != "SYNTHETIC-CODE" {
		t.Fatal("import result aliases stored data")
	}
	var archive profileArchive
	_ = json.Unmarshal(data, &archive)
	archive.Profiles = append(archive.Profiles, Server{})
	invalid, _ := json.Marshal(archive)
	if _, err := s.ImportProfiles(invalid); err == nil {
		t.Fatal("invalid second profile accepted")
	}
	if len(s.List()) != 2 {
		t.Fatal("partial import committed")
	}
	if _, err := s.ImportProfiles(make([]byte, MaxProfileImport+1)); err == nil {
		t.Fatal("unbounded archive")
	}
}

func TestRestoreBackupKeepsPreviousRevision(t *testing.T) {
	s := testProfileStore(t)
	p, err := s.Save(DefaultServer())
	if err != nil {
		t.Fatal(err)
	}
	p.Name = "new"
	if _, err = s.Save(p); err != nil {
		t.Fatal(err)
	}
	if err = s.RestoreBackup(); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(p.ID); got.Name != "" {
		t.Fatal("did not restore backup")
	}
}

func TestVPNProfileTransferPreservesKeysAndConnectionOptions(t *testing.T) {
	s := testProfileStore(t)
	p := ownerServer()
	p.ObfKey = strings.Repeat("ab", 32)
	p.WgClientConf = guestConf()
	p.ConnMode = ConnModeVPN
	p, err := s.Save(p)
	if err != nil {
		t.Fatal(err)
	}
	data, err := s.ExportProfile(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := s.ImportProfiles(data)
	if err != nil {
		t.Fatal(err)
	}
	got := profiles[0]
	if got.WgClientConf != p.WgClientConf || got.ObfKey != p.ObfKey || got.ConnMode != p.ConnMode || got.ManualCaptcha != p.ManualCaptcha || got.DNSMode != p.DNSMode || got.VKLinks[0] != p.VKLinks[0] {
		t.Fatal("portable connection options changed")
	}
}

func TestInvalidBackupDoesNotReplaceGoodProfiles(t *testing.T) {
	s := testProfileStore(t)
	if _, err := s.Save(DefaultServer()); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(s.path)
	if err := os.WriteFile(s.path+".bak", []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreBackup(); err == nil {
		t.Fatal("malformed backup accepted")
	}
	after, _ := os.ReadFile(s.path)
	if !bytes.Equal(before, after) || len(s.List()) != 1 {
		t.Fatal("failed restore damaged profiles")
	}
}
