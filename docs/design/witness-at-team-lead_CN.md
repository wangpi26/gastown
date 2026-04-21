# Witness AT 团队负责人：实现规格

> **状态：未来架构 — 尚未实现**
> 当前系统使用基于 tmux 的会话管理。本文档描述了计划中的架构变更，使用 Claude Code Agent Teams (AT) 作为传输层。尚无相关代码。

> **Bead：** gt-ky4jf
> **日期：** 2026-02-08
> **作者：** furiosa（gastown polecat）
> **依赖：** AT 试探报告（gt-3nqoz）、AT 集成设计（agent-teams-integration.md）
> **状态：** 阶段 1 实现规格

---

## 概述

本文档规定了 Witness 如何成为 AT 团队负责人，用 Claude Code Agent Teams 替换当前基于 tmux 的 polecat 会话管理。

Witness 进入委托模式（结构化强制执行的仅协调模式），为分配的工作生成 polecat 队友，通过 AT 的原生生命周期 hook 监控它们，并在任务边界将完成状态同步到 beads。

**变更内容：** 会话管理层（tmux → AT）。
**保持不变：** Beads 作为账本、gt mail 用于跨 rig、molecules/formulas、`gt done`。

---

## AT 试探发现摘要

> 摘自 AT 试探报告（gt-3nqoz，2026-02-08，作者：nux）。

**建议：有条件 GO 进行阶段 1 实验。**

### Go/No-Go 决策矩阵

| 标准 | 状态 | 备注 |
|-----------|--------|-------|
| 队友工作目录 | 变通方案 | PreToolUse hook 用于强制执行 |
| Hook 为队友触发 | GO | 所有相关 hook 已确认 |
| 自定义 agent 定义 | GO | `.claude/agents/*.md` 可用 |
| 委托模式强制执行 | GO | 结构化，非行为化 |
| 队友轮换 | 变通方案 | Handoff + 重生模式 |
| Token 成本可接受 | 有条件 | Sonnet 队友降低成本 |
| gt/bd 命令访问 | GO | PATH 通过 SessionStart hook |
| 带依赖的任务列表 | GO | 原生匹配 Gas Town 工作流 |

8 项中 5 项明确 GO。2 项需要变通方案（可行的缓解措施）。1 项取决于阶段 1 成本验证。

### 关键阻碍

1. **无每队友工作目录** — AT 队友继承负责人的 cwd。变通方案：在生成提示中 `cd` + PreToolUse hook（`gt validate-worktree-scope`）用于结构化强制执行。
2. **无队友会话恢复** — 崩溃的队友无法恢复。变通方案：PreCompact handoff + beads 状态恢复 + Witness 重生。
3. **Token 成本约为每队友 7 倍** — 通过对 polecat 队友使用 Sonnet、仅 Witness 负责人使用 Opus 来缓解。

### 风险登记摘要

| 风险级别 | 关键风险 |
|------------|-----------|
| **高** | 无每队友 cwd、无会话恢复、实验性功能 |
| **中** | 7 倍 token 成本、hook 兼容性差距、AT API 变更 |
| **低** | PATH/环境设置、任务列表映射、委托模式差距 |

### 关键优势

AT 的文件锁定任务认领消除了 Dolt 写入争用（估计减少 80-90%）。这是采纳的最强论据。

---

## 1. 委托模式中的 Witness

### Witness 保留的工具

在委托模式中，Witness 可以访问：

| 工具 | 用途 |
|------|---------|
| `Teammate` | 生成/关闭队友、发送消息、管理团队 |
| `TaskCreate` | 为 polecat 工作创建 AT 任务 |
| `TaskUpdate` | 更新任务状态、设置依赖 |
| `TaskList` | 监控团队进度 |
| `TaskGet` | 读取任务详情 |
| `Bash` | 委托模式中**不可用** |
| `Read/Write/Edit` | 委托模式中**不可用** |
| `Glob/Grep` | 委托模式中**不可用** |

### ZFC 升级

当前状态："Witness 不实现"由 CLAUDE.md 指令强制执行。Agent 在压力下可以且确实会违反这一点。

新状态：委托模式在结构上移除了实现工具。Witness *无法*编辑文件。这是最强可能的 ZFC 合规性 — 约束在机制中，不在指令中。

### Witness 需要 Bash 来运行 gt/bd 命令

