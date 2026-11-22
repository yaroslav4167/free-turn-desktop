//go:build linux

package tray

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
	"github.com/samosvalishe/free-turn-proxy/desktop/locale"
)

const (
	itemInterface = "org.kde.StatusNotifierItem"
	menuInterface = "com.canonical.dbusmenu"
	watcherName   = "org.kde.StatusNotifierWatcher"
)

// Tray implements StatusNotifierItem (KDE, GNOME AppIndicator, Xfce, etc.)
// over the session bus without introducing a second GTK event loop.
type Tray struct {
	conn           *dbus.Conn
	onShow         func()
	onQuit         func()
	once           sync.Once
	registered     atomic.Bool
	iconProperties *prop.Properties
	lightIcon      bool
	iconMu         sync.Mutex
	status         Status
	frame          int
	lastKey        iconKey
	lastTip        string
	iconReady      bool
	closing        bool
	reducedMotion  atomic.Bool
}

type trayItem struct{ tray *Tray }

func (i *trayItem) Activate(x, y int32) *dbus.Error                    { go i.tray.onShow(); return nil }
func (i *trayItem) SecondaryActivate(x, y int32) *dbus.Error           { go i.tray.onShow(); return nil }
func (i *trayItem) ContextMenu(x, y int32) *dbus.Error                 { go i.tray.onShow(); return nil }
func (i *trayItem) Scroll(delta int32, orientation string) *dbus.Error { return nil }

type menuLayout struct {
	ID         int32
	Properties map[string]dbus.Variant
	Children   []dbus.Variant
}

type menuProperties struct {
	ID         int32
	Properties map[string]dbus.Variant
}

type trayMenu struct{ tray *Tray }

func menuProps(id int32) map[string]dbus.Variant {
	if id == 0 {
		return map[string]dbus.Variant{"children-display": dbus.MakeVariant("submenu")}
	}
	label := locale.Text("Открыть FreeTurn", "Open FreeTurn")
	if id == 2 {
		label = locale.Text("Выход", "Quit")
	}
	return map[string]dbus.Variant{
		"label":   dbus.MakeVariant(label),
		"enabled": dbus.MakeVariant(true),
		"visible": dbus.MakeVariant(true),
	}
}

func (t *Tray) RefreshLanguage() {
	if t == nil || t.conn == nil {
		return
	}
	removed := []struct {
		ID    int32
		Names []string
	}{}
	_ = t.conn.Emit("/MenuBar", menuInterface+".ItemsPropertiesUpdated", []menuProperties{{1, menuProps(1)}, {2, menuProps(2)}}, removed)
}

func (m *trayMenu) GetLayout(parentID, depth int32, names []string) (uint32, menuLayout, *dbus.Error) {
	if parentID == 1 || parentID == 2 {
		return 1, menuLayout{parentID, menuProps(parentID), []dbus.Variant{}}, nil
	}
	if parentID != 0 {
		return 0, menuLayout{}, dbus.MakeFailedError(fmt.Errorf("unknown menu item %d", parentID))
	}
	root := menuLayout{0, menuProps(0), []dbus.Variant{}}
	if depth != 0 {
		for _, id := range []int32{1, 2} {
			root.Children = append(root.Children, dbus.MakeVariant(menuLayout{id, menuProps(id), []dbus.Variant{}}))
		}
	}
	return 1, root, nil
}

func (m *trayMenu) GetGroupProperties(ids []int32, names []string) ([]menuProperties, *dbus.Error) {
	if len(ids) == 0 {
		ids = []int32{0, 1, 2}
	}
	result := []menuProperties{}
	for _, id := range ids {
		if id >= 0 && id <= 2 {
			result = append(result, menuProperties{id, menuProps(id)})
		}
	}
	return result, nil
}

func (m *trayMenu) GetProperty(id int32, name string) (dbus.Variant, *dbus.Error) {
	if value, ok := menuProps(id)[name]; ok && id >= 0 && id <= 2 {
		return value, nil
	}
	return dbus.MakeVariant(""), dbus.MakeFailedError(fmt.Errorf("unknown menu property"))
}

func (m *trayMenu) Event(id int32, event string, data dbus.Variant, timestamp uint32) *dbus.Error {
	if event == "clicked" {
		if id == 1 {
			go m.tray.onShow()
		}
		if id == 2 {
			go m.tray.onQuit()
		}
	}
	return nil
}

