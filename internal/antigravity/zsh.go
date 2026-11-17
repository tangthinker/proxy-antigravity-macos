package antigravity

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ProxyBlock renders the managed rc block. It is deliberately conditional:
// Antigravity spawns `zsh -ilc` to read the login-shell environment before
// starting language_server, and sets ANTIGRAVITY_RESOLVING_ENVIRONMENT=1 for
// that probe only. Regular shells never see the proxy.
func ProxyBlock(u *url.URL) string {
	lines := []string{
		BlockStart,
		"# Managed by proxy-antigravity. The proxy is exported only for the",
		"# login-shell environment probe that Antigravity runs before spawning",
		"# its Go language_server, so regular shells and other apps stay direct.",
		`if [[ "${ANTIGRAVITY_RESOLVING_ENVIRONMENT:-}" == "1" ]]; then`,
		"  export HTTPS_PROXY=" + ShellQuote(u.String()),
		`  export HTTP_PROXY="$HTTPS_PROXY"`,
		`  export ALL_PROXY="$HTTPS_PROXY"`,
		"  export NO_PROXY=" + ShellQuote(NoProxy),
		"fi",
		BlockEnd,
	}
	return strings.Join(lines, "\n") + "\n"
}

// UpsertBlock installs or replaces the managed block. It is idempotent: a
// second call with the same block reports changed=false.
func UpsertBlock(content, block string) (string, bool) {
	cleaned := strings.TrimRight(stripBlock(content), "\n")
	var out string
	if cleaned == "" {
		out = block
	} else {
		out = cleaned + "\n\n" + block
	}
	if out == content {
		return content, false
	}
	return out, true
}

// RemoveBlock removes the managed block, leaving the rest of the file intact.
func RemoveBlock(content string) (string, bool) {
	out := stripBlock(content)
	if out == content {
		return content, false
	}
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out, true
}

func stripBlock(content string) string {
	start := strings.Index(content, BlockStart)
	if start < 0 {
		return content
	}
	rel := strings.Index(content[start:], BlockEnd)
	if rel < 0 {
		return content
	}
	end := start + rel + len(BlockEnd)
	for end < len(content) && content[end] != '\n' {
		end++
	}
	if end < len(content) {
		end++
	}
	prefix := strings.TrimRight(content[:start], "\n")
	suffix := strings.TrimLeft(content[end:], "\n")
	switch {
	case prefix == "" && suffix == "":
		return ""
	case prefix == "":
		return suffix
	case suffix == "":
		return prefix
	default:
		return prefix + "\n\n" + suffix
	}
}

// InstallZshrc writes the block into path, keeping a timestamped backup of the
// previous content. changed is false when the file already had the block.
func InstallZshrc(path string, block string) (backup string, changed bool, err error) {
	content, mode, err := readFileMode(path)
	if err != nil {
		return "", false, err
	}
	out, changed := UpsertBlock(content, block)
	if !changed {
		return "", false, nil
	}
	if content != "" {
		backup, err = backupFile(path, content, mode)
		if err != nil {
			return "", false, err
		}
	} else {
		mode = 0o644
	}
	if err := writeFileAtomic(path, []byte(out), mode); err != nil {
		return backup, false, err
	}
	return backup, true, nil
}

// UninstallZshrc removes the block. The previous file is backed up first.
func UninstallZshrc(path string) (backup string, changed bool, err error) {
	content, mode, err := readFileMode(path)
	if err != nil {
		return "", false, err
	}
	out, changed := RemoveBlock(content)
	if !changed {
		return "", false, nil
	}
	if backup, err = backupFile(path, content, mode); err != nil {
		return "", false, err
	}
	if err := writeFileAtomic(path, []byte(out), mode); err != nil {
		return backup, false, err
	}
	return backup, true, nil
}

func backupFile(path string, content string, mode os.FileMode) (string, error) {
	backup := fmt.Sprintf("%s.bak-%s", path, time.Now().Format("20060102-150405"))
	if err := os.WriteFile(backup, []byte(content), mode); err != nil {
		return "", fmt.Errorf("write backup %s: %w", backup, err)
	}
	return backup, nil
}

func readFileMode(path string) (string, os.FileMode, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", 0o644, nil
		}
		return "", 0, fmt.Errorf("read %s: %w", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", 0, fmt.Errorf("stat %s: %w", path, err)
	}
	return string(b), info.Mode().Perm(), nil
}

func writeFileAtomic(path string, content []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".proxy-antigravity-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmpName, path, err)
	}
	return nil
}
