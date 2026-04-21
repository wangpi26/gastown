# Gas Town 架构

Gas Town 多 agent 工作空间管理的技术架构。

## 两级 Beads 架构

Gas Town 使用两级 beads 架构，将组织协调与项目实现工作分离。

| 级别 | 位置 | 前缀 | 用途 |
|------|------|------|------|
| **Town** | `~/gt/.beads/` | `hq-*` | 跨 rig 协调、Mayor 邮件、agent 身份 |
| **Rig** | `<rig>/mayor/rig/.beads/` | 项目前缀 | 实现工作、MR、项目 issue |

### Town 级 Beads（`~/gt/.beads/`）

用于跨 rig 协调的组织链：
- Mayor 邮件和消息
- Convoy 协调（跨 rig 批量工作）
- 战略性 issue 和决策
- **Town 级 agent beads**（Mayor、Deacon）
- **角色定义 beads**（全局模板）

### Rig 级 Beads（`<rig>/mayor/rig/.beads/`）

用于实现工作的项目链：
- 项目的 bug、feature、task
- Merge request 和代码审查
- 项目特定的 molecules
- **Rig 级 agent beads**（Witness、Refinery、Polecats）

## Agent Bead 存储

Agent beads 追踪每个 agent 的生命周期状态。存储位置取决于 agent 的作用域。

| Agent 类型 | 作用域 | Bead 位置 | Bead ID 格式 |
|------------|--------|-----------|--------------|
| Mayor | Town | `~/gt/.beads/` | `hq-mayor` |
| Deacon | Town | `~/gt/.beads/` | `hq-deacon` |
| Boot | Town | `~/gt/.beads/` | `hq-boot` |
| Dogs | Town | `~/gt/.beads/` | `hq-dog-<name>` |
| Witness | Rig | `<rig>/.beads/` | `<prefix>-<rig>-witness` |
| Refinery | Rig | `<rig>/.beads/` | `<prefix>-<rig>-refinery` |
| Polecats | Rig | `<rig>/.beads/` | `<prefix>-<rig>-polecat-<name>` |
| Crew | Rig | `<rig>/.beads/` | `<prefix>-<rig>-crew-<name>` |

### 角色 Beads

角色 beads 是存储在 town beads 中的全局模板，使用 `hq-` 前缀：
- `hq-mayor-role` - Mayor 角色定义
- `hq-deacon-role` - Deacon 角色定义
- `hq-boot-role` - Boot 角色定义
- `hq-witness-role` - Witness 角色定义
- `hq-refinery-role` - Refinery 角色定义
- `hq-polecat-role` - Polecat 角色定义
- `hq-crew-role` - Crew 角色定义
- `hq-dog-role` - Dog 角色定义

每个 agent bead 通过 `role_bead` 字段引用其角色 bead。

## Agent 分类

### Town 级 Agent（跨 Rig）

| Agent | 角色 | 持久性 |
|-------|------|--------|
| **Mayor** | 全局协调器，处理跨 rig 通信和升级 | 持久 |
| **Deacon** | 守护信标 — 接收心跳，运行插件和监控 | 持久 |
| **Boot** | Deacon 看门狗 — 由 daemon 生成用于分诊决策，当 Deacon 宕机时启动 | 临时 |
| **Dogs** | 用于跨 rig 批量工作的长时运行 worker | 可变 |

### Rig 级 Agent（按项目）

| Agent | 角色 | 持久性 |
|-------|------|--------|
| **Witness** | 监控 polecat 健康，处理催促和清理 | 持久 |
| **Refinery** | 处理合并队列，运行验证 | 持久 |
| **Polecats** | 持久身份的 worker，被分配到特定 issue | 持久身份，临时会话 |
| **Crew** | 人类工作空间 — 完整 git clone，用户管理生命周期 | 持久 |

## 目录结构

