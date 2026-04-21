# Gas Town 邮件协议

> Gas Town 中 agent 间邮件通信的参考文档

## 概述

Gas Town agent 通过 beads 系统路由的邮件消息进行协调。
邮件使用 `type=message` 的 bead，路由由 `gt mail` 处理。

## 消息类型

### POLECAT_DONE

**路由**：Polecat → Witness

**用途**：通知工作完成，触发清理流程。

**主题格式**：`POLECAT_DONE <polecat-name>`

**正文格式**：
```
Exit: MERGED|ESCALATED|DEFERRED
Issue: <issue-id>
MR: <mr-id>          # 如果 exit=MERGED
Branch: <branch>
```

**触发**：`gt done` 命令自动生成此消息。

**处理器**：Witness 为该 polecat 创建清理 wisp。

### MERGE_READY

**路由**：Witness → Refinery

**用途**：通知分支已准备好进入合并队列处理。

**主题格式**：`MERGE_READY <polecat-name>`

**正文格式**：
```
Branch: <branch>
Issue: <issue-id>
Polecat: <polecat-name>
Verified: clean git state, issue closed
```

**触发**：Witness 在验证 polecat 工作完成后发送。

**处理器**：Refinery 添加到合并队列，就绪时处理。

### MERGED

**路由**：Refinery → Witness

**用途**：确认分支已成功合并，可以销毁 polecat。

**主题格式**：`MERGED <polecat-name>`

**正文格式**：
```
Branch: <branch>
Issue: <issue-id>
Polecat: <polecat-name>
Rig: <rig>
Target: <target-branch>
Merged-At: <timestamp>
Merge-Commit: <sha>
```

**触发**：Refinery 在成功合并到 main 后发送。

**处理器**：Witness 完成清理 wisp，销毁 polecat worktree。

### MERGE_FAILED

**路由**：Refinery → Witness

**用途**：通知合并尝试失败（测试、构建或其他非冲突错误）。

**主题格式**：`MERGE_FAILED <polecat-name>`

**正文格式**：
```
Branch: <branch>
Issue: <issue-id>
Polecat: <polecat-name>
Rig: <rig>
Target: <target-branch>
Failed-At: <timestamp>
Failure-Type: <tests|build|push|other>
Error: <error-message>
```

**触发**：Refinery 在合并非冲突原因失败时发送。

**处理器**：Witness 通知 polecat，将工作重新分配进行返工。

### REWORK_REQUEST

**路由**：Refinery → Witness

**用途**：要求 polecat 因合并冲突而 rebase 分支。

**主题格式**：`REWORK_REQUEST <polecat-name>`

**正文格式**：
```
Branch: <branch>
Issue: <issue-id>
Polecat: <polecat-name>
Rig: <rig>
Target: <target-branch>
Requested-At: <timestamp>
Conflict-Files: <file1>, <file2>, ...

请将你的变更 rebase 到 <target-branch> 上：

  git fetch origin
  git rebase origin/<target-branch>
  # 解决冲突
  git push -f

Rebase 完成后 Refinery 将重试合并。
```

**触发**：Refinery 在与目标分支有冲突时发送。

**处理器**：Witness 通知 polecat 进行 rebase。

### RECOVERED_BEAD

**路由**：Witness → Deacon

**用途**：通知 Deacon 已恢复死亡 polecat 的废弃工作
并需要重新调度。

**主题格式**：`RECOVERED_BEAD <bead-id>`

**正文格式**：
```
从死亡 polecat 恢复了废弃 bead。

Bead: <bead-id>
Polecat: <rig>/<polecat-name>
Previous Status: <hooked|in_progress>

Bead 已重置为 open 且无 assignee。
请重新调度到可用的 polecat。
```

**触发**：Witness 检测到工作仍处于 hooked/in_progress 状态的
僵尸 polecat。Bead 被重置为 open 状态，此邮件发送以重新调度。

**处理器**：Deacon 运行 `gt deacon redispatch <bead-id>`：
- 限制重新调度速率（每 bead 5 分钟冷却）
- 追踪失败计数（3 次失败后升级到 Mayor）
- 从 bead 前缀自动检测目标 rig
- 通过 `gt sling` 将 bead sling 到可用 polecat

### RECOVERY_NEEDED

**路由**：Witness → Deacon

