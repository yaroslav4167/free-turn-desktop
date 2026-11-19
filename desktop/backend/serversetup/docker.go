package serversetup

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const HackdiazDocker = "hackdiaz-docker"

var desktopDeviceName = regexp.MustCompile(`^desktop-[a-zA-Z0-9._-]{1,24}$`)

type ContainerInstance struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Image     string `json:"image"`
	Running   bool   `json:"running"`
	Supported bool   `json:"supported"`
}

// Discovery is read-only. A failed Docker query must never look like an empty VPS.
const discoverDocker = `set -eu
if ! command -v docker >/dev/null 2>&1; then echo FT_DOCKER_OK; exit 0; fi
ids=$(docker ps -aq) || exit 1
[ -n "$ids" ] || { echo FT_DOCKER_OK; exit 0; }
docker inspect --format '{{json .Id}} {{json .Name}} {{json .Config.Image}} {{json .State.Running}}' $ids
echo FT_DOCKER_OK
`

func dockerInstances(cfg SSHConfig) ([]ContainerInstance, error) {
	r, err := exec(cfg, rootPrefix(cfg.RootMode)+"bash -s", scriptStdin(cfg, discoverDocker))
	if err != nil {
		return nil, err
	}
	out := strings.TrimSpace(r.output)
	if !strings.HasSuffix(out, "FT_DOCKER_OK") {
		return nil, errors.New("не удалось проверить Docker; установка отменена")
	}
	return parseDockerInstances(strings.TrimSuffix(out, "FT_DOCKER_OK"))
}

func parseDockerInstances(output string) ([]ContainerInstance, error) {
	instances := []ContainerInstance{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		d := json.NewDecoder(strings.NewReader(line))
		var c ContainerInstance
		if d.Decode(&c.ID) != nil || d.Decode(&c.Name) != nil || d.Decode(&c.Image) != nil || d.Decode(&c.Running) != nil {
			return nil, errors.New("неожиданный ответ Docker; установка отменена")
		}
		c.Name = strings.TrimPrefix(c.Name, "/")
		if c.Name != "free-turn-proxy" && !strings.HasPrefix(c.Image, "ghcr.io/hackdiaz-dev/free-turn-proxy:") && !strings.HasPrefix(c.Image, "ghcr.io/hackdiaz-dev/free-turn-proxy@sha256:") {
			continue
		}
		c.Supported = c.Name == "free-turn-proxy" && (strings.HasPrefix(c.Image, "ghcr.io/hackdiaz-dev/free-turn-proxy:") || strings.HasPrefix(c.Image, "ghcr.io/hackdiaz-dev/free-turn-proxy@sha256:"))
		instances = append(instances, c)
	}
	return instances, nil
}

// Use the root-owned controller already installed by hackdiaz, never download
// or run an arbitrary path from container metadata. Never call apply/update:
// those commands remove the other systemd/Docker installation.
const dockerGuard = `set -eu
unset FT_PREFIX FT_WG_DIR
for p in /opt /opt/free-turn-proxy /opt/free-turn-proxy/install.sh /opt/free-turn-proxy/install.conf; do
 [ ! -L "$p" ] && [ "$(stat -c %u "$p")" = 0 ] || exit 1
 [ -z "$(find "$p" -maxdepth 0 -perm /022 -print)" ] || exit 1
done
grep -qx 'INSTALL_METHOD="docker"' /opt/free-turn-proxy/install.conf || exit 1
image=$(docker inspect -f '{{.Config.Image}}' free-turn-proxy)
case "$image" in ghcr.io/hackdiaz-dev/free-turn-proxy:*|ghcr.io/hackdiaz-dev/free-turn-proxy@sha256:*) ;; *) exit 1;; esac
[ "$(docker inspect -f '{{.HostConfig.NetworkMode}}' free-turn-proxy)" = host ] || exit 1
docker inspect -f '{{range .Mounts}}{{if eq .Destination "/opt/free-turn-proxy/auth"}}{{.Source}}{{end}}{{end}}' free-turn-proxy | grep -qx /opt/free-turn-proxy/auth || exit 1
`

func dockerRPC(cfg SSHConfig, args ...string) (*ControlResponse, error) {
	if len(args) == 0 {
		return nil, errors.New("missing Docker command")
	}
	switch args[0] {
	case "probe", "client-list", "client-add", "client-conf", "client-remove", "logs":
	default:
		return nil, errors.New("операция запрещена для существующего контейнера")
	}
	quoted := []string{}
	for _, a := range args {
		quoted = append(quoted, shellQuote(a))
	}
	payload := dockerGuard + "\nexec bash /opt/free-turn-proxy/install.sh " + strings.Join(quoted, " ") + "\n"
	r, err := exec(cfg, rootPrefix(cfg.RootMode)+"bash -s", scriptStdin(cfg, payload))
	if err != nil {
		return nil, err
	}
	resp, err := parseEnvelope(r.output)
	if err != nil {
		return nil, errors.New("контейнер не подтверждён: требуется штатный контроллер hackdiaz с root-owned install.conf/install.sh")
	}
	if resp.Proto != 3 {
		return nil, errors.New("неподдерживаемая версия контроллера контейнера")
	}
	if !resp.IsOK() {
		return nil, controlErr(resp)
	}
	return resp, nil
}

