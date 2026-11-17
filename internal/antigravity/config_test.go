package antigravity

import (
	"net/url"
	"testing"
)

func mustProxy(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := NormalizeProxy(raw)
	if err != nil {
		t.Fatalf("NormalizeProxy(%q): %v", raw, err)
	}
	return u
}

func TestNormalizeProxy(t *testing.T) {
	tests := []struct {
		in   string
		want string
		err  bool
	}{
		{"127.0.0.1:1080", "http://127.0.0.1:1080", false},
		{"http://127.0.0.1:7890", "http://127.0.0.1:7890", false},
		{"socks5://127.0.0.1:1080", "socks5://127.0.0.1:1080", false},
		{"socks5h://127.0.0.1:1080", "socks5://127.0.0.1:1080", false},
		{"  http://user:pass@127.0.0.1:1080  ", "http://user:pass@127.0.0.1:1080", false},
		{"ftp://127.0.0.1:1080", "", true},
		{"http://127.0.0.1:1080/some/path", "", true},
		{"http://", "", true},
	}
	for _, tc := range tests {
		got, err := NormalizeProxy(tc.in)
		if tc.err {
			if err == nil {
				t.Errorf("NormalizeProxy(%q) = %v, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizeProxy(%q): %v", tc.in, err)
			continue
		}
		if got.String() != tc.want {
			t.Errorf("NormalizeProxy(%q) = %q, want %q", tc.in, got.String(), tc.want)
		}
	}
}

func TestMergeEnvRemovesStaleProxy(t *testing.T) {
	base := []string{
		"PATH=/bin",
		"HTTPS_PROXY=http://old:1",
		"http_proxy=http://old:2",
		"OTHER=1",
	}
	overrides := ProxyEnv(mustProxy(t, "127.0.0.1:1080"))
	got := MergeEnv(base, overrides)
	seen := map[string]string{}
	for _, kv := range got {
		key, value, _ := cut(kv)
		if _, dup := seen[key]; dup {
			t.Fatalf("duplicate key %q in %v", key, got)
		}
		seen[key] = value
	}
	if seen["HTTPS_PROXY"] != "http://127.0.0.1:1080" {
		t.Errorf("HTTPS_PROXY = %q", seen["HTTPS_PROXY"])
	}
	if _, ok := seen["http_proxy"]; ok {
		t.Errorf("lowercase http_proxy should have been dropped: %v", got)
	}
	if seen["PATH"] != "/bin" || seen["OTHER"] != "1" {
		t.Errorf("unrelated env lost: %v", got)
	}
}

func TestShellQuote(t *testing.T) {
	tests := map[string]string{
		"":            "''",
		"abc":         "'abc'",
		"a'b":         `'a'\''b'`,
		`$HOME "x"`:   `'$HOME "x"'`,
		"http://h:80": "'http://h:80'",
	}
	for in, want := range tests {
		if got := ShellQuote(in); got != want {
			t.Errorf("ShellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func cut(kv string) (string, string, bool) {
	for i := 0; i < len(kv); i++ {
		if kv[i] == '=' {
			return kv[:i], kv[i+1:], true
		}
	}
	return kv, "", false
}
