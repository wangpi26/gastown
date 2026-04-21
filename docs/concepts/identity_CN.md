# Agent 身份与归属

> Gas Town 中 Agent 身份的规范格式

## 为什么身份很重要

当你在规模上部署 AI Agent 时，匿名工作会带来实际问题：

- **调试：**"AI 弄坏了"是不可操作的。是*哪个* AI？
- **质量追踪：**你无法改进你无法衡量的东西。
- **合规：**审计员会问"谁批准了这段代码？"——你需要一个答案。
- **绩效管理：**某些 Agent 在特定任务上比其他 Agent 表现更好。

Gas Town 通过**通用归属**解决这个问题：每一个动作、每一个提交、每一个 Bead 更新都关联到特定的 Agent 身份。这使工作历史追踪、基于能力的路由和客观的质量衡量成为可能。

## BD_ACTOR 格式约定

`BD_ACTOR` 环境变量使用斜杠分隔的路径格式标识 Agent。它在 Agent 被生成时自动设置，用于所有归属记录。

### 按角色类型的格式

| 角色类型 | 格式 | 示例 |
|---------|------|------|
| **Mayor** | `mayor` | `mayor` |
| **Deacon** | `deacon` | `deacon` |
| **Witness** | `{rig}/witness` | `gastown/witness` |
| **Refinery** | `{rig}/refinery` | `gastown/refinery` |
| **Crew** | `{rig}/crew/{name}` | `gastown/crew/joe` |
| **Polecat** | `{rig}/polecats/{name}` | `gastown/polecats/toast` |

### 为什么用斜杠？

斜杠格式镜像文件系统路径，支持：
- 层次化解析（提取 rig、角色、名称）
- 一致的邮件寻址（`gt mail send gastown/witness`）
- Bead 操作中类似路径的路由
- 对 Agent 位置的直观可见性

## 归属模型

Gas Town 使用三个字段实现完整的溯源：

### Git 提交

```bash
GIT_AUTHOR_NAME="gastown/crew/joe"      # 谁做了工作（Agent）
GIT_AUTHOR_EMAIL="steve@example.com"    # 谁拥有工作（Overseer）
```

在 git log 中的结果：
```
abc123 Fix bug (gastown/crew/joe <steve@example.com>)
```

**解读**：
- Agent `gastown/crew/joe` 创作了变更
- 工作属于工作区所有者（`steve@example.com`）
- 两者都永久保留在 git 历史中

### Beads 记录

```json
{
  "id": "gt-xyz",
  "created_by": "gastown/crew/joe",
  "updated_by": "gastown/witness"
}
```

`created_by` 字段在创建 Bead 时从 `BD_ACTOR` 填充。
`updated_by` 字段追踪谁最后修改了记录。

### 事件日志

所有事件都包含参与者归属：

```json
{
  "ts": "2025-01-15T10:30:00Z",
  "type": "sling",
  "actor": "gastown/crew/joe",
  "payload": { "bead": "gt-xyz", "target": "gastown/polecats/toast" }
}
```

## 环境设置

Gas Town 使用集中的 `config.AgentEnv()` 函数在所有 Agent 生成路径（managers、daemon、boot）中一致地设置环境变量。

### 示例：Polecat 环境

```bash
# 为 Rig 'gastown' 中的 Polecat 'toast' 自动设置
export GT_ROLE="polecat"
export GT_RIG="gastown"
export GT_POLECAT="toast"
export BD_ACTOR="gastown/polecats/toast"
export GIT_AUTHOR_NAME="gastown/polecats/toast"
export GT_ROOT="/home/user/gt"
export BEADS_DIR="/home/user/gt/gastown/.beads"
export BEADS_AGENT_NAME="gastown/toast"
```

### 示例：Crew 环境

```bash
# 为 Rig 'gastown' 中的 Crew 成员 'joe' 自动设置
export GT_ROLE="crew"
export GT_RIG="gastown"
export GT_CREW="joe"
export BD_ACTOR="gastown/crew/joe"
export GIT_AUTHOR_NAME="gastown/crew/joe"
export GT_ROOT="/home/user/gt"
export BEADS_DIR="/home/user/gt/gastown/.beads"
export BEADS_AGENT_NAME="gastown/joe"
```

### 手动覆盖

用于本地测试或调试：

```bash
export BD_ACTOR="gastown/crew/debug"
bd create --title="Test issue"  # 将显示 created_by: gastown/crew/debug
```

