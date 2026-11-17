# proxy-antigravity-macos

[简体中文](README.zh-CN.md) | **English**

[![Release](https://img.shields.io/github/v/release/tangthinker/proxy-antigravity-macos?sort=semver)](https://github.com/tangthinker/proxy-antigravity-macos/releases)
[![Release workflow](https://github.com/tangthinker/proxy-antigravity-macos/actions/workflows/release.yml/badge.svg)](https://github.com/tangthinker/proxy-antigravity-macos/actions/workflows/release.yml)
[![Go version](https://img.shields.io/github/go-mod/go-version/tangthinker/proxy-antigravity-macos)](go.mod)
[![License: GPL-3.0](https://img.shields.io/badge/license-GPL--3.0-blue)](LICENSE)

Route Google Antigravity through a local HTTP/SOCKS proxy **without enabling TUN
(virtual network interface) mode** in Clash / mihomo / sing-box / hysteria.

No TUN, no root, no system-wide proxy. Only two things are touched:

- a conditional block in `~/.zshrc`;
- an `Antigravity Proxy.app` wrapper in `~/Applications`.

```text
Antigravity.app
├── Chromium (window, webviews) ──(proxy switches)──► 127.0.0.1:1080 ──► internet
└── language_server (Go)        ──(HTTPS_PROXY)─────► 127.0.0.1:1080 ──► internet
```

## Why this exists

Antigravity 2.x is not a single process talking to a single network stack. It is
an Electron shell plus a bundled Go backend:

| Component | Traffic | Proxy support |
|---|---|---|
| `language_server` (Go) | `daily-cloudcode-pa.googleapis.com`, `oauth2.googleapis.com`, `www.googleapis.com`, `generativelanguage.googleapis.com`, `jetski-webchannel.googleapis.com`, telemetry | `HTTPS_PROXY` / `HTTP_PROXY` / `ALL_PROXY` (HTTP, HTTPS, SOCKS5). **Does not read the macOS system proxy.** |
| Chromium (window / embedded browser) | The hub UI is served from `https://127.0.0.1:<language_server port>/`; embedded tabs visit arbitrary sites | macOS system proxy or `--proxy-server`. **Does not read environment variables.** |

That split is why people end up enabling TUN:

- With only the **system proxy** on, Chromium is covered but
  `language_server` connects directly and the app hangs on a blank window;
- With **TUN** on, both stacks are captured at the routing layer — but the whole
  machine is captured too, which often breaks Tailscale, corporate VPNs and
  internal DNS.

This tool feeds the two stacks separately and leaves everything else alone.

## How it works

1. **Login-shell environment injection.** Before spawning `language_server`,
   Antigravity runs `zsh -ilc` to read the login-shell environment and sets
   `ANTIGRAVITY_RESOLVING_ENVIRONMENT=1` for that probe only. `install` adds a
   guarded block to `~/.zshrc` that exports the proxy only when that variable is
   present. The Go backend (and the tools it spawns) therefore use the proxy;
   your ordinary terminals do not.
2. **Wrapper app.** `install` generates `~/Applications/Antigravity Proxy.app`.
   Its launcher exports the same proxy variables and starts the real binary with
   `--proxy-server=... --proxy-bypass-list=...`, so Chromium and the embedded
   browser use the proxy as well. Loopback is bypassed explicitly — otherwise the
   local UI is pushed through the tunnel and fails with
   `ERR_TUNNEL_CONNECTION_FAILED`.

## Requirements

- macOS 12 or newer (Apple Silicon or Intel)
- Google Antigravity 2.x installed at `/Applications/Antigravity.app`
- A local proxy exposing an HTTP or SOCKS5 port — for example the Clash /
  mihomo mixed port, `127.0.0.1:1080` by default in this tool

## Install

### Option 1 — prebuilt binary

Download the archive for your CPU from
[Releases](https://github.com/tangthinker/proxy-antigravity-macos/releases), then:

```sh
tar -xzf proxy-antigravity_<version>_darwin_arm64.tar.gz   # or darwin_amd64
cd proxy-antigravity_<version>_darwin_arm64
xattr -dr com.apple.quarantine . 2>/dev/null || true       # only needed for browser downloads
./proxy-antigravity install --proxy 127.0.0.1:1080
```

### Option 2 — build from source

```sh
git clone git@github.com:tangthinker/proxy-antigravity-macos.git
cd proxy-antigravity-macos
make build
./bin/proxy-antigravity install --proxy 127.0.0.1:1080
```

`--proxy` accepts `127.0.0.1:1080`, `http://127.0.0.1:1080`,
`socks5://127.0.0.1:1080`, or basic-auth URLs. It defaults to
`http://127.0.0.1:1080` and can also be set with `ANTIGRAVITY_PROXY`.

`install` backs up `~/.zshrc` to `~/.zshrc.bak-<timestamp>` before editing it,
and is idempotent — running it again only replaces the managed block.

## Launch

Quit Antigravity if it is running, then use either:

- **Spotlight / Finder:** open **Antigravity Proxy** from `~/Applications` (drag
  it to the Dock for daily use);
- **Terminal:**

  ```sh
  ./bin/proxy-antigravity run --restart
  ```

## Verify

```sh
./bin/proxy-antigravity doctor
```

A healthy install looks like this:

```text
✓ proxy    http://127.0.0.1:1080 reachable (gstatic 204 in 495ms)
✓ zshrc    managed block present in ~/.zshrc
✓ wrapper  ~/Applications/Antigravity Proxy.app
✓ process  language_server (pid 642) has 8 connection(s) to 127.0.0.1:1080
✓ log      no proxy 502s in the last 300 language_server log lines
```

Manual check:

```sh
LS=$(pgrep -f 'Antigravity.app/Contents/Resources/bin/language_server' | head -1)
lsof -nP -a -p "$LS" -iTCP -sTCP:ESTABLISHED | grep 1080
```

## Coverage

| How you launch | `language_server` | Embedded browser / webviews |
|---|---|---|
| Plain Antigravity (Dock / Spotlight) | ✅ via the `~/.zshrc` block | ❌ direct |
| **Antigravity Proxy** wrapper | ✅ | ✅ |

Use the **Antigravity Proxy** app for day-to-day work.

## Commands

| Command | Description |
|---|---|
| `proxy-antigravity install` | Add the `~/.zshrc` block and generate the wrapper app |
| `proxy-antigravity run [--restart] [--dry-run]` | Start through the wrapper; `--restart` quits a running instance first |
| `proxy-antigravity doctor` | Check proxy reachability, install state, live connections and recent 502s |
| `proxy-antigravity env` | Print `export` lines to `eval` in a shell |
| `proxy-antigravity uninstall` | Remove the block and the wrapper (with backup) |
| `proxy-antigravity version` | Print the version |

Common flags: `--proxy URL`, `--app PATH` (default
`/Applications/Antigravity.app`), `--zshrc PATH`, `--app-dir DIR` (default
`~/Applications`). Install also supports `--no-zshrc` and `--no-wrapper`.

## FAQ

**Can I change the proxy port later?**

Yes:

```sh
./bin/proxy-antigravity install --proxy 127.0.0.1:7890
```

Then quit Antigravity and relaunch it through **Antigravity Proxy**.

**Why not just turn on the system proxy?**

`language_server` is a Go program and ignores the macOS system proxy. With only
the system proxy on, the backend connects directly and the hub never finishes
loading.

**Why is loopback bypassed?**

The window loads `https://127.0.0.1:<port>/` from the backend. If Chromium sends
that request through the proxy, the UI fails with
`ERR_TUNNEL_CONNECTION_FAILED`.

**What about the Google sign-in page?**

Sign-in opens in your default browser, which uses its own proxy configuration.
This tool does not change it.

**Do Antigravity updates break it?**

No. The block and the wrapper point at the fixed path
`/Applications/Antigravity.app`; replacing the app keeps working. If a future
release changes how `language_server` is spawned, re-run `install`.

**Does it need admin rights or change network settings?**

No. It only writes to `~/.zshrc`, `~/Applications` and `~/.zshrc.bak-*`. It does
not enable TUN, change system proxy settings, or install a privileged helper.

**Where are the logs?**

`~/Library/Logs/Antigravity/language_server.log` and `main.log`. `doctor` checks
the recent lines for `Bad Gateway`.

## Uninstall

```sh
./bin/proxy-antigravity uninstall
```

The previous `~/.zshrc` is backed up before the block is removed, and
`~/Applications/Antigravity Proxy.app` is deleted.

## Development

```sh
make build     # bin/proxy-antigravity, version from git describe
make test      # go test ./...
make vet       # go vet ./...
make doctor    # run the built binary against the local install
```

Layout:

- `main.go` — CLI entry point and subcommands
- `internal/antigravity/zsh.go` — `~/.zshrc` block management and backups
- `internal/antigravity/wrapper.go` — `Antigravity Proxy.app` generation
- `internal/antigravity/launch.go` — process detection, quit/launch
- `internal/antigravity/doctor.go` — health checks
- `.github/workflows/release.yml` — tag-driven release builds

## License

GPL-3.0. See [LICENSE](LICENSE).
