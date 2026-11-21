//go:build windows

package captchaview

import (
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"
)

var user32 = windows.NewLazySystemDLL("user32.dll")
var ole32 = windows.NewLazySystemDLL("ole32.dll")
var captchaHWND atomic.Uintptr

type windowClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
	IconSmall                          uintptr
}
type windowMessage struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Point          struct{ X, Y int32 }
	Private        uint32
}

func winCall(name string, args ...uintptr) uintptr {
	result, _, _ := user32.NewProc(name).Call(args...)
	return result
}
func wide(s string) *uint16 { p, _ := windows.UTF16PtrFromString(s); return p }
func nativeCommand(command int) {
	if hwnd := captchaHWND.Load(); hwnd != 0 {
		message := uintptr(0x8001)
		if command == 2 {
			message = 0x0010
		}
		if command == 3 {
			message = 0x8002
		}
		winCall("PostMessageW", hwnd, message, 0, 0)
	}
}

// Small bridge-free COM event handlers for the base WebView2 navigation API.
// The indices below are the stable ICoreWebView2 / event-args ABI, not Wails IPC.
type viewEvent struct {
	vtbl   *[4]uintptr
	invoke func(*comObject)
}

type comObject struct{ vtbl *[64]uintptr }

var eventVTable = [4]uintptr{
	syscall.NewCallback(func(this *viewEvent, iid unsafe.Pointer, out *unsafe.Pointer) uintptr {
		if out == nil {
			return 0x80004003
		}
		*out = unsafe.Pointer(this)
		return 0
	}),
	syscall.NewCallback(func(*viewEvent) uintptr { return 1 }),
	syscall.NewCallback(func(*viewEvent) uintptr { return 1 }),
	syscall.NewCallback(func(this *viewEvent, sender *comObject, args *comObject) uintptr { this.invoke(args); return 0 }),
}

func comCall(object *comObject, slot int, args ...uintptr) uintptr {
	result, _, _ := syscall.SyscallN(object.vtbl[slot], append([]uintptr{uintptr(unsafe.Pointer(object))}, args...)...)
	return result
}
func eventURI(args *comObject) string {
	var value *uint16
	if comCall(args, 3, uintptr(unsafe.Pointer(&value))) != 0 || value == nil {
		return ""
	}
	defer ole32.NewProc("CoTaskMemFree").Call(uintptr(unsafe.Pointer(value)))
	return windows.UTF16PtrToString(value)
}

