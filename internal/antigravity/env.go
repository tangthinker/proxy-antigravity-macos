package antigravity

import (
	"net/url"
	"strings"
)

// ProxyEnv is the environment exported to Antigravity's language_server.
// The Go backend honours HTTP(S)_PROXY/ALL_PROXY but not the macOS system
// proxy, which is why TUN mode is normally the only thing that "just works".
func ProxyEnv(u *url.URL) []string {
	p := u.String()
	return []string{
		"HTTP_PROXY=" + p,
		"HTTPS_PROXY=" + p,
		"ALL_PROXY=" + p,
		"NO_PROXY=" + NoProxy,
	}
}

// ChromiumArgs are passed to the Antigravity binary so the Chromium side
// (embedded browser, webviews, updater through the network service) uses the
// same proxy. Loopback is bypassed explicitly for the local hub UI.
func ChromiumArgs(u *url.URL) []string {
	return []string{
		"--proxy-server=" + u.String(),
		"--proxy-bypass-list=" + strings.Join(BypassList, ";"),
	}
}

// MergeEnv returns base with every override key removed and overrides
// appended, so duplicate keys can never shadow the values we inject.
func MergeEnv(base, overrides []string) []string {
	keys := make(map[string]struct{}, len(overrides))
	for _, kv := range overrides {
		if i := strings.IndexByte(kv, '='); i > 0 {
			keys[strings.ToUpper(kv[:i])] = struct{}{}
		}
	}
	out := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		if i := strings.IndexByte(kv, '='); i > 0 {
			if _, drop := keys[strings.ToUpper(kv[:i])]; drop {
				continue
			}
		}
		out = append(out, kv)
	}
	return append(out, overrides...)
}
