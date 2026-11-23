//go:build windows

package integration

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWindowsPowerShellSeparatesDiagnosticsFromResult(t *testing.T) {
	out, err := runPowerShell("[Console]::Error.Write('diagnostic');[Console]::Write('1')")
	if err != nil || out != "1" {
		t.Fatalf("got %q, %v; diagnostics must not change the autostart value", out, err)
	}
	out, err = runPowerShell("[Console]::Write('Проверка')")
	if err != nil || out != "Проверка" {
		t.Fatalf("UTF-8 result got %q, %v", out, err)
	}
	_, err = runPowerShell("[Console]::Error.Write('autostart failure');exit 23")
	if err == nil || !strings.Contains(err.Error(), "autostart failure") {
		t.Fatalf("missing failure detail: %v", err)
	}
}

func TestWindowsPowerShellReportsTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := runPowerShellContext(ctx, "Start-Sleep -Seconds 30")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want deadline exceeded", err)
	}
}

// Create a definition in memory only: tests must not change the user's actual
// login task or launch an executable.
func TestWindowsAutostartDefinition(t *testing.T) {
	_, sid, err := taskIdentity()
	if err != nil {
		t.Fatal(err)
	}
	target := `C:\Users\O'Brien\FreeTurn; Desktop\freeturn-desktop.exe`
	script := "$s=New-Object -ComObject Schedule.Service;$s.Connect();" +
		autostartDefinitionScript(target, sid) + "[Console]::Write($d.XmlText)"
	out, err := runPowerShell(script)
	if err != nil {
		t.Fatal(err)
	}
	var task struct {
		Description string `xml:"RegistrationInfo>Description"`
		Principal   struct {
			UserID    string `xml:"UserId"`
			LogonType string
			RunLevel  string
		} `xml:"Principals>Principal"`
		TriggerUser string `xml:"Triggers>LogonTrigger>UserId"`
		Action      struct {
			Command          string
			Arguments        string
			WorkingDirectory string
		} `xml:"Actions>Exec"`
		Settings struct {
			Enabled                    bool
			DisallowStartIfOnBatteries bool
			StopIfGoingOnBatteries     bool
			ExecutionTimeLimit         string
			MultipleInstancesPolicy    string
		}
	}
	// XmlText describes a UTF-16 BSTR; the console transports its text as UTF-8.
	xmlText := strings.Replace(out, `encoding="UTF-16"`, `encoding="UTF-8"`, 1)
	if err := xml.Unmarshal([]byte(xmlText), &task); err != nil {
		t.Fatalf("decode task: %v\n%s", err, out)
	}
	if task.Description != managedLauncher || task.Action.Command != target || task.Action.Arguments != "--autostart" || task.Action.WorkingDirectory != filepath.Dir(target) {
		t.Fatalf("incorrect owned action: %+v", task)
	}
	if task.Principal.UserID != sid || task.TriggerUser != sid || task.Principal.LogonType != "InteractiveToken" || task.Principal.RunLevel != "HighestAvailable" {
		t.Fatalf("incorrect per-user elevated logon: %+v", task)
	}
	if !task.Settings.Enabled || task.Settings.DisallowStartIfOnBatteries || task.Settings.StopIfGoingOnBatteries || task.Settings.ExecutionTimeLimit != "PT0S" || task.Settings.MultipleInstancesPolicy != "IgnoreNew" {
		t.Fatalf("incorrect task settings: %+v", task.Settings)
	}
}

func TestWindowsAutostartOwnershipChecksArguments(t *testing.T) {
	_, sid, err := taskIdentity()
	if err != nil {
		t.Fatal(err)
	}
	target := `C:\Users\O'Brien\FreeTurn\freeturn-desktop.exe`
	script := "$s=New-Object -ComObject Schedule.Service;$s.Connect();" + autostartDefinitionScript(target, sid) +
		"$t=[pscustomobject]@{Definition=$d;Enabled=$true};" +
		"if(" + managedAutostartCondition(target) + "){[Console]::Write('1')}else{[Console]::Write('0')};" +
		"$a.Arguments='--unrelated';" +
		"if(" + managedAutostartCondition(target) + "){[Console]::Write('1')}else{[Console]::Write('0')}"
	out, err := runPowerShell(script)
	if err != nil || out != "10" {
		t.Fatalf("ownership check got %q, %v", out, err)
	}
}

func TestWindowsAutostartMissingTaskIsNotAnError(t *testing.T) {
	name := fmt.Sprintf("FreeTurn-Test-Missing-%d", time.Now().UnixNano())
	out, err := runPowerShell(autostartLookupScript(name) + "if($null -eq $t){[Console]::Write('missing')}")
	if err != nil || out != "missing" {
		t.Fatalf("missing task got %q, %v", out, err)
	}
}

func TestWindowsInstallPathAndUnmanagedFiles(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	target, err := InstalledPath()
	if err != nil || filepath.Base(target) != "freeturn-desktop.exe" {
		t.Fatalf("target %q: %v", target, err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("user-owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(); err == nil {
		t.Fatal("overwrote an unmanaged file")
	}
	if err := Uninstall(); err == nil {
		t.Fatal("uninstalled an unmanaged directory")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "user-owned" {
		t.Fatal("unmanaged file changed")
	}
}

func TestWindowsPowerShellPathQuoting(t *testing.T) {
	path := `C:\Users\O'Brien\FreeTurn;Remove-Item.txt`
	if got := psLiteral(path); got != `'C:\Users\O''Brien\FreeTurn;Remove-Item.txt'` {
		t.Fatalf("literal: %s", got)
	}
	script := shortcutScript(path, "DesktopDirectory")
	if !strings.Contains(script, psLiteral(path)) || !strings.Contains(script, managedLauncher) {
		t.Fatal("shortcut lacks escaped path or ownership check")
	}
}

func TestWindowsUpdateAcknowledgementIsConstrainedAndExclusive(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	target, _ := InstalledPath()
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	stage, err := os.MkdirTemp(filepath.Dir(target), ".freeturn-update-")
	if err != nil {
		t.Fatal(err)
	}
	ack := filepath.Join(stage, "restart-ready")
	t.Setenv("FREETURN_UPDATE_ACK_FILE", ack)
	AcknowledgeUpdate()
	data, err := os.ReadFile(ack)
	if err != nil || string(data) != "\x01" {
		t.Fatalf("ack: %q, %v", data, err)
	}
	if err := os.WriteFile(ack, []byte("do-not-overwrite"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FREETURN_UPDATE_ACK_FILE", ack)
	AcknowledgeUpdate()
	data, _ = os.ReadFile(ack)
	if string(data) != "do-not-overwrite" {
		t.Fatal("acknowledgement overwrote an existing file")
	}
	outside := filepath.Join(t.TempDir(), "restart-ready")
	t.Setenv("FREETURN_UPDATE_ACK_FILE", outside)
	AcknowledgeUpdate()
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatal("wrote acknowledgement outside the installation stage")
	}
}
