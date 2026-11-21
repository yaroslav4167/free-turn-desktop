package main

import (
	"errors"
	"time"

	"github.com/samosvalishe/free-turn-proxy/desktop/captchaview"
	"github.com/samosvalishe/free-turn-proxy/desktop/locale"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) presentCaptcha(url string) {
	if a.captchaView == nil {
		return
	}
	if url == "" {
		a.captchaView.Hide()
		return
	}
	if err := a.captchaView.Show(url, captchaview.Labels{
		Title:    locale.Text("FreeTurn · Проверка VK", "FreeTurn · VK verification"),
		Reload:   locale.Text("Обновить", "Reload"),
		External: locale.Text("Открыть в браузере", "Open in browser"),
	}); err != nil {
		a.engine.OnLog("warn", locale.Text("Не удалось открыть окно проверки VK", "Could not open the VK verification window"), time.Now().UnixMilli())
		// Show rejects invalid URLs and requests arriving during shutdown.
		// Startup/runtime failures use the controller's guarded fallback instead.
	}
}

func (a *App) openCaptchaExternal(url string) {
	if a.ctx == nil || !captchaview.ValidURL(url) || a.engine.Status().CaptchaURL != url {
		return
	}
	wailsruntime.BrowserOpenURL(a.ctx, url)
}

// The frontend does not supply a URL. Only the currently active challenge may
// be opened; an expired notification cannot launch a different page.
func (a *App) OpenCaptcha() error {
	url := a.engine.Status().CaptchaURL
	if !captchaview.ValidURL(url) {
		return errors.New(locale.Text("Активная проверка VK отсутствует", "No active VK verification"))
	}
	a.presentCaptcha(url)
	return nil
}

func (a *App) OpenCaptchaExternal() error {
	url := a.engine.Status().CaptchaURL
	if !captchaview.ValidURL(url) {
		return errors.New(locale.Text("Активная проверка VK отсутствует", "No active VK verification"))
	}
	if a.captchaView != nil {
		a.captchaView.Hide()
	}
	a.openCaptchaExternal(url)
	return nil
}
