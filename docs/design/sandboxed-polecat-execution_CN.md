# 沙箱化 Polecat 执行（exitbox + daytona）

> **日期:** 2026-03-02
> **作者:** mayor
> **状态:** 提案
> **相关:** polecat-lifecycle-patrol.md、architecture.md

---

## 1. 问题陈述

当前每个 polecat 直接在宿主机上以用户自己的 UID 在 tmux 会话中运行，拥有对宿主机文件系统、网络和凭据的完全访问权限。这产生了两个不同的问题：

**安全性。** 行为异常或被操纵的 agent（例如通过恶意 MCP 服务器）可以读取其 worktree 外的文件、写入 `~/.ssh` 或 `~/.gitconfig`、发起任意出站网络连接，或以伪造身份调用 `gt`/`bd`。凭据泄露是真实威胁。

**可扩展性。** 开发者笔记本电脑无法在不产生资源争用的情况下维持 10-20 个同时运行的 Claude 会话。将工作负载分发到云容器（daytona）将吞吐量与本地硬件解耦。

两个问题通过单一机制解决：可配置的 polecat 执行后端。

---

## 2. 核心问题分解

Agent 会话做两件独立的事，需要不同的处理：

| 平面 | 运行什么 | 必须在哪里运行 |
|------|----------|---------------|
| **Agent 工作** | LLM 推理、文件编辑、代码执行、`git` 操作 | 在沙箱/容器内 — 需要 worktree |
| **控制平面** | `gt prime`、`gt done`、`gt mail`、`bd show/update`、事件、nudge | 回到宿主机 — 需要 Dolt、`.runtime/`、mail |

保持这些平面分离是简洁设计的关键。

---

## 3. 架构

### 3.1 当前（仅本地）

```
宿主机
┌─────────────────────────────────────────────────────┐
│                                                     │
│  GasTown daemon                                     │
│  ┌──────────────────────────────────────────────┐   │
│  │  SessionManager.Start()                      │   │
│  │    exec env GT_RIG=... GT_POLECAT=...        │   │
│  │    claude --mode=direct                      │   │
│  └──────────────┬───────────────────────────────┘   │
│                 │  tmux new-session                  │
│                 ▼                                    │
│           ┌──────────┐   gt prime / gt done          │
│           │  polecat │ ──────────────────────────►  │
│           │  (tmux)  │   bd show / bd update         │
│           └──────────┘   (直接，回环 Dolt)            │
│                                                     │
│   Dolt SQL  127.0.0.1:3307                          │
│   .runtime/  ~/gt/                                  │
└─────────────────────────────────────────────────────┘
```

### 3.2 目标：exitbox（本地沙箱）

将所有内容保持在宿主机上；在 exitbox 强制的文件系统和网络策略中包裹 agent 进程。控制平面路径不变，因为回环仍然可达。

```
宿主机
┌─────────────────────────────────────────────────────┐
│                                                     │
│  GasTown daemon                                     │
│  ┌──────────────────────────────────────────────┐   │
│  │  exec env GT_RIG=... GT_POLECAT=...          │   │
│  │  exitbox run --profile=gastown-polecat --    │   │
│  │  claude --mode=direct                        │   │
│  └──────────────┬───────────────────────────────┘   │
│                 │  tmux new-session                  │
│                 ▼                                    │
│  ┌─────────────────────────┐                        │
│  │  exitbox 沙箱           │  gt / bd 调用           │
│  │  ┌─────────────────┐    │ ──────────────────────► │
│  │  │ polecat (agent) │    │   回环 — 直接           │
│  │  └─────────────────┘    │   (Dolt, .runtime/)     │
│  │  策略：                 │                        │
│  │  - rw：仅 worktree      │                        │
│  │  - net：仅回环          │                        │
│  └─────────────────────────┘                        │
│                                                     │
│   Dolt SQL  127.0.0.1:3307   (回环可达)              │
└─────────────────────────────────────────────────────┘
```

