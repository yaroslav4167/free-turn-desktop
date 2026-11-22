package backend

import (
	"encoding/json"
	"net"
	"runtime"
	"time"
)

// An allowlist rather than regex redaction: raw logs, errors, profile names,
// addresses, URLs, credentials, file paths and environment are never included.
type DiagnosticEvent struct {
	At      string `json:"at"`
	State   string `json:"state"`
	Streams int    `json:"streams"`
	Total   int    `json:"total"`
}

func safeState(state string) string {
	switch state {
	case "idle", "error", "connecting", "connected", "captcha":
		return state
	}
	return "unknown"
}

func (e *Engine) DiagnosticSnapshot() (Status, []DiagnosticEvent) {
	status := e.Status()
	status.ServerID, status.CaptchaURL, status.ErrMsg = "", "", ""
	status.State = safeState(status.State)
	e.mu.Lock()
	defer e.mu.Unlock()
	return status, append([]DiagnosticEvent(nil), e.diagnosticEvents...)
}

func Diagnostics(version string, helper bool, profiles []Server, status Status, events []DiagnosticEvent) ([]byte, error) {
	interfaces, err := net.Interfaces()
	up, nonLoopback := 0, 0
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp != 0 {
			up++
			if iface.Flags&net.FlagLoopback == 0 {
				nonLoopback++
			}
		}
	}
	vpn, routes, dnsAuto, dnsPlain, dnsDoH := 0, 0, 0, 0, 0
	for _, p := range profiles {
		if p.ConnMode == ConnModeVPN {
			vpn++
		}
		for _, list := range p.RoutingLists {
			if list.Enabled {
				routes++
			}
		}
		switch p.DNSMode {
		case "plain":
			dnsPlain++
		case "doh":
			dnsDoH++
		default:
			dnsAuto++
		}
	}
	return json.MarshalIndent(struct {
		Format              string            `json:"format"`
		Created             string            `json:"created"`
		Version             string            `json:"version"`
		OS                  string            `json:"os"`
		Arch                string            `json:"arch"`
		Go                  string            `json:"go"`
		Helper              bool              `json:"vpnHelperInstalled"`
		InterfacesKnown     bool              `json:"interfacesKnown"`
		InterfacesUp        int               `json:"interfacesUp"`
		NetworkInterfacesUp int               `json:"nonLoopbackInterfacesUp"`
		Profiles            int               `json:"profiles"`
		VPNProfiles         int               `json:"vpnProfiles"`
		EnabledRoutingLists int               `json:"enabledRoutingLists"`
		DNSModes            [3]int            `json:"dnsAutoPlainDoH"`
		State               string            `json:"state"`
		Streams             int               `json:"streams"`
		Total               int               `json:"total"`
		TxTotal             int64             `json:"txTotal"`
		RxTotal             int64             `json:"rxTotal"`
		Events              []DiagnosticEvent `json:"recentStates"`
		Privacy             string            `json:"privacy"`
	}{
		Format: "freeturn-diagnostics-v1", Created: time.Now().UTC().Format(time.RFC3339),
		Version: version, OS: runtime.GOOS, Arch: runtime.GOARCH, Go: runtime.Version(),
		Helper: helper, InterfacesKnown: err == nil, InterfacesUp: up, NetworkInterfacesUp: nonLoopback,
		Profiles: len(profiles), VPNProfiles: vpn, EnabledRoutingLists: routes, DNSModes: [3]int{dnsAuto, dnsPlain, dnsDoH},
		State: safeState(status.State), Streams: status.Streams, Total: status.Total, TxTotal: status.TxTotal, RxTotal: status.RxTotal,
		Events: events, Privacy: "No keys, endpoints, names, paths, captcha URLs or raw logs. Local snapshot only; no live VPN/leak test performed.",
	}, "", "  ")
}