```
~/gt/                           Town 根目录
├── .beads/                     Town 级 beads（hq-* 前缀）
│   ├── metadata.json           Beads 配置（dolt_mode、dolt_database）
│   └── routes.jsonl            前缀 → rig 路由表
├── .dolt-data/                 集中式 Dolt 数据目录
│   ├── hq/                     Town beads 数据库（hq-* 前缀）
│   ├── gastown/                Gastown rig 数据库（gt-* 前缀）
│   ├── beads/                  Beads rig 数据库（bd-* 前缀）
│   └── <其他 rigs>/           按 rig 的数据库
├── daemon/                     Daemon 运行时状态
│   ├── dolt-state.json         Dolt 服务器状态（pid、port、databases）
│   ├── dolt-server.log         服务器日志
│   └── dolt.pid                服务器 PID 文件
├── deacon/                     Deacon 工作空间
│   └── dogs/<name>/            Dog worker 目录
├── mayor/                      Mayor agent 主目录
│   ├── town.json               Town 配置
│   ├── rigs.json               Rig 注册表
│   ├── daemon.json             Daemon 巡逻配置
│   └── accounts.json           Claude Code 账户管理
├── settings/                   Town 级设置
│   ├── config.json             Town 设置（agents、themes）
│   └── escalation.json         升级路由和联系人
├── directives/                 Town 级角色指令（操作员策略）
│   └── <role>.md               prime 时注入的 Markdown
├── formula-overlays/           Town 级 formula 覆盖
│   └── <formula>.toml          TOML 步骤覆盖（replace/append/skip）
├── config/
│   └── messaging.json          邮件列表、队列、频道
└── <rig>/                      项目容器（不是 git clone）
    ├── config.json             Rig 身份和 beads 前缀
    ├── directives/             Rig 级角色指令（覆盖 town）
    │   └── <role>.md
    ├── formula-overlays/       Rig 级 formula 覆盖（完全优先）
    │   └── <formula>.toml
    ├── mayor/rig/              规范 clone（beads 存在于此，不是 agent）
    │   └── .beads/             Rig 级 beads（重定向到 Dolt）
    ├── refinery/               Refinery agent 主目录
    │   └── rig/                来自 mayor/rig 的 worktree
    ├── witness/                Witness agent 主目录（无 clone）
    ├── crew/                   Crew 父目录
    │   └── <name>/             人类工作空间（完整 clone）
    └── polecats/               Polecats 父目录
        └── <name>/<rigname>/   来自 mayor/rig 的 worker worktree
```

**注意**：不创建每目录的 CLAUDE.md 或 AGENTS.md。磁盘上仅存在 `~/gt/CLAUDE.md`
（town 根身份锚点）。完整上下文由 `gt prime` 通过 SessionStart hook 注入。

### Worktree 架构

Polecats 和 refinery 是 git worktree，而非完整 clone。这使得快速生成
和共享对象存储成为可能。Worktree 基础是 `mayor/rig`：

```go
// 来自 polecat/manager.go - worktree 基于 mayor/rig
git worktree add -b polecat/<name>-<timestamp> polecats/<name>
```

Crew 工作空间（`crew/<name>/`）是完整 git clone，面向需要独立仓库的人类开发者。
Polecat 会话是临时的，受益于 worktree 效率。

## 存储层：Dolt SQL Server

所有 beads 数据存储在每个 town 的单个 Dolt SQL Server 进程中。
没有嵌入式 Dolt 回退 — 如果服务器宕机，`bd` 会快速失败并显示明确的
错误，指向 `gt dolt start`。

```
┌─────────────────────────────────┐
│  Dolt SQL Server（每个 town）     │
│  端口 3307，由 daemon 管理       │
│  数据: ~/gt/.dolt-data/         │
└──────────┬──────────────────────┘
           │ MySQL 协议
    ┌──────┼──────┬──────────┐
    │      │      │          │
  USE hq  USE gastown  USE beads  ...
```

每个 rig 数据库是 `.dolt-data/` 下的子目录。Daemon 在每次心跳时
监控服务器，崩溃时自动重启。

对于写入并发，所有 agent 直接写入 `main`，使用事务纪律
（`BEGIN` / `DOLT_COMMIT` / `COMMIT` 原子操作）。这消除了
分支增殖并确保跨 agent 的即时可见性。

详见 [dolt-storage.md](dolt-storage.md)。

## Beads 路由

`routes.jsonl` 文件将 issue ID 前缀映射到 rig 位置（相对于 town 根目录）：

```jsonl
{"prefix":"hq-","path":"."}
{"prefix":"gt-","path":"gastown/mayor/rig"}
{"prefix":"bd-","path":"beads/mayor/rig"}
```

