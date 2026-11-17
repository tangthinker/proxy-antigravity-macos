package antigravity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertBlockIdempotent(t *testing.T) {
	u := mustProxy(t, "127.0.0.1:1080")
	block := ProxyBlock(u)

	out, changed := UpsertBlock("", block)
	if !changed || out != block {
		t.Fatalf("upsert on empty: changed=%v out=%q", changed, out)
	}
	if again, changed := UpsertBlock(out, block); changed || again != out {
		t.Fatalf("expected idempotent upsert, changed=%v", changed)
	}

	withPrefix := "# my zshrc\nexport PATH=/bin\n"
	out2, changed := UpsertBlock(withPrefix, block)
	if !changed {
		t.Fatal("expected change")
	}
	if !strings.Contains(out2, "# my zshrc") || !strings.Contains(out2, BlockStart) {
		t.Fatalf("prefix or block missing:\n%s", out2)
	}
	if again, changed := UpsertBlock(out2, block); changed || again != out2 {
		t.Fatalf("expected idempotent upsert with prefix, changed=%v\n%s", changed, again)
	}
}

func TestUpsertBlockReplacesProxy(t *testing.T) {
	first := ProxyBlock(mustProxy(t, "127.0.0.1:1080"))
	second := ProxyBlock(mustProxy(t, "socks5://127.0.0.1:1086"))
	out, _ := UpsertBlock("# head\n", first)
	out, changed := UpsertBlock(out, second)
	if !changed {
		t.Fatal("expected change when proxy changes")
	}
	if strings.Count(out, BlockStart) != 1 || strings.Count(out, BlockEnd) != 1 {
		t.Fatalf("block duplicated:\n%s", out)
	}
	if strings.Contains(out, "1080") {
		t.Fatalf("old proxy still present:\n%s", out)
	}
	if !strings.Contains(out, "socks5://127.0.0.1:1086") {
		t.Fatalf("new proxy missing:\n%s", out)
	}
}

func TestRemoveBlock(t *testing.T) {
	block := ProxyBlock(mustProxy(t, "127.0.0.1:1080"))
	out, _ := UpsertBlock("# head\nexport A=1\n", block)
	removed, changed := RemoveBlock(out)
	if !changed {
		t.Fatal("expected removal")
	}
	if strings.Contains(removed, BlockStart) || strings.Contains(removed, "HTTPS_PROXY") {
		t.Fatalf("block survived:\n%s", removed)
	}
	if !strings.Contains(removed, "export A=1") {
		t.Fatalf("unrelated content lost:\n%s", removed)
	}
	if again, changed := RemoveBlock(removed); changed || again != removed {
		t.Fatalf("second removal should be a no-op, changed=%v", changed)
	}
}

func TestInstallZshrcCreatesBackupAndPreservesMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")
	original := "# my zshrc\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	backup, changed, err := InstallZshrc(path, ProxyBlock(mustProxy(t, "127.0.0.1:1080")))
	if err != nil {
		t.Fatal(err)
	}
	if !changed || backup == "" {
		t.Fatalf("changed=%v backup=%q", changed, backup)
	}
	backupData, err := os.ReadFile(backup)
	if err != nil || string(backupData) != original {
		t.Fatalf("backup content = %q, err=%v", string(backupData), err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
	if _, changed, err := InstallZshrc(path, ProxyBlock(mustProxy(t, "127.0.0.1:1080"))); err != nil || changed {
		t.Fatalf("second install: changed=%v err=%v", changed, err)
	}
}

func TestInstallZshrcOnMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zshrc")
	backup, changed, err := InstallZshrc(path, ProxyBlock(mustProxy(t, "127.0.0.1:1080")))
	if err != nil {
		t.Fatal(err)
	}
	if !changed || backup != "" {
		t.Fatalf("changed=%v backup=%q", changed, backup)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), BlockStart) {
		t.Fatalf("block missing:\n%s", data)
	}
}

func TestUninstallZshrc(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zshrc")
	block := ProxyBlock(mustProxy(t, "127.0.0.1:1080"))
	if _, _, err := InstallZshrc(path, block); err != nil {
		t.Fatal(err)
	}
	backup, changed, err := UninstallZshrc(path)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || backup == "" {
		t.Fatalf("changed=%v backup=%q", changed, backup)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), BlockStart) {
		t.Fatalf("block survived:\n%s", data)
	}
}

func TestProxyBlockOnlyTargetsAntigravityProbe(t *testing.T) {
	block := ProxyBlock(mustProxy(t, "127.0.0.1:1080"))
	if !strings.Contains(block, "ANTIGRAVITY_RESOLVING_ENVIRONMENT") {
		t.Fatal("block must be conditional on the probe env var")
	}
	if !strings.Contains(block, "https://127.0.0.1") && !strings.Contains(block, "127.0.0.1") {
		t.Fatal("block should point at the proxy")
	}
	if !strings.Contains(block, NoProxy) {
		t.Fatalf("NO_PROXY missing:\n%s", block)
	}
}