func dockerLifecycle(cfg SSHConfig, action string) error {
	if action != "restart" && action != "stop" {
		return errors.New("unsupported lifecycle action")
	}
	// Operate on the selected container only. The upstream restart/stop command
	// reads install.conf's method and could target an unrelated systemd service.
	payload := dockerGuard + "\ndocker " + action + " free-turn-proxy >/dev/null\n"
	if action == "restart" {
		payload += "sleep 2\n[ \"$(docker inspect -f '{{.State.Running}}' free-turn-proxy)\" = true ]\n"
	}
	r, err := exec(cfg, rootPrefix(cfg.RootMode)+"bash -s", scriptStdin(cfg, payload+"echo FT_DOCKER_OK\n"))
	if err != nil {
		return err
	}
	if !strings.HasSuffix(strings.TrimSpace(r.output), "FT_DOCKER_OK") {
		return fmt.Errorf("Docker %s failed", action)
	}
	return nil
}

type dockerProbeData struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	Method    string `json:"method"`
	Version   string `json:"version"`
	Config    struct {
		Backend    string `json:"backend"`
		ListenPort int    `json:"listen_port"`
		WgPort     int    `json:"wg_port"`
	} `json:"config"`
}

func probeDocker(cfg SSHConfig) (*ProbeResult, error) {
	r, err := dockerRPC(cfg, "probe")
	if err != nil {
		return nil, err
	}
	var d dockerProbeData
	if err := json.Unmarshal(r.Data, &d); err != nil {
		return nil, err
	}
	if !d.Installed || d.Method != "docker" || d.Config.Backend != "new" || d.Config.ListenPort < 1 || d.Config.ListenPort > 65535 {
		return nil, errors.New("конфигурация контейнера не поддерживает создание VPN-профиля; импортируйте готовую ссылку")
	}
	return &ProbeResult{Installed: true, Running: d.Running, Version: d.Version, ExistingService: "docker:free-turn-proxy", WgPort: d.Config.WgPort}, nil
}

// DockerClient returns the official link with a distinct key/IP/Client ID.
// Reissuing the same name after a lost SSH response does not create another peer.
func DockerClient(cfg SSHConfig, name string) (string, error) {
	if !desktopDeviceName.MatchString(name) {
		return "", errors.New("имя устройства должно начинаться с desktop- и содержать только латиницу, цифры, . _ - (до 32 символов)")
	}
	if _, err := probeDocker(cfg); err != nil {
		return "", err
	}
	r, err := dockerRPC(cfg, "client-conf", "--name="+name)
	if err == nil {
		return dockerClientLink(r)
	}
	if !strings.Contains(err.Error(), "not_found:") {
		return "", err
	}
	r, err = dockerRPC(cfg, "client-add", "--name="+name)
	if err != nil && strings.Contains(err.Error(), "exists:") {
		r, err = dockerRPC(cfg, "client-conf", "--name="+name)
	}
	if err != nil {
		return "", err
	}
	return dockerClientLink(r)
}

func dockerClientLink(r *ControlResponse) (string, error) {
	var d struct {
		Link string `json:"link"`
	}
	if err := json.Unmarshal(r.Data, &d); err != nil {
		return "", err
	}
	if !strings.HasPrefix(d.Link, "freeturn://") {
		return "", errors.New("контейнер не вернул ссылку клиента")
	}
	return d.Link, nil
}

func refuseDockerDeployment(cfg SSHConfig) error {
	cs, err := dockerInstances(cfg)
	if err != nil {
		return err
	}
	if len(cs) > 0 {
		return errors.New("найден существующий контейнер FreeTurn: подключитесь к нему вместо повторной установки")
	}
	return nil
}

func AttachDocker(cfg SSHConfig, id, name string) (string, error) {
	if cfg.HostFingerprint == "" {
		return "", errors.New("сначала подтвердите отпечаток SSH-сервера через проверку VPS")
	}
	cs, err := dockerInstances(cfg)
	if err != nil {
		return "", err
	}
	for _, c := range cs {
		if c.ID == id && c.Supported {
			if !c.Running {
				return "", errors.New("контейнер остановлен: запустите его через исходный установщик и повторите")
			}
			return DockerClient(cfg, name)
		}
	}
	return "", errors.New("выбранный контейнер изменился или не поддерживается; повторите проверку VPS")
}