**问题：** 委托模式移除了 Bash 访问，但 Witness 需要运行 `gt mail`、`bd show`、`bd close` 和其他协调命令。

**解决方案选项（按优先级排序）：**

1. **带选择性工具的自定义 agent 定义。** 创建 `.claude/agents/witness-lead.md`，使用 `permissionMode: delegate` 但通过 `tools` 白名单添加回 Bash。这为文件编辑提供结构化强制执行，同时保留命令访问：

   ```yaml
   ---
   name: witness-lead
   permissionMode: delegate
   tools: Teammate, TaskCreate, TaskUpdate, TaskList, TaskGet, Bash
   ---
   ```

   **风险：** Bash 访问意味着 Witness *可能*通过 sed/echo 编辑文件。缓解：对 Bash 使用 PreToolUse hook，拒绝文件修改命令。

2. **Hook 作为命令代理。** Witness 不直接运行命令。相反，hook 在回合边界触发，根据 AT 任务状态执行 gt/bd 命令。Witness 纯粹通过 AT 工具协调；hook 处理 beads 桥接。

   **风险：** 灵活性较低 — Witness 无法进行临时 bd 查询。但这是最纯粹的委托模式实现。

3. **队友作为命令运行器。** 生成一个轻量级"ops"队友，其唯一工作是为 Witness 运行 gt/bd 命令。Witness 通过 AT 消息发送命令；ops 队友执行并返回结果。

   **风险：** 简单命令代理的 token 开销。但它为 Witness 保留了严格的委托模式。

**建议：** 选项 1（带选择性工具的自定义 agent）。它务实，保留了 Witness 查询 beads 状态的能力，PreToolUse hook 提供了足够的防护。纯委托模式是理想但 Witness 确实需要读取 beads 状态来做协调决策。

### Witness Bash 的 PreToolUse 守卫

```json
{
  "PreToolUse": [{
    "matcher": "Bash",
    "hooks": [{
      "type": "command",
      "command": "gt witness-bash-guard"
    }]
  }]
}
```

`gt witness-bash-guard` 脚本：
- 允许：`gt *`、`bd *`、`git status`、`git log`、只读命令
- 阻止：`echo >`、`cat >`、`sed -i`、`vim`、`nano`、任何写入操作
- 阻止时返回退出码 2 并附带原因

---

## 2. 队友生成：工作分配 → AT 任务创建

### 生成流程

当工作到达（通过 convoy、gt sling 或直接分配）：

```
1. Witness 接收工作（mail、convoy 分派、bd ready）
2. Witness 创建 AT 团队（如果尚未活跃）
3. 对每个要分派的 issue：
   a. 创建带 issue 详情和依赖的 AT 任务
   b. 生成分配到该任务的 polecat 队友
4. 队友自行认领任务并开始执行
```

### 团队创建

```
Teammate({
  operation: "spawnTeam",
  team_name: "<rig-name>-work",
  description: "Polecat work team for <convoy/sprint description>"
})
```

团队命名约定：`<rig>-work` 用于主要工作团队。
每个 rig 每个活跃 convoy 一个团队。多个 convoy = 多个团队（AT 限制：每个会话一个团队，所以 Witness 每次管理一个 convoy）。

### 从 Beads Issue 创建 AT 任务

对每个分派到 polecat 的 issue：

```
TaskCreate({
  subject: "<issue title>",
  description: "Issue: <issue-id>\n<issue description>\n\nWorktree: /path/to/<polecat>/\nFormula: mol-polecat-work",
  activeForm: "Working on <issue title>",
  metadata: {
    "bead_id": "<issue-id>",
    "worktree": "/path/to/worktree",
    "molecule": "<mol-id>"
  }
})
```

**元数据中的关键字段：**
- `bead_id`：将 AT 任务链接回 beads issue 以进行同步
- `worktree`：此 polecat 应使用的 git worktree 路径
- `molecule`：此 issue 的 mol-polecat-work 实例

### 依赖映射

Beads issue 依赖映射到 AT 任务依赖：

```
# 如果 issue B 依赖 issue A：
# 创建两个任务后：
TaskUpdate({
  taskId: "<task-B-id>",
  addBlockedBy: ["<task-A-id>"]
})
```

这启用了 AT 的原生自行认领：当任务 A 完成时，任务 B 被解除阻塞，下一个空闲队友自动认领它。

### Polecat 队友生成

