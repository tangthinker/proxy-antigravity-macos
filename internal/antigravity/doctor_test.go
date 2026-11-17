package antigravity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTailLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log")
	var b strings.Builder
	for i := 0; i < 1000; i++ {
		b.WriteString("line ")
		b.WriteByte(byte('0' + i%10))
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, err := tailLines(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 10 {
		t.Fatalf("got %d lines, want 10", len(lines))
	}
	if lines[9] != "line 9" {
		t.Fatalf("last line = %q", lines[9])
	}
}

func TestTailLinesMissingFile(t *testing.T) {
	if _, err := tailLines(filepath.Join(t.TempDir(), "nope"), 10); err == nil {
		t.Fatal("expected an error")
	}
}

func TestProxyHostPort(t *testing.T) {
	tests := map[string]string{
		"http://127.0.0.1:1080": "127.0.0.1:1080",
		"http://proxy.local":    "proxy.local:80",
		"https://proxy.local":   "proxy.local:443",
		"socks5://10.0.0.1:1":   "10.0.0.1:1",
	}
	for in, want := range tests {
		u := mustProxy(t, in)
		if got := proxyHostPort(u); got != want {
			t.Errorf("proxyHostPort(%s) = %q, want %q", in, got, want)
		}
	}
}
