//go:build windows

package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/samosvalishe/free-turn-proxy/desktop/locale"
	"golang.org/x/sys/windows"
)

const Supported = true
const CanInstallUpdates = true
const managedLauncher = "FreeTurn managed launcher"

func nativeSystemLanguage() string {
	var name [85]uint16
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")
	if n, _, _ := proc.Call(uintptr(unsafe.Pointer(&name[0])), uintptr(len(name))); n != 0 {
		return locale.MatchSystemLanguage(windows.UTF16ToString(name[:]))
	}
	return "en"
}

func InstalledPath() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if !filepath.IsAbs(base) {
		return "", errors.New("LOCALAPPDATA is not an absolute path")
	}
	return filepath.Join(base, "Programs", "FreeTurn", "freeturn-desktop.exe"), nil
}

func psLiteral(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }

// Encode scripts rather than interpolate paths into a cmd.exe command line.
func runPowerShell(script string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return runPowerShellContext(ctx, script)
}

func runPowerShellContext(ctx context.Context, script string) (string, error) {
	units := utf16.Encode([]rune("$ErrorActionPreference='Stop';$ProgressPreference='SilentlyContinue';[Console]::OutputEncoding=[Text.UTF8Encoding]::new($false);" + script))
	data := make([]byte, len(units)*2)
	for i, unit := range units {
		binary.LittleEndian.PutUint16(data[i*2:], unit)
	}
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(data))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("Windows integration: %w", ctx.Err())
		}
		return "", fmt.Errorf("Windows integration: %w: %s", err, strings.TrimSpace(stderr.String()+stdout.String()))
	}
	// EncodedCommand may emit CLIXML diagnostics on stderr even on success.
	// They are not part of the requested value (e.g. the autostart '1'/'0').
	return strings.TrimSpace(stdout.String()), nil
}

func copyExecutable(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.CreateTemp(filepath.Dir(target), ".freeturn-*.exe")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name())
	_, copyErr := io.Copy(out, in)
	if err := errors.Join(copyErr, out.Sync(), out.Close()); err != nil {
		return err
	}
	return os.Rename(out.Name(), target)
}

func Install() (string, error) {
	target, err := InstalledPath()
	if err != nil {
		return "", err
	}
	source, err := os.Executable()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return "", err
	}
	info, err := os.Lstat(filepath.Dir(target))
	if err != nil || !info.IsDir() {
		return "", errors.New("unsafe installation directory")
	}
	marker := filepath.Join(filepath.Dir(target), "freeturn-managed.txt")
	if !strings.EqualFold(filepath.Clean(source), filepath.Clean(target)) {
		if _, err := os.Lstat(target); err == nil {
			data, markerErr := os.ReadFile(marker)
			if markerErr != nil || string(data) != managedLauncher {
				return "", errors.New("existing installation is not managed by FreeTurn")
			}
		} else if !os.IsNotExist(err) {
			return "", err
		}
		if err := copyExecutable(source, target); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(marker, []byte(managedLauncher), 0o600); err != nil {
		return "", err
	}
	_, err = runPowerShell(shortcutScript(target, "StartMenu"))
	return target, err
}

func shortcutScript(target, folder string) string {
	return "$dir=[Environment]::GetFolderPath(" + psLiteral(folder) + ");" +
		"$path=Join-Path $dir 'FreeTurn.lnk';$s=New-Object -ComObject WScript.Shell;$l=$s.CreateShortcut($path);" +
		"if((Test-Path -LiteralPath $path)-and $l.Description -ne " + psLiteral(managedLauncher) + "){throw 'Shortcut is not managed by FreeTurn'};" +
		"$l.TargetPath=" + psLiteral(target) + ";$l.WorkingDirectory=" + psLiteral(filepath.Dir(target)) +
		";$l.Description=" + psLiteral(managedLauncher) + ";$l.Save();[Console]::Write($path)"
}

func CreateDesktopShortcut() (string, error) {
	target, err := Install()
	if err != nil {
		return "", err
	}
	return runPowerShell(shortcutScript(target, "DesktopDirectory"))
}

func taskIdentity() (string, string, error) {
	u, err := user.Current()
	if err != nil {
		return "", "", err
	}
	return "FreeTurnDesktop-" + u.Uid, u.Uid, nil
}

func AutostartEnabled() (bool, error) {
	name, _, err := taskIdentity()
	if err != nil {
		return false, err
	}
	target, err := InstalledPath()
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := runPowerShellContext(ctx, autostartLookupScript(name)+
		"if("+managedAutostartCondition(target)+" -and $t.Enabled){[Console]::Write('1')}else{[Console]::Write('0')}")
	return out == "1", err
}

// Use the Task Scheduler COM API rather than the ScheduledTasks CIM module.
// COM avoids module/WMI startup on every settings refresh, and reports missing
// tasks separately from access or service failures.
func autostartLookupScript(name string) string {
	return "$s=New-Object -ComObject Schedule.Service;$s.Connect();$f=$s.GetFolder('\\');$t=$null;" +
		"try{$t=$f.GetTask(" + psLiteral(name) + ")}catch{if($_.Exception.GetBaseException().HResult -ne -2147024894){throw}};"
}