```
Task({
  subagent_type: "polecat",
  team_name: "<rig>-work",
  name: "<polecat-name>",
  model: "sonnet",
  prompt: "You are polecat <name>. Your worktree is <path>.\n\nAssigned issue: <id> - <title>\n<description>\n\nWorkflow:\n1. cd <worktree>\n2. Run `gt prime` for full context\n3. Follow mol-polecat-work steps\n4. When done: commit, push, run `gt done`"
})
```

**模型选择：**
- Polecat 队友：`model: "sonnet"`（执行导向，成本高效）
- Witness 负责人：Opus（判断、协调、质量审查）
- Refinery 队友（阶段 2）：`model: "sonnet"`（机械合并工作）

### `.claude/agents/polecat.md` 定义

```yaml
---
name: polecat
description: Gas Town polecat worker agent (persistent identity, ephemeral sessions)
model: sonnet
hooks:
  SessionStart:
    - hooks:
        - type: command
          command: "export PATH=\"$HOME/go/bin:$HOME/.local/bin:$PATH\" && gt prime --hook"
  PreToolUse:
    - matcher: "Write|Edit"
      hooks:
        - type: command
          command: "gt validate-worktree-scope"
  PreCompact:
    - matcher: "auto"
      hooks:
        - type: command
          command: "gt handoff --reason compaction"
  Stop:
    - hooks:
        - type: command
          command: "gt signal stop"
---

You are a Gas Town polecat (persistent identity, ephemeral sessions).

## Startup
1. `cd` to your assigned worktree (given in your spawn prompt)
2. Run `gt prime` for full context
3. Check your hook: `gt hook`
4. Follow molecule steps: `bd mol current`

## Work Protocol
- Mark steps in_progress before starting: `bd update <id> --status=in_progress`
- Close steps when done: `bd close <id>`
- Commit frequently with descriptive messages
- Never batch-close steps

## Completion
When all steps done:
1. `git status` — must be clean
2. `git push`
3. `gt done` — submits to merge queue, nukes your sandbox
```

### Worktree 分配

每个 polecat 队友在自己的 git worktree 中操作。由于 AT 不原生支持每队友工作目录，强制执行通过：

1. **生成提示：** 第一条指令是 `cd /path/to/worktree`
2. **PreToolUse hook：** `gt validate-worktree-scope` 拒绝目标路径在分配的 worktree 之外的 Write/Edit 操作
3. **环境变量：** `GT_WORKTREE=/path/to/worktree` 通过 SessionStart hook 设置

Witness 在生成队友之前创建 worktree：
```bash
git worktree add /path/to/polecats/<name>/<rig> -b polecat/<name>/<issue-id>
```

这与当前的 worktree 管理一致 — 变更的是谁创建它们（Witness 通过 AT，而非 `gt sling` 通过 Go daemon）。

---

## 3. Bead 同步协议

### 两层模型

```
层 1（AT，临时）：     任务认领、状态、消息
层 2（Beads/Dolt，持久）：Issue 创建、完成、审计轨迹
```

### 同步点

| AT 事件 | Beads 动作 | 触发 |
|----------|-------------|---------|
| 任务认领（in_progress） | `bd update <id> --status=in_progress` | TaskCompleted hook / polecat 提示 |
| 任务完成 | `bd close <step-id>` | TaskCompleted hook |
| 发现新 issue | Witness 创建 AT 任务 | Witness 读取 polecat 消息 |
| 队友空闲 | 检查 beads 中的更多工作 | TeammateIdle hook |
| 团队关闭 | 验证所有 beads 已同步 | Witness 清理例程 |

### 用于 Bead 同步的 TaskCompleted Hook

`TaskCompleted` hook 在 AT 任务标记为完成时触发。这是主要同步机制：

```bash
#!/bin/bash
# .claude/hooks/task-completed-sync.sh
# 在 TaskCompleted hook 时触发

BEAD_ID=$(echo "$TASK_METADATA" | jq -r '.bead_id // empty')
if [ -n "$BEAD_ID" ]; then
  export PATH="$HOME/go/bin:$HOME/.local/bin:$PATH"
  bd close "$BEAD_ID" 2>/dev/null
fi
exit 0
```

Hook 配置：
```json
{
  "TaskCompleted": [{
    "hooks": [{
      "type": "command",
      "command": ".claude/hooks/task-completed-sync.sh"
    }]
  }]
}
```