**用途**：升级需要人工恢复才能清理的脏 polecat
（有未推送/未提交的工作）。

**主题格式**：`RECOVERY_NEEDED <rig>/<polecat-name>`

**正文格式**：
```
Polecat: <rig>/<polecat-name>
Cleanup Status: <has_uncommitted|has_stash|has_unpushed>
Branch: <branch>
Issue: <issue-id>
Detected: <timestamp>
```

**触发**：Witness 检测到有脏 git 状态的僵尸 polecat。

**处理器**：Deacon 在授权清理前协调恢复（推送分支、保存工作）。
仅在 Deacon 无法解决时升级到 Mayor。

### HELP

**路由**：任意 → 升级目标（通常是 Mayor）

**用途**：请求干预以处理卡住/阻塞的工作。

**主题格式**：`HELP: <简短描述>`

**正文格式**：
```
Agent: <agent-id>
Issue: <issue-id>       # 如果适用
Problem: <description>
Tried: <已尝试的内容>
```

**触发**：Agent 无法继续，需要外部帮助。

**处理器**：升级目标评估并干预。

### HANDOFF

**路由**：Agent → 自身（或继任者）

**用途**：跨上下文限制/重启的会话连续性。

**主题格式**：`🤝 HANDOFF: <简短上下文>`

**正文格式**：
```
attached_molecule: <molecule-id>   # 如果工作正在进行
attached_at: <timestamp>

## 上下文
<给继任者的自由格式笔记>

## 状态
<当前进展>

## 下一步
<继任者应该做什么>
```

**触发**：`gt handoff` 命令，或会话结束前手动发送。

**处理器**：下一会话读取 handoff，从上下文继续。

## 格式约定

### 主题行

- **类型前缀**：大写，标识消息类型
- **冒号分隔符**：类型之后用于结构化信息
- **简短上下文**：人类可读的摘要

示例：
```
POLECAT_DONE nux
MERGE_READY greenplace/nux
HELP: Polecat 在测试失败上卡住
🤝 HANDOFF: Schema 工作进行中
```

### 正文结构

- **键值对**：用于结构化数据（每行一个）
- **空行**：将结构化数据与自由格式内容分开
- **Markdown 章节**：用于自由格式内容（##、列表、代码块）

### 地址

格式：`<rig>/<role>` 或 `<rig>/<type>/<name>`

示例：
```
greenplace/witness       # greenplace rig 的 Witness
beads/refinery           # beads rig 的 Refinery
greenplace/polecats/nux  # 特定 polecat
mayor/                # Town 级 Mayor
deacon/               # Town 级 Deacon
```

## 协议流程

### Polecat 完成流程

```
Polecat                    Witness                    Refinery
   │                          │                          │
   │ POLECAT_DONE             │                          │
   │─────────────────────────>│                          │
   │                          │                          │
   │                    （验证清洁状态）                  │
   │                          │                          │
   │                          │ MERGE_READY              │
   │                          │─────────────────────────>│
   │                          │                          │
   │                          │                    （合并尝试）
   │                          │                          │
   │                          │ MERGED（成功）           │
   │                          │<─────────────────────────│
   │                          │                          │
   │                    （销毁 polecat）                   │
   │                          │                          │
```

### 合并失败流程

```
                           Witness                    Refinery
                              │                          │
                              │                    （合并失败）
                              │                          │
                              │ MERGE_FAILED             │
   ┌──────────────────────────│<─────────────────────────│
   │                          │                          │
   │ （失败通知）              │                          │
   │<─────────────────────────│                          │
   │                          │                          │
Polecat（需要返工）
```

### Rebase 要求流程

```
                           Witness                    Refinery
                              │                          │
                              │                    （检测到冲突）
                              │                          │
                              │ REWORK_REQUEST           │
   ┌──────────────────────────│<─────────────────────────│
   │                          │                          │
   │ （rebase 指令）           │                          │
   │<─────────────────────────│                          │
   │                          │                          │
Polecat                       │                          │
   │                          │                          │
   │ （rebase 后，gt done）    │                          │
   │─────────────────────────>│ MERGE_READY              │
   │                          │─────────────────────────>│
   │                          │                    （重试合并）
```

### 废弃工作恢复流程