### 3.3 目标：daytona（远程云容器）

Agent 在远程 Linux 容器中运行。所有通信 — 控制平面、git fetch 和 git push — 通过宿主机的 mTLS 代理。容器有**零出站互联网访问**。

```
宿主机                          Daytona 云容器
┌───────────────────────────┐     ┌──────────────────────────────────────┐
│                           │     │                                      │
│  GasTown daemon           │     │  tmux pane：daytona exec <ws>        │
│  ┌──────────────────────┐ │     │  ┌────────────────────────────────┐  │
│  │ SessionManager       │ │     │  │ claude --mode=direct           │  │
│  │  - 签发证书           │ │     │  │                                │  │
│  │  - 注入环境变量       │ │     │  │  gt prime / gt done / bd show  │  │
│  │  - 启动代理          │ │     │  │  ↓ (proxy-client 检测环境)     │  │
│  └──────────────────────┘ │     │  │  POST /v1/exec 通过 mTLS       │  │
│                           │     │  └───────────────────┬────────────┘  │
│  gt-proxy-server          │     │                      │ mTLS（证书 CN   │
│  ┌──────────────────────┐ │◄────┼──────────────────────┘  gt-rig-name)  │
│  │ /v1/exec             │ │     │                                      │
│  │  - 验证证书 CN       │ │     │  git fetch / git push origin         │
│  │  - 注入 --identity   │ │     │  (origin = 代理 git 端点)            │
│  │  - 在宿主机运行 gt/bd│ │     │  ↓                                   │
│  │                      │ │◄────┼──── git smart HTTP 通过 mTLS ────────┘
│  │ /v1/git/<rig>/       │ │     │
│  │  upload-pack (fetch) │ │     │  容器 git remote：
│  │  receive-pack (push) │ │     │    origin = https://host:9876/v1/git/<rig>
│  │  ↕ 宿主机上的 .repo.git│ │     │
│  └──────────────────────┘ │     容器需要：
│         │                 │     - gt-proxy-client 二进制（作为 gt + bd）
│         │ daemon 推送     │     - GT_PROXY_URL, GT_PROXY_CERT, GT_PROXY_KEY
│         ▼ 到 GitHub       │     - GIT_SSL_CERT, GIT_SSL_KEY, GIT_SSL_CAINFO
│  GitHub  ◄───────────    │     (全部在会话生成时注入)
│  (上游，仅宿主机)         │
└───────────────────────────┘
```

容器从不联系 GitHub。所有 git 流量通过：**容器 ↔ 代理 ↔ `.repo.git`**。宿主机 daemon 异步推送到 GitHub。

---

## 4. 设计

### 4.1 启动命令包裹 — `ExecWrapper`

最简单的干预：在 `RuntimeConfig` 中添加 `ExecWrapper []string` 字段。启动命令构建器在 `exec env VAR=val ...` 和 agent 二进制之间插入包裹标记。

```
# 本地（无包裹）：
exec env GT_RIG=gastown GT_POLECAT=furiosa ... claude --mode=direct

# exitbox：
exec env GT_RIG=gastown GT_POLECAT=furiosa ... \
    exitbox run --profile=gastown-polecat -- claude --mode=direct

# daytona：
exec env GT_RIG=gastown GT_POLECAT=furiosa ... \
    daytona exec furiosa-ws -- claude --mode=direct
```

这包裹了整个会话；tmux 仍然管理 pane，`tmux send-keys` 仍然传递 nudge — 消息层无需变更。

暴露方式：
- `settings/config.json`：`agent.exec_wrapper: ["exitbox", "run", "--profile=gastown-polecat", "--"]`
- CLI 标志：`gt sling <bead> --exec-wrapper "..."`

### 4.2 mTLS 代理 — `gt-proxy-server` 和 `gt-proxy-client`

两个新的轻量级二进制文件处理容器 → 宿主机的所有通信。

#### gt-proxy-server（运行在宿主机上）

