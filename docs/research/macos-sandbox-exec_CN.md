# macOS sandbox-exec 调研报告

**Bead：** gt-6qt
**日期：** 2026-03-08
**阻塞：** gt-2pb (Spike: macOS sandbox-exec Polecat 隔离)

## 1. sandbox-exec 尽管已弃用，是否仍可用？

**是的 — 在 macOS Sequoia 15.x 上尽管已弃用仍完全可用。**

- 手册页写着 DEPRECATED 但运行无问题
- 在本机本地测试中未观察到运行时警告
- macOS 本身大量使用底层的 Seatbelt 内核扩展
- 系统沙盒配置文件位于 `/usr/share/sandbox/` 和 `/System/Library/Sandbox/Profiles/`（100+ 个配置文件）
- 在生产中被以下项目使用：Claude Code (Anthropic)、OpenAI Codex、Bazel、Chromium
- 短期内不会消失 — Apple 自己的服务依赖该内核机制

弃用是一个"请不要使用"的信号，而非即将移除。内核级沙盒子系统（Seatbelt/MACF）必须为 Apple 自身的使用而保留。

## 2. 它能否限制文件系统路径（默认拒绝，允许特定）？

**是的 — 核心能力，运行良好。**

**操作：** `file-read*`、`file-write*`、`file-read-data`、`file-read-metadata`、`file-write-create` 等。

**路径过滤器：**
- `(literal "/exact/path")` — 精确匹配
- `(subpath "/dir")` — 目录及所有后代
- `(regex "^/pattern")` — POSIX 正则

**示例（默认拒绝，允许特定）：**
```scheme
(version 1)
(deny default)
(allow file-read* (subpath "/usr/lib"))
(allow file-read* (subpath "/System/Library"))
(allow file-read* file-write* (subpath (param "PROJECT_DIR")))
(allow file-write* (subpath "/private/tmp"))
```

**参数化路径：** `sandbox-exec -D PROJECT_DIR=/path -f profile.sb command`

**本地测试确认：** 当仅允许 `/usr` 时，读取 `/etc/passwd` 被拒绝，退出码 134。

## 3. 它能否限制网络仅允许回环？

**是的 — 工作可靠。**

**网络操作：** `network*`、`network-outbound`、`network-inbound`、`network-bind`

**过滤器：** `(local ip "localhost:*")`、`(remote ip "localhost:*")`、`(remote unix-socket)`

```scheme
;; 仅回环配置
(allow network* (local ip "localhost:*"))
(allow network* (remote ip "localhost:*"))
(allow network* (remote unix-socket))
```

**本地测试确认：** 在 `(deny default)` 下，`curl` 访问外部主机被拒绝，退出码 6。

OpenAI Codex 发现网络执行"过于有效" — 他们的 `network_access=true` 配置被 seatbelt 静默忽略（GitHub issues #6807、#10390）。

## 4. 它能否限制进程生成（仅允许特定二进制文件）？

**是的 — 通过 `process-exec` 和 `process-fork` 操作。**

```scheme
(allow process-exec (literal "/usr/bin/python3"))
(deny process-exec (literal "/bin/ls"))
```

**关键行为：**
- 子进程继承父进程的沙盒 — 无法逃逸
- `process-exec` 控制哪些二进制文件可以被 exec
- `process-fork` 控制 fork/vfork 权限
- 注意：`/bin/sh` 在 macOS 上重定向到 `/bin/bash`，所以两者都必须允许

**本地测试确认：** 在同一 shell 会话中 `/bin/ls` 被拒绝（退出码 126），而 `/bin/echo` 被允许。

## 5. 它是否与 Node.js（Claude Code 运行时）兼容？

**是的 — 在本机上 Node.js v25.6.0 确认可用。**

**Node.js 所需的 SBPL 规则：**

| 规则 | 原因 |
|------|------|
| `(allow file-ioctl)` | 终端 raw 模式 / `setRawMode` |
| `(allow mach-host*)` | `os.cpus()` / CPU 检测 |
| `(allow pseudo-tty)` | PTY 分配 |
| `(allow ipc-posix-sem)` | 信号量 |
| `(allow iokit-open)` | IOKit 访问 |
| 设备路径: `/dev/ptmx`, `/dev/ttys*` | PTY 设备 |
| `/dev/random`, `/dev/urandom` | 加密/随机 |
| `/private/var/folders`, `/private/tmp` | 临时目录 |