func runNative(cfg childConfig) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := ole32.NewProc("CoInitializeEx").Call(0, 2)
	if hr != 0 && hr != 1 {
		return errors.New("WebView2 COM initialization unavailable")
	}
	defer ole32.NewProc("CoUninitialize").Call()
	var browser *edge.Chromium
	var reloadButton, externalButton uintptr
	closing := false
	proc := syscall.NewCallback(func(hwnd uintptr, message uint32, wparam, lparam uintptr) uintptr {
		switch message {
		case 0x0005: // WM_SIZE
			if wparam == 1 {
				return 0
			} // A minimised window has no drawable webview bounds.
			width := int32(uint16(lparam))
			height := int32(uint16(lparam >> 16))
			winCall("MoveWindow", externalButton, uintptr(max(168, width-268)), 8, 260, 28, 1)
			if width > 0 && height > 44 && browser != nil && browser.GetController() != nil {
				browser.Resize()
			}
			return 0
		case 0x0111: // WM_COMMAND
			if wparam&0xffff == 1 && browser != nil && browser.GetController() != nil {
				browser.Navigate(cfg.URL)
			}
			if wparam&0xffff == 2 {
				fmt.Println("EXTERNAL")
				winCall("PostMessageW", hwnd, 0x0010, 0, 0)
			}
			return 0
		case 0x8001:
			winCall("ShowWindow", hwnd, 9)
			winCall("SetForegroundWindow", hwnd)
			return 0
		case 0x8002:
			if !closing {
				fmt.Println("FAILED")
			}
			winCall("PostMessageW", hwnd, 0x0010, 0, 0)
			return 0
		case 0x0010:
			if !closing {
				fmt.Println("CLOSED")
			}
			closing = true
			if browser != nil {
				browser.ShuttingDown()
			}
			winCall("DestroyWindow", hwnd)
			return 0
		case 0x0002:
			captchaHWND.Store(0)
			winCall("PostQuitMessage", 0)
			return 0
		}
		return winCall("DefWindowProcW", hwnd, uintptr(message), wparam, lparam)
	})
	instance, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	brush, _, _ := windows.NewLazySystemDLL("gdi32.dll").NewProc("CreateSolidBrush").Call(0x241c1b)
	defer windows.NewLazySystemDLL("gdi32.dll").NewProc("DeleteObject").Call(brush)
	className := wide("FreeTurnCaptchaWindow")
	class := windowClass{Size: uint32(unsafe.Sizeof(windowClass{})), Proc: proc, Instance: instance, ClassName: className, Background: brush}
	if winCall("RegisterClassExW", uintptr(unsafe.Pointer(&class))) == 0 {
		return errors.New("captcha window registration failed")
	}
	hwnd := winCall("CreateWindowExW", 0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(wide(cfg.Labels.Title))), 0x10cf0000, 0x80000000, 0x80000000, 740, 740, 0, 0, instance, 0)
	if hwnd == 0 {
		return errors.New("captcha window creation failed")
	}
	captchaHWND.Store(hwnd)
	defer func() {
		if captchaHWND.Swap(0) != 0 {
			winCall("DestroyWindow", hwnd)
		}
	}()
	reloadButton = winCall("CreateWindowExW", 0, uintptr(unsafe.Pointer(wide("BUTTON"))), uintptr(unsafe.Pointer(wide(cfg.Labels.Reload))), 0x50010000, 8, 8, 150, 28, hwnd, 1, instance, 0)
	externalButton = winCall("CreateWindowExW", 0, uintptr(unsafe.Pointer(wide("BUTTON"))), uintptr(unsafe.Pointer(wide(cfg.Labels.External))), 0x50010000, 450, 8, 260, 28, hwnd, 2, instance, 0)
	font, _, _ := windows.NewLazySystemDLL("gdi32.dll").NewProc("GetStockObject").Call(17)
	winCall("SendMessageW", reloadButton, 0x0030, font, 1)
	winCall("SendMessageW", externalButton, 0x0030, font, 1)
	var dark int32 = 1
	windows.NewLazySystemDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute").Call(hwnd, 20, uintptr(unsafe.Pointer(&dark)), 4)
	browser = edge.NewChromium()
	browser.SetGlobalPermission(edge.CoreWebView2PermissionStateDeny)
	browser.DataPath = cfg.CacheDir
	browser.SetErrorCallback(func(error) { nativeCommand(3) })
	if !browser.Embed(hwnd) || browser.GetController() == nil || closing {
		return errors.New("WebView2 unavailable")
	}
	settings, err := browser.GetSettings()
	if err != nil {
		return err
	}
	for _, configure := range []func(bool) error{settings.PutIsWebMessageEnabled, settings.PutAreHostObjectsAllowed, settings.PutAreDevToolsEnabled, settings.PutAreDefaultContextMenusEnabled, settings.PutAreDefaultScriptDialogsEnabled} {
		if err = configure(false); err != nil {
			return err
		}
	}
	browser.SetBackgroundColour(14, 15, 20, 255)
	browser.SetPadding(edge.Rect{Top: 44})
	webview, err := browser.GetController().GetCoreWebView2()
	if err != nil {
		return err
	}
	blockedNavigation := false
	navigation := &viewEvent{vtbl: &eventVTable, invoke: func(args *comObject) {
		if uri := eventURI(args); !ValidURL(uri) && uri != "about:blank" {
			blockedNavigation = true
			comCall(args, 8, 1)
		}
	}}
	popup := &viewEvent{vtbl: &eventVTable, invoke: func(args *comObject) { comCall(args, 6, 1) }}
	var pinned runtime.Pinner
	pinned.Pin(navigation)
	pinned.Pin(popup)
	defer pinned.Unpin()
	var tokens [3]int64
	for i, registration := range []struct {
		slot    int
		handler *viewEvent
	}{{7, navigation}, {17, navigation}, {44, popup}} {
		if comCall((*comObject)(unsafe.Pointer(webview)), registration.slot, uintptr(unsafe.Pointer(registration.handler)), uintptr(unsafe.Pointer(&tokens[i]))) != 0 {
			return errors.New("WebView2 navigation isolation unavailable")
		}
	}
	browser.NavigationCompletedCallback = func(_ *edge.ICoreWebView2, args *edge.ICoreWebView2NavigationCompletedEventArgs) {
		var success int32
		if blockedNavigation {
			blockedNavigation = false
			return
		}
		if !closing && (comCall((*comObject)(unsafe.Pointer(args)), 3, uintptr(unsafe.Pointer(&success))) != 0 || success == 0) {
			nativeCommand(3)
		}
	}
	fmt.Println("READY")
	browser.Navigate(cfg.URL)
	browser.Resize()
	var message windowMessage
	for int32(winCall("GetMessageW", uintptr(unsafe.Pointer(&message)), 0, 0, 0)) > 0 {
		winCall("TranslateMessage", uintptr(unsafe.Pointer(&message)))
		winCall("DispatchMessageW", uintptr(unsafe.Pointer(&message)))
	}
	// Close on the owning UI thread. No host objects/message callbacks were bound.
	comCall((*comObject)(unsafe.Pointer(browser.GetController())), 24)
	runtime.KeepAlive(navigation)
	runtime.KeepAlive(popup)
	runtime.KeepAlive(browser)
	return nil
}