- 在配置的地址和端口上监听（`proxy_listen_addr`，例如 `0.0.0.0:9876`）
- 要求 mTLS：客户端证书必须由 GasTown CA 签名
- **CLI 中继模型**：将 argv 转发到宿主机上的 `gt`/`bd`，并将 stdout/stderr/exitCode 原样流回
- 注入 `--identity <rig>/<name>`（从证书 `CN=gt-<rig>-<name>` 提取）用于需要它的命令
- 维护允许的子命令的显式白名单 — 不允许任意 shell 执行

```
POST /v1/exec
  body:     {"argv": ["gt", "mail", "inbox", "--json"]}
  response: {"stdout": "...", "stderr": "...", "exitCode": 0}
```

CLI 中继方式意味着：
- 零维护开销：新的 `gt`/`bd` 子命令和标志变更自动生效
- 构造即正确：代理执行与本地调用相同的代码路径
- 身份由证书建立，作为 CLI 标志注入 — 无需内部 API 连接

#### gt-proxy-client（运行在容器中，替代 `gt` 和 `bd`）

- 检测环境中的 `GT_PROXY_URL` + `GT_PROXY_CERT` + `GT_PROXY_KEY`
- 如果设置：通过 mTLS 将 argv 整体转发给代理服务器，打印响应，以服务器的退出码退出
- 如果未设置：透传到正常本地执行（向后兼容；本地 polecat 使用）
- 通过符号链接安装为 `gt` 和 `bd`

#### Git 中继 — 通过 `.repo.git` fetch 和 push

容器中的所有 git 操作通过代理路由到宿主机上的 `.repo.git`。代理通过 mTLS 使用 git smart HTTP：

```
# 克隆 / fetch（upload-pack）
GET  /v1/git/<rig>/info/refs?service=git-upload-pack
POST /v1/git/<rig>/git-upload-pack

# Push（receive-pack）
GET  /v1/git/<rig>/info/refs?service=git-receive-pack
POST /v1/git/<rig>/git-receive-pack
```

代理作为子进程对 `~/gt/<rig>/.repo.git` 运行 `git upload-pack` 或 `git receive-pack`。

**容器从不联系 GitHub。** 其 `origin` remote 指向代理：
```
remote.origin.url = https://<host>:9876/v1/git/<rig>
```

分支范围授权由证书 CN 强制：polecat 只能推送 `polecat/<cn-name>-*` 下的 ref；尝试推送 `main` 或其他 polecat 的分支会被拒绝（403）。Fetch 不受限（只读）。

`.repo.git`（GasTown 已在 `~/gt/<rig>/.repo.git` 维护的裸仓库）是理想的端点：
- 它已有 `origin` → GitHub 的配置在宿主机侧
- 它是裸仓库 — 既可以服务 fetch 也可以无条件接收 push
- `gt done` 已将其作为备用推送目标
- 所有 polecat worktree 从它创建

**宿主机 → GitHub 同步：** 成功的 receive-pack 后，代理将异步上游推送任务入队（`git -C .repo.git push origin <branch>`）。宿主机也定期从 GitHub fetch，以便 `.repo.git` 为新容器克隆保持最新。

### 4.3 CA 和按 Polecat 证书

GasTown 在 daemon 启动时生成自签名 CA（`~/gt/.runtime/ca/`）。对每个 daytona 模式的 polecat，它签发短期叶证书：

- **CN**：`gt-<rig>-<name>`（例如 `gt-gastown-furiosa`）
- **SAN**：`session:<sessionID>`
- **TTL**：通过 `proxy_cert_ttl` 配置（默认 24h）

五个环境变量设置在 polecat 的启动环境中：