参见 [reference.md](reference.md#environment-variables) 获取完整的环境变量参考。

## 身份解析

该格式支持程序化解析：

```go
// identityToBDActor 将 daemon 身份转换为 BD_ACTOR 格式
// Town 级别: mayor, deacon
// Rig 级别: {rig}/witness, {rig}/refinery
// 工作者: {rig}/crew/{name}, {rig}/polecats/{name}
```

| 输入 | 解析结果 |
|------|---------|
| `mayor` | role=mayor |
| `deacon` | role=deacon |
| `gastown/witness` | rig=gastown, role=witness |
| `gastown/refinery` | rig=gastown, role=refinery |
| `gastown/crew/joe` | rig=gastown, role=crew, name=joe |
| `gastown/polecats/toast` | rig=gastown, role=polecat, name=toast |

## 审计查询

归属机制支持强大的审计查询：

```bash
# 某个 Agent 的所有工作
bd audit --actor=gastown/crew/joe

# 某 Rig 中的所有工作
bd audit --actor=gastown/*

# 所有 Polecat 的工作
bd audit --actor=*/polecats/*

# 按 Agent 查看 Git 历史
git log --author="gastown/crew/joe"
```

## 设计原则

1. **Agent 不是匿名的** — 每个动作都有归属
2. **工作是被拥有的，而非被创作的** — Agent 创作，Overseer 拥有
3. **归属是永久的** — Git 提交保留历史
4. **格式可解析** — 支持程序化分析
5. **跨系统一致** — 在 git、beads、events 中使用相同格式

## CV 与技能积累

### 人类身份是全局的

全局标识符是你的**邮箱** — 它已经在每个 git 提交中了。不需要单独的"实体 Bead"。

```
steve@example.com                ← 全局身份（来自 git author）
├── Town A (home)                ← 工作区
│   ├── gastown/crew/joe         ← Agent 执行者
│   └── gastown/polecats/toast   ← Agent 执行者
└── Town B (work)                ← 工作区
    └── acme/polecats/nux        ← Agent 执行者
```

### Agent 与所有者

| 字段 | 作用域 | 用途 |
|------|-------|------|
| `BD_ACTOR` | 本地（town） | Agent 归属，用于调试 |
| `GIT_AUTHOR_EMAIL` | 全局 | 人类身份，用于 CV |
| `created_by` | 本地 | 谁创建了该 Bead |
| `owner` | 全局 | 谁拥有该工作 |

**Agent 执行。人类拥有。** Polecat 名称 `completed-by: gastown/polecats/toast` 是执行者归属。CV 归属于人类所有者（`steve@example.com`）。

### Polecat 拥有持久身份

Polecat 拥有**持久身份但临时会话**。就像打卡上下班的员工：每个工作会话都是全新的（新的 tmux、新的 worktree），但身份在会话之间持续存在。

- **身份（持久）**：Agent Bead、CV 链、工作历史
- **会话（临时）**：Claude 实例、上下文窗口
- **沙盒（临时）**：Git worktree、分支

工作归功于 Polecat 身份，支持：
- 按 Polecat 的绩效追踪
- 基于能力的路由（将 Go 工作发送给有 Go 履历的 Polecat）
- 模型对比（通过不同 Polecat 进行 A/B 测试不同模型）

详见 [polecat-lifecycle.md](polecat-lifecycle.md#polecat-identity)。

### 技能是派生的

你的 CV 从查询工作证据中涌现：

```bash
# 按所有者查询所有工作（跨所有 Agent）
git log --author="steve@example.com"
bd list --owner=steve@example.com

# 从证据中派生技能
# - 触碰过的 .go 文件 → Go 技能
# - issue 标签 → 领域技能
# - 提交模式 → 活动类型
```

### 跨 Town 聚合

拥有多个 Town 的人类有一个 CV：

```bash
# 未来：联邦 CV 查询
bd cv steve@example.com
# 发现所有 Town，聚合工作，派生技能
```

参见 `~/gt/docs/hop/decisions/008-identity-model.md` 了解架构决策。

## 企业用例

### 合规与审计

```bash
# 过去 90 天谁触碰过这个文件？
git log --since="90 days ago" -- path/to/sensitive/file.go

# 特定 Agent 的所有变更
bd audit --actor=gastown/polecats/toast --since=2025-01-01
```

### 绩效追踪

```bash
# 按 Agent 的完成率
bd stats --group-by=actor

# 平均完成时间
bd stats --actor=gastown/polecats/* --metric=cycle-time
```

### 模型对比

当 Agent 使用不同的底层模型时，归属机制支持 A/B 对比：

```bash
# 按模型标记 Agent
# gastown/polecats/claude-1 使用 Claude
# gastown/polecats/gpt-1 使用 GPT-4

# 对比质量信号
bd stats --actor=gastown/polecats/claude-* --metric=revision-count
bd stats --actor=gastown/polecats/gpt-* --metric=revision-count
```

较低的修订次数表明更高的首次通过质量。