**缺少这些规则的已知问题：**
- `setRawMode` 失败 errno:1（需要 `file-ioctl`）
- `os.cpus()` 返回空数组（需要 `mach-host*`）
- npm/yarn 串行化所有工作（从零 CPU 级联导致）
- Python 多进程失败（需要 `ipc-posix-sem`）

**生产用户：** Claude Code (Anthropic)、Codex (OpenAI)、ai-jail

## 6. 它是否干扰代码签名 / Gatekeeper？

**通常不会。**

- sandbox-exec 不检查或执行代码签名
- 在不同层操作（内核 MAC 框架）
- 二进制文件不需要特殊签名即可被沙盒化
- `sandbox-exec` 本身在 `/usr/bin/sandbox-exec` 受 SIP 保护
- 与 SIP 共存 — 纵深防御中的互补层
- 一个小交互：沙盒化应用对创建的文件应用隔离 xattr
- 在 sandbox-exec 下运行 Node.js 不会触发 Gatekeeper 警告

## 7. .sb 配置文件语法

### 头部
```scheme
(version 1)
```

### 默认动作
```scheme
(deny default)    ;; 白名单模式（推荐用于安全）
(allow default)   ;; 黑名单模式
```

### 调试/日志
```scheme
(debug deny)      ;; 将被拒绝的操作记录到系统日志
(debug all)       ;; 记录所有操作（详细）
```

### 规则结构
```scheme
(allow|deny  operation  [filter...])
```

### 完整操作参考

**文件：** `file*`、`file-read*`、`file-read-data`、`file-read-metadata`、`file-read-xattr`、`file-write*`、`file-write-data`、`file-write-create`、`file-write-flags`、`file-write-mode`、`file-write-mount`、`file-write-owner`、`file-write-setugid`、`file-write-times`、`file-write-unmount`、`file-write-xattr`、`file-ioctl`、`file-revoke`、`file-chroot`

**网络：** `network*`、`network-outbound`、`network-inbound`、`network-bind`

**进程：** `process*`、`process-exec`、`process-fork`

**IPC：** `ipc*`、`ipc-posix*`、`ipc-posix-sem`、`ipc-posix-shm`、`ipc-sysv*`、`ipc-sysv-msg`、`ipc-sysv-sem`、`ipc-sysv-shm`

**Mach：** `mach*`、`mach-bootstrap`、`mach-lookup`、`mach-priv*`、`mach-priv-host-port`、`mach-priv-task-port`、`mach-task-name`、`mach-per-user-lookup`、`mach-host*`

**系统：** `sysctl*`、`sysctl-read`、`sysctl-write`、`system*`、`system-acct`、`system-audit`、`system-fsctl`、`system-lcid`、`system-mac-label`、`system-nfssvc`、`system-reboot`、`system-set-time`、`system-socket`、`system-swap`、`system-write-bootstrap`

**其他：** `pseudo-tty`、`iokit-open`、`job-creation`、`process-info*`、`signal`、`send-signal`

### 过滤器谓词

**路径过滤器：**
```scheme
(literal "/exact/path/to/file")
(subpath "/dir")                        ;; 匹配 /dir 及所有后代
(regex "^/pattern/.*\\.txt$")
```

**网络过滤器：**
```scheme
(local ip "localhost:*")
(remote ip "localhost:80")
(remote unix-socket)
(local tcp "*:8080")
```

**Mach 服务过滤器：**
```scheme
(global-name "com.apple.system.logger")
(local-name "com.example.service")
```

**逻辑组合器：**
```scheme
(require-all (subpath "/tmp") (require-not (vnode-type SYMLINK)))
(require-any (literal "/path/a") (literal "/path/b"))
```

**其他过滤器：**
```scheme
(signing-identifier "com.example.app")
(target same-sandbox)
(sysctl-name "kern.hostname")
```

### 动作修饰符
```scheme
(deny (with no-report) file-write*)              ;; 抑制违规日志
(deny (with send-signal SIGUSR1) network*)
(allow (with report) sysctl (sysctl-name "...")) ;; 即使允许也记录日志
```

### 参数
```scheme
;; CLI: sandbox-exec -D KEY=value -f profile.sb command
(allow file-read* file-write* (subpath (param "PROJECT_DIR")))

;; 条件逻辑
(if (equal? (param "FEATURE") "YES")
  (allow network-outbound))
```