| 变量 | 用途 |
|------|------|
| `GT_PROXY_URL` | `https://<host>:9876` |
| `GT_PROXY_CERT` | 客户端证书 PEM 路径 |
| `GT_PROXY_KEY` | 客户端密钥 PEM 路径 |
| `GIT_SSL_CERT` | 同一证书 — git 用于与代理的 mTLS |
| `GIT_SSL_KEY` | 同一密钥 — git 用于与代理的 mTLS |
| `GIT_SSL_CAINFO` | CA 证书 — git 用于信任代理 TLS 证书 |

会话结束时，证书被添加到内存拒绝列表。来自该证书的后续代理调用立即被拒绝。

### 4.4 Daytona 工作空间生命周期

#### `daytona exec` 不创建容器

`daytona exec <ws> -- cmd` 连接到已经运行的工作空间容器。它类似于 `docker exec` 或 `ssh user@host cmd` — 它要求工作空间已经存在并运行。GasTown 必须拥有完整的工作空间生命周期：

```
daytona create → daytona start → [daytona exec，反复] → daytona stop → daytona delete
      ▲                ▲                    ▲                      ▲               ▲
  gt sling         创建时自动          polecat 会话          gt session     清理
  （每个 polecat                                                         stop
   一次）
```

#### 工作空间状态与 GasTown 操作

| 状态 | daytona CLI | GasTown 触发器 |
|------|-------------|---------------|
| 不存在 | `daytona create <repo> --name <ws>` | `gt sling`（此 polecat 首次） |
| 已停止 | `daytona start <ws>` | `gt session start` / `gt sling` 恢复 |
| 运行中 | `daytona exec <ws> -- cmd` | 正常 polecat 操作 |
| 运行中，polecat 完成 | `daytona stop <ws>` | `gt session stop` / TTL 过期 |
| 不再需要 | `daytona delete <ws>` | `gt polecat remove` / 手动 |

GasTown 在会话结束时停止（而非删除）工作空间，为下次会话保留 git 状态。删除是显式操作员操作。

#### `gt sling` 时的完整配置序列

```
gt sling <bead> --daytona
  │
  ├─ 1. 创建 polecat 分支（宿主机，即时）：
  │       git -C ~/gt/<rig>/.repo.git fetch origin
  │       git -C ~/gt/<rig>/.repo.git branch polecat/<name>-<ts> origin/main
  │
├─ 2. 签发 polecat mTLS 证书（宿主机，即时）
  │
  ├─ 3. 配置 daytona 工作空间（慢：30-120s）：
  │       daytona create https://<host>:9876/v1/git/<rig>
  │         --name gt-<rig>-<polecat>
  │         --branch polecat/<name>-<ts>
  │         --devcontainer-path .devcontainer/gastown-polecat
  │       （从代理克隆 → .repo.git；运行 onCreateCommand）
  │
  ├─ 4. 注入证书到工作空间：
  │       daytona exec gt-<rig>-<polecat> -- mkdir -p /run/gt-proxy
  │       daytona exec gt-<rig>-<polecat> -- tee /run/gt-proxy/client.crt < <cert>
  │       daytona exec gt-<rig>-<polecat> -- tee /run/gt-proxy/client.key < <key>
  │       daytona exec gt-<rig>-<polecat> -- tee /run/gt-proxy/ca.crt < <ca>
  │
  ├─ 5. 创建后设置：
  │       daytona exec gt-<rig>-<polecat> -- gt prime --write-prime-md
  │       daytona exec gt-<rig>-<polecat> -- [覆盖文件，设置 hook]
  │
  ├─ 6. 通过代理注册 agent bead：
  │       （代理客户端调用 bd create/update 设置 state=spawning）
  │
  └─ 7. 启动 tmux pane：
          tmux new-window -n <polecat>
          tmux send-keys "daytona exec gt-<rig>-<polecat> \
            --env GT_RIG=<rig> --env GT_POLECAT=<name> \
            --env GT_PROXY_URL=... --env GT_PROXY_CERT=... \
            --env GT_PROXY_KEY=... --env GIT_SSL_CERT=... \
            --env GIT_SSL_KEY=... --env GIT_SSL_CAINFO=... \
            -- claude --mode=direct" Enter
```