func (m *trayMenu) AboutToShow(id int32) (bool, *dbus.Error) { return false, nil }

type menuEvent struct {
	ID        int32
	Event     string
	Data      dbus.Variant
	Timestamp uint32
}

func (m *trayMenu) EventGroup(events []menuEvent) ([]int32, *dbus.Error) {
	invalid := []int32{}
	for _, event := range events {
		if event.ID < 0 || event.ID > 2 {
			invalid = append(invalid, event.ID)
			continue
		}
		_ = m.Event(event.ID, event.Event, event.Data, event.Timestamp)
	}
	return invalid, nil
}

func (m *trayMenu) AboutToShowGroup(ids []int32) ([]int32, []int32, *dbus.Error) {
	invalid := []int32{}
	for _, id := range ids {
		if id < 0 || id > 2 {
			invalid = append(invalid, id)
		}
	}
	return []int32{}, invalid, nil
}

func Start(title string, onShow, onQuit func()) (*Tray, error) {
	normalIcon, err := applicationIcon(false)
	if err != nil {
		return nil, fmt.Errorf("decode application icon: %w", err)
	}
	lightIcon, err := applicationIcon(true)
	if err != nil {
		return nil, fmt.Errorf("decode light tray icon: %w", err)
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	signals := make(chan *dbus.Signal, 16)
	conn.Signal(signals)
	// Theme support is optional: an unavailable portal must not disable the tray.
	_ = conn.AddMatchSignal(dbus.WithMatchSender(portalName), dbus.WithMatchObjectPath("/org/freedesktop/portal/desktop"), dbus.WithMatchInterface(settingsInterface), dbus.WithMatchMember("SettingChanged"))
	_ = conn.AddMatchSignal(dbus.WithMatchObjectPath("/org/freedesktop/DBus"), dbus.WithMatchInterface("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged"), dbus.WithMatchArg(0, portalName))
	t := &Tray{conn: conn, onShow: onShow, onQuit: onQuit, lightIcon: preferLightTrayIcon(conn)}
	icon := normalIcon
	if t.lightIcon {
		icon = lightIcon
	}
	fail := func(err error) (*Tray, error) { _ = conn.Close(); return nil, err }
	item, menu := &trayItem{t}, &trayMenu{t}
	if err := conn.Export(item, "/StatusNotifierItem", itemInterface); err != nil {
		return fail(err)
	}
	if err := conn.Export(menu, "/MenuBar", menuInterface); err != nil {
		return fail(err)
	}
	for path, iface := range map[dbus.ObjectPath]introspect.Interface{
		"/StatusNotifierItem": {Name: itemInterface, Methods: introspect.Methods(item), Signals: []introspect.Signal{{Name: "NewIcon"}, {Name: "NewToolTip"}}},
		"/MenuBar":            {Name: menuInterface, Methods: introspect.Methods(menu)},
	} {
		if err := conn.Export(introspect.NewIntrospectable(&introspect.Node{Interfaces: []introspect.Interface{iface, prop.IntrospectData}}), path, "org.freedesktop.DBus.Introspectable"); err != nil {
			return fail(err)
		}
	}
	readonly := func(value interface{}) *prop.Prop {
		return &prop.Prop{Value: value, Writable: false, Emit: prop.EmitConst}
	}
	iconProperties, err := prop.Export(conn, "/StatusNotifierItem", prop.Map{itemInterface: {
		"Category": readonly("Communications"), "Id": readonly("freeturn-desktop"),
		"Title": readonly(title), "Status": readonly("Active"),
		"IconName": readonly(""), "IconPixmap": {Value: icon, Writable: false, Emit: prop.EmitFalse}, "ItemIsMenu": readonly(false),
		"Menu":    readonly(dbus.ObjectPath("/MenuBar")),
		"ToolTip": {Value: statusToolTip{Title: "FreeTurn", Description: (Status{}).Description(), IconPixmap: []iconPixmap{}}, Writable: false, Emit: prop.EmitFalse},
	}})
	if err != nil {
		return fail(err)
	}
	t.iconProperties = iconProperties
	if reduced, ok := readReducedMotion(conn); ok {
		t.reducedMotion.Store(reduced)
	}
	t.UpdateStatus(Status{}, 0)
	if _, err := prop.Export(conn, "/MenuBar", prop.Map{menuInterface: {
		"Version": readonly(uint32(3)), "TextDirection": readonly("ltr"),
		"Status": readonly("normal"), "IconThemePath": readonly([]string{}),
	}}); err != nil {
		return fail(err)
	}
	// Re-register if the desktop shell (and its watcher) restarts.
	if err := conn.AddMatchSignal(dbus.WithMatchObjectPath("/org/freedesktop/DBus"), dbus.WithMatchInterface("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged"), dbus.WithMatchArg(0, watcherName)); err != nil {
		return fail(err)
	}
	// The desktop shell may not be ready yet during login autostart. Keep
	// listening instead of permanently losing the tray for this session.
	_ = t.register()
	go func() {
		for signal := range signals {
			if signal.Name == settingsInterface+".SettingChanged" && len(signal.Body) == 3 {
				if reduced, ok := motionPreference(signal.Body[0], signal.Body[1], signal.Body[2]); ok {
					if current, known := readReducedMotion(conn); known {
						reduced = current
					}
					t.reducedMotion.Store(reduced)
				}
			}
			if signal.Name == settingsInterface+".SettingChanged" && len(signal.Body) == 3 && signal.Body[0] == appearanceNamespace && signal.Body[1] == "color-scheme" {
				t.updateIcon(lightIconForScheme(signal.Body[2], themeFallback()))
			}
			if signal.Name == "org.freedesktop.DBus.NameOwnerChanged" && len(signal.Body) == 3 && signal.Body[0] == portalName {
				if signal.Body[2] != "" {
					if reduced, ok := readReducedMotion(conn); ok {
						t.reducedMotion.Store(reduced)
					}
					t.updateIcon(preferLightTrayIcon(conn))
				} else {
					t.updateIcon(themeFallback())
				}
			}
			if len(signal.Body) == 3 && signal.Body[0] == watcherName {
				t.registered.Store(false)
				if signal.Body[2] != "" {
					_ = t.register()
				} else {
					go t.onShow()
				}
			}
		}
	}()
	return t, nil
}

func (t *Tray) register() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	names := t.conn.Names()
	if len(names) == 0 {
		return fmt.Errorf("session bus is disconnected")
	}
	err := t.conn.Object(watcherName, "/StatusNotifierWatcher").CallWithContext(ctx, watcherName+".RegisterStatusNotifierItem", 0, names[0]).Err
	t.registered.Store(err == nil)
	return err
}

