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
	light := preferLightWindowsTrayIcon()
	if light == t.lightIcon {
		return
	}
	icon, err := loadWindowsTrayIcon(light)
	if err != nil {
		return
	}
	nid := notifyIconDataW{hWnd: t.hwnd, uID: 1, uFlags: nifIcon, hIcon: icon}
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	if ret, _, _ := pShell_NotifyIconW.Call(uintptr(nimModify), uintptr(unsafe.Pointer(&nid))); ret == 0 {
		pDestroyIcon.Call(uintptr(icon))
		return
	}
	previous := t.hIcon
	t.hIcon, t.lightIcon = icon, light
	if previous != 0 {
		pDestroyIcon.Call(uintptr(previous))
	}
}
