//go:build linux

package tray

import (
	"bufio"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

func TestLightIconFollowsPortalAppearance(t *testing.T) {
	for _, test := range []struct {
		name           string
		value          interface{}
		fallback, want bool
	}{
		{"dark theme", uint32(1), false, true},
		{"light theme", uint32(2), true, false},
		{"dark variant", dbus.MakeVariant(uint32(1)), false, true},
		{"nested light variant", dbus.MakeVariant(dbus.MakeVariant(uint32(2))), true, false},
		{"no preference dark fallback", uint32(0), true, true},
		{"no preference normal fallback", uint32(0), false, false},
		{"unknown scheme", uint32(99), false, false},
		{"invalid value", "dark", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := lightIconForScheme(test.value, test.fallback); got != test.want {
				t.Fatalf("light icon = %v, want %v", got, test.want)
			}
		})
	}
}

func TestIconChangesNotifyTheTrayHost(t *testing.T) {
	daemon, err := exec.LookPath("dbus-daemon")
	if err != nil {
		t.Skip("dbus-daemon is needed for an isolated test bus")
	}
	cmd := exec.Command(daemon, "--session", "--nofork", "--print-address=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	address, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	connection, err := dbus.Connect(strings.TrimSpace(address))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	observer, err := dbus.Connect(strings.TrimSpace(address))
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Close()
	normal, _ := statusPixmaps(Idle, false, 0)
	light, _ := statusPixmaps(Idle, true, 0)
	properties, err := prop.Export(connection, "/StatusNotifierItem", prop.Map{itemInterface: {
		"IconPixmap": {Value: normal, Writable: false, Emit: prop.EmitFalse},
		"ToolTip":    {Value: statusToolTip{Title: "FreeTurn", IconPixmap: []iconPixmap{}}, Writable: false, Emit: prop.EmitFalse},
	}})
	if err != nil {
		t.Fatal(err)
	}
	signals := make(chan *dbus.Signal, 4)
	observer.Signal(signals)
	if err := observer.AddMatchSignal(dbus.WithMatchObjectPath("/StatusNotifierItem"), dbus.WithMatchInterface(itemInterface), dbus.WithMatchMember("NewIcon")); err != nil {
		t.Fatal(err)
	}
	tray := &Tray{conn: connection, iconProperties: properties}
	for _, preferLight := range []bool{true, false} {
		tray.updateIcon(preferLight)
		select {
		case <-signals:
		case <-time.After(2 * time.Second):
			t.Fatal("tray host did not receive NewIcon")
		}
		var value dbus.Variant
		if err := observer.Object(connection.Names()[0], "/StatusNotifierItem").Call("org.freedesktop.DBus.Properties.Get", 0, itemInterface, "IconPixmap").Store(&value); err != nil {
			t.Fatal(err)
		}
		var got []iconPixmap
		if err := value.Store(&got); err != nil {
			t.Fatal(err)
		}
		want := normal
		if preferLight {
			want = light
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatal("tray host read the wrong icon variant")
		}
	}
	status := Status{State: "connected", Streams: 3, Total: 12, HasCaptcha: true}
	tray.UpdateStatus(status, 0)
	select {
	case <-signals:
	case <-time.After(2 * time.Second):
		t.Fatal("status icon did not notify tray host")
	}
	tray.updateIcon(true)
	select {
	case <-signals:
	case <-time.After(2 * time.Second):
		t.Fatal("theme switch lost current status")
	}
	var current dbus.Variant
	if err := observer.Object(connection.Names()[0], "/StatusNotifierItem").Call("org.freedesktop.DBus.Properties.Get", 0, itemInterface, "IconPixmap").Store(&current); err != nil {
		t.Fatal(err)
	}
	var got []iconPixmap
	if err := current.Store(&got); err != nil {
		t.Fatal(err)
	}
	want, _ := statusPixmaps(Attention, true, 0)
	if !reflect.DeepEqual(got, want) {
		t.Fatal("theme switch reset the working VPN/captcha badge")
	}
	var tooltip dbus.Variant
	if err := observer.Object(connection.Names()[0], "/StatusNotifierItem").Call("org.freedesktop.DBus.Properties.Get", 0, itemInterface, "ToolTip").Store(&tooltip); err != nil {
		t.Fatal(err)
	}
	if tooltip.Signature().String() != "(sa(iiay)ss)" {
		t.Fatal("invalid tooltip signature")
	}
	var tip statusToolTip
	if err := tooltip.Store(&tip); err != nil || tip.Description != status.Description() {
		t.Fatalf("wrong tooltip: %+v, %v", tip, err)
	}
	tray.UpdateStatus(status, 7)
	select {
	case <-signals:
		t.Fatal("static status unnecessarily rerendered")
	case <-time.After(40 * time.Millisecond):
	}
}

func TestReducedMotionPreferences(t *testing.T) {
	for _, test := range []struct {
		namespace, key string
		value          interface{}
		want, valid    bool
	}{
		{"org.gnome.desktop.interface", "enable-animations", false, true, true},
		{"org.gnome.desktop.interface", "enable-animations", dbus.MakeVariant(true), false, true},
		{"org.kde.kdeglobals.KDE", "AnimationDurationFactor", float64(0), true, true},
		{"org.kde.kdeglobals.KDE", "AnimationDurationFactor", float64(1), false, true},
		{"unknown", "enable-animations", false, false, false},
	} {
		got, valid := motionPreference(test.namespace, test.key, test.value)
		if got != test.want || valid != test.valid {
			t.Fatalf("motion preference: %v %v", got, valid)
		}
	}
}

func TestGTKThemeFallback(t *testing.T) {
	for theme, want := range map[string]bool{"Adwaita:dark": true, "Breeze-Dark": true, "Adwaita": false, "": false} {
		t.Setenv("GTK_THEME", theme)
		if got := themeFallback(); got != want {
			t.Fatalf("GTK_THEME=%s: %v", theme, got)
		}
	}
}

func TestBothTrayVariantsHaveValidContrastingPixmaps(t *testing.T) {
	normal, err := applicationIcon(false)
	if err != nil {
		t.Fatal(err)
	}
	light, err := applicationIcon(true)
	if err != nil {
		t.Fatal(err)
	}
	for _, icons := range [][]iconPixmap{normal, light} {
		if len(icons) != 1 || icons[0].Width != 64 || icons[0].Height != 64 || len(icons[0].Pixels) != 64*64*4 {
			t.Fatal("invalid tray pixmap")
		}
		if dbus.SignatureOf(icons).String() != "a(iiay)" {
			t.Fatal("invalid D-Bus pixmap signature")
		}
		if icons[0].Pixels[0] != 0 {
			t.Fatal("icon corners must remain transparent")
		}
	}
	// Both variants use the same transparent silhouette, not a background tile.
	opaque := 0
	for offset := 0; offset < len(normal[0].Pixels); offset += 4 {
		if normal[0].Pixels[offset] != light[0].Pixels[offset] {
			t.Fatal("theme variants must preserve the dense symbol silhouette")
		}
		if normal[0].Pixels[offset] != 255 {
			continue
		}
		opaque++
		for channel := 1; channel <= 3; channel++ {
			if light[0].Pixels[offset+channel] < 200 || normal[0].Pixels[offset+channel] > 100 {
				t.Fatal("tray symbols must contrast on light and dark panels")
			}
		}
	}
	if opaque < 100 || opaque > 64*64/2 {
		t.Fatal("missing symbol or opaque background tile")
	}
}