```
死亡 Polecat               Witness                    Deacon
     │                        │                          │
     │ （会话死亡）             │                          │
     │                        │                          │
     │                  （检测到僵尸）                     │
     │                  （bead 状态=hooked）              │
     │                        │                          │
     │                  resetAbandonedBead()             │
     │                  bd update --status=open          │
     │                        │                          │
     │                        │ RECOVERED_BEAD           │
     │                        │─────────────────────────>│
     │                        │                          │
     │                        │                    gt deacon redispatch
     │                        │                    gt sling <bead> <rig>
     │                        │                          │
     │                        │                          ├──> 新 Polecat
     │                        │                          │    （重新调度）
```

### 二级监控

```
Witness-1 ──┐
            │ （检查 agent bead 的 last_activity）
Witness-2 ──┼────────────────> Deacon agent bead
            │
Witness-N ──┘
                                 │
                          （如果过期 >5min）
                                 │
            ─────────────────────┘
            ALERT 到 Mayor（仅在失败时发邮件）
```

## 通信卫生：Mail vs Nudge

Agent 过度使用邮件进行日常通信，为本应是临时性的消息
生成永久 bead 和 Dolt commit。每次 `gt mail send` 都会在
Dolt 中创建一个 wisp bead — 一个带有自己 commit 的永久记录，
存在于 git 式历史中。这是一个关键的污染源。

### 两个通道

**`gt nudge`（临时，日常通信首选）**
- 直接向 agent 的 tmux 会话发送消息
- 不创建 bead。不产生 Dolt commit。零存储成本。
- 消息在 agent 的上下文中以 `<system-reminder>` 形式出现
- 适用于：健康检查、状态请求、简单指令、"唤醒"信号
- 限制：如果目标会话已死，nudge 会丢失

**`gt mail send`（持久，仅用于结构化协议消息）**
- 在 Dolt 数据库中创建一个 bead（wisp）
- 至少生成一个 Dolt commit（写入操作）
- 跨会话重启持久存在 — agent 死亡后仍然存活
- 适用于：HANDOFF 上下文、MERGE_READY/MERGED 协议、升级、HELP
  请求、任何必须在接收方会话死亡后仍然存活的内容

### 规则

**默认使用 `gt nudge`。仅在消息必须在接收方会话死亡后仍然存活时
才使用 `gt mail send`。**

试金石测试："如果接收方的会话死亡并重启，他们需要这条消息吗？"
如果需要 → mail。如果不需要 → nudge。

### 角色特定指引

| 角色 | 邮件预算 | 何时用 Mail | 何时用 Nudge |
|------|----------|------------|--------------|
| **Polecat** | 每会话 0-1 | 仅 HELP/ESCALATE（优先用 gt escalate） | 其他一切 |
| **Witness** | 仅协议消息 | MERGE_READY、RECOVERED_BEAD、RECOVERY_NEEDED、升级到 Mayor | Polecat 健康检查、状态 ping、催促与观察 |
| **Refinery** | 仅协议消息 | MERGED、MERGE_FAILED、REWORK_REQUEST | 向 Witness 的状态更新 |
| **Deacon** | 仅升级 | 升级到 Mayor、向自身的 HANDOFF | TIMER 回调、HEALTH_CHECK、生命周期催促 |
| **Dogs** | 零 | 永远不发邮件（结果进入事件 bead 或日志） | 通过 nudge 向 Deacon 报告完成 |
| **Mayor** | 仅战略 | 跨 rig 协调、向自身的 HANDOFF | 向 Deacon/Witness 的指令 |

### 为什么这很重要（提交图）

Dolt 底层是 git。每次邮件创建一个 Dolt commit。一天正常操作中：
- 4 个 agent x 15 个巡逻周期 x 每周期 2 封邮件 = 仅日常闲聊就有 120 个 commit
- 这些 commit 永远存在于 git 历史中，即使邮件行被删除后
- Rebase 可以移除它们，但预防始终比清理更经济

### 反模式

**DOG_DONE 作为邮件** — Dog 不应该邮寄其完成状态。改用
`gt nudge deacon/ "DOG_DONE: plugin-name success"`。

**重复升级** — Witness 在几分钟内发送关于同一问题的
2+ 封邮件。发送前检查收件箱：如果你已经发送了关于此主题的邮件，
不要再发。

**常规周期的 HANDOFF** — 巡逻 agent（Witness、Deacon）进行
常规 handoff 时应使用最少的邮件。如果没有异常情况，直接循环 —
下一会话从 beads 中发现状态，而非从邮件中。