**重要：** Hook 不应阻止任务完成（始终 exit 0）。如果 `bd close` 失败（Dolt 争用），它将在下一个同步点重试。AT 任务列表是实时真相；beads 在边界处追上。

### Polecat 端 Bead 更新

Polecat 仍作为其 molecule 工作流的一部分直接运行 `bd update` 和 `bd close`。TaskCompleted hook 是安全网，而非主要机制。这意味着：

- Polecat 标记 molecule 步骤 in_progress → `bd update --status=in_progress`
- Polecat 完成 molecule 步骤 → `bd close <step-id>`
- AT 任务完成 → TaskCompleted hook 也触发 `bd close`（幂等的）

双重关闭是安全的：对已关闭 bead 的 `bd close` 是无操作。

### 团队关闭时的同步验证

Witness 关闭团队之前，验证 beads 已同步：

```
对每个标记为完成的 AT 任务：
  1. 读取任务元数据获取 bead_id
  2. 验证 bead 已关闭（bd show <id> | 检查状态）
  3. 如果 bead 仍开放：bd close <id> 并附带备注
  4. 如果关闭失败：记录警告，继续（Dolt 重试会处理）
```

这是集成设计中的"边界同步"模式：AT 处理实时协调，beads 在生命周期边界（团队关闭、convoy 完成）追上。

---

## 4. 会话轮换：压缩 → 重生 → 恢复

### 问题

AT 队友在关闭后无法恢复。当队友达到上下文限制并压缩，或崩溃时，必须生成新的队友。

### 生命周期

```
队友运行中
    │
    ├── 上下文填满 → PreCompact hook 触发
    │   │
    │   └── gt handoff --reason compaction
    │       ├── 将当前 molecule 步骤保存到 beads
    │       ├── 保存进度备注
    │       └── 保存 git 分支状态
    │
    ├── 自动压缩发生
    │   │
    │   └── SessionStart hook 触发（来源："compact"）
    │       └── gt prime --compact-resume
    │           └── 读取 beads 状态，恢复上下文
    │
    └── 队友以压缩上下文继续
```

### 当压缩不够时（队友死亡）

如果队友崩溃或被关闭（不只是压缩）：

```
队友停止
    │
    └── SubagentStop hook 在 Witness（负责人）上触发
        │
        ├── 从 beads 读取队友的最后已知状态
        │   └── 哪个 molecule 步骤是 in_progress？
        │   └── 什么分支正在工作？
        │
        ├── 评估：可恢复还是升级？
        │   ├── 正常完成：AT 任务完成，beads 已同步 → 无操作
        │   ├── 未完成工作：带恢复上下文重生
        │   └── 反复崩溃：升级到 Witness mail → Mayor
        │
        └── 如果可恢复：生成替换队友
            └── Task({ subagent_type: "polecat", ... 恢复提示 ... })
```

### SubagentStop Hook（Witness 端）

```json
{
  "SubagentStop": [{
    "matcher": "polecat",
    "hooks": [{
      "type": "command",
      "command": "gt witness-teammate-stopped"
    }]
  }]
}
```

`gt witness-teammate-stopped` 脚本：
1. 读取停止 agent 的转录路径（在 hook 输入中可用）
2. 检查 AT 任务状态 — 任务是否已完成？
3. 检查 beads — `gt done` 是否已运行？
4. 如果已完成：无操作（正常生命周期）
5. 如果未完成：输出 `{ "decision": "block", "reason": "Teammate <name> stopped before completing task <id>. Beads state: <status>. Respawn needed." }`

"block" 决定阻止 Witness 进入空闲，将重生指令注入为 Witness 行动的上下文。

### 重生提示模板

```
Teammate <name> stopped before completing work.

Last known state:
- Issue: <bead-id> (<title>)
- Molecule step: <step-id> (in_progress)
- Branch: <branch-name>
- Worktree: <path>

Spawn a replacement polecat with this context. The new teammate
should read beads state and continue from the last checkpoint.
```

### 崩溃循环预防

跟踪每个 issue 的重生尝试。如果队友在同一 issue 上崩溃 3 次：

1. 将 AT 任务标记为阻塞
2. 提交 bead：`bd create --title "Polecat crash loop on <issue>" --type bug`
3. 向 Witness/Mayor 发送 mail 以升级
4. 不再重生 — issue 存在结构性问题

跟踪：使用 AT 任务元数据 `{ "respawn_count": N }`，在每次重生时递增。这是临时的（随团队消亡），这是正确的 — 崩溃跟踪仅在当前团队会话中有意义。