func managedAutostartCondition(target string) string {
	return "$t -and $t.Definition.RegistrationInfo.Description -eq " + psLiteral(managedLauncher) +
		" -and $t.Definition.Actions.Count -eq 1 -and $t.Definition.Actions.Item(1).Type -eq 0" +
		" -and $t.Definition.Actions.Item(1).Path -eq " + psLiteral(target) +
		" -and $t.Definition.Actions.Item(1).Arguments -eq '--autostart'"
}

func autostartDefinitionScript(target, sid string) string {
	return "$d=$s.NewTask(0);$d.RegistrationInfo.Description=" + psLiteral(managedLauncher) + ";" +
		"$d.Principal.UserId=" + psLiteral(sid) + ";$d.Principal.LogonType=3;$d.Principal.RunLevel=1;" +
		"$g=$d.Triggers.Create(9);$g.UserId=" + psLiteral(sid) + ";$g.Enabled=$true;" +
		"$a=$d.Actions.Create(0);$a.Path=" + psLiteral(target) + ";$a.Arguments='--autostart';" +
		"$a.WorkingDirectory=" + psLiteral(filepath.Dir(target)) + ";" +
		"$d.Settings.Enabled=$true;$d.Settings.AllowDemandStart=$true;$d.Settings.StartWhenAvailable=$true;" +
		"$d.Settings.DisallowStartIfOnBatteries=$false;$d.Settings.StopIfGoingOnBatteries=$false;" +
		"$d.Settings.ExecutionTimeLimit='PT0S';$d.Settings.MultipleInstances=2;"
}

func autostartMutationScript(name, target, sid string, enabled bool) string {
	script := autostartLookupScript(name) +
		"if($t -and !(" + managedAutostartCondition(target) + ")){throw 'Task is not managed by FreeTurn'};"
	if !enabled {
		return script + "if($t){$f.DeleteTask(" + psLiteral(name) + ",0)}"
	}
	return script + autostartDefinitionScript(target, sid) +
		"$null=$f.RegisterTaskDefinition(" + psLiteral(name) + ",$d,6," + psLiteral(sid) + ",$null,3,$null)"
}

func SetAutostart(enabled bool) error {
	name, sid, err := taskIdentity()
	if err != nil {
		return err
	}
	target, err := InstalledPath()
	if err != nil {
		return err
	}
	if enabled {
		source, err := os.Executable()
		if err != nil {
			return err
		}
		// An installed app needs no copy or Start Menu shortcut rewrite just to
		// enable autostart. Portable builds still install a stable launch target.
		if !strings.EqualFold(filepath.Clean(source), filepath.Clean(target)) {
			if _, err := Install(); err != nil {
				return err
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = runPowerShellContext(ctx, autostartMutationScript(name, target, sid, enabled))
	return err
}

func VPNHelperAvailable() bool { return windows.GetCurrentProcessToken().IsElevated() }
func ConfigureVPNHelper(bool) error {
	return errors.New("Windows uses native Wintun; launch the application as administrator")
}

// The CLI exits after launching this worker. Delete only known app files;
// profiles, keys, other VPNs and the shared Wintun driver are deliberately kept.
func Uninstall() error {
	target, err := InstalledPath()
	if err != nil {
		return err
	}
	marker := filepath.Join(filepath.Dir(target), "freeturn-managed.txt")
	data, err := os.ReadFile(marker)
	if err != nil || string(data) != managedLauncher {
		return errors.New("refusing to uninstall an unmanaged directory")
	}
	_, err = runPowerShell("$p=Get-CimInstance Win32_Process -Filter \"Name='freeturn-desktop.exe'\";" +
		"if($p | Where-Object {$_.ExecutablePath -eq " + psLiteral(target) + " -and $_.ProcessId -ne " + fmt.Sprint(os.Getpid()) + "}){throw 'Close the running FreeTurn client before uninstalling'}")
	if err != nil {
		return err
	}
	if err := SetAutostart(false); err != nil {
		return err
	}
	_, err = runPowerShell("$s=New-Object -ComObject WScript.Shell;foreach($f in @('StartMenu','DesktopDirectory')){" +
		"$p=Join-Path ([Environment]::GetFolderPath($f)) 'FreeTurn.lnk';if(Test-Path -LiteralPath $p){$l=$s.CreateShortcut($p);" +
		"if($l.Description -eq " + psLiteral(managedLauncher) + " -and $l.TargetPath -eq " + psLiteral(target) + "){Remove-Item -LiteralPath $p}}}")
	if err != nil {
		return err
	}
	worker, err := os.CreateTemp("", "freeturn-uninstall-*.ps1")
	if err != nil {
		return err
	}
	script := fmt.Sprintf("Wait-Process -Id %d -ErrorAction SilentlyContinue;", os.Getpid()) +
		"Remove-Item -LiteralPath " + psLiteral(target) + " -ErrorAction Stop;Remove-Item -LiteralPath " + psLiteral(marker) + ";" +
		"Remove-Item -LiteralPath $PSCommandPath -ErrorAction SilentlyContinue"
	if _, err := worker.WriteString(script); err != nil {
		worker.Close()
		return err
	}
	if err := worker.Close(); err != nil {
		return err
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", worker.Name())
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