步骤 3 是慢步骤。步骤 1-2 是即时的。对于生产环境，工作空间可以预配置（热池），带有通用 devcontainer 设置；步骤 3 变为 `daytona start` 而非 `daytona create`。

#### Git 拓扑：代理服务的克隆

对于本地 polecat，`AddWithOptions` 创建 git worktree — 从 `.repo.git` 的链接检出，共享对象存储。对于 daytona polecat，容器独立从代理的 git 端点克隆。分支在 `.repo.git` 中本地创建；配置前不需要 GitHub 推送。

```
宿主机（.repo.git）                  容器
┌──────────────────┐                 ┌──────────────────────┐
│ origin → GitHub  │   git clone     │  origin → 代理        │
│                  │ ◄──── 通过 ────► │  （完整独立           │
│ polecat/nova-42  │   mTLS 代理      │   .git，非 worktree） │
└──────────────────┘                 └──────────────────────┘
        ▲                                     │
        │ daemon 推送                         │ git push origin
        ▼                                     ▼
      GitHub                            代理 receive-pack
                                        → .repo.git → GitHub
```

#### Daytona 不需要而本地需要的

- 不需要宿主机侧 `polecats/<name>/<rig>/` 目录 — 容器本身就是 worktree
- 不需要 `git worktree add` — 容器从代理克隆，代理从 `.repo.git` 服务
- 不需要 `.beads` 重定向文件 — 所有 Dolt 访问通过 mTLS 代理
- 不需要 `manager.go` 中的 `WorktreeAddFromRef` 调用 — daytona 模式跳过它
- 不需要配置前的 GitHub 推送 — 分支只需要存在于 `.repo.git` 中
- 不需要单独的 `pushurl` 覆盖 — `origin` 同时指向代理的 fetch 和 push

#### Devcontainer 配置

```json
// .devcontainer/gastown-polecat/devcontainer.json
{
  "name": "GasTown Polecat",
  "image": "ubuntu:24.04",
  "onCreateCommand": "bash .devcontainer/gastown-polecat/setup.sh",
  "remoteUser": "vscode"
}
```

```bash
# .devcontainer/gastown-polecat/setup.sh
set -e
npm install -g @anthropic-ai/claude-code
curl -fsSL https://releases.gastown.dev/gt-proxy-client/latest/linux-amd64 -o /usr/local/bin/gt
chmod +x /usr/local/bin/gt
ln -sf /usr/local/bin/gt /usr/local/bin/bd
apt-get install -y git
```

或者，GasTown 可以分发预构建的 Docker 镜像（`ghcr.io/steveyegge/gastown-polecat:latest`）并直接引用它，绕过设置脚本。这对生产使用更可靠。

`DaytonaConfig` 结构：

```go
type DaytonaConfig struct {
    WorkspaceID string `json:"workspace_id"`
    Profile     string `json:"profile,omitempty"`     // devcontainer 名称
    Image       string `json:"image,omitempty"`       // 直接覆盖镜像
    AutoStop    bool   `json:"auto_stop,omitempty"`   // 会话结束后停止工作空间
    AutoDelete  bool   `json:"auto_delete,omitempty"` // 会话结束后删除工作空间
}
```

---

## 5. Nudge、观察与多 Polecat 会话

### 5.1 Nudge 如何仍然工作

`NudgeSession` 通过 `tmux send-keys -l` 向本地 tmux pane 发送按键。`daytona exec <ws> -- claude --mode=direct` 的行为与 SSH 连接的进程完全一样：本地 tmux pane 运行 `daytona` CLI，它将 stdin/stdout 代理到远程容器。从本地 tmux 服务器的角度看，pane 是活跃的并接受输入；`send-keys` 将按键传递到 `daytona exec` stdin 流，后者转发到远程 Claude 进程。**`NudgeSession`、`WaitForIdle` 或 nudge 队列无需变更。**

