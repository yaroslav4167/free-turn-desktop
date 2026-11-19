package serversetup

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Immutable upstream commit, with independently pinned script bytes. Never run
// curl|bash or the moving master/latest installer on a VPS.
const dockerInstallerURL = "https://raw.githubusercontent.com/hackdiaz-dev/free-turn-proxy/11f81346cb398b157975107872aad5d6a4143149/scripts/install.sh"
const dockerInstallerSHA = "e17725c29237d344c1607d1391c91d977521834508640dc158f9b21c06414916"

func verifyDockerInstaller(script []byte) error {
	sum := sha256.Sum256(script)
	if hex.EncodeToString(sum[:]) != dockerInstallerSHA {
		return errors.New("контрольная сумма установщика не совпала; VPS не изменён")
	}
	return nil
}

// CreateDocker installs the shared Android-compatible runtime only on an empty
// host. No automatic fallback to a second systemd server is permitted.
func CreateDocker(cfg SSHConfig, listenPort, wgPort int, obfProfile, obfKey string, progress func(string)) error {
	if cfg.HostFingerprint == "" {
		return errors.New("сначала подтвердите отпечаток SSH-сервера")
	}
	probe, err := Probe(cfg)
	if err != nil {
		return err
	}
	if probe.Installed || probe.Running {
		return errors.New("установка уже существует; повторите проверку и подключитесь к ней")
	}
	if listenPort < 1 || listenPort > 65535 || wgPort < 1 || wgPort > 65535 || listenPort == wgPort {
		return errors.New("некорректные порты")
	}
	client := &http.Client{Timeout: 45 * time.Second}
	if progress != nil {
		progress("Downloading verified FreeTurn Docker installer v4.2.1")
	}
	r, err := client.Get(dockerInstallerURL)
	if err != nil {
		return fmt.Errorf("скачивание установщика: %w", err)
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("скачивание установщика: HTTP %d", r.StatusCode)
	}
	script, err := io.ReadAll(io.LimitReader(r.Body, 256<<10))
	if err != nil {
		return err
	}
	if err := verifyDockerInstaller(script); err != nil {
		return err
	}
	args := []string{"apply", "--method=docker", "--backend=new", "--host=" + cfg.IP,
		fmt.Sprintf("--listen-port=%d", listenPort), fmt.Sprintf("--wg-port=%d", wgPort),
		"--mode=udp", "--obf-profile=" + obfProfile, "--obf-key=" + obfKey, "--version=v4.2.1"}
	for i := range args {
		args[i] = shellQuote(args[i])
	}
	// Recheck on the VPS immediately before installation. Preserve any existing
	// layout, even an incomplete/unknown installation, rather than let upstream's
	// ft_remove delete it. Docker failures are not absence.
	payload := `set -euo pipefail
unset FT_PREFIX FT_WG_DIR
[ ! -e /opt/free-turn-proxy ] || exit 1
for unit in free-turn-proxy.service freeturn-proxy.service freeturn-server.service free-turn-server.service; do
 state=$(systemctl show "$unit" -p LoadState --value 2>/dev/null || true)
 case "$state" in ""|not-found) ;; *) exit 1;; esac
done
(
` + discoverDocker + `
)
if command -v docker >/dev/null 2>&1; then
 if docker ps -a --format '{{.Names}} {{.Image}}' | grep -E '(^free-turn-proxy |ghcr.io/hackdiaz-dev/free-turn-proxy[:@])'; then exit 1; fi
fi
umask 077
stage=$(mktemp /tmp/freeturn-desktop-install.XXXXXX)
trap 'rm -f "$stage"' EXIT
base64 -d > "$stage" <<'FT_INSTALLER'
` + base64.StdEncoding.EncodeToString(script) + "\nFT_INSTALLER\nbash \"$stage\" " + strings.Join(args, " ") + "\n"
	if progress != nil {
		progress("Installing shared Docker server; this can take several minutes")
	}
	res, err := exec(cfg, rootPrefix(cfg.RootMode)+"bash -s", scriptStdin(cfg, payload))
	if err != nil {
		return err
	}
	resp, err := parseEnvelope(res.output)
	if err != nil {
		return errors.New("установка не подтверждена; повторите проверку VPS перед новой попыткой")
	}
	if !resp.IsOK() {
		return controlErr(resp)
	}
	_, err = probeDocker(cfg)
	return err
}
