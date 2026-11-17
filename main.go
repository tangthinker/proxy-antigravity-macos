// Command proxy-antigravity routes Google Antigravity's traffic through an
// upstream HTTP/SOCKS proxy without enabling TUN (virtual NIC) mode.
//
// Antigravity has two network stacks:
//
//   - language_server, a Go binary that talks to Google APIs and honours
//     HTTP(S)_PROXY but not the macOS system proxy;
//   - Chromium, which loads the hub UI from https://127.0.0.1:<port>/ and
//     honours system proxy settings or --proxy-server, but not env vars.
//
// install therefore (1) adds a conditional proxy block to the user's zsh rc
// file, which only affects the login-shell probe Antigravity runs before
// spawning language_server, and (2) generates an "Antigravity Proxy.app"
// wrapper that launches the real binary with both proxy env and Chromium
// switches. Loopback is bypassed so the local UI keeps working.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/tangthinker/proxy-antigravity-macos/internal/antigravity"
)

// version is overridden at build time with
// -ldflags "-X main.version=<version>".
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "install":
		err = cmdInstall(os.Args[2:])
	case "run":
		err = cmdRun(os.Args[2:])
	case "doctor":
		err = cmdDoctor(os.Args[2:])
	case "uninstall":
		err = cmdUninstall(os.Args[2:])
	case "env":
		err = cmdEnv(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Println("proxy-antigravity", version)
		return
	case "help", "-h", "--help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		if errors.Is(err, antigravity.ErrChecksFailed) {
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`proxy-antigravity - route Antigravity through a proxy without TUN mode

Usage:
  proxy-antigravity install    [--proxy URL] [--no-zshrc] [--no-wrapper]
  proxy-antigravity run        [--proxy URL] [--restart] [--dry-run]
  proxy-antigravity doctor     [--proxy URL]
  proxy-antigravity uninstall  [--proxy URL]
  proxy-antigravity env        [--proxy URL]
  proxy-antigravity version

Common flags:
  --proxy URL     upstream proxy (default ` + antigravity.DefaultProxyURL + `, env ANTIGRAVITY_PROXY)
  --app PATH      Antigravity.app (default ` + antigravity.DefaultAppPath + `)
  --zshrc PATH    zsh rc file (default ~/.zshrc)
  --app-dir DIR   where to create the wrapper app (default ~/Applications)
`)
}

func addCommon(fs *flag.FlagSet, opts *antigravity.Options) {
	fs.StringVar(&opts.ProxyURL, "proxy", opts.ProxyURL, "upstream proxy URL")
	fs.StringVar(&opts.AppPath, "app", opts.AppPath, "path to Antigravity.app")
	fs.StringVar(&opts.ZshrcPath, "zshrc", opts.ZshrcPath, "zsh rc file")
	fs.StringVar(&opts.AppDir, "app-dir", opts.AppDir, "directory for the wrapper app")
}

func cmdInstall(args []string) error {
	opts := antigravity.DefaultOptions()
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	addCommon(fs, &opts)
	fs.BoolVar(&opts.NoZshrc, "no-zshrc", false, "do not edit the zsh rc file")
	fs.BoolVar(&opts.NoWrapper, "no-wrapper", false, "do not create the wrapper app")
	if err := fs.Parse(args); err != nil {
		return err
	}
	u, err := opts.Normalize()
	if err != nil {
		return err
	}
	if _, err := opts.MainBinary(); err != nil {
		return err
	}

	fmt.Printf("proxy:   %s\n", u.String())

	if !opts.NoZshrc {
		backup, changed, err := antigravity.InstallZshrc(opts.ZshrcPath, antigravity.ProxyBlock(u))
		if err != nil {
			return err
		}
		switch {
		case changed && backup != "":
			fmt.Printf("zshrc:   updated %s (backup: %s)\n", antigravity.DisplayPath(opts.ZshrcPath), antigravity.DisplayPath(backup))
		case changed:
			fmt.Printf("zshrc:   created %s\n", antigravity.DisplayPath(opts.ZshrcPath))
		default:
			fmt.Printf("zshrc:   %s already up to date\n", antigravity.DisplayPath(opts.ZshrcPath))
		}
	}

	if !opts.NoWrapper {
		app, err := antigravity.CreateWrapper(opts.AppDir, u, opts.AppPath)
		if err != nil {
			return err
		}
		fmt.Printf("wrapper: %s\n", antigravity.DisplayPath(app))
	}

	fmt.Print(`
Next:
  1. Quit Antigravity if it is running.
  2. Launch "Antigravity Proxy" (Spotlight or ~/Applications), or:
       proxy-antigravity run
  3. Verify:
       proxy-antigravity doctor

Launching plain Antigravity still proxies language_server through the zsh
block, but the embedded browser stays direct. Use the wrapper for full
coverage.
`)
	return nil
}

func cmdRun(args []string) error {
	opts := antigravity.DefaultOptions()
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	addCommon(fs, &opts)
	fs.BoolVar(&opts.Restart, "restart", false, "quit a running Antigravity first")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "print what would be executed")
	if err := fs.Parse(args); err != nil {
		return err
	}
	u, err := opts.Normalize()
	if err != nil {
		return err
	}
	if opts.DryRun {
		return antigravity.PrintLaunch(opts, u)
	}
	return antigravity.Launch(opts, u)
}

func cmdDoctor(args []string) error {
	opts := antigravity.DefaultOptions()
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	addCommon(fs, &opts)
	if err := fs.Parse(args); err != nil {
		return err
	}
	u, err := opts.Normalize()
	if err != nil {
		return err
	}
	results, err := antigravity.Doctor(opts, u)
	for _, r := range results {
		symbol := "✓"
		switch {
		case !r.OK && r.Warn:
			symbol = "!"
		case !r.OK:
			symbol = "✗"
		}
		fmt.Printf("%s %-8s %s\n", symbol, r.Label, r.Detail)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nchecks failed")
	}
	return err
}

func cmdUninstall(args []string) error {
	opts := antigravity.DefaultOptions()
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	addCommon(fs, &opts)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if _, err := opts.Normalize(); err != nil {
		return err
	}
	backup, changed, err := antigravity.UninstallZshrc(opts.ZshrcPath)
	if err != nil {
		return err
	}
	if changed {
		fmt.Printf("zshrc:   removed block from %s (backup: %s)\n", antigravity.DisplayPath(opts.ZshrcPath), antigravity.DisplayPath(backup))
	} else {
		fmt.Printf("zshrc:   no managed block in %s\n", antigravity.DisplayPath(opts.ZshrcPath))
	}
	removed, err := antigravity.RemoveWrapper(opts.AppDir)
	if err != nil {
		return err
	}
	if removed {
		fmt.Printf("wrapper: removed %s\n", antigravity.DisplayPath(antigravity.WrapperDir(opts.AppDir)))
	} else {
		fmt.Printf("wrapper: %s not present\n", antigravity.DisplayPath(antigravity.WrapperDir(opts.AppDir)))
	}
	return nil
}

func cmdEnv(args []string) error {
	opts := antigravity.DefaultOptions()
	fs := flag.NewFlagSet("env", flag.ContinueOnError)
	fs.StringVar(&opts.ProxyURL, "proxy", opts.ProxyURL, "upstream proxy URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	u, err := opts.Normalize()
	if err != nil {
		return err
	}
	for _, kv := range antigravity.ProxyEnv(u) {
		key, value, _ := strings.Cut(kv, "=")
		fmt.Printf("export %s=%s\n", key, antigravity.ShellQuote(value))
	}
	return nil
}
