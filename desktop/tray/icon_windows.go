//go:build windows

package tray

import (
	"fmt"
	"runtime"
	"unsafe"

	appassets "github.com/samosvalishe/free-turn-proxy/desktop/assets"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	pCreateIconFromResourceEx = user32.NewProc("CreateIconFromResourceEx")
	pGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
	pGetSysColor              = user32.NewProc("GetSysColor")
)

func preferLightWindowsTrayIcon() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err == nil {
		defer key.Close()
		if value, _, err := key.GetIntegerValue("SystemUsesLightTheme"); err == nil {
			return value == 0
		}
	}
	// Fall back to a system menu colour for classic/high-contrast themes.
	c, _, _ := pGetSysColor.Call(4)
	r, g, b := c&255, (c>>8)&255, (c>>16)&255
	return 299*r+587*g+114*b < 128000
}

func loadWindowsTrayIcon(light bool) (windows.Handle, error) {
	data := appassets.TrayIconICO
	if light {
		data = appassets.TrayLightIconICO
	}
	size, _, _ := pGetSystemMetrics.Call(49) // SM_CXSMICON
	if size < 16 || size > 256 {
		size = 16
	}
	image, err := icoFrame(data, int(size))
	if err != nil {
		return 0, err
	}
	handle, _, callErr := pCreateIconFromResourceEx.Call(uintptr(unsafe.Pointer(&image[0])), uintptr(len(image)), 1, 0x00030000, size, size, 0)
	runtime.KeepAlive(image)
	if handle == 0 {
		return 0, fmt.Errorf("create dense tray icon: %v", callErr)
	}
	return windows.Handle(handle), nil
}

func (t *Tray) refreshIcon() {
	if t.stopping.Load() {
		return
	}
	t.refreshMotion()
	light := preferLightWindowsTrayIcon()
	t.statusMu.Lock()
	status, frame := t.status, t.frame
	t.statusMu.Unlock()
	if t.reducedMotion.Load() {
		frame = 0
	}
	size, _, _ := pGetSystemMetrics.Call(49)
	if size < 16 || size > 256 {
		size = 16
	}
	key := iconKey{status.Phase(), light, animationFrame(status.Phase(), frame), int(size)}
	tip := status.Tooltip()
	if key == t.lastKey && tip == t.lastTip {
		return
	}
	var icon windows.Handle
	nid := notifyIconDataW{hWnd: t.hwnd, uID: 1, uFlags: nifTip}
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	if key != t.lastKey {
		var err error
		icon, err = createWindowsStatusIcon(key)
		if err != nil {
			return
		}
		nid.uFlags |= nifIcon
		nid.hIcon = icon
	}
	chars, err := windows.UTF16FromString(tip)
	if err != nil {
		if icon != 0 {
			pDestroyIcon.Call(uintptr(icon))
		}
		return
	}
	copy(nid.szTip[:127], chars)
	if ret, _, _ := pShell_NotifyIconW.Call(uintptr(nimModify), uintptr(unsafe.Pointer(&nid))); ret == 0 {
		if icon != 0 {
			pDestroyIcon.Call(uintptr(icon))
		}
		return
	}
	if icon != 0 {
		previous := t.hIcon
		t.hIcon = icon
		if previous != 0 {
			pDestroyIcon.Call(uintptr(previous))
		}
	}
	t.lightIcon, t.lastKey, t.lastTip = light, key, tip
}

func createWindowsStatusIcon(key iconKey) (windows.Handle, error) {
	data, err := statusPNG(key.Phase, key.Light, key.Frame, key.Size)
	if err != nil {
		return 0, err
	}
	handle, _, callErr := pCreateIconFromResourceEx.Call(uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), 1, 0x00030000, uintptr(key.Size), uintptr(key.Size), 0)
	runtime.KeepAlive(data)
	if handle == 0 {
		return 0, fmt.Errorf("create status tray icon: %v", callErr)
	}
	return windows.Handle(handle), nil
}

func (t *Tray) restoreIcon() {
	if t.stopping.Load() || t.hIcon == 0 {
		return
	}
	t.statusMu.Lock()
	tip := t.status.Tooltip()
	t.statusMu.Unlock()
	nid := notifyIconDataW{hWnd: t.hwnd, uID: 1, uFlags: nifIcon | nifMessage | nifTip, uCallbackMessage: wmTrayIcon, hIcon: t.hIcon}
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	chars, _ := windows.UTF16FromString(tip)
	copy(nid.szTip[:127], chars)
	if ret, _, _ := pShell_NotifyIconW.Call(uintptr(nimAdd), uintptr(unsafe.Pointer(&nid))); ret != 0 {
		t.refreshIcon()
	}
}

func (t *Tray) refreshMotion() {
	var enabled int32 = 1
	result, _, _ := user32.NewProc("SystemParametersInfoW").Call(0x1042, 0, uintptr(unsafe.Pointer(&enabled)), 0) // SPI_GETCLIENTAREAANIMATION
	t.reducedMotion.Store(result != 0 && enabled == 0)
}