路由指向 `mayor/rig`，因为那是规范 `.beads/` 所在位置。
这实现了透明的跨 rig beads 操作：

```bash
bd show hq-mayor    # 路由到 town beads（~/.gt/.beads）
bd show gt-xyz      # 路由到 gastown/mayor/rig/.beads
```

## Beads 重定向

Worktree（polecats、refinery、crew）没有自己的 beads 数据库。
相反，它们使用 `.beads/redirect` 文件指向规范的 beads 位置：

```
polecats/alpha/.beads/redirect → ../../mayor/rig/.beads
refinery/rig/.beads/redirect   → ../../mayor/rig/.beads
```

`ResolveBeadsDir()` 遵循重定向链（最大深度 3）并进行循环检测。
这确保 rig 中的所有 agent 通过 Dolt 服务器共享单个 beads 数据库。

## 合并队列：Batch-then-Bisect

Refinery 通过 batch-then-bisect 合并队列（Bors 风格）处理 MR。
这是核心能力，而非可插拔策略。

### 工作原理

```
等待的 MRs:  [A, B, C, D]
                    ↓
批量:        将 A..D 作为栈 rebase 到 main
                    ↓
测试栈顶:    在 D 上运行测试（栈顶）
                    ↓
如果通过:      快进合并所有 4 个 → 完成
如果失败:      二分查找 → 测试 B（中点）
                    ↓
              如果 B 通过: C 或 D 导致失败 → 二分 [C,D]
              如果 B 失败: A 或 B 导致失败 → 二分 [A,B]
```

### 实现阶段

| 阶段 | Bead | 内容 | 状态 |
|------|------|------|------|
| 1: GatesParallel | gt-8b2i | 并行运行每个 MR 的测试 + lint | 进行中 |
| 2: Batch-then-bisect | gt-i2vm | Bors 风格批量合并与二分查找 | 被阶段 1 阻塞 |
| 3: Pre-verification | gt-lu84 | Polecats 在提交 MR 前运行测试 | 被阶段 2 阻塞 |

Gates（测试命令、lint 等）是可插拔的。批量合并策略是核心。

设计文档：由 gt-yxx0 review 产出。

## Polecat 生命周期：自主完成

Polecats 端到端管理自己的生命周期。Witness 观察，但不控制完成。
这防止 Witness 成为瓶颈。

### Polecat 完成流程

```
Polecat 完成工作
  → 推送分支到远程
  → 提交 MR（bd update --mr-ready）
  → 更新 bead 状态
  → 拆除 worktree
  → 进入空闲（可接受下一个分配）
```

Witness 监控卡住/僵尸的 polecats（长时间无活动）
并进行催促或升级。它不处理完成 — 那是 polecat 的工作。

设计 bead：gt-0wkk。

## 数据平面生命周期

所有 beads 数据流经由 Dogs 管理的六阶段生命周期：

```
CREATE → LIVE → CLOSE → DECAY → COMPACT → FLATTEN
  │        │       │        │        │          │
  Dolt   active   done   DELETE   REBASE     SQUASH
  commit  work    bead    rows    commits    all history
                         >7-30d  together   to 1 commit
```

阶段 1-3 目前已自动化。阶段 4-6 正在通过 Dog 自动化交付
（gt-at0i Reaper DELETE、gt-l8dc Compactor REBASE、gt-emm4 Doctor gc）。

详见 [dolt-storage.md](dolt-storage.md)。

## 部署制品

Gas Town 和 Beads 通过多个渠道分发。标签推送（`v*`）触发 GitHub Actions
发布工作流来构建和发布所有内容。

### Gas Town（`gt`）

| 渠道 | 制品 | 触发器 |
|------|------|--------|
| **GitHub Releases** | 平台二进制文件（darwin/linux/windows，amd64/arm64）+ 校验和 | 标签推送时的 GoReleaser |
| **Homebrew** | `brew install steveyegge/gastown/gt` — 发布时自动更新的 formula | `update-homebrew` job 推送到 `steveyegge/homebrew-gastown` |
| **npm** | `npx @gastown/gt` — 下载正确二进制文件的封装 | OIDC 受信发布（无 token） |
| **本地构建** | `go build -o $(go env GOPATH)/bin/gt ./cmd/gt` | 手动 |