---

## 5. 错误处理

### 错误类别和响应

| 错误 | 检测 | 响应 |
|-------|-----------|----------|
| 队友崩溃 | SubagentStop hook | 重生或升级（见上文） |
| 队友卡住（无进展） | TeammateIdle hook | 发送消息询问状态 |
| 测试失败 | TaskCompleted hook（exit 2） | 阻止完成，队友必须修复 |
| 合并冲突 | Polecat 消息通知 Witness | Witness 建议或重新分配 |
| Dolt 写入失败 | bd 命令退出码 | 带退避重试（现有机制） |
| AT 团队崩溃 | Witness 会话死亡 | Daemon/Boot/Deacon 链检测，重启 Witness |
| Worktree 范围违规 | PreToolUse hook | 阻止操作，警告 polecat |

### TeammateIdle Hook

```bash
#!/bin/bash
# gt witness-teammate-idle
# 当队友即将空闲时触发

export PATH="$HOME/go/bin:$HOME/.local/bin:$PATH"

# 检查 beads 中是否有更多工作
READY=$(bd ready --count 2>/dev/null)
if [ "$READY" -gt 0 ]; then
  echo "There is more work available. Run 'bd ready' to see unblocked tasks." >&2
  exit 2  # 阻止空闲，发送反馈
fi

# 检查是否运行了 gt done
if git log --oneline -1 | grep -q "gt done"; then
  exit 0  # 正常完成
fi

# 队友似乎真正空闲但未完成
echo "Your work doesn't appear complete. Run 'bd ready' to check remaining steps, or 'gt done' if finished." >&2
exit 2
```

### TaskCompleted 质量门控

```bash
#!/bin/bash
# 在 TaskCompleted hook 时触发
# 在标记完成前验证工作满足最低质量

export PATH="$HOME/go/bin:$HOME/.local/bin:$PATH"

# 检查未提交的变更
if [ -n "$(git status --porcelain 2>/dev/null)" ]; then
  echo "Uncommitted changes detected. Commit your work before marking complete." >&2
  exit 2
fi

# 检查分支是否已推送
BRANCH=$(git branch --show-current 2>/dev/null)
if ! git log "origin/$BRANCH" --oneline -1 >/dev/null 2>&1; then
  echo "Branch not pushed to remote. Run 'git push' before completing." >&2
  exit 2
fi

exit 0
```

---

## 6. Convoy 到 AT 团队的映射

### 自然映射

| Gas Town | AT 等价物 |
|----------|--------------|
| Convoy | AT 团队生命周期 |
| Convoy issue | AT 任务 |
| War Rig（按 rig 的 convoy 执行） | AT 团队实例 |
| Ready front（未阻塞 issue） | 未阻塞 AT 任务 |
| 分派 | AT 任务创建 + 队友生成 |
| 完成跟踪 | AT 任务列表状态 |

### 一个 Convoy = 一个 AT 团队会话

一个 convoy 到达一个 rig。Witness 为该 convoy 创建 AT 团队：

```
Convoy hq-abc 到达 gastown
    │
    ├── Witness 创建团队："gastown-convoy-abc"
    │
    ├── 对 convoy 中的每个 issue：
    │   ├── 创建 AT 任务（元数据中包含 bead_id）
    │   └── 设置依赖（来自 beads 依赖图）
    │
    ├── 生成 N 个 polecat 队友（N = min(issues, max_polecats)）
    │
    ├── 队友从 ready front 自行认领任务
    │
    ├── 随着任务完成：
    │   ├── 依赖解除下一个任务的阻塞
    │   ├── 空闲队友自动认领新就绪任务
    │   └── 通过 TaskCompleted hook 同步 Beads
    │
    └── 所有任务完成：
        ├── Witness 验证 beads 同步
        ├── Witness 通过 gt mail 向 Mayor 发送 convoy 完成通知
        └── 团队关闭
```

### 多个 Convoy

AT 限制：每个会话一个团队。如果第二个 convoy 在第一个活跃时到达：

**选项 A：顺序处理。** 完成 convoy 1，然后开始 convoy 2。简单，无并发问题。如果 convoy 吞吐量足够，可以接受。

**选项 B：Convoy 队列。** Witness 将传入 convoy 排队并按序处理。队列存在于 beads 中（mail 收件箱）— Witness 在当前团队完成时检查新 convoy。

