package antigravity

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// RunningPIDs finds Antigravity main processes (helpers excluded).
func RunningPIDs(appPath string) []int {
	mainBinary := filepath.Join(appPath, MainBinaryRel)
	out, err := exec.Command("pgrep", "-f", regexp.QuoteMeta(mainBinary)).Output()
	if err != nil {
		return nil
	}
	return parsePIDs(string(out))
}

// LanguageServerPIDs finds the bundled Go backend.
func LanguageServerPIDs(appPath string) []int {
	ls := filepath.Join(appPath, LSBinaryRel)
	out, err := exec.Command("pgrep", "-f", regexp.QuoteMeta(ls)).Output()
	if err != nil {
		return nil
	}
	return parsePIDs(string(out))
}

func parsePIDs(out string) []int {
	var pids []int
	for _, f := range strings.Fields(out) {
		if pid, err := strconv.Atoi(f); err == nil && pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids
}

func joinInts(nums []int) string {
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ", ")
}

// QuitAntigravity asks the app to quit and escalates to SIGTERM/SIGKILL.
// AppleScript quit fails with -10004 for this Electron app, so give it only a
// short chance before falling back to signals.
func QuitAntigravity(appPath string, timeout time.Duration) error {
	if len(RunningPIDs(appPath)) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	_ = exec.CommandContext(ctx, "osascript", "-e", `tell application "Antigravity" to quit`).Run()
	cancel()
	if waitGone(appPath, 2*time.Second) {
		return nil
	}
	for _, pid := range RunningPIDs(appPath) {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
	if waitGone(appPath, timeout) {
		return nil
	}
	for _, pid := range RunningPIDs(appPath) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	if waitGone(appPath, 3*time.Second) {
		return nil
	}
	return fmt.Errorf("Antigravity is still running after SIGKILL")
}

func waitGone(appPath string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if len(RunningPIDs(appPath)) == 0 {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(150 * time.Millisecond)
	}
}

// Launch starts Antigravity through the generated wrapper app via
// LaunchServices. Going through `open` matters: the wrapper is a normal GUI
// bundle, so the app gets a proper Aqua session (a direct exec from a
// background shell can die before Electron initialises). The wrapper already
// contains the proxy environment and Chromium switches.
func Launch(opts Options, u *url.URL) error {
	if _, err := MainBinaryAt(opts.AppPath); err != nil {
		return err
	}
	if pids := RunningPIDs(opts.AppPath); len(pids) > 0 {
		if !opts.Restart {
			return fmt.Errorf("Antigravity is already running (pid %s); quit it first or pass --restart", joinInts(pids))
		}
		if err := QuitAntigravity(opts.AppPath, 8*time.Second); err != nil {
			return err
		}
	}
	app, err := CreateWrapper(opts.AppDir, u, opts.AppPath)
	if err != nil {
		return err
	}
	if out, err := exec.Command("open", "-n", app).CombinedOutput(); err != nil {
		return fmt.Errorf("open %s: %v: %s", app, err, strings.TrimSpace(string(out)))
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if pids := RunningPIDs(opts.AppPath); len(pids) > 0 {
			fmt.Printf("launched Antigravity (pid %s)\n  proxy:  %s\n  bundle: %s\n", joinInts(pids), u.String(), DisplayPath(app))
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("Antigravity did not start within 15s; see ~/Library/Logs/Antigravity/main.log")
}

// PrintLaunch shows exactly what the wrapper will export/execute (--dry-run).
func PrintLaunch(opts Options, u *url.URL) error {
	mainBinary, err := MainBinaryAt(opts.AppPath)
	if err != nil {
		return err
	}
	fmt.Printf("bundle: %s\n", DisplayPath(WrapperDir(opts.AppDir)))
	fmt.Printf("binary: %s\n", mainBinary)
	for _, arg := range ChromiumArgs(u) {
		fmt.Printf("arg:    %s\n", arg)
	}
	for _, kv := range ProxyEnv(u) {
		key, value, _ := strings.Cut(kv, "=")
		fmt.Printf("env:    %s=%s\n", key, ShellQuote(value))
	}
	return nil
}
