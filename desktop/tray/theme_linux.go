//go:build linux

package tray

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	portalName          = "org.freedesktop.portal.Desktop"
	settingsInterface   = "org.freedesktop.portal.Settings"
	appearanceNamespace = "org.freedesktop.appearance"
)

// The portal reports 1 for a dark appearance, 2 for light and 0 for no
// preference. Some portal implementations wrap the variant a second time.
func lightIconForScheme(value interface{}, fallback bool) bool {
	for depth := 0; depth < 8; depth++ {
		if variant, ok := value.(dbus.Variant); ok {
			value = variant.Value()
			continue
		}
		scheme, ok := value.(uint32)
		if !ok {
			return fallback
		}
		switch scheme {
		case 1:
			return true
		case 2:
			return false
		default:
			return fallback
		}
	}
	return fallback
}

func themeFallback() bool {
	return strings.Contains(strings.ToLower(os.Getenv("GTK_THEME")), "dark")
}

func preferLightTrayIcon(conn *dbus.Conn) bool {
	fallback := themeFallback()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var value dbus.Variant
	err := conn.Object(portalName, "/org/freedesktop/portal/desktop").CallWithContext(
		ctx, settingsInterface+".Read", 0, appearanceNamespace, "color-scheme",
	).Store(&value)
	if err != nil {
		return fallback
	}
	return lightIconForScheme(value, fallback)
}

func (t *Tray) updateIcon(light bool) {
	t.iconMu.Lock()
	defer t.iconMu.Unlock()
	if t.lightIcon == light {
		return
	}
	t.lightIcon = light
	// StatusNotifierItem hosts refresh IconPixmap on NewIcon. The property is
	// read-only to clients but mutable internally (not annotated as constant).
	t.renderStatusLocked()
}

func motionPreference(namespace, key, value interface{}) (bool, bool) {
	for i := 0; i < 8; i++ {
		variant, ok := value.(dbus.Variant)
		if !ok {
			break
		}
		value = variant.Value()
	}
	if namespace == "org.gnome.desktop.interface" && key == "enable-animations" {
		if enabled, ok := value.(bool); ok {
			return !enabled, true
		}
	}
	if namespace == "org.kde.kdeglobals.KDE" && key == "AnimationDurationFactor" {
		if factor, ok := value.(float64); ok {
			return factor <= 0, true
		}
	}
	return false, false
}

func readReducedMotion(conn *dbus.Conn) (bool, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	known := false
	for _, pref := range [][2]string{{"org.gnome.desktop.interface", "enable-animations"}, {"org.kde.kdeglobals.KDE", "AnimationDurationFactor"}} {
		var value dbus.Variant
		if err := conn.Object(portalName, "/org/freedesktop/portal/desktop").CallWithContext(ctx, settingsInterface+".Read", 0, pref[0], pref[1]).Store(&value); err == nil {
			if reduced, ok := motionPreference(pref[0], pref[1], value); ok {
				known = true
				if reduced {
					return true, true
				}
			}
		}
	}
	return false, known
}