**选项 C：多个 Witness 会话。** Daemon 为第二个 convoy 生成第二个 Witness 会话。每个 Witness 管理自己的 AT 团队。这需要 daemon 支持每个 rig 多个 Witness 实例。

**建议：** 阶段 1 选择 A（顺序）。如果吞吐量需要，阶段 2+ 选择 C。选项 B 中的 convoy 队列在 beads 中已隐式存在（未处理的 convoy mail = 排队的工作）。

### 稳态工作者池

对于大型 convoy（20+ issue），Witness 不会一次生成 20 个队友。而是：

```
max_teammates = 5  # 按 rig 可配置

1. 生成 max_teammates 个 polecat
2. 创建所有 AT 任务（带依赖）
3. 队友从 ready front 自行认领
4. 当队友完成任务：
   - 自动认领下一个未阻塞任务
   - 无需重生（同一队友，新任务）
5. 所有任务完成时：团队关闭
```

AT 的自行认领机制是关键使能器。队友在一个任务后不会消亡 — 他们拾取下一个。这消除了当前每个 issue 的生成/nuke 开销。

**当队友需要轮换时**（压缩），Witness 生成替换者，而非额外队友。池大小保持在 max_teammates。

---

## 7. 邮件桥接：gt mail ↔ AT 消息

### 边界

```
                    ┌─────────────────┐
                    │    Witness       │
                    │  (AT 团队负责人)  │
                    │                  │
    gt mail ←──────│── 桥接 ──────→ AT 消息
    (跨 rig，       │                  (团队内，
     持久)          │                   临时)
                    └─────────────────┘
```

### 入站：gt mail → AT 消息

当 Witness 收到与活跃队友相关的 gt mail 时：

```
gt mail inbox
    │
    ├── 来自 Mayor："优先级变更 — issue X 现为 P0"
    │   └── Witness 向相关队友发送 AT 消息：
    │       Teammate({ operation: "write", target_agent_id: "<polecat>",
    │                  value: "Priority update: <issue> is now P0. Expedite." })
    │
    ├── 来自 Refinery："<branch> 上合并冲突"
    │   └── Witness 向该分支上的 polecat 发送 AT 消息：
    │       Teammate({ operation: "write", target_agent_id: "<polecat>",
    │                  value: "Merge conflict detected. Rebase on main." })
    │
    └── 来自其他 rig 的 Witness："依赖 <issue> 已完成"
        └── Witness 为下游工作创建/解除阻塞 AT 任务
```

### 出站：AT 事件 → gt mail

当 AT 事件需要到达团队外的实体时：

```
队友完成最终任务
    │
    └── Witness 检测到所有任务完成
        │
        ├── gt mail send gastown/refinery -s "MERGE_READY: <branch>"
        │   └── Refinery 处理合并队列
        │
        ├── gt mail send mayor/ -s "CONVOY COMPLETE: hq-abc"
        │   └── Mayor 更新 convoy 跟踪
        │
        └── gt mail send gastown/witness -s "POLECAT_DONE: <name>"
            └──（自发送用于 beads 记录）
```

### 什么走什么通道

| 通信 | 通道 | 原因 |
|--------------|---------|-----|
| Witness ↔ Polecat | AT 消息 | 同一团队，实时，临时 |
| Polecat ↔ Polecat | AT 消息 | 同一团队，协调交流 |
| Witness → Refinery | gt mail | 不同生命周期，需要持久性 |
| Witness → Mayor | gt mail | 跨 rig，需要持久性 |
| Mayor → Witness | gt mail | 跨 rig，需要持久性 |
| Polecat 升级 | AT 消息到 Witness，Witness 通过 gt mail 中继 | 桥接模式 |

### 中继模式

Polecat 无法直接向团队外的实体发送 gt mail（AT 消息是团队范围的）。而是：

```
Polecat 需要向 Mayor 升级：
    │
    ├── Polecat 向 Witness 发送 AT 消息：
    │   "ESCALATE: Need Mayor decision on auth approach"
    │
    └── Witness 通过 gt mail 中继：
        gt mail send mayor/ -s "ESCALATE from polecat <name>" -m "..."
```

这类似于当前模型，其中 polecat 向 Witness 发送 mail，Witness 升级。区别：AT 消息是实时的（无 Dolt 同步延迟），Witness 可以立即中继。

---

## 8. 配置

### `.claude/settings.json`（项目级别）

