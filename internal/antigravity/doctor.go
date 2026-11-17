package antigravity

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ErrChecksFailed is returned by Doctor when a required check fails.
var ErrChecksFailed = errors.New("one or more checks failed")

// Result is one doctor line.
type Result struct {
	Label  string
	OK     bool
	Warn   bool
	Detail string
}

// CheckProxy verifies that the upstream proxy can reach a Google host.
func CheckProxy(u *url.URL, timeout time.Duration) (time.Duration, error) {
	transport := &http.Transport{Proxy: http.ProxyURL(u)}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: timeout}
	start := time.Now()
	resp, err := client.Get("https://www.gstatic.com/generate_204")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusNoContent {
		return 0, fmt.Errorf("unexpected status %s", resp.Status)
	}
	return time.Since(start), nil
}

// Doctor inspects the installed pieces and the live language_server.
func Doctor(opts Options, u *url.URL) ([]Result, error) {
	var results []Result
	add := func(label string, ok, warn bool, format string, args ...any) {
		results = append(results, Result{Label: label, OK: ok, Warn: warn, Detail: fmt.Sprintf(format, args...)})
	}

	if elapsed, err := CheckProxy(u, 8*time.Second); err != nil {
		add("proxy", false, false, "%s unreachable: %v", u.String(), err)
	} else {
		add("proxy", true, false, "%s reachable (gstatic 204 in %s)", u.String(), elapsed.Round(time.Millisecond))
	}

	hasZsh := false
	if data, err := os.ReadFile(opts.ZshrcPath); err != nil {
		add("zshrc", false, true, "cannot read %s: %v", DisplayPath(opts.ZshrcPath), err)
	} else if strings.Contains(string(data), BlockStart) {
		hasZsh = true
		add("zshrc", true, false, "managed block present in %s", DisplayPath(opts.ZshrcPath))
	} else {
		add("zshrc", false, true, "managed block missing in %s (run `install`)", DisplayPath(opts.ZshrcPath))
	}

	hasWrapper := WrapperExists(opts.AppDir)
	if hasWrapper {
		add("wrapper", true, false, "%s", DisplayPath(WrapperDir(opts.AppDir)))
	} else {
		add("wrapper", false, true, "missing %s (run `install`)", DisplayPath(WrapperDir(opts.AppDir)))
	}

	if !hasZsh && !hasWrapper {
		results = append(results, Result{Label: "install", Detail: "nothing is configured: run `proxy-antigravity install`"})
	}

	appPIDs := RunningPIDs(opts.AppPath)
	if len(appPIDs) == 0 {
		add("process", true, true, "Antigravity is not running")
	} else {
		lsPIDs := LanguageServerPIDs(opts.AppPath)
		switch {
		case len(lsPIDs) == 0:
			add("process", true, true, "Antigravity running (pid %s), language_server not up yet", joinInts(appPIDs))
		default:
			conns := establishedTo(lsPIDs, u)
			if conns > 0 {
				add("process", true, false, "language_server (pid %s) has %d connection(s) to %s", joinInts(lsPIDs), conns, proxyHostPort(u))
			} else {
				add("process", false, true, "language_server (pid %s) has no established connection to %s; restart via the wrapper if it was launched directly", joinInts(lsPIDs), proxyHostPort(u))
			}
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		logPath := filepath.Join(home, "Library", "Logs", "Antigravity", "language_server.log")
		if lines, err := tailLines(logPath, 300); err == nil {
			bad := 0
			for _, line := range lines {
				if strings.Contains(line, "Bad Gateway") {
					bad++
				}
			}
			if bad == 0 {
				add("log", true, true, "no proxy 502s in the last 300 language_server log lines")
			} else {
				add("log", false, true, "%d \"Bad Gateway\" line(s) in the last 300 log lines; check the proxy node/rules", bad)
			}
		}
	}

	for _, r := range results {
		if !r.OK && !r.Warn {
			return results, ErrChecksFailed
		}
	}
	return results, nil
}

func proxyHostPort(u *url.URL) string {
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return host + ":" + port
}

// establishedTo counts lsof connections from pids to the proxy port.
func establishedTo(pids []int, u *url.URL) int {
	port := u.Port()
	if port == "" {
		port = "80"
	}
	count := 0
	for _, pid := range pids {
		out, err := exec.Command("lsof", "-nP", "-a", "-p", strconv.Itoa(pid), "-iTCP", "-sTCP:ESTABLISHED").Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "->") && strings.Contains(line, ":"+port+" ") {
				count++
			}
		}
	}
	return count
}

func tailLines(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	const maxTail = 256 * 1024
	size := info.Size()
	offset := int64(0)
	if size > maxTail {
		offset = size - maxTail
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	buf := make([]byte, size-offset)
	if _, err := io.ReadFull(f, buf); err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(string(buf), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}