**健康检查响应通过邮件** — 当 Deacon 发送健康检查 nudge 时，
不要用邮件回复。Deacon 通过会话状态追踪健康，不通过邮件
响应。

## 实现

### 发送邮件

```bash
# 基本发送
gt mail send <addr> -s "Subject" -m "Body"

# 带结构化正文
gt mail send greenplace/witness -s "MERGE_READY nux" -m "Branch: feature-xyz
Issue: gp-abc
Polecat: nux
Verified: clean"
```

### 接收邮件

```bash
# 检查收件箱
gt mail inbox

# 读取特定消息
gt mail read <msg-id>

# 标记为已读
gt mail ack <msg-id>
```

### 在巡逻 Formula 中

Formula 应该：
1. 每个周期开始时检查收件箱
2. 解析主题前缀以路由处理
3. 从正文中提取结构化数据
4. 采取适当的行动
5. 处理后将邮件标记为已读

## 可扩展性

新消息类型遵循此模式：
1. 定义主题前缀（TYPE: 或 TYPE_SUBTYPE）
2. 记录正文格式（键值对 + 自由格式）
3. 指定路由（发送方 → 接收方）
4. 在相关巡逻 formula 中实现处理器

协议有意保持简单 — 足够结构化以便解析，
足够灵活以便人类调试。

## Beads 原生消息

除了直接的 agent 间邮件外，消息系统支持三种基于 bead 的
原语用于群组和广播通信。全部使用 `hq-` 前缀
（跨 rig 的 town 级实体）。

### 群组（`gt:group`**

用于邮件分发的命名地址集合。发送到群组
会投递给所有成员。

**Bead ID 格式**：`hq-group-<name>`

**成员类型**：直接地址（`gastown/crew/max`）、通配符模式
（`*/witness`、`gastown/crew/*`）、特殊模式（`@town`、`@crew`、
`@witnesses`）或嵌套群组名。

### 队列（`gt:queue`**

每条消息仅给一个认领者的工作队列（不同于群组）。

**Bead ID 格式**：`hq-q-<name>`（town 级）或 `gt-q-<name>`（rig 级）

字段：`status`（active/paused/closed）、`max_concurrency`、`processing_order`
（fifo/priority），加上计数字段（available、processing、completed、failed）。

### 频道（`gt:channel`**

可配置消息保留的发布/订阅广播流。

**Bead ID 格式**：`hq-channel-<name>`

字段：`subscribers`、`status`（active/closed）、`retention_count`、
`retention_hours`。

### 群组和频道 CLI 命令

```bash
# 群组
gt mail group list
gt mail group show <name>
gt mail group create <name> [members...]
gt mail group add <name> <member>
gt mail group remove <name> <member>
gt mail group delete <name>

# 频道
gt mail channel list
gt mail channel show <name>
gt mail channel create <name> [--retain-count=N] [--retain-hours=N]
gt mail channel delete <name>
```

### 发送到群组、队列和频道

```bash
gt mail send my-group -s "Subject" -m "Body"           # 群组（展开到成员）
gt mail send queue:my-queue -s "Work item" -m "Details" # 队列（单认领者）
gt mail send channel:alerts -s "Alert" -m "Content"     # 频道（广播）
```

### 地址解析顺序

发送邮件时，地址按以下顺序解析：

1. **显式前缀** — `group:`、`queue:` 或 `channel:` 直接使用该类型
2. **包含 `/`** — 视为 agent 地址或模式（直接投递）
3. **以 `@` 开头** — 特殊模式（`@town`、`@crew` 等）或群组
4. **名称查找** — 按名称搜索 群组 → 队列 → 频道

如果名称匹配多个类型，解析器返回需要显式前缀的错误。

### 保留策略

频道支持基于计数（`--retain-count=N`）和基于时间
（`--retain-hours=N`）的保留。保留在写入时（发布后）
和巡逻时（Deacon 运行 `PruneAllChannels()`，带 10% 缓冲以避免
抖动）强制执行。

## 相关文档

- `docs/agent-as-bead.md` - Agent 身份和槽位
- `.beads/formulas/mol-witness-patrol.formula.toml` - Witness 处理
- `internal/mail/` - Mail 路由实现
- `internal/protocol/` - Witness-Refinery 通信的协议处理器