```json
{
  "env": {
    "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"
  },
  "hooks": {
    "TaskCompleted": [{
      "hooks": [{
        "type": "command",
        "command": ".claude/hooks/task-completed-sync.sh"
      }]
    }],
    "TeammateIdle": [{
      "hooks": [{
        "type": "command",
        "command": ".claude/hooks/teammate-idle-check.sh"
      }]
    }],
    "SubagentStop": [{
      "matcher": "polecat",
      "hooks": [{
        "type": "command",
        "command": ".claude/hooks/teammate-stopped.sh"
      }]
    }]
  }
}
```

### `.claude/agents/witness-lead.md`

```yaml
---
name: witness-lead
description: Gas Town Witness operating as AT team lead
model: opus
permissionMode: delegate
hooks:
  SessionStart:
    - hooks:
        - type: command
          command: "export PATH=\"$HOME/go/bin:$HOME/.local/bin:$PATH\" && gt prime --hook"
  PreToolUse:
    - matcher: "Bash"
      hooks:
        - type: command
          command: "gt witness-bash-guard"
  Stop:
    - hooks:
        - type: command
          command: "gt signal stop"
---

You are the Gas Town Witness for this rig.

## Role
You coordinate polecat workers. You NEVER implement code directly.
Delegate mode enforces this structurally — you cannot edit files.

## Startup
1. Check for incoming work: `gt mail inbox`, `bd ready`
2. Create AT team if work is available
3. Spawn polecat teammates for each issue
4. Monitor progress via AT task list

## During Work
- Monitor teammate progress via TaskList
- Relay cross-rig messages (gt mail ↔ AT messages)
- Handle teammate crashes (respawn or escalate)
- Enforce quality via plan approval

## Completion
- Verify all AT tasks completed
- Verify beads are synced (all issues closed)
- Send MERGE_READY to Refinery via gt mail
- Send convoy completion to Mayor via gt mail
- Shutdown team
```

### `.claude/agents/polecat.md`

见第 2 节上方的完整定义。

---

## 9. 被替换的内容

### 被移除的基础设施（阶段 1）

| 组件 | 替代 | 备注 |
|-----------|-------------|-------|
| `gt sling`（polecat 生成） | `Teammate({ operation: "spawn" })` | AT 原生 |
| `gt polecat nuke` | `Teammate({ operation: "requestShutdown" })` | AT 原生 |
| tmux 会话管理 | AT 管理队友会话 | Polecat 不再使用 tmux |
| `gt nudge`（tmux send-keys） | `Teammate({ operation: "write" })` | AT 消息 |
| 僵尸检测（基于 tmux） | SubagentStop / TeammateIdle hook | 结构化 |
| Witness "你卡住了吗？" 轮询 | TeammateIdle hook（自动） | 事件驱动 |
| Polecat 间隔离 | 提示 + PreToolUse hook | 行为化 → hook 强制 |

### 保留的基础设施（阶段 1）

| 组件 | 原因 |
|-----------|-----|
| Beads (Dolt) | 持久账本 — AT 任务是临时的 |
| gt mail | 跨 rig 通信 — AT 是团队范围的 |
| Molecules/formulas | 工作模板 — AT 任务从这些创建 |
| `gt done` | Polecat 自清理 — 生命周期不变 |
| Git worktree | 文件系统隔离 — AT 不提供此功能 |
| Daemon/Boot/Deacon | 健康监控 — AT 无崩溃恢复 |
| Refinery（独立） | 不同生命周期（阶段 2 将其纳入频带内） |
| Convoy 跟踪 | 跨 rig 工作订单 — 超出 AT 范围 |

### Dolt 写入压力减少

**当前：** 每个 polecat 的每次 `bd update`、`bd close`、`bd create` = 并发 Dolt 写入。20 个 polecat = 20+ 并发提交。

**使用 AT：** 实时任务协调在 AT 中进行（文件锁定，无 Dolt）。Dolt 写入仅在边界处：
- molecule 步骤完成时的 `bd close`（每任务 1 次）
- polecat 发现新 issue 时的 `bd create`（罕见）

**估计减少：80-90%。** 剩余写入在分钟级别自然交错（任务完成），而非毫秒级（并发状态更新）。

---

## 10. Witness 启动流程（更新）

