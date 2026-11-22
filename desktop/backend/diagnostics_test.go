package backend

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDiagnosticsExcludePrivateFieldsAndBoundHistory(t *testing.T) {
	p := DefaultServer()
	p.Name = "SECRET-NAME"
	p.Peer = "SECRET-PEER"
	p.ObfKey = "SECRET-KEY"
	p.WgClientConf = "SECRET-WG"
	p.VKLinks = []string{"SECRET-VK"}
	e := &Engine{}
	for i := 0; i < 120; i++ {
		e.OnState("connected", 1, 12, "SECRET-ERROR")
	}
	status, events := e.DiagnosticSnapshot()
	if len(events) != 100 {
		t.Fatal("unbounded event history")
	}
	status.ServerID = "SECRET-ID"
	status.CaptchaURL = "SECRET-CAPTCHA"
	status.ErrMsg = "SECRET-ERROR"
	data, err := Diagnostics("test", false, []Server{p}, status, events)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "SECRET-") {
		t.Fatal("private data exported")
	}
	if !json.Valid(data) || !strings.Contains(string(data), "recentStates") {
		t.Fatal("invalid diagnostics")
	}
}

func TestConnectionProgressCannotBeOverwrittenByCancelledPlan(t *testing.T) {
	e := &Engine{planGeneration: 2, preparing: true, progressStage: "dns"}
	e.setProgress(1, "routes")
	if e.ConnectionProgress().Stage != "dns" {
		t.Fatal("stale plan overwrote progress")
	}
	e.setProgress(2, "helper")
	if e.ConnectionProgress().Stage != "helper" {
		t.Fatal("current plan progress missing")
	}
	e.finishConnectPlan(2, nil)
	if e.ConnectionProgress().Stage != "" {
		t.Fatal("finished plan retained progress")
	}
}