```
宿主机 tmux 服务器
┌──────────────────────────────────────────────────────────────────┐
│ session：gt-gastown-furiosa                                      │
│ pane %3                                                          │
│   process：daytona ◄── tmux send-keys 目标此 pane              │
│              │                                                   │
│              │ stdin/stdout 隧道（daytona exec 协议）            │
│              ▼                                                   │
│        ┌────────────────────────────────────┐  (远程)           │
│        │ daytona 工作空间：furiosa-ws       │                   │
│        │   claude --mode=direct              │                   │
│        └────────────────────────────────────┘                   │
└──────────────────────────────────────────────────────────────────┘
```

### 5.2 存活检测

当前 `IsAgentAlive` 遍历本地进程树查找 `claude`。以 `daytona exec` 作为 pane 进程，`claude` 远程运行，对本地进程树不可见。

**选项 1（初始实现选择）：** 在会话生成时将 `daytona` 添加到 `GT_PROCESS_NAMES` — 存活性即"daytona exec 连接正常"。简单且实际中正确：如果 `daytona exec` 退出，会话已死。这由 G5 处理（`ExecWrapper[0]` 自动添加到接受进程名称）。

**选项 2（未来）：** 健康检查端点 — polecat 通过 mTLS 代理定期写入心跳；daemon 检查过期心跳。更准确但更复杂。

### 5.3 人工观察

在宿主机上 attach 到任何 polecat 的 tmux pane：

```bash
tmux attach -t gt-gastown-furiosa        # 交互式
tmux attach -t gt-gastown-furiosa -r     # 只读
```

终端输出是通过 `daytona exec` 隧道渲染的远程 Claude TUI — 与观看本地 polecat 相同。

### 5.4 多 Polecat 窗口分组（可选）

对于远程 polecat，将它们分组到一个 tmux 会话的多个窗口中是符合人体工学的 — 每个 polecat 一个窗口：

```
tmux session：gt-gastown（每个 rig 一个会话）
  window 0：furiosa    ← daytona exec furiosa-ws -- claude
  window 1：nova       ← daytona exec nova-ws -- claude
  window 2：drake      ← daytona exec drake-ws -- claude
  window 3：overseer   ← 供人类操作员使用的自由 shell
```

`FindAgentPane` 已经处理多窗口会话（通过 `tmux list-panes -s` 枚举所有 pane），因此 nudge 路径无需变更。窗口分组通过 `group_sessions: true` 按 rig 启用。启用后，`gt sling` 在现有 rig 会话中创建新窗口而非新会话。

### 5.5 Nudge/观察所需变更总结

| 关注点 | 所需变更 |
|--------|----------|
| Nudge 投递 | **无** — `send-keys` 到本地 pane，daytona exec 隧道传递 |
| Mail nudge 队列 | **无** — 相同路径，相同代码 |
| 存活检测 | **G5** — 将 `daytona` 添加到 `GT_PROCESS_NAMES` |
| 人工观察 | **无** — `tmux attach` 原样工作 |
| 多 Polecat 窗口分组 | **可选** — 新的 `group_sessions` 设置 + G6 中的窗口创建 |

---

## 6. 实现计划

交付物按顺序排列：首先是独立工作（无 GasTown 变更），然后是按依赖顺序的 GasTown 变更。

### 6.1 独立交付物（无 GasTown 变更）

**S1 — exitbox 策略配置**

编写允许 polecat 会话的策略文件：
- 读取 + 执行：`gt`、`bd`、`claude`、`node`、`git`
- 读取 + 写入：polecat worktree（`~/gt/<rig>/polecats/<name>/`）
- 读取：town 共享目录（`~/gt/.beads/`、`~/gt/.runtime/`）
- 网络：仅回环（`127.0.0.1:3307`）
- 写入：心跳和 nudge 队列目录

手动测试：在 tmux pane 中运行 `exitbox run --profile=gastown-polecat -- claude --mode=direct`。运行 `gt prime` → `gt done`。

