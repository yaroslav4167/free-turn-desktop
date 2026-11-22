package main

import (
	"runtime"
	"testing"

	"github.com/samosvalishe/free-turn-proxy/desktop/integration"
	"github.com/samosvalishe/free-turn-proxy/desktop/updates"
)

func TestUpdateStateReportsNativeInstallCapabilityWithoutDesktopSettings(t *testing.T) {
	a := &App{updater: updates.NewForPlatform("v3.0.7", runtime.GOARCH, "example/fork", runtime.GOOS)}
	state := a.UpdateState()
	if state.CanInstallUpdates != integration.CanInstallUpdates {
		t.Fatal("auto-install capability lost")
	}
	if !state.CanInstallUpdates {
		t.Fatal("native desktop update should support automatic installation")
	}
}
