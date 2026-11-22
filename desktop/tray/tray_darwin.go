//go:build darwin

package tray

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
void freeturn_tray_start(const char *title, const char *open, const char *quit, const unsigned char *icon, int length);
void freeturn_tray_close(void);
void freeturn_tray_language(const char *open, const char *quit);
void freeturn_tray_status(const unsigned char*,int,const unsigned char*,int,const char*,int);
int freeturn_tray_reduced_motion(void);
*/
import "C"
import (
	"errors"
	appassets "github.com/samosvalishe/free-turn-proxy/desktop/assets"
	"github.com/samosvalishe/free-turn-proxy/desktop/locale"
	"sync"
	"sync/atomic"
	"unsafe"
)

type Tray struct {
	onShow, onQuit func()
	available      atomic.Bool
	statusMu       sync.Mutex
	lastKey        iconKey
	lastTip        string
}

var trayMu sync.Mutex
var currentTray *Tray

func Start(title string, onShow, onQuit func()) (*Tray, error) {
	trayMu.Lock()
	defer trayMu.Unlock()
	if currentTray != nil {
		return nil, errors.New("menu bar item already exists")
	}
	t := &Tray{onShow: onShow, onQuit: onQuit}
	t.available.Store(true)
	currentTray = t
	a, b, c := C.CString(title), C.CString(locale.Text("Открыть FreeTurn", "Open FreeTurn")), C.CString(locale.Text("Выйти", "Quit"))
	defer C.free(unsafe.Pointer(a))
	defer C.free(unsafe.Pointer(b))
	defer C.free(unsafe.Pointer(c))
	data := appassets.TrayIconPNG
	C.freeturn_tray_start(a, b, c, (*C.uchar)(unsafe.Pointer(&data[0])), C.int(len(data)))
	return t, nil
}
func (t *Tray) Available() bool { return t != nil && t.available.Load() }
func (t *Tray) Close() {
	trayMu.Lock()
	defer trayMu.Unlock()
	if currentTray == t {
		t.available.Store(false)
		C.freeturn_tray_close()
		currentTray = nil
	}
}
func (t *Tray) RefreshLanguage() {
	if !t.Available() {
		return
	}
	a, b := C.CString(locale.Text("Открыть FreeTurn", "Open FreeTurn")), C.CString(locale.Text("Выйти", "Quit"))
	defer C.free(unsafe.Pointer(a))
	defer C.free(unsafe.Pointer(b))
	C.freeturn_tray_language(a, b)
}

func (t *Tray) ReducedMotion() bool { return t == nil || C.freeturn_tray_reduced_motion() != 0 }

func (t *Tray) UpdateStatus(status Status, frame int) {
	if !t.Available() {
		return
	}
	if t.ReducedMotion() {
		frame = 0
	}
	key := iconKey{status.Phase(), false, animationFrame(status.Phase(), frame), 64}
	tip := status.Tooltip()
	t.statusMu.Lock()
	defer t.statusMu.Unlock()
	if key == t.lastKey && tip == t.lastTip {
		return
	}
	dark, err := statusPNG(key.Phase, false, key.Frame, 64)
	if err != nil {
		return
	}
	light, err := statusPNG(key.Phase, true, key.Frame, 64)
	if err != nil {
		return
	}
	text := C.CString(tip)
	defer C.free(unsafe.Pointer(text))
	trayMu.Lock()
	defer trayMu.Unlock()
	if currentTray != t || !t.Available() {
		return
	}
	template := 0
	if key.Phase == Idle {
		template = 1
	}
	C.freeturn_tray_status((*C.uchar)(unsafe.Pointer(&dark[0])), C.int(len(dark)), (*C.uchar)(unsafe.Pointer(&light[0])), C.int(len(light)), text, C.int(template))
	t.lastKey, t.lastTip = key, tip
}

//export freeturn_tray_action
func freeturn_tray_action(action C.int) {
	trayMu.Lock()
	t := currentTray
	var callback func()
	if t != nil {
		if action == 1 {
			callback = t.onShow
		} else if action == 2 {
			callback = t.onQuit
		}
	}
	trayMu.Unlock()
	if callback != nil {
		go callback()
	}
}
