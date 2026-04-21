# gt-proxy-server 和 gt-proxy-client

代理服务器和客户端为 Polecat 实现沙箱执行：容器可以调用 `gt` 和 `bd` 命令，以及 push/pull git 仓库，通过加密且双向认证的通道 — 无需直接访问主机文件系统、凭据或 GitHub。

## 概述

当 Polecat 在容器或隔离执行环境（如 [Daytona](https://www.daytona.io/)）中运行时，它仍需与 Gas Town 的控制面板交互。具体来说，它需要：

- 调用 `gt` 和 `bd` 命令（邮件、状态、交接、问题更新等）
- 将工作推送到 Rig 的 `.repo.git` 裸仓库中的 Polecat 分支

代理通过运行两个小型 Go 二进制文件来解决此问题：

| 二进制文件 | 运行位置 | 用途 |
|-----------|---------|------|
| `gt-proxy-server` | 主机 | 接受 mTLS 连接；执行 `gt`/`bd` 并提供 git smart-HTTP |
| `gt-proxy-client` | 容器 | 安装为 `gt` 和 `bd`；通过 mTLS 将调用转发到服务器 |

```
 容器                              主机
 ─────────────────────            ──────────────────────────────────────────
  gt mail inbox           ──mTLS──► gt-proxy-server ──► exec gt mail inbox
  git push origin/proxy   ──mTLS──► gt-proxy-server ──► git-receive-pack ~/gt/MyRig/.repo.git
```

双方使用由服务器生成和管理的单个 CA 签名的证书进行认证。所有流量使用 TLS 1.3。

---

## 前提条件

| 工具 | 版本 | 安装方式 |
|------|------|----------|
| **Go** | 1.21+ | [go.dev](https://go.dev/doc/install) |
| **git** | 2.20+ | `brew install git` / `apt install git` |

二进制文件与 `gt` 一起位于同一模块中：

```bash
# 构建两个二进制文件
go install github.com/steveyegge/gastown/cmd/gt-proxy-server@latest
go install github.com/steveyegge/gastown/cmd/gt-proxy-client@latest
```

---

## gt-proxy-server

### 功能

服务器在 mTLS 端口上监听，提供两个端点：

- **`POST /v1/exec`** — 代表 Polecat 运行 `gt` 或 `bd` 子命令
- **`GET/POST /v1/git/<rig>/...`** — 代理 Rig 裸仓库的 git smart-HTTP

每个客户端必须提供由服务器 CA 签名的证书。仅接受 Common Name 匹配 `gt-<rig>-<name>` 的证书（Polecat 身份格式）。

### 启动服务器

```bash
gt-proxy-server \
  --listen 0.0.0.0:9876 \
  --ca-dir ~/gt/.runtime/ca \
  --allowed-cmds gt,bd \
  --town-root ~/gt
```

服务器在首次运行时生成或加载 CA，然后自签发服务器证书。启动后你将看到：

```
gt-proxy-server: listening  addr=0.0.0.0:9876  tls=mTLS
```

### CLI 标志

| 标志 | 默认值 | 描述 |
|------|--------|------|
| `--listen` | `0.0.0.0:9876` | 监听的 TCP 地址 |
| `--admin-listen` | `127.0.0.1:9877` | 本地管理 HTTP 服务器地址；设为 `""` 禁用 |
| `--ca-dir` | `~/gt/.runtime/ca` | 存储 `ca.crt` 和 `ca.key` 的目录 |
| `--allowed-cmds` | `gt,bd` | 容器可调用的二进制名称逗号分隔列表 |
| `--allowed-subcmds` | *（自动发现）* | 每个二进制文件的子命令白名单，分号分隔，如 `gt:prime,hook,done;bd:create,update` |
| `--town-root` | `$GT_TOWN` 或 `~/gt` | Gas Town 根目录；用于定位裸仓库 |
| `--config` | `~/gt/.runtime/proxy/config.json` | JSON 配置文件路径；文件值被显式 CLI 标志覆盖 |

### 环境变量

| 变量 | 描述 |
|------|------|
| `GT_TOWN` | 覆盖 Town 根目录（等同于 `--town-root`） |

### 允许的命令和子命令

只有 `--allowed-cmds` 中列出的二进制名称可以通过 `/v1/exec` 调用。默认的 `gt,bd` 适用于生产环境。条目必须为纯名称（不含 `/` 或 `\`）；含路径分隔符的条目在启动时被记录并丢弃。

二进制路径在启动时一次性解析，以防止服务器运行后的 PATH 劫持。

如需进一步限制，传入子集：

```bash
# 仅允许 gt；不允许 bd 访问
gt-proxy-server --allowed-cmds gt
```

子命令过滤在每次 `/v1/exec` 请求上执行。如果命令在 `--allowed-subcmds` 中有条目，`argv[1]` 必须出现在该列表中，否则请求以 HTTP 403 拒绝。如果命令没有条目，则该命令的所有子命令都允许（不推荐用于 `gt` 或 `bd`）。

默认子命令白名单：

| 二进制 | 子命令 |
|--------|--------|
| `gt` | `prime`、`hook`、`done`、`mail`、`nudge`、`mol`、`status`、`handoff`、`version`、`convoy`、`sling` |
| `bd` | `create`、`update`、`close`、`show`、`list`、`ready`、`dep`、`export`、`prime`、`stats`、`blocked`、`doctor` |

#### 通过 `gt proxy-subcmds` 自动发现

服务器启动时运行 `gt proxy-subcmds`，让已安装的 `gt` 二进制声明自己的安全子命令列表。如果命令成功且产生非空输出，该输出替代上面的内置默认值。如果失败或返回空输出，则使用内置默认值。

这意味着在主机上升级 `gt` 会在下次重启时自动将新允许的子命令传播到代理，无需手动配置更改。你始终可以通过显式传入 `--allowed-subcmds` 覆盖结果。

### CA 和证书生命周期

CA 是存储在 `--ca-dir` 中的自签名证书：

```
~/gt/.runtime/ca/
  ca.crt   ← CA 证书（分发到容器作为 GT_PROXY_CA）
  ca.key   ← CA 私钥（仅保留在主机；绝不分发）
```

首次运行时 CA 自动创建。你可以预创建或通过让 `gt-proxy-server --ca-dir` 指向新目录来轮换。

Polecat 叶证书按每个 Polecat 签发，需要单独生成（见下方"签发 Polecat 证书"）。

### HTTP 超时

| 超时 | 值 | 备注 |
|------|----|------|
| ReadTimeout | 30 秒 | 整个请求头 + 请求体 |
| WriteTimeout | 5 分钟 | 为 git push/fetch 流预留的充裕时间 |
| IdleTimeout | 2 分钟 | Keep-alive 连接空闲 |
| Shutdown drain | 30 秒 | 进程收到 SIGINT/SIGTERM 时的优雅退出期 |

### 速率限制和并发

服务器对 `/v1/exec` 请求应用两层独立保护：

| 限制 | 默认值 | 配置字段 |
|------|--------|---------|
| 每客户端持续速率 | 10 请求/秒 | `exec_rate_limit` |
| 每客户端突发 | 20 请求 | `exec_rate_burst` |
| 全局并发子进程 | 32 | `max_concurrent_exec` |
| 每命令超时 | 60 秒 | `exec_timeout` |

客户端通过其 mTLS 证书 CN 标识。超过速率限制的客户端收到 HTTP 429；服务器完全占用时返回 HTTP 503。默认值可在 JSON 配置文件中覆盖。

---

## gt-proxy-client

### 功能

客户端在容器内安装为 `gt` 和 `bd` 二进制文件（或作为指向单个 `gt-proxy-client` 二进制的符号链接）。被调用时：

1. 如果 `GT_PROXY_URL`、`GT_PROXY_CERT` 和 `GT_PROXY_KEY` 全部设置 → 通过 mTLS 将调用转发到代理服务器。
2. 否则 → `exec` 位于 `GT_REAL_BIN` 的真实二进制文件（默认：`/usr/local/bin/gt.real`）。

回退机制意味着同一个二进制文件在沙箱内外都能工作，无需对代理代码做任何修改。

### 环境变量

| 变量 | 必需 | 描述 |
|------|------|------|
| `GT_PROXY_URL` | 是（代理模式） | 代理服务器的基础 URL，如 `https://192.168.1.10:9876` |
| `GT_PROXY_CERT` | 是（代理模式） | Polecat 客户端证书路径（PEM） |
| `GT_PROXY_KEY` | 是（代理模式） | Polecat 客户端私钥路径（PEM） |
| `GT_PROXY_CA` | 推荐 | 用于验证服务器 TLS 证书的 CA 证书路径 |
| `GT_REAL_BIN` | 否 | 回退时真实 `gt` 二进制的路径（默认：`/usr/local/bin/gt.real`） |

如果 `GT_PROXY_URL`、`GT_PROXY_CERT` 或 `GT_PROXY_KEY` 任一缺失，客户端静默回退到 `execReal()`。这使其可安全地无条件安装 — 未沙箱化的 Polecat 只是 exec 真实二进制文件。

### Git 集成

对于 git 操作，配置 git 使用代理的 git smart-HTTP 端点：

```bash
# 告诉 git 使用此 Rig 仓库的代理服务器
git remote set-url origin https://<proxy-host>:9876/v1/git/<RigName>

# 告诉 git 使用 CA 证书和 Polecat 证书进行 TLS
export GIT_SSL_CAINFO=$GT_PROXY_CA
export GIT_SSL_CERT=$GT_PROXY_CERT
export GIT_SSL_KEY=$GT_PROXY_KEY
```

Git 客户端使用与 exec 客户端相同的 mTLS 证书进行认证。分支授权在服务器端强制执行：名为 `rust` 的 Polecat 只能推送到 `refs/heads/polecat/rust-*`。

---

## 端到端设置

### 第 1 步：在主机上启动服务器

```bash
# 首次运行时安装 CA
gt-proxy-server --listen 0.0.0.0:9876

# CA 证书现在位于 ~/gt/.runtime/ca/ca.crt
```

### 第 2 步：签发 Polecat 证书

使用 Go API 或小型辅助程序：

```go
ca, _ := proxy.LoadOrGenerateCA("~/gt/.runtime/ca")
certPEM, keyPEM, _ := ca.IssuePolecat("gt-MyRig-rust", 365*24*time.Hour)
```

保存输出文件：

```
~/gt/.runtime/polecats/rust/
  polecat.crt   ← 此 Polecat 的客户端证书
  polecat.key   ← 此 Polecat 的客户端私钥
```

### 第 3 步：在容器中安装客户端二进制

```bash
# 方案 A：复制二进制两次
cp gt-proxy-client /usr/local/bin/gt
cp gt-proxy-client /usr/local/bin/bd

# 方案 B：复制一次并创建符号链接
cp gt-proxy-client /usr/local/bin/gt-proxy-client
ln -s gt-proxy-client /usr/local/bin/gt
ln -s gt-proxy-client /usr/local/bin/bd

# 如果真实 gt 二进制应作为回退可用：
mv /usr/local/bin/gt.original /usr/local/bin/gt.real
```

### 第 4 步：配置容器环境

```bash
export GT_PROXY_URL=https://192.168.1.10:9876
export GT_PROXY_CERT=/secrets/polecat.crt
export GT_PROXY_KEY=/secrets/polecat.key
export GT_PROXY_CA=/secrets/ca.crt

# 用于 git 操作：
export GIT_SSL_CAINFO=$GT_PROXY_CA
export GIT_SSL_CERT=$GT_PROXY_CERT
export GIT_SSL_KEY=$GT_PROXY_KEY
```

你可以将 `ca.crt`、`polecat.crt` 和 `polecat.key` 作为容器秘密挂载（Docker secrets、Kubernetes secrets、Daytona 工作空间环境等）。

### 第 5 步：验证连接

在容器内：

```bash
gt version           # 应通过代理打印 Gas Town 版本
gt status            # 应显示来自主机的 Town 状态
git push origin HEAD # 应通过代理推送到 Polecat 分支
```

---

## 配置文件

服务器端选项可在 JSON 配置文件中设置。默认路径为 `~/gt/.runtime/proxy/config.json`；使用 `--config` 覆盖。CLI 标志始终优先于文件值。

```json
{
  "listen_addr":        "0.0.0.0:9876",
  "admin_listen_addr":  "127.0.0.1:9877",
  "ca_dir":             "",
  "town_root":          "",
  "allowed_commands":   ["gt", "bd"],
  "allowed_subcommands": {
    "gt": ["prime", "hook", "done", "mail", "nudge", "mol", "status", "handoff", "version", "convoy", "sling"],
    "bd": ["create", "update", "close", "show", "list", "ready", "dep", "export", "prime", "stats", "blocked", "doctor"]
  },
  "extra_san_ips":      ["10.0.1.5", "172.20.0.1"],
  "extra_san_hosts":    ["my-dev-vm.local", "proxy.corp.example.com"],
  "max_concurrent_exec": 32,
  "exec_rate_limit":    10.0,
  "exec_rate_burst":    20,
  "exec_timeout":       "60s"
}
```

| 字段 | 类型 | 描述 |
|------|------|------|
| `listen_addr` | `string` | mTLS 服务器的 TCP 地址（默认：`0.0.0.0:9876`） |
| `admin_listen_addr` | `string` | 本地管理 HTTP 服务器的 TCP 地址（默认：`127.0.0.1:9877`）；设为 `""` 禁用 |
| `ca_dir` | `string` | 存放 `ca.crt` 和 `ca.key` 的目录（默认：`~/gt/.runtime/ca`） |
| `town_root` | `string` | Gas Town 根目录（默认：`$GT_TOWN` 或 `~/gt`） |
| `allowed_commands` | `[]string` | Polecat 可执行的二进制名称 |
| `allowed_subcommands` | `map[string][]string` | 每命令的子命令白名单 |
| `extra_san_ips` | `[]string` | 要包含在服务器证书 SAN 列表中的额外 IP 地址 |
| `extra_san_hosts` | `[]string` | 要包含在服务器证书 SAN 列表中的额外主机名（DNS 名称） |
| `max_concurrent_exec` | `int` | 最大并发 exec 子进程数（默认：32） |
| `exec_rate_limit` | `float64` | 每客户端每秒持续 exec 请求数（默认：10） |
| `exec_rate_burst` | `int` | 每客户端速率限制器突发大小（默认：20） |
| `exec_timeout` | `string` | 单个 exec 子进程的最大持续时间，如 `"60s"`（默认：60 秒） |

### 本地 IP vs 外部/NAT IP

服务器自动检测并包含所有本地网络接口 IP（通过 `net.Interfaces()`）在其 TLS 证书的 Subject Alternative Names 中。这覆盖直接 LAN 连接。

**外部/NAT IP 地址不会被自动检测。** 出口 IP 在路由器上 — 它不存在于任何 OS 网络接口上 — 因此无法在不联系外部服务的情况下可靠发现。

如果容器通过 NAT 边界连接到代理（例如，主机在家庭路由器后面而容器运行在云 VM 上），请将外部 IP 添加到 `extra_san_ips`：

```json
{
  "extra_san_ips": ["203.0.113.42"]
}
```

你可以通过以下方式查找你的外部 IP：

```bash
curl -s https://api.ipify.org
```

---

## 安全模型

### 强制执行的内容

| 层 | 内容 | 方式 |
|----|------|------|
| **传输** | 所有流量加密 | TLS 1.3 最低 |
| **服务器身份** | 容器验证主机合法 | 服务器证书由共享 CA 签名 |
| **客户端身份** | 服务器验证每个请求来自已知 Polecat | 客户端证书由同一 CA 签名；CN 格式 `gt-<rig>-<name>` 必需 |
| **Exec 白名单** | 容器只能调用 `gt` 和 `bd`（或配置的集合） | 每次 `/v1/exec` 请求检查 `--allowed-cmds` |
| **子命令白名单** | Polecat 只能调用 `gt`/`bd` 的允许子命令 | 每次 `/v1/exec` 请求检查 `--allowed-subcmds`；缺失或不允许的子命令 → 403 |
| **子命令注入** | Polecat 身份以 `--identity <rig>/<name>` 注入且不可覆盖 | 服务器从客户端证书派生身份，而非从请求体 |
| **分支范围** | Polecat 只能推送到 `refs/heads/polecat/<name>-*` | 在调用 `git-receive-pack` 前解析并验证 pkt-line 流 |
| **路径遍历** | Rig 名称按 `[a-zA-Z0-9_-]+` 验证 | 拒绝 `../` 和其他遍历尝试 |
| **请求体大小限制** | `/v1/exec` 请求体限制 1 MiB；receive-pack 引用列表限制 32 MiB | 读取前应用 `http.MaxBytesReader` |
| **环境隔离** | `gt`/`bd`/`git` 子进程仅看到 `HOME` 和 `PATH` | 服务器不传递自己的 `GITHUB_TOKEN`、`GT_TOKEN` 或其他凭据 |
| **速率限制** | 每客户端 exec 速率限制（默认：10 请求/秒，突发 20） | `golang.org/x/time/rate` 按每个 mTLS 证书 CN 限制器；超出返回 HTTP 429 |
| **并发上限** | 全局 exec 子进程限制（默认：32） | 信号量；满时返回 HTTP 503 |
| **证书吊销** | 被泄露的证书序列号可在运行时拒绝 | TLS 握手时检查内存拒绝列表；通过本地管理 API 更新 |

### 未强制执行的内容

- **容器内的文件系统访问** — 代理仅中介 `gt`/`bd` 和 git；有卷挂载的容器仍可直接读取这些文件。
- **容器的网络出站** — 代理不阻止容器向 GitHub 或其他服务发起出站连接。

---

## 本地管理服务器

服务器启动第二个 HTTP 监听器，绑定到 `127.0.0.1:9877`（可通过 `--admin-listen` 配置；设为 `""` 禁用）。此服务器**没有 TLS** — 故意仅限本地，依赖 OS 级访问控制来保障安全。

### 管理端点

| 方法 | 路径 | 描述 |
|------|------|------|
| `POST` | `/v1/admin/issue-cert` | 签发新的 Polecat 客户端证书 |
| `POST` | `/v1/admin/deny-cert` | 将证书序列号添加到运行时拒绝列表 |

### 签发 Polecat 证书

通过提供 Rig 名称、Polecat 名称和可选的 TTL（默认 720h / 30 天）来签发客户端证书：

```bash
curl -s -X POST http://127.0.0.1:9877/v1/admin/issue-cert \
  -H 'Content-Type: application/json' \
  -d '{"rig": "MyRig", "name": "rust", "ttl": "720h"}'
```

返回 HTTP 200，JSON 体包含 PEM 编码的证书、密钥和 CA 证书，以及元数据：

```json
{
  "cn":         "gt-MyRig-rust",
  "cert":       "-----BEGIN CERTIFICATE-----\n...",
  "key":        "-----BEGIN EC PRIVATE KEY-----\n...",
  "ca":         "-----BEGIN CERTIFICATE-----\n...",
  "serial":     "3f2a1b...",
  "expires_at": "2026-04-01T22:37:00Z"
}
```

| 字段 | 类型 | 描述 |
|------|------|------|
| `rig` | `string` | **必需。** Rig 名称（如 `"MyRig"`） |
| `name` | `string` | **必需。** Polecat 名称（如 `"rust"`） |
| `ttl` | `string` | 可选的 Go 持续时间（如 `"720h"`）。默认：`720h`（30 天） |

### 吊销证书

在请求体中以小写十六进制发送证书序列号：

```bash
curl -s -X POST http://127.0.0.1:9877/v1/admin/deny-cert \
  -H 'Content-Type: application/json' \
  -d '{"serial": "3f2a1b"}'
```

成功返回 HTTP 204。序列号添加到内存拒绝列表；任何未来使用该证书的 TLS 握手立即被拒绝。拒绝列表不在重启间持久化 — 如果证书必须在重启后仍被吊销，请勿重新签发它。

---

## Git 代理工作原理

服务器通过 mTLS 实现 [git smart-HTTP 协议](https://git-scm.com/docs/http-backend)。容器内的 Git 客户端配置其远程 URL 指向代理：

```
https://<proxy-host>:9876/v1/git/<RigName>
```

Git 然后发出与任何 HTTPS git 服务器相同的请求：

```
# Clone / fetch
GET  /v1/git/MyRig/info/refs?service=git-upload-pack
POST /v1/git/MyRig/git-upload-pack

# Push
GET  /v1/git/MyRig/info/refs?service=git-receive-pack
POST /v1/git/MyRig/git-receive-pack
```

服务器将每个请求转换为本地子进程调用：

```
git-upload-pack  --stateless-rpc [--advertise-refs] ~/gt/MyRig/.repo.git
git-receive-pack --stateless-rpc [--advertise-refs] ~/gt/MyRig/.repo.git
```

对于推送（`git-receive-pack`），服务器在将请求体传递给 git **之前**读取 pkt-line 引用列表，并拒绝任何超出 Polecat 允许范围的引用：

```
refs/heads/polecat/<name>-*   ✓ 允许
refs/heads/main               ✗ 拒绝（403 Forbidden）
refs/heads/polecat/other-*    ✗ 拒绝（属于另一个 Polecat）
```

pkt-line 流随后被倒回并原样传递给 `git-receive-pack`，所以 git 看到正常的推送请求体。

---

## 故障排除

### `x509: certificate is valid for ..., not <IP>`

容器通过一个未列在服务器证书 Subject Alternative Names 中的 IP 地址连接到服务器。

**修复**：将 IP 添加到 `~/gt/.runtime/proxy/config.json` 中的 `extra_san_ips` 并重启服务器（每次启动时重新签发服务器证书）。

```json
{ "extra_san_ips": ["10.0.2.15"] }
```

### `remote error: tls: bad certificate`

客户端证书不是由服务器信任的 CA 签发的，或 `GT_PROXY_CA` 指向了错误的文件。

验证：

```bash
# 检查客户端证书是否由 ca.crt 签名
openssl verify -CAfile ~/gt/.runtime/ca/ca.crt /path/to/polecat.crt

# 检查 GT_PROXY_CA 是否指向正确的 CA
openssl x509 -in $GT_PROXY_CA -noout -subject
```

### `command not allowed: "sh"`

容器尝试执行不在 `--allowed-cmds` 中的二进制。服务器返回 HTTP 403 并记录尝试。

如果这是合理的，将命令添加到 `--allowed-cmds`。如果不是，说明代理正在尝试执行 Shell — 这被有意阻止。

### `push to "refs/heads/main" denied`

Polecat 尝试推送到不属于它的分支。Polecat 只能推送到 `refs/heads/polecat/<their-name>-*`。Refinery 合并这些分支；Polecat 不直接推送到 `main` 或 `proxy`。

### `gt-proxy-client: proxy request failed: ...`（回退激活）

如果 `GT_PROXY_URL`、`GT_PROXY_CERT` 或 `GT_PROXY_KEY` 任一未设置，客户端回退到 `execReal()`（位于 `GT_REAL_BIN` 的真实 `gt` 二进制）。检查容器内所有三个环境变量是否设置：

```bash
echo $GT_PROXY_URL
echo $GT_PROXY_CERT
echo $GT_PROXY_KEY
```

### 服务器证书仅包含 `gt-proxy-server` 作为 SAN

如果未配置 `extra_san_ips` / `extra_san_hosts`，这是预期行为。测试时你可以传入 `--insecure` 或设置 `GIT_SSL_NO_VERIFY=1` 临时使用，但生产环境请始终配置正确的 SAN 或使用主机名。

---

## 参考

### 服务器端点

**mTLS 服务器（默认：`0.0.0.0:9876`）**

| 方法 | 路径 | 描述 |
|------|------|------|
| `POST` | `/v1/exec` | 执行 `gt` 或 `bd` 命令 |
| `GET` | `/v1/git/<rig>/info/refs?service=<svc>` | git smart-HTTP 能力公告 |
| `POST` | `/v1/git/<rig>/git-upload-pack` | git fetch / clone |
| `POST` | `/v1/git/<rig>/git-receive-pack` | git push（CN 限定的分支授权） |

**本地管理服务器（默认：`127.0.0.1:9877`，无 TLS）**

| 方法 | 路径 | 描述 |
|------|------|------|
| `POST` | `/v1/admin/issue-cert` | 签发新的 Polecat 客户端证书 |
| `POST` | `/v1/admin/deny-cert` | 将证书序列号添加到运行时拒绝列表 |

### 证书 CN 格式

| 角色 | CN 格式 | 示例 |
|------|---------|------|
| 服务器 | `gt-proxy-server` | `gt-proxy-server` |
| Polecat 客户端 | `gt-<rig>-<name>` | `gt-GasTown-rust` |

服务器在请求时从 CN 派生 Polecat 的身份（`<rig>/<name>`）。去掉 `gt-` 后的剩余部分中最后一个 `-` 是 Rig/名称分隔符，因此带连字符的 Rig 名称（如 `my-rig`）可以正确解析：

```
CN: gt-my-rig-rust   →   rig=my-rig, name=rust, identity=my-rig/rust
```

### 文件布局

```
~/gt/
  .runtime/
    ca/
      ca.crt           ← CA 证书（可安全分发到容器）
      ca.key           ← CA 私钥（仅限主机；绝不能离开此机器）
    proxy/
      config.json      ← 可选：extra_san_ips、extra_san_hosts
    polecats/
      <name>/
        polecat.crt    ← 每个 Polecat 的客户端证书
        polecat.key    ← 每个 Polecat 的私钥
  <RigName>/
    .repo.git/         ← 由 git 端点代理的裸仓库
```