```
Witness 会话启动（由 daemon 管理）
    │
    ├── SessionStart hook：gt prime --hook
    │   └── 加载角色上下文，检查 hook
    │
    ├── 检查工作：
    │   ├── gt mail inbox（convoy 分派、优先级变更）
    │   ├── bd ready（未阻塞 issue）
    │   └── gt hook（已 hook 的工作）
    │
    ├── 如果有可用工作：
    │   │
    │   ├── 创建 AT 团队：
    │   │   Teammate({ operation: "spawnTeam", team_name: "<rig>-work" })
    │   │
    │   ├── 从 beads issue 创建 AT 任务：
    │   │   对每个 issue：TaskCreate({ subject, description, metadata: { bead_id } })
    │   │   设置依赖：TaskUpdate({ addBlockedBy: [...] })
    │   │
    │   ├── 为 polecat 创建 worktree：
    │   │   对每个 polecat：git worktree add ...
    │   │
    │   ├── 生成 polecat 队友：
    │   │   对每个（最多 max_teammates）：
    │   │     Task({ subagent_type: "polecat", team_name: "...", name: "..." })
    │   │
    │   └── 进入监控循环：
    │       ├── 观察 AT 任务列表中的完成
    │       ├── 处理队友崩溃（SubagentStop）
    │       ├── 中继 gt mail ↔ AT 消息
    │       ├── 检查新 convoy 到达
    │       └── 所有任务完成时：清理和报告
    │
    └── 如果无工作：
        └── Stop hook 定期检查排队工作
            └── 如果工作到达：唤醒并创建团队
```

---

## 11. 阶段 1 范围和验证标准

### 范围内

1. Witness 作为委托模式中的 AT 团队负责人（带 Bash 用于 gt/bd）
2. 带 `.claude/agents/polecat.md` 的 Polecat 队友
3. 通过 TaskCompleted hook 的 Bead 同步
4. 通过 PreCompact handoff + 重生的会话轮换
5. 基本错误处理（崩溃检测、重生、崩溃循环预防）
6. 邮件桥接（gt mail ↔ AT 消息）
7. 单 convoy 顺序处理

### 范围外（阶段 2+）

1. Refinery 作为 AT 队友
2. 多个并发 convoy
3. 跨 rig AT 协调
4. Crew 小队 / 影子工作者
5. 高级计划审批工作流
6. 性能优化（token 成本调优）

### 验证标准

| 标准 | 测试 |
|-----------|------|
| Witness 保持委托模式 | 验证 Witness 无法写入/编辑文件 |
| Polecat 完成工作 | 端到端：生成 → 实现 → 推送 → gt done |
| Beads 正确同步 | AT 任务完成 → bd close 触发 → bead 已关闭 |
| 会话轮换工作 | 强制压缩 → 新队友从 beads 恢复 |
| 崩溃恢复工作 | 杀死队友 → Witness 检测 → 重生 |
| 邮件桥接工作 | Mayor 发送 mail → Witness 中继到 polecat |
| Dolt 写入减少 | 测量 bd 命令频率：前后对比 |
| Token 成本可接受 | `/cost` 显示 < 3 倍于当前模型的开销 |
| Convoy 完成 | 完整 convoy 生命周期：分派 → 工作 → 合并 → 完成 |

---

## 12. 迁移路径

### 当前架构 → 阶段 1

迁移是增量的：AT 在验证期间与现有基础设施并行运行。如果 AT 失败，Witness 可以回退到基于 tmux 的管理。

```
步骤 1：在 gastown .claude/settings.json 中启用 AT 功能标志
步骤 2：创建 .claude/agents/polecat.md 和 .claude/agents/witness-lead.md
步骤 3：实现 hook 脚本（task-completed-sync、teammate-idle、teammate-stopped）
步骤 4：实现 gt witness-bash-guard
步骤 5：实现 gt validate-worktree-scope
步骤 6：实现 gt witness-teammate-stopped
步骤 7：更新 Witness 启动以创建 AT 团队而非 tmux polecat 会话
步骤 8：用 2 个 polecat 在小型 convoy 上测试
步骤 9：验证上述所有标准
步骤 10：如果验证通过：扩展到 3-5 个 polecat，更大的 convoy
```

### 回滚计划

如果阶段 1 失败：
1. 禁用 AT 功能标志
2. Witness 回退到基于 tmux 的 polecat 管理
3. 无 beads 数据丢失（beads 同步是增量的）
4. 为阶段 1 重试记录经验教训 bead

---

*"传输改变。账本长存。"*