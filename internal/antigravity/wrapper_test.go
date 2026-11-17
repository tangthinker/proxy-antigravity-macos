package antigravity

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fakeAntigravity(t *testing.T) string {
	t.Helper()
	app := filepath.Join(t.TempDir(), "Antigravity.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(app, "Contents", "Resources", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(app, MainBinaryRel)
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, LSBinaryRel), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Contents", "Resources", "icon.icns"), []byte("icon"), 0o644); err != nil {
		t.Fatal(err)
	}
	return app
}

func TestCreateWrapper(t *testing.T) {
	app := fakeAntigravity(t)
	appDir := t.TempDir()
	u := mustProxy(t, "127.0.0.1:1080")

	dir, err := CreateWrapper(appDir, u, app)
	if err != nil {
		t.Fatal(err)
	}
	if dir != WrapperDir(appDir) {
		t.Fatalf("dir = %q", dir)
	}
	if !WrapperExists(appDir) {
		t.Fatal("WrapperExists = false")
	}

	scriptPath := WrapperExe(appDir)
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(script)
	for _, want := range []string{
		"export HTTPS_PROXY='http://127.0.0.1:1080'",
		"export NO_PROXY='localhost,127.0.0.1,::1",
		"--proxy-server=http://127.0.0.1:1080",
		"--proxy-bypass-list=",
		"127.0.0.1;localhost;::1",
		"exec '" + filepath.Join(app, MainBinaryRel) + "'",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("wrapper script missing %q:\n%s", want, s)
		}
	}
	if out, err := exec.Command("sh", "-n", scriptPath).CombinedOutput(); err != nil {
		t.Fatalf("wrapper script has a syntax error: %v\n%s", err, out)
	}

	plist, err := os.ReadFile(filepath.Join(dir, "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plist), BundleID) {
		t.Fatalf("Info.plist missing bundle id:\n%s", plist)
	}
	icon, err := os.ReadFile(filepath.Join(dir, "Contents", "Resources", "AppIcon.icns"))
	if err != nil || string(icon) != "icon" {
		t.Fatalf("icon not copied: %v %q", err, icon)
	}

	// Rebuild should not fail and should not leave nested bundles behind.
	if _, err := CreateWrapper(appDir, u, app); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveWrapper(t *testing.T) {
	app := fakeAntigravity(t)
	appDir := t.TempDir()
	if _, err := CreateWrapper(appDir, mustProxy(t, "127.0.0.1:1080"), app); err != nil {
		t.Fatal(err)
	}
	removed, err := RemoveWrapper(appDir)
	if err != nil || !removed {
		t.Fatalf("removed=%v err=%v", removed, err)
	}
	if WrapperExists(appDir) {
		t.Fatal("wrapper still present")
	}
	if removed, err := RemoveWrapper(appDir); err != nil || removed {
		t.Fatalf("second remove: removed=%v err=%v", removed, err)
	}
}

func TestWrapperScriptQuotesPathsWithSpaces(t *testing.T) {
	base := filepath.Join(t.TempDir(), "My Apps")
	app := filepath.Join(base, "Antigravity.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(app, MainBinaryRel)
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	appDir := filepath.Join(base, "Applications")
	if _, err := CreateWrapper(appDir, mustProxy(t, "127.0.0.1:1080"), app); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(WrapperExe(appDir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), "'"+bin+"'") {
		t.Fatalf("binary path not quoted:\n%s", script)
	}
	if out, err := exec.Command("sh", "-n", WrapperExe(appDir)).CombinedOutput(); err != nil {
		t.Fatalf("syntax error: %v\n%s", err, out)
	}
}
