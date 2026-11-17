# proxy-antigravity-macos

**简体中文** | [English](README.md)

[![Release](https://img.shields.io/github/v/release/tangthinker/proxy-antigravity-macos?sort=semver)](https://github.com/tangthinker/proxy-antigravity-macos/releases)
[![Release workflow](https://github.com/tangthinker/proxy-antigravity-macos/actions/workflows/release.yml/badge.svg)](https://github.com/tangthinker/proxy-antigravity-macos/actions/workflows/release.yml)
[![Go version](https://img.shields.io/github/go-mod/go-version/tangthinker/proxy-antigravity-macos)](go.mod)
[![License: GPL-3.0](https://img.shields.io/badge/license-GPL--3.0-blue)](LICENSE)

让 Google Antigravity 单独走本地 HTTP/SOCKS 代理，**不需要开启 Clash / mihomo
/ sing-box / hysteria 的 TUN（虚拟网卡）模式**。

不启用 TUN、不需要 root、不改系统代理。只动两处：

- `~/.zshrc` 里的一段条件注入；
- `~/Applications` 下的 `Antigravity Proxy.app` 启动器。

```text
Antigravity.app
├── Chromium（窗口、内嵌浏览器）──(启动参数)──────► 127.0.0.1:1080 ──► 外网
└── language_server（Go）      ──(HTTPS_PROXY)──► 127.0.0.1:1080 ──► 外网
```

## 为什么需要它

Antigravity 2.x 不是一个进程只走一条网络栈，而是 Electron 外壳 + 内置 Go 后端：

| 组件 | 流量 | 认什么代理 |
|---|---|---|
| `language_server`（Go） | `daily-cloudcode-pa.googleapis.com`、`oauth2.googleapis.com`、`www.googleapis.com`、`generativelanguage.googleapis.com`、`jetski-webchannel.googleapis.com`、遥测等 | `HTTPS_PROXY` / `HTTP_PROXY` / `ALL_PROXY`（HTTP、HTTPS、SOCKS5）。**不读 macOS 系统代理** |
| Chromium（窗口 / 内嵌浏览器） | 主界面由后端在 `https://127.0.0.1:<language_server 端口>/` 提供；内嵌标签访问任意外网 | macOS 系统代理或 `--proxy-server`。**不读环境变量** |

这就是为什么大家最后都去开 TUN：

- 只开**系统代理**：Chromium 能出去，但 `language_server` 仍然直连，App 卡在空白窗口；
- 开 **TUN**：两条栈都在路由层被接管——但整台机器也一起被接管，经常和 Tailscale、公司 VPN、内网 DNS 打架。

本项目把两条栈分开喂，其它流量一概不碰。

## 工作原理

1. **登录 shell 环境注入。** Antigravity 启动 `language_server` 之前会运行
   `zsh -ilc` 读取登录 shell 环境，并且只给这个探测进程设置
   `ANTIGRAVITY_RESOLVING_ENVIRONMENT=1`。`install` 在 `~/.zshrc` 末尾加一段带
   条件判断的块：只有这个变量存在时才导出代理。于是 Go 后端（以及它拉起的工具）
   走代理，你平时开的终端完全不受影响。
2. **包装 App。** `install` 生成 `~/Applications/Antigravity Proxy.app`，启动时导出
   同样的代理变量，并给真实二进制加上
   `--proxy-server=... --proxy-bypass-list=...`，让 Chromium 和内嵌浏览器也走代理。
   同时显式绕过 loopback——否则本地 UI 会被塞进代理隧道，报
   `ERR_TUNNEL_CONNECTION_FAILED`。

## 要求

- macOS 12 或更高（Apple Silicon / Intel）
- 已在 `/Applications/Antigravity.app` 安装 Google Antigravity 2.x
- 本地有 HTTP 或 SOCKS5 代理端口（例如 Clash / mihomo 的混合端口；本工具默认用
  `127.0.0.1:1080`）

## 安装

### 方式一：下载预编译包

从 [Releases](https://github.com/tangthinker/proxy-antigravity-macos/releases) 下载对应
CPU 的压缩包，然后：

```sh
tar -xzf proxy-antigravity_<version>_darwin_arm64.tar.gz   # Intel 用 darwin_amd64
cd proxy-antigravity_<version>_darwin_arm64
xattr -dr com.apple.quarantine . 2>/dev/null || true       # 仅浏览器下载时需要
./proxy-antigravity install --proxy 127.0.0.1:1080
```

### 方式二：从源码构建

```sh
git clone git@github.com:tangthinker/proxy-antigravity-macos.git
cd proxy-antigravity-macos
make build
./bin/proxy-antigravity install --proxy 127.0.0.1:1080
```

`--proxy` 支持 `127.0.0.1:1080`、`http://127.0.0.1:1080`、
`socks5://127.0.0.1:1080` 以及带 basic auth 的 URL；不传时默认
`http://127.0.0.1:1080`，也可以用环境变量 `ANTIGRAVITY_PROXY` 指定。

`install` 在改 `~/.zshrc` 之前会先备份为 `~/.zshrc.bak-<时间戳>`，并且是幂等的——
重复执行只会替换自己管理的那一段。

## 启动

先完全退出 Antigravity，然后二选一：

- **Spotlight / Finder：** 打开 `~/Applications` 里的 **Antigravity Proxy**（建议拖到
  Dock，日常都用它启动）；
- **终端：**

  ```sh
  ./bin/proxy-antigravity run --restart
  ```

## 验证

```sh
./bin/proxy-antigravity doctor
```

正常输出类似：

```text
✓ proxy    http://127.0.0.1:1080 reachable (gstatic 204 in 495ms)
✓ zshrc    managed block present in ~/.zshrc
✓ wrapper  ~/Applications/Antigravity Proxy.app
✓ process  language_server (pid 642) has 8 connection(s) to 127.0.0.1:1080
✓ log      no proxy 502s in the last 300 language_server log lines
```

手动确认：

```sh
LS=$(pgrep -f 'Antigravity.app/Contents/Resources/bin/language_server' | head -1)
lsof -nP -a -p "$LS" -iTCP -sTCP:ESTABLISHED | grep 1080
```

## 覆盖范围

| 启动方式 | `language_server` | 内嵌浏览器 / 网页预览 |
|---|---|---|
| 普通 Antigravity（Dock / Spotlight） | ✅ 靠 `~/.zshrc` 条件块 | ❌ 直连 |
| **Antigravity Proxy** 包装 App | ✅ | ✅ |

日常建议固定用 **Antigravity Proxy** 启动。

## 命令

| 命令 | 说明 |
|---|---|
| `proxy-antigravity install` | 写入 `~/.zshrc` 条件块并生成包装 App |
| `proxy-antigravity run [--restart] [--dry-run]` | 通过包装 App 启动；`--restart` 会先退出旧实例 |
| `proxy-antigravity doctor` | 检查代理连通性、安装状态、实时连接和最近的 502 |
| `proxy-antigravity env` | 输出 `export` 语句，可 `eval` 到当前 shell |
| `proxy-antigravity uninstall` | 移除条件块和包装 App（先备份） |
| `proxy-antigravity version` | 打印版本 |

通用参数：`--proxy URL`、`--app PATH`（默认
`/Applications/Antigravity.app`）、`--zshrc PATH`、`--app-dir DIR`（默认
`~/Applications`）。`install` 额外支持 `--no-zshrc`、`--no-wrapper`。

## 常见问题

**以后改代理端口怎么办？**

```sh
./bin/proxy-antigravity install --proxy 127.0.0.1:7890
```

然后退出 Antigravity，用 **Antigravity Proxy** 重新启动。

**为什么不能只开系统代理？**

`language_server` 是 Go 程序，不读 macOS 系统代理。只开系统代理时后端仍然直连，
Hub 会一直加载不出来。

**为什么要绕过 loopback？**

窗口加载的是后端提供的 `https://127.0.0.1:<端口>/`。如果 Chromium 把这个请求也
送进代理，UI 会报 `ERR_TUNNEL_CONNECTION_FAILED`。

**Google 登录页走哪？**

登录页由系统默认浏览器打开，走浏览器自己的代理设置，本工具不干预。

**Antigravity 更新后还有效吗？**

有效。条件块和包装 App 都指向固定路径 `/Applications/Antigravity.app`，替换应用
后依然可用；如果未来版本改变了 `language_server` 的启动方式，重新执行一次
`install` 即可。

**需要管理员权限或改系统网络设置吗？**

不需要。只写 `~/.zshrc`、`~/Applications` 和 `~/.zshrc.bak-*`，不启用 TUN、不改
系统代理、不安装特权 helper。

**日志在哪？**

`~/Library/Logs/Antigravity/language_server.log` 和 `main.log`。`doctor` 会检查最近
的日志里有没有 `Bad Gateway`。

## 卸载

```sh
./bin/proxy-antigravity uninstall
```

会先备份 `~/.zshrc`，再移除条件块和 `~/Applications/Antigravity Proxy.app`。

## 开发

```sh
make build     # 生成 bin/proxy-antigravity，版本号取自 git describe
make test      # go test ./...
make vet       # go vet ./...
make doctor    # 用构建出的二进制检查本机安装
```

目录结构：

- `main.go` — CLI 入口与子命令
- `internal/antigravity/zsh.go` — `~/.zshrc` 块管理与备份
- `internal/antigravity/wrapper.go` — 生成 `Antigravity Proxy.app`
- `internal/antigravity/launch.go` — 进程检测、退出与启动
- `internal/antigravity/doctor.go` — 健康检查
- `.github/workflows/release.yml` — 按 tag 自动构建发布

## 许可证

GPL-3.0，见 [LICENSE](LICENSE)。