**S2 — 独立的 `gt-proxy-server` + `gt-proxy-client`**

完全在 GasTown 外构建和测试。启动任何 Docker 容器，注入证书环境变量，从内部运行 `gt prime` 和 `gt done`。

此步骤回答的开放问题：`daytona exec` 是继承父环境还是需要显式 `--env` 标志？

**S3 — daytona 冒烟测试**

在宿主机上运行 S2 代理的情况下，手动演练完整的 polecat 生命周期：
1. 测试 `daytona create` 是否接受自定义 git 端点 URL 作为仓库源：
   ```bash
   daytona create https://<host>:9876/v1/git/<rig> \
     --name test-polecat --branch polecat/test-1
   ```
   如果成功：容器从代理克隆 → `.repo.git`。理想路径。
   如果 daytona 只接受 GitHub URL：回退 — `daytona create <github-url>` + 创建后通过 `daytona exec` 执行 `git remote set-url origin https://<proxy>/v1/git/<rig>`。
2. 显式注入证书和环境变量，运行 `gt prime`、`gt hook`、`gt done`。
3. 验证 `git push origin` 路由到代理 → 落入宿主机的 `.repo.git`。
4. 验证 `git fetch origin` 从代理拉取 → `.repo.git`（而非从 GitHub）。
5. `daytona stop test-polecat` — 验证工作空间持久；`daytona start` + 重新 exec 工作。

此步骤确认：(a) 从 daytona 容器内部可达哪个宿主机 IP/地址，(b) 容器的 git 二进制是否遵守 `GIT_SSL_*` 变量，(c) daytona 是否支持自定义 git 端点用于克隆。

### 6.2 GasTown 代码变更

| ID | 变更 | 文件 | 大小 |
|----|------|------|------|
| G1 | `BD_DOLT_HOST` / `BD_DOLT_PORT` 环境变量 | `internal/beads/beads.go` | ~8 行 |
| G2 | CA 管理 + 证书签发 | `internal/proxy/ca.go`（新） | ~50 行 |
| G3 | 代理服务器集成到 daemon | `internal/proxy/server.go`（新） | ~80 行 |
| G4 | `ExecWrapper` 字段 + 启动命令穿线 | `internal/config/types.go`、`internal/config/loader.go` | ~35 行 |
| G5 | 包裹启动器的进程检测 | `internal/tmux/tmux.go` | ~12 行 |
| G6 | `DaytonaConfig` + 工作空间配置 | `internal/config/types.go`、`internal/daytona/`（新） | ~150 行 |
| G7 | daytona 模式 polecat 跳过本地 worktree 创建 | `internal/polecat/manager.go` | ~25 行 |

### 6.3 依赖顺序

```
S1 ──────────────────────────────────────────────────────► exitbox 验证
S2 ──────────────────────────────────────────────────────► 代理验证
S3（依赖 S2） ──────────────────────────────────────────► daytona 未知项解决
        │
        ▼
G1  BD_DOLT_HOST/PORT
G4  RuntimeConfig 中的 ExecWrapper
G5  进程检测修复
        │
        ├──────────────────────────────────────────────────► exitbox 端到端 ✓
        │
G2  CA + 证书签发
G3  daemon 中的代理服务器（包裹 S2 二进制）
G6  DaytonaConfig + 配置
G7  跳过本地 worktree
        │
        └──────────────────────────────────────────────────► daytona 端到端 ✓
```

---

## 7. 考虑的替代方案

### 7.1 `SessionBackend` 接口 / 远程 tmux

一个替代层替换 `tmux new-session` 为通用后端接口。初始实现拒绝：`daytona exec` 从 tmux 角度已经表现得像本地进程，因此后端抽象无所获。仅在 `daytona exec` 证明对 nudge 投递不够用时重新考虑。

### 7.2 exitbox 使用 mTLS 代理

过度。由于 exitbox 将所有内容保持在宿主机上且回环 Dolt 访问已经安全，代理对 exitbox 情况不增加安全收益。