### 导入
```scheme
(import "/System/Library/Sandbox/Profiles/bsd.sb")
```

### 完整 Node.js 沙盒配置

```scheme
(version 1)
(deny default)
(debug deny)

;; 进程控制
(allow process-exec)
(allow process-fork)
(allow signal (target same-sandbox))
(allow process-info* (target same-sandbox))

;; 系统信息（Node.js 需要）
(allow sysctl-read)
(allow mach-host*)
(allow mach-lookup)
(allow iokit-open)
(allow ipc-posix-sem)
(allow ipc-posix-shm-read*)

;; 终端支持
(allow file-ioctl)
(allow pseudo-tty)
(allow file-read* file-write* (literal "/dev/ptmx"))
(allow file-read* file-write* (regex "^/dev/ttys[0-9]+"))

;; 标准设备
(allow file-write* (literal "/dev/null"))
(allow file-write* (literal "/dev/zero"))
(allow file-read* (literal "/dev/random"))
(allow file-read* (literal "/dev/urandom"))

;; 系统读取访问（只读）
(allow file-read* (subpath "/usr/lib"))
(allow file-read* (subpath "/usr/bin"))
(allow file-read* (subpath "/usr/sbin"))
(allow file-read* (subpath "/System"))
(allow file-read* (subpath "/Library"))
(allow file-read* (subpath "/private/etc"))
(allow file-read-metadata)

;; Homebrew（如果 Node.js 通过 Homebrew 安装）
(allow file-read* (subpath "/opt/homebrew"))

;; 项目目录（读写）
(allow file-read* file-write* (subpath (param "PROJECT_DIR")))

;; 临时目录
(allow file-read* file-write* (subpath (param "TMPDIR")))
(allow file-read* file-write* (subpath "/private/var/folders"))
(allow file-read* file-write* (subpath "/private/tmp"))

;; 网络：仅回环
(allow network* (local ip "localhost:*"))
(allow network* (remote ip "localhost:*"))
(allow network* (remote unix-socket))
```

**用法：**
```bash
sandbox-exec -D PROJECT_DIR=/path/to/project -D TMPDIR=$TMPDIR -f profile.sb node app.js
```

## 8. 替代方案评估

### App Sandbox（基于 Entitlements）
- 需要 `.app` bundle 和代码签名 entitlements
- 无法对任意 CLI 工具沙盒化
- **不可行**用于 Agent 沙盒化用例

### Endpoint Security Framework
- 需要 System Extension + Apple 签发的 entitlement（`com.apple.developer.endpoint-security.client`）
- 为安全产品（杀毒、MDM）设计，非进程沙盒化
- 对此用例开销巨大
- **不可行**用于轻量级 Agent 隔离

### 第三方工具（底层都包装了 sandbox-exec）
- **ai-jail**（Rust，约 880KB）：运行时生成 SBPL，支持 Claude Code + Codex
- **claude-sandbox**：macOS 专用的 Claude Code 沙盒化
- **scode**："AI 编码的安全带"
- **agent-seatbelt-sandbox**：专注于防止数据外泄
- **Alcoholless**：Homebrew/AI Agent 的轻量级沙盒

### 结论

**sandbox-exec 是 macOS 上 CLI 工具沙盒化的唯一实用选择。** 此用例不存在 Apple 支持的替代方案。Anthropic（Claude Code）和 OpenAI（Codex）都在生产中使用它。弃用只是表面上的 — 内核子系统是永久基础设施。

Apple 开发者技术支持的 Quinn "The Eskimo" 在 Apple Developer Forums 上承认了这个差距，指出 Endpoint Security 是"一种完全不同的机制"，没有为 CLI 用例提供直接的 sandbox-exec 替代方案。

## 本地测试结果（macOS Sequoia，2026-03-08）

| 测试 | 结果 |
|------|------|
| 基本 sandbox-exec 调用 | 可用，无警告 |
| 文件系统拒绝（仅允许 /usr 时读取 /etc/passwd） | 被拒绝，退出码 134 |
| 网络拒绝（curl 外部主机） | 被拒绝，退出码 6 |
| Node.js v25.6.0 在沙盒下 | 可用，检测到 CPU（14），平台正确 |
| 进程执行限制（拒绝 /bin/ls，允许 /bin/echo） | ls 退出码 126，echo 可用 |
| /bin/sh → /bin/bash 重定向 | shell 脚本必须两者都允许 |