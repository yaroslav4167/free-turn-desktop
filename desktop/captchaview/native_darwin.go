//go:build darwin && cgo

package captchaview

/*
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#include <stdlib.h>
int ft_captcha_run(const char*, const char*, const char*, const char*);
void ft_captcha_command(int);
*/
import "C"

import (
	"errors"
	"runtime"
	"unsafe"
)

func runNative(cfg childConfig) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	url, title, reload, external := C.CString(cfg.URL), C.CString(cfg.Labels.Title), C.CString(cfg.Labels.Reload), C.CString(cfg.Labels.External)
	defer C.free(unsafe.Pointer(url))
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(reload))
	defer C.free(unsafe.Pointer(external))
	if C.ft_captcha_run(url, title, reload, external) != 0 {
		return errors.New("native captcha window failed")
	}
	return nil
}

func nativeCommand(command int) { C.ft_captcha_command(C.int(command)) }