### Beads（`bd`）

| 渠道 | 制品 | 触发器 |
|------|------|--------|
| **GitHub Releases** | 平台二进制文件 + 校验和 | 标签推送时的 GoReleaser |
| **Homebrew** | `brew install steveyegge/beads/bd` | `update-homebrew` job |
| **npm** | `npx @beads/bd` — 下载正确二进制文件的封装 | OIDC 受信发布（无 token） |
| **PyPI** | `beads-mcp` — MCP 服务器集成 | 带 `PYPI_API_TOKEN` secret 的 `publish-pypi` job |
| **本地构建** | `go build -o $(go env GOPATH)/bin/bd ./cmd/bd` | 手动 |

### npm 认证

两个仓库均使用 **OIDC 受信发布** — 不需要 `NPM_TOKEN` secret。
认证由 GitHub 的 OIDC 提供方处理。工作流需要：

```yaml
permissions:
  id-token: write  # npm 受信发布所需
```

在 npmjs.com 上配置：Package Settings → Trusted Publishers → 链接到
GitHub 仓库和 `release.yml` 工作流文件。

### 二进制文件嵌入的内容

Go 二进制文件是主要的分发载体。它嵌入：
- **角色模板** — Agent priming 上下文，由 `gt prime` 提供
- **Formula 定义** — 工作流 molecules，由 `bd mol` 提供
- **Doctor 检查** — 健康诊断，包括迁移检查
- **默认配置** — `daemon.json` 生命周期默认值、操作阈值

这意味着升级二进制文件会自动传播大多数修复。非嵌入文件
（需要 `gt doctor` 或 `gt upgrade` 来更新）：
- Town 根的 `CLAUDE.md`（在 `gt install` 时创建）
- `daemon.json` 巡逻条目（在安装时创建，由 `EnsureLifecycleDefaults` 扩展）
- Claude Code hooks（`.claude/settings.json` 托管部分）
- Dolt schema（升级后首次 `bd` 命令时运行迁移）

## 角色 Directive 和 Formula Overlay

操作员可以在 town 或 rig 级别自定义 agent 行为，而无需修改
Go 二进制文件或嵌入模板。这遵循属性层模型（rig > town > system）
和 hooks 覆盖先例。

### 角色 Directive

在 `gt prime` 期间注入的按角色 Markdown 文件，位于角色模板之后、
上下文文件和 handoff 内容之前。操作员策略，在冲突时覆盖
formula 指令。

```
~/gt/directives/<role>.md              # Town 级（所有 rig）
~/gt/<rig>/directives/<role>.md        # Rig 级
```

两级内容连接（rig 内容在最后出现，冲突时优先）。
在 `internal/config/directives.go`（`LoadRoleDirective`）中实现，
通过 `internal/cmd/prime_output.go` 中的 `outputRoleDirectives()` 集成。

### Formula Overlay

修改单个步骤的按 formula TOML 文件。在 `showFormulaStepsFull()` 中
解析后、渲染前应用。

```
~/gt/formula-overlays/<formula>.toml   # Town 级
~/gt/<rig>/formula-overlays/<formula>.toml  # Rig 级（完全优先）
```

Rig 级 overlay 完全替代 town 级（不合并）。三种覆盖模式：

| 模式 | 效果 |
|------|------|
| `replace` | 完全替换步骤描述 |
| `append` | 在现有步骤描述后追加文本 |
| `skip` | 移除步骤（依赖方继承其 needs） |

在 `internal/formula/overlay.go`（`LoadFormulaOverlay`、`ApplyOverlays`）
中实现。`gt doctor` 根据当前 formula 定义验证 overlay 步骤 ID，
并可自动修复过时引用。

详见 [directives-and-overlays.md](directives-and-overlays.md) 获取
包含示例和设计理据的完整参考。

## 另见

- [dolt-storage.md](dolt-storage.md) - Dolt 存储架构
- [reference.md](../reference.md) - 命令参考
- [directives-and-overlays.md](directives-and-overlays.md) - Directive 和 overlay 参考
- [molecules.md](../concepts/molecules.md) - 工作流 molecules
- [identity.md](../concepts/identity.md) - Agent 身份和 BD_ACTOR