### 7.3 其他运行时（Docker、Nix 沙箱、Firecracker）

`ExecWrapper` 一旦模式验证，可推广到所有这些运行时。特定运行时的配置结构（如 `DaytonaConfig`）可以单独添加而无需架构变更。

### 7.4 多宿主联邦或代理链

超出范围。

---

## 8. 验收标准

### exitbox

- [ ] `exitbox run --profile=gastown-polecat -- gt prime` 在沙箱内成功（回环 Dolt 可达）
- [ ] `gt sling <bead> --exec-wrapper "exitbox run --profile=gastown-polecat --"` 启动活跃会话
- [ ] Polecat 通过 `tmux send-keys` 在 exitbox pane 中接收 nudge
- [ ] `gt done` 在沙箱内完全完成：git push 到远程 + 通过回环 Dolt 的 bd update
- [ ] 存活检测看到正确的进程（exitbox 或 agent，取决于 exec 行为）
- [ ] 现有本地 polecat 不受影响（无回归）

### daytona + 代理

- [ ] `gt-proxy-server` 在宿主机上启动；CA 在 `~/gt/.runtime/ca/` 初始化
- [ ] Polecat 证书签发并注入 daytona 工作空间 `/run/gt-proxy/`
- [ ] 容器内 `gt prime` 成功（控制平面通过代理路由）
- [ ] 容器内 `gt done`：`git push origin` → 代理 receive-pack → 宿主机 `.repo.git` → daemon 推送到 GitHub
- [ ] 容器内 `git fetch origin`：从代理 fetch → `.repo.git`（不从 GitHub）
- [ ] 代理拒绝推送到 `main` 或其他 polecat 的分支（CN 范围授权）
- [ ] 代理拒绝来自已撤销或不匹配证书的控制平面调用
- [ ] `gt sling <bead> --daytona <workspace>` 端到端配置工作空间、签发证书、启动会话
- [ ] Nudge 通过运行 `daytona exec` 的 tmux pane 投递
- [ ] daytona 模式 polecat 跳过本地 worktree 创建
- [ ] 会话结束：证书加入拒绝列表；后续代理调用被拒绝
- [ ] 容器在零出站互联网访问下运行，所有操作成功

---

## 9. 开放问题

1. **宿主机可达性** — 从 daytona 云容器内部可达什么地址：固定宿主机 IP、`host.docker.internal`，还是 daytona 特定隧道？决定 `GT_PROXY_URL` 的值。由 S3 回答。

2. **`daytona create` 的自定义 git 端点** — `daytona create` 接受任意 HTTPS URL 作为仓库源，还是仅接受 GitHub/GitLab URL？如果仅后者，回退方案是：`daytona create <github-url>` + 创建后通过 `daytona exec` 执行 `git remote set-url origin <proxy-url>`。由 S3 回答。

3. **上游推送触发器** — Daemon 如何检测新分支到达 `.repo.git` 以推送到 GitHub？选项：代理侧成功 receive-pack 后入队（当前计划）；`.repo.git/hooks/post-receive` 中的 post-receive hook；daemon ref-watcher。代理侧入队最简单。

4. **宿主机侧 `.repo.git` 新鲜度** — Daemon 必须定期 `git fetch origin` 到 `.repo.git`，以便容器 fetch 看到最新的 ref。多久一次？按需由代理 upload-pack 触发，还是定时？

5. **工作空间热池** — 首次 `daytona create` 需要 30-120 秒。对于低延迟的 `gt sling`，GasTown 是否应维护预配置的热工作空间池？可选优化，初始实现不需要。

6. **Devcontainer 分发** — 在 GasTown 仓库中发布 `.devcontainer/gastown-polecat/`，还是发布独立的 Docker 镜像（`ghcr.io/steveyegge/gastown-polecat:latest`）？镜像方式对生产更可靠；devcontainer 更透明且自包含。