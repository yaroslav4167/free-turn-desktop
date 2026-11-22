package tray

import (
	"fmt"

	"github.com/samosvalishe/free-turn-proxy/desktop/locale"
)

type Phase uint8

const (
	Idle Phase = iota
	Connecting
	Connected
	Captcha
	Attention // A working VPN with an outstanding verification/preparation warning.
	Failed
)

const AnimationFrames = 8

// Status contains no endpoints, credentials or private captcha URL.
type Status struct {
	State                                    string
	Streams, Total                           int
	HasCaptcha, Preparing, PreparationFailed bool
}

func (s Status) Working() bool {
	return s.State == "connected" || ((s.State == "captcha" || s.State == "connecting") && s.Streams > 0)
}

func (s Status) Phase() Phase {
	// A blocking Connect call can be waiting on a manual challenge. Do not
	// keep spinning merely because helper/route preparation has not returned.
	if (s.HasCaptcha || s.State == "captcha") && s.State != "idle" && s.State != "" && s.State != "error" {
		if s.Working() && !s.Preparing {
			return Attention
		}
		return Captcha
	}
	if s.Preparing {
		return Connecting
	}
	if s.Working() {
		if s.HasCaptcha || s.State == "captcha" || s.PreparationFailed {
			return Attention
		}
		return Connected
	}
	if s.PreparationFailed || s.State == "error" {
		return Failed
	}
	if s.State == "idle" || s.State == "" {
		return Idle
	}
	if s.HasCaptcha || s.State == "captcha" {
		return Captcha
	}
	if s.State == "connecting" {
		return Connecting
	}
	return Idle
}

func (s Status) Description() string {
	var label string
	switch s.Phase() {
	case Connecting:
		if s.Preparing {
			label = locale.Text("Подготовка подключения…", "Preparing connection…")
		} else {
			label = locale.Text("Подключение…", "Connecting…")
		}
	case Connected:
		label = locale.Text("Подключено", "Connected")
	case Captcha:
		label = locale.Text("Требуется проверка VK", "VK verification required")
	case Attention:
		if s.PreparationFailed && !s.HasCaptcha && s.State != "captcha" {
			label = locale.Text("VPN работает · ошибка нового подключения", "VPN is working · new connection failed")
		} else {
			label = locale.Text("VPN работает · требуется проверка VK", "VPN is working · VK verification required")
		}
	case Failed:
		label = locale.Text("Ошибка подключения", "Connection error")
	default:
		label = locale.Text("Отключено", "Disconnected")
	}
	if s.Working() && s.Total > 0 {
		label += fmt.Sprintf(" · TURN %d/%d", max(0, s.Streams), s.Total)
	}
	return label
}

func (s Status) Tooltip() string { return "FreeTurn — " + s.Description() }

func animationFrame(phase Phase, frame int) int {
	if phase != Connecting {
		return 0
	}
	return (frame%AnimationFrames + AnimationFrames) % AnimationFrames
}
