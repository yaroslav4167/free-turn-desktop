//go:build (!linux && !darwin && !windows) || ((linux || darwin) && !cgo)

package captchaview

import "errors"

func runNative(childConfig) error { return errors.New("native captcha view unavailable in this build") }
func nativeCommand(int)           {}
