// Package antigravity knows how Google Antigravity (the Electron hub and its
// bundled Go language_server) talks to the network, and how to route that
// traffic through an upstream proxy without enabling TUN/virtual-NIC mode.
package antigravity

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultProxyURL is the usual local mixed HTTP/SOCKS port. Override with
	// ANTIGRAVITY_PROXY or the --proxy flag.
	DefaultProxyURL = "http://127.0.0.1:1080"

	DefaultAppPath = "/Applications/Antigravity.app"
	DefaultAppName = "Antigravity Proxy"
	BundleID       = "dev.tangthinker.antigravity-proxy"

	// ProxyEnvVar lets the proxy be configured once for install/run/doctor.
	ProxyEnvVar = "ANTIGRAVITY_PROXY"

	MainBinaryRel = "Contents/MacOS/Antigravity"
	LSBinaryRel   = "Contents/Resources/bin/language_server"
	WrapperRel    = "Contents/MacOS/launch"

	// Markers delimit the managed block in the user's zsh rc file.
	BlockStart = "# >>> antigravity-proxy >>>"
	BlockEnd   = "# <<< antigravity-proxy <<<"
)

// NoProxy is exported to Antigravity's language_server (and its children).
// Loopback must stay direct: the UI itself is served from https://127.0.0.1.
const NoProxy = "localhost,127.0.0.1,::1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,100.64.0.0/10,169.254.0.0/16,.local,.ts.net"

// BypassList is the Chromium --proxy-bypass-list. The local hub UI is loaded
// from https://127.0.0.1:<language_server port>/, so loopback must be bypassed
// or the window fails with ERR_TUNNEL_CONNECTION_FAILED.
var BypassList = []string{
	"127.0.0.1", "localhost", "::1",
	"*.local", "*.ts.net",
	"10.*", "172.16.*", "192.168.*", "100.64.*",
}

// Options are shared by install, run, doctor and uninstall.
type Options struct {
	ProxyURL  string
	AppPath   string
	ZshrcPath string
	AppDir    string

	NoZshrc   bool
	NoWrapper bool
	Restart   bool
	DryRun    bool
}

func DefaultOptions() Options {
	home, _ := os.UserHomeDir()
	proxy := strings.TrimSpace(os.Getenv(ProxyEnvVar))
	if proxy == "" {
		proxy = DefaultProxyURL
	}
	return Options{
		ProxyURL:  proxy,
		AppPath:   DefaultAppPath,
		ZshrcPath: filepath.Join(home, ".zshrc"),
		AppDir:    filepath.Join(home, "Applications"),
	}
}

// Normalize expands ~, fills defaults and validates the proxy URL.
func (o *Options) Normalize() (*url.URL, error) {
	o.AppPath = ExpandHome(o.AppPath)
	o.ZshrcPath = ExpandHome(o.ZshrcPath)
	o.AppDir = ExpandHome(o.AppDir)
	if o.AppDir == "" {
		home, _ := os.UserHomeDir()
		o.AppDir = filepath.Join(home, "Applications")
	}
	u, err := NormalizeProxy(o.ProxyURL)
	if err != nil {
		return nil, err
	}
	o.ProxyURL = u.String()
	return u, nil
}

// NormalizeProxy accepts "127.0.0.1:1080", "http://host:port" or
// "socks5://host:port" and returns a URL safe for both Go and Chromium.
func NormalizeProxy(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = DefaultProxyURL
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy %q: %w", raw, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	case "socks5", "socks5h":
		u.Scheme = "socks5"
	default:
		return nil, fmt.Errorf("unsupported proxy scheme %q: use http, https or socks5", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("proxy %q has no host", raw)
	}
	if u.Path != "" && u.Path != "/" {
		return nil, fmt.Errorf("proxy %q must not have a path", raw)
	}
	return &url.URL{Scheme: u.Scheme, Host: u.Host, User: u.User}, nil
}

// MainBinary returns the Antigravity executable of the configured bundle.
func (o Options) MainBinary() (string, error) {
	return MainBinaryAt(o.AppPath)
}

// LanguageServerBinary returns the bundled Go backend path (may not exist).
func (o Options) LanguageServerBinary() string {
	return filepath.Join(o.AppPath, LSBinaryRel)
}

func MainBinaryAt(appPath string) (string, error) {
	p := filepath.Join(appPath, MainBinaryRel)
	info, err := os.Stat(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("Antigravity not found at %s (is it installed?): %w", p, err)
		}
		return "", fmt.Errorf("stat %s: %w", p, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory, not an executable", p)
	}
	return p, nil
}

func ExpandHome(p string) string {
	if p == "" {
		return p
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		if p == "~" {
			return home
		}
		return filepath.Join(home, p[2:])
	}
	return p
}

// ShellQuote returns s quoted for /bin/sh (single quotes, with the usual
// '\” escape for embedded single quotes).
func ShellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// DisplayPath shortens $HOME to ~ for human output.
func DisplayPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}
