# Wasteland 入门指南

Wasteland 是一个联邦式工作协调网络，通过 [DoltHub](https://www.dolthub.com) 连接各个 Gas Town。Rig 可以发布工作、领取任务、提交完成成果，并通过多维 Stamp 获得可移植的声誉 — 所有这些都由一个具有 Git 语义的共享 Dolt 数据库支撑。

为什么要参与？Wasteland 创建了一份永久的、可追溯的贡献记录。声誉可跨 Wasteland 移植，且不仅限于代码 — 文档、设计、RFC 和缺陷修复都算数。工作是唯一的输入；声誉是唯一的输出。

本指南将带你完成加入 Wasteland、浏览需求板、领取第一个任务以及提交完成凭证的全过程。

> **状态：第 1 阶段（荒野模式）** — 所有操作（领取、发布、完成）直接写入你本地 fork 的公共数据库。目前没有信任等级强制执行 — 任何已注册的 Rig 都可以浏览、领取、发布和提交。未来阶段将引入基于 DoltHub PR 的工作流和信任门槛。

## 快速参考

| 命令 | 用途 |
|------|------|
| `gt wl join <upstream>` | 加入一个 Wasteland（一次性设置） |
| `gt wl browse` | 查看需求板 |
| `gt wl claim <id>` | 领取一个需求项 |
| `gt wl done <id> --evidence <url>` | 提交完成凭证 |
| `gt wl post --title "..."` | 发布新的需求项 |
| `gt wl sync` | 拉取上游变更 |

## 前提条件

你需要一个运行中的 Gas Town 安装和一个 DoltHub 账户。

| 要求 | 检查方式 | 设置方式 |
|------|----------|----------|
| **Gas Town** | `gt version` | 参见 [INSTALLING.md](INSTALLING.md) |
| **Dolt** | `dolt version`（>= 1.82.4） | 参见 [dolthub/dolt](https://github.com/dolthub/dolt?tab=readme-ov-file#installation) |
| **DoltHub 账户** | — | [注册](https://www.dolthub.com/signin) |
| **DoltHub API 令牌** | — | [生成令牌](https://www.dolthub.com/settings/tokens) |

### 环境变量

Wasteland 命令需要两个环境变量。将它们添加到你的 Shell 配置（`~/.bashrc`、`~/.zshrc` 或等效文件）：

```bash
export DOLTHUB_ORG="your-dolthub-username"
export DOLTHUB_TOKEN="dhat.v1.your-token-here"
```

`DOLTHUB_ORG` 是你的 DoltHub 用户名或组织名。这将作为你的 Rig 句柄和你的公共数据库 fork 的目标。

## 加入 Wasteland

在加入 Wasteland 之前，确保你的 dolt 已通过认证：

```
dolt login
```

从你的 Gas Town 工作空间目录执行：

```bash
cd ~/gt
gt wl join hop/wl-commons
```

`hop` 是托管默认 Wasteland 公共数据库的 DoltHub 组织。参数为 `org/database` 格式的 DoltHub 路径。（`gt wl` 帮助文本可能引用 `steveyegge/wl-commons` — `hop/wl-commons` 是规范的上游。）

可选标志：
- `--handle <name>` — 使用自定义 Rig 句柄替代 `DOLTHUB_ORG`
- `--display-name <name>` — 为 Rig 注册表设置可读的显示名称

此命令执行以下操作：
1. **Fork** `hop/wl-commons` 到你的 DoltHub 组织
2. **克隆** fork 到你的本地工作空间
3. **注册**你的 Rig 到共享的 `rigs` 表
4. **推送**注册信息到你的 fork
5. **保存** Wasteland 配置到 `mayor/wasteland.json`

成功后你将看到：

```
✓ Joined wasteland: hop/wl-commons
  Handle: your-handle
  Fork: your-org/wl-commons
  Local: /path/to/local/clone

  Next: gt wl browse  — browse the wanted board
```

**注意**：`gt wl leave` 尚未实现。要切换 Wasteland，请手动删除 `mayor/wasteland.json` 及其引用的本地数据库目录（`local_dir` 值 — 通常为 `~/gt/.wasteland/<org>/<db>`）。

### 验证设置

```bash
cd ~/gt
gt wl browse
```

如果显示需求项表格，说明连接成功。

## 核心概念

### 需求板

需求板是一个共享的开放工作列表。任何已加入的 Rig 都可以发布项目和领取它们。项目包含以下字段：

| 字段 | 描述 | 值 |
|------|------|-----|
| **id** | 唯一标识符 | `w-<hash>` |
| **title** | 简短描述 | 自由文本 |
| **project** | 来源项目 | `gastown`、`beads`、`hop` 等 |
| **type** | 工作类型 | `feature`、`bug`、`design`、`rfc`、`docs` |
| **priority** | 紧急程度 | 0=关键, 1=高, 2=中, 3=低, 4=积压 |
| **effort** | 预估规模 | `trivial`、`small`、`medium`、`large`、`epic` |
| **posted_by** | 创建该项目的 Rig | Rig 句柄 |
| **status** | 生命周期状态 | `open`、`claimed`、`in_review`、`completed`、`withdrawn` |

### Rig

在 Wasteland 语境下，**Rig** 是你的参与者身份 — 不同于 Gas Town 中作为项目容器的 Rig。当你加入时，你的 DoltHub 组织名成为你的 Rig 句柄。每次领取、完成和 Stamp 都归属于你的 Rig。

### Stamp 和声誉

当验证者审查你完成的工作时，他们会签发一个 **Stamp** — 一个涵盖质量、可靠性和创造力的多维认证。Stamp 累积成可移植的声誉，随你的 Rig 在各个 Wasteland 之间转移。

**毕业册规则**适用：你不能给自己的工作盖章。声誉是他人对你的评价。

### 信任等级（计划中）

数据库按 Rig 跟踪信任等级，但**第 1 阶段不强制执行** — 所有已注册的 Rig 都可以浏览、领取、发布和提交。计划的等级递进：

| 等级 | 名称 | 计划中的能力 |
|------|------|-------------|
| 0 | 已注册 | 浏览、发布 |
| 1 | 参与者 | 领取、提交完成 |
| 2 | 贡献者 | 经过验证的工作记录 |
| 3 | 维护者 | 验证并为他人工作盖章 |

新 Rig 从等级 1（参与者）开始。一旦启用强制执行，随着你积累经过验证的完成和 Stamp，信任等级将会提升。

## 浏览需求板

```bash
cd ~/gt
gt wl browse                          # 所有开放项目
gt wl browse --project gastown        # 按项目筛选
gt wl browse --type bug               # 仅缺陷
gt wl browse --type docs              # 仅文档工作
gt wl browse --status claimed         # 查看已被领取的项目
gt wl browse --priority 0             # 仅关键优先级
gt wl browse --limit 10              # 限制结果数量
gt wl browse --json                   # JSON 输出（用于脚本）
```

浏览总是查询最新的上游状态，因此无论你的本地 fork 状态如何，你都能看到当前可用的内容。

## 领取工作

找到想做的任务？领取它：

```bash
cd ~/gt
gt wl claim w-abc123
```

这会将 `claimed_by` 设置为你的 Rig 句柄，并将本地数据库中的状态从 `open` 改为 `claimed`。

### 领取的传播方式（第 1 阶段）

在第 1 阶段，领取仅写入你的**本地** `wl_commons` 数据库。其他 Rig 在公共数据库更新之前看不到你的领取（例如，通过你 fork 的 DoltHub PR）。这意味着两个 Rig 可以独立领取同一个项目 — 领取是意图信号，而非分布式锁。

数据库对每个需求项强制执行一次完成（`NOT EXISTS` 守卫），但此约束是每个数据库的。在第 1 阶段，两个在本地都领取了的 Rig 都可以在本地完成。冲突在 fork 合并到上游时浮现 — 实际工作（你的 GitHub PR）决定优先权。

未来阶段将通过 DoltHub PR 引入自动领取传播。

### 选择领取什么

选择第一个任务的建议：

- 从 `docs` 或 `small` 规模的项目开始以建立熟悉度
- 优先检查 `--priority 0` 和 `--priority 1` — 这些是项目最需要的
- 如果你了解特定代码库，按 `--project` 筛选
- 使用 `--json` 将结果管道到脚本或其他工具

## 执行工作

领取后，执行实际工作。这发生在 Wasteland 命令之外 — 使用你正常的开发工作流：

1. **Fork 相关仓库**（如果是贡献代码）
2. **创建功能分支**，遵循目标项目的贡献指南（Gas Town 使用 `docs/*`、`fix/*`、`feat/*`、`refactor/*` — 参见 [CONTRIBUTING.md](../CONTRIBUTING.md)）
3. **进行修改**
4. **向上游仓库发起 Pull Request**

对于文档工作，PR 发送到托管文档的仓库。对于代码工作，PR 发送到需求项中指定的项目。

## 提交完成

工作完成后，有了凭证（PR URL、提交哈希或描述），提交它：

```bash
cd ~/gt
gt wl done w-abc123 --evidence "https://github.com/steveyegge/gastown/pull/99"
```

项目必须处于 `claimed` 状态且由**你**的 Rig 领取。如果你跳过了 `gt wl claim`，此命令将失败。

此操作会：
1. 创建一个带有唯一 `c-<hash>` ID 的**完成记录**
2. 将需求项状态更新为 `in_review`
3. 将你的凭证链接到完成记录

`--evidence` 标志是必需的。提供你能给出的最具体的引用 — PR URL 最理想，因为审查者可以直接查看工作内容。

### 提交后会发生什么

你的完成进入 `in_review` 状态。维护者可以验证工作并签发 Stamp。Stamp 记录了他们在质量、可靠性和创造力维度的评估。

## 发布新工作

发现需要做的事情？发布到需求板：

```bash
cd ~/gt
gt wl post \
  --title "Add retry logic to federation sync" \
  --project gastown \
  --type feature \
  --priority 2 \
  --effort medium \
  --tags "go,federation" \
  --description "Federation sync fails silently on transient network errors.
Add exponential backoff with 3 retries."
```

必需标志：`--title`。其他都有合理默认值（`priority` 默认为 2，`effort` 默认为 `medium`）。使用 `-d` 作为 `--description` 的简写。

## 与上游同步

拉取上游公共数据库的最新变更：

```bash
cd ~/gt
gt wl sync                # 拉取上游变更
gt wl sync --dry-run      # 预览变更但不拉取
```

在其他 Rig 发布了新项目、领取了工作或提交了完成之后，同步很有用。定期运行以保持本地状态最新。

同步后，命令会打印公共数据库的状态摘要：

```
✓ Synced with upstream

  Open wanted:       12
  Total wanted:      47
  Total completions: 23
  Total stamps:      18
```

## 完整工作流示例

以下是首次贡献的端到端流程：

```bash
# 1. 设置环境（一次性）
export DOLTHUB_ORG="your-username"
export DOLTHUB_TOKEN="dhat.v1.your-token"

# 2. 加入 Wasteland（一次性，从 Gas Town 工作空间执行）
cd ~/gt
gt wl join hop/wl-commons

# 3. 浏览工作
gt wl browse --type docs

# 4. 领取一个项目
gt wl claim w-abc123

# 5. 执行工作（在相关仓库中）
cd ~/path/to/relevant/repo
git checkout -b docs/my-contribution
# ... 进行修改 ...
git add . && git commit -m "Add my contribution"
git push -u origin HEAD

# 6. 在 GitHub 上发起 PR
gh pr create --title "docs: My contribution"

# 7. 提交完成凭证（回到 Gas Town 工作空间）
cd ~/gt
gt wl done w-abc123 --evidence "https://github.com/org/repo/pull/123"

# 8. 同步查看更新后的状态
gt wl sync
```

## 故障排除

### `gt wl join` 因 DoltHub API 错误失败

Fork API 需要有效的 `DOLTHUB_TOKEN`。验证你的令牌：

```bash
echo $DOLTHUB_TOKEN   # 应以 "dhat.v1." 开头
echo $DOLTHUB_ORG     # 应为你的 DoltHub 用户名
```

如果令牌正确但 fork 失败，你可以手动绕过：

```bash
# 直接克隆上游
dolt clone hop/wl-commons /tmp/wl-setup/wl-commons
cd /tmp/wl-setup/wl-commons

# 注册你的 Rig（trust_level=1 与 gt wl join 设置的相同）
dolt sql -q "INSERT INTO rigs (handle, display_name, dolthub_org, \
  trust_level, registered_at, last_seen) \
  VALUES ('$DOLTHUB_ORG', 'Your Name', '$DOLTHUB_ORG', 1, NOW(), NOW());"
dolt add -A && dolt commit -m "Register rig: $DOLTHUB_ORG"

# 作为 fork 推送到你的 DoltHub 组织
dolt remote add myfork https://doltremoteapi.dolthub.com/$DOLTHUB_ORG/wl-commons
dolt push myfork main

# 将克隆放到 gt wl join 会放置的位置
mkdir -p ~/gt/.wasteland/hop
cp -r /tmp/wl-setup/wl-commons ~/gt/.wasteland/hop/wl-commons
cd ~/gt/.wasteland/hop/wl-commons

# 修复远程：origin 必须指向你的 fork（gt wl join 克隆的是 fork，
# 所以 origin 默认 = fork；我们的克隆 origin = 上游）
dolt remote remove origin
dolt remote add origin https://doltremoteapi.dolthub.com/$DOLTHUB_ORG/wl-commons
dolt remote add upstream https://doltremoteapi.dolthub.com/hop/wl-commons

# 清理
rm -rf /tmp/wl-setup
```

手动设置完成后，在 `~/gt/mayor/wasteland.json` 创建配置文件：

```json
{
  "upstream": "hop/wl-commons",
  "fork_org": "your-dolthub-org",
  "fork_db": "wl-commons",
  "local_dir": "/path/to/your/gt/.wasteland/hop/wl-commons",
  "rig_handle": "your-dolthub-org",
  "joined_at": "2026-01-01T00:00:00Z"
}
```

### `gt wl browse` 显示 "No wanted items found"

上游公共数据库可能为空，或你的筛选条件过窄。尝试不同组合：

```bash
gt wl browse                          # 默认：仅开放项目
gt wl browse --status claimed         # 尝试不同状态
gt wl browse --limit 50              # 增加限制
```

### `gt wl claim` 提示 "not in a Gas Town workspace"

所有 `gt wl` 命令必须从 Gas Town 工作空间内运行（通常是 `~/gt`）：

```bash
cd ~/gt
gt wl claim w-abc123
```

### `gt wl sync` 拉取失败

确保本地 fork 中存在上游远程。从 `~/gt/mayor/wasteland.json` 中的 `local_dir` 找到克隆路径，然后检查：

```bash
cd /path/from/local_dir            # 例如 ~/gt/.wasteland/hop/wl-commons
dolt remote -v                     # 应显示 'upstream' 远程
```

如果没有配置上游远程：

```bash
dolt remote add upstream https://doltremoteapi.dolthub.com/hop/wl-commons
```

## 数据库 Schema 参考

Wasteland 公共数据库（`wl_commons`）有七张表。完整的 Schema 定义在 `internal/doltserver/wl_commons.go` 中。

| 表 | 用途 |
|----|------|
| **_meta** | Schema 版本和 Wasteland 名称 |
| **rigs** | Rig 注册表 — 句柄、显示名称、DoltHub 组织、信任等级、类型 |
| **wanted** | 工作项 — 标题、项目、类型、优先级、状态、claimed_by、effort、标签、沙箱字段 |
| **completions** | 已提交的工作 — 关联需求 ID 到 Rig、凭证 URL 和验证者 |
| **stamps** | 声誉认证 — 作者、主体、valence（JSON）、置信度、严重度 |
| **badges** | 成就标记 — Rig 句柄、徽章类型、凭证 |
| **chain_meta** | 联邦元数据 — 链 ID、类型、父链、HOP URI |

`stamps` 表在数据库层面强制执行毕业册规则：`CHECK (NOT(author = subject))`。

## 下一步

完成第一个任务后：

- **发布你发现的工作**：`gt wl post --title "..." --type feature`
- **定期同步**：`gt wl sync` 保持最新
- **建设声誉**：持续、高质量的完成会获得 Stamp
- **探索联邦**：可以存在多个 Wasteland — 你的身份可在所有 Wasteland 之间移植

关于 Wasteland 的完整设计理念，参见 Steve Yegge 的 [Welcome to the Wasteland](https://steve-yegge.medium.com/welcome-to-the-wasteland-a-thousand-gas-towns-a5eb9bc8dc1f)。

关于本文档中引用的 Gas Town 概念，参见 [overview.md](overview.md) 和 [glossary.md](glossary.md)。