// Never hide the window if there is no desktop host to bring it back.
func (t *Tray) Available() bool {
	if t == nil || !t.conn.Connected() || !t.registered.Load() {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var value dbus.Variant
	err := t.conn.Object(watcherName, "/StatusNotifierWatcher").CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, watcherName, "IsStatusNotifierHostRegistered").Store(&value)
	host, _ := value.Value().(bool)
	return err == nil && host
}

func (t *Tray) Close() {
	t.once.Do(func() { t.iconMu.Lock(); t.closing = true; t.iconMu.Unlock(); _ = t.conn.Close() })
}

type statusToolTip struct {
	IconName           string
	IconPixmap         []iconPixmap
	Title, Description string
}

func (t *Tray) ReducedMotion() bool { return t == nil || t.reducedMotion.Load() }

func (t *Tray) UpdateStatus(status Status, frame int) {
	if t == nil {
		return
	}
	t.iconMu.Lock()
	defer t.iconMu.Unlock()
	t.status = status
	t.frame = frame
	t.renderStatusLocked()
}

func (t *Tray) renderStatusLocked() {
	if t.closing || t.conn == nil || t.iconProperties == nil {
		return
	}
	frame := t.frame
	if t.reducedMotion.Load() {
		frame = 0
	}
	key := iconKey{t.status.Phase(), t.lightIcon, animationFrame(t.status.Phase(), frame), 64}
	if !t.iconReady || key != t.lastKey {
		if icons, err := statusPixmaps(key.Phase, key.Light, key.Frame); err == nil {
			t.iconProperties.SetMust(itemInterface, "IconPixmap", icons)
			_ = t.conn.Emit("/StatusNotifierItem", itemInterface+".NewIcon")
			t.lastKey = key
			t.iconReady = true
		}
	}
	if description := t.status.Description(); description != t.lastTip {
		t.iconProperties.SetMust(itemInterface, "ToolTip", statusToolTip{Title: "FreeTurn", Description: description, IconPixmap: []iconPixmap{}})
		_ = t.conn.Emit("/StatusNotifierItem", itemInterface+".NewToolTip")
		t.lastTip = description
	}
}
