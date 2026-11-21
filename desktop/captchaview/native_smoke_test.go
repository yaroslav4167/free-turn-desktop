package captchaview

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Opt-in UI smoke test of the packaged desktop executable, never VK or a VPN.
// Linux CI runs this under Xvfb; other platforms can run it on a real desktop.
func TestNativeCaptchaWindow(t *testing.T) {
	binary := os.Getenv("FREETURN_TEST_CAPTCHA_BINARY")
	if binary == "" {
		t.Skip("set FREETURN_TEST_CAPTCHA_BINARY for native UI smoke test")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:8765")
	if err != nil {
		t.Fatal(err)
	}
	probe, stillHere := make(chan bool, 1), make(chan struct{}, 1)
	mediaDenied := make(chan bool, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		select {
		case probe <- r.URL.Query().Get("bridge") == "false":
		default:
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("/still-here", func(w http.ResponseWriter, r *http.Request) {
		select {
		case stillHere <- struct{}{}:
		default:
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("/media-probe", func(w http.ResponseWriter, r *http.Request) {
		select {
		case mediaDenied <- r.URL.Query().Get("denied") == "true":
		default:
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<!doctype html><html><body style="background:#0e0f14;color:white">Synthetic verification: no VK connection<script>
async function rejectsCapture(method,constraints) {
  const devices=navigator.mediaDevices;
  if (!devices || typeof devices[method]!=='function') return true;
  try { const stream=await devices[method](constraints);stream.getTracks().forEach(t=>t.stop());return false; }
  catch(e) {return true;}
}
fetch('/probe?bridge='+String(!!(window.go||window.wails||window.runtime))).then(async()=>{
  const camera=await rejectsCapture('getUserMedia',{video:true});
  const microphone=await rejectsCapture('getUserMedia',{audio:true});
  const screen=await rejectsCapture('getDisplayMedia',{video:true});
  await fetch('/media-probe?denied='+String(camera&&microphone&&screen));
  try { location.href='file:///freeturn-captcha-do-not-open'; } catch(e) {}
  setTimeout(()=>fetch('/still-here'),200);
});</script></body></html>`)
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: time.Second}
	go server.Serve(listener)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, ChildFlag)
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &protocolWriter{emit: func(string) {}}
	configDir := t.TempDir()
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+configDir, "APPDATA="+configDir)
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { finished <- cmd.Wait() }()
	cfg := childConfig{URL: "http://127.0.0.1:8765/not_robot_captcha?synthetic=1", CacheDir: t.TempDir(), Labels: Labels{Title: "FreeTurn synthetic verification", Reload: "Reload", External: "Open in browser"}}
	if err = json.NewEncoder(input).Encode(cfg); err != nil {
		cancel()
		t.Fatal(err)
	}
	select {
	case safe := <-probe:
		if !safe {
			cancel()
			t.Fatal("verification page received the main app bridge")
		}
	case err := <-finished:
		t.Fatalf("native view exited before rendering: %v; %s", err, stderr.String())
	case <-ctx.Done():
		t.Fatal("native view did not render synthetic page")
	}
	select {
	case denied := <-mediaDenied:
		if !denied {
			cancel()
			t.Fatal("verification page received a capture stream")
		}
	case err := <-finished:
		t.Fatalf("native view closed during media permission check: %v", err)
	case <-ctx.Done():
		t.Fatal("media capture did not reject without prompting")
	}
	select {
	case <-stillHere:
	case err := <-finished:
		t.Fatalf("native view closed unexpectedly: %v", err)
	case <-ctx.Done():
		t.Fatal("local page was replaced by forbidden file navigation")
	}
	_ = input.Close()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("native view did not close cleanly: %v; %s", err, stderr.String())
		}
	case <-ctx.Done():
		t.Fatal("orphaned native verification window")
	}
	if _, err = os.Stat(filepath.Join(configDir, "FreeTurnDesktop")); !os.IsNotExist(err) {
		t.Fatal("verification child accessed/created desktop preferences")
	}
}
