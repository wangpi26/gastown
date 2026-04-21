# Polecat 生命周期

> 理解 Polecat 工作者的三层架构

## 概述

Polecat 拥有三个独立运作的独立生命周期层。关键设计原则：**Polecat 是持久化的**。它们在工作完成后继续存在，可以在不同任务之间复用。

## 四种运行状态

Polecat 有四种运行状态：

| 状态 | 说明 | 触发条件 |
|------|------|---------|
| **Working** | 正在执行分配的工作 | `gt sling` 后的正常运行 |
| **Idle** | 工作完成，沙盒保留以供复用 | `gt done` 成功完成后 |
| **Stalled** | 会话在工作途中停止 | 被中断、崩溃或超时且未被 Nudge |
| **Zombie** | 完成了工作但未能退出 | `gt done` 在清理阶段失败 |

**状态循环（正常路径）：**

```
         ┌──────────┐
    ┌───>│  IDLE    │<──── 同步沙盒到 main，清除 Hook
    │    └────┬─────┘
    │         │ gt sling
    │         v
    │    ┌──────────┐
    │    │ WORKING  │<──── 会话活跃，Hook 已设置
    │    └────┬─────┘
    │         │ gt done
    │         v
    │    ┌──────────┐
    └────┤  IDLE    │──── 推送分支，提交 MR，转为空闲
         └──────────┘
```

正常路径中没有 `nuke`。Polecat 循环：IDLE -> WORKING -> IDLE。

**关键区别：**

- **Working** = 正在执行。会话活跃，Hook 已设置，正在工作。
- **Idle** = 工作完成，会话已终止，沙盒保留。准备好接受下一次 `gt sling`。
- **Stalled** = 应该在工作但已停止。需要 Witness 介入。
- **Zombie** = 完成了工作，尝试退出，但清理失败。卡在中间状态。

## 持久 Polecat 模型（gt-4ac）

**Polecat 在完成工作后继续存在。** 当 Polecat 完成其任务时：

1. 通过 `gt done` 发出完成信号
2. 推送分支，向合并队列提交 MR
3. 清除其 Hook（工作已完成）
4. 将 Agent 状态设为"idle"
5. 终止自己的会话
6. **沙盒（worktree）被保留以供复用**

下一次 `gt sling` 会复用空闲的 Polecat 而非分配新的，避免了创建全新 worktree 的开销。

### 为什么要持久化？

- **更快的周转** — 复用已有的 worktree 比创建新的更快
- **保留身份** — Polecat 的 Agent Bead、CV 链和工作历史持续存在
- **更简单的生命周期** — 任务之间没有 nuke/respawn 循环
- **Done 即 Idle** — 会话终止，沙盒存活，Polecat 等待下一个任务

### 待合并的 MR 怎么办？

Refinery 拥有合并队列。一旦 `gt done` 提交了工作：
- 分支已推送到 origin
- 工作存在于 MQ 中，而非在 Polecat 中
- 如果 rebase 失败，Refinery 会创建冲突解决任务
- 空闲的 Polecat 可以被复用于冲突解决工作

## 三层架构

### 问题：三个概念曾被混淆

早期设计将 Polecat 视为单体。这导致了反复出现的问题：

| 概念 | 生命周期 | 旧行为 |
|------|---------|-------|
| **身份** | 长期存在（名称、CV、账本） | 在 nuke 时销毁 |
| **沙盒** | 每次任务（worktree、分支） | 在 nuke 时销毁 |
| **会话** | 临时性（Claude 上下文窗口） | = Polecat 生命周期 |

将这三层分离意味着空闲的 Polecat 是健康状态（而非浪费），消除了不必要的 worktree 创建开销，并在任务之间保留了能力记录（CV、完成历史）。

### 层级摘要

| 层 | 组成 | 生命周期 | 持续性 |
|---|------|---------|--------|
| **身份** | Agent Bead、CV 链、工作历史 | 永久 | 永不消亡 |
| **沙盒** | Git worktree、分支 | 跨任务持久 | 首次 sling 时创建，之后复用 |
| **会话** | Claude（tmux 面板）、上下文窗口 | 每步临时 | 每步/交接时循环 |

### 身份层

Polecat 的**身份是永久的**。它包括：

- Agent Bead（创建一次，永不删除）
- CV 链（工作历史在所有任务中累积）
- 邮箱和归属记录

身份在所有会话循环和沙盒重置中存活。在 HOP 模型中，身份就是 Polecat 本身——其他一切都是来了又走的基础设施。详见下文的 [Polecat 身份](#polecat-identity)。

### 会话层

Claude 会话是**临时性的**。它频繁循环：

- 每个 Molecule 步骤后（通过 `gt handoff`）
- 上下文压缩时
- 崩溃/超时时
- 长时间工作后

**关键洞察**：会话循环是**正常操作**，不是故障。Polecat 继续工作——只是 Claude 上下文在刷新。

```
Session 1: Steps 1-2 → handoff
Session 2: Steps 3-4 → handoff
Session 3: Step 5 → gt done
```

三个会话都是**同一个 Polecat**。沙盒贯穿始终。

### 沙盒层

沙盒是**git worktree**——Polecat 的工作目录：

```
~/gt/gastown/polecats/Toast/
```

这个 worktree：
- 从第一次 `gt sling` 起存在，跨任务持久
- 在所有会话循环中存活
- 在被 `gt sling` 复用时被修复（重置为 main 的新分支）
- 在活跃工作期间包含未提交的工作、暂存的变更、分支状态

Witness 从不销毁沙盒。只有显式的 `gt polecat nuke` 才会移除它们。

#### 沙盒同步（任务之间）

当工作完成、Polecat 进入空闲状态时，沙盒会同步到 main：

```bash
# 在 Polecat 的 worktree 中（由 gt done / gt sling 自动执行）
git checkout main
git pull origin main
git branch -D polecat/<name>/<old-issue>@<timestamp>
# Worktree 现在干净了，在 main 上，准备好接受下一个任务
```

当新工作被 Sling 时：
```bash
# 从当前 main 创建新分支
git checkout -b polecat/<name>/<new-issue>@<timestamp>
# 开始工作
```

任务之间不需要 worktree add/remove。只需在已有 worktree 上做分支操作。这避免了约 5 秒的创建新 worktree 开销。

### Slot 层

Slot 是 Polecat 池中的**名称分配**：

```bash
# 池: [Toast, Shadow, Copper, Ash, Storm...]
# Toast 被分配去处理工作 gt-abc
```

Slot：
- 决定沙盒路径（`polecats/Toast/`）
- 映射到 tmux 会话（`gt-gastown-Toast`）
- 出现在归属记录中（`gastown/polecats/Toast`）
- 持续存在直到显式 nuke

## 正确的生命周期

```
┌─────────────────────────────────────────────────────────────┐
│                        gt sling                             │
│  → 找到空闲 Polecat 或从池中分配 Slot (Toast)              │
│  → 创建/修复沙盒（worktree 在新分支上）                     │
│  → 启动会话（tmux 中的 Claude）                             │
│  → 将 Molecule 挂载到 Polecat                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     工作进行中                               │
│                                                             │
│  会话循环发生在这里：                                       │
│  - 步骤之间的 gt handoff                                   │
│  - 压缩触发 respawn                                        │
│  - 崩溃 → Witness respawn                                  │
│                                                             │
│  沙盒在所有会话循环中持久存在                               │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                  gt done（持久模型）                          │
│  → 推送分支到 origin                                       │
│  → 向合并队列提交工作（MR bead）                            │
│  → 将 Agent 状态设为"idle"                                │
│  → 终止会话                                                │
│                                                             │
│  工作现在存在于 MQ 中。Polecat 是 IDLE，不是消失了。        │
│  沙盒保留以供下一次 gt sling 复用。                         │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Refinery：合并队列                          │
│  → Rebase 并合并到目标分支                                 │
│    （main 或 integration branch — 见下文）                  │
│  → 关闭 issue                                              │
│  → 如果有冲突：为可用的 Polecat 创建任务                    │
│                                                             │
│  Integration branch 路径：                                  │
│  → 来自 Epic 子项的 MR 合并到 integration/<epic>            │
│  → 当所有子项关闭：以一个提交着陆到 main                    │
└─────────────────────────────────────────────────────────────┘
```

## "循环"意味着什么

**会话循环**：正常。Claude 重启，沙盒保留，Slot 保留。

```bash
gt handoff  # 会话循环，Polecat 继续
```

**沙盒修复**：复用时。`gt sling` 将 worktree 重置为新分支。

```bash
gt sling gt-xyz gastown  # 复用空闲的 Toast，修复 worktree
```

会话循环不断发生。沙盒修复在任务之间发生。

## 反模式

### 手动状态转换

**反模式：**
```bash
gt polecat done Toast    # 不要：外部状态操作
gt polecat reset Toast   # 不要：手动生命周期控制
```

**正确做法：**
```bash
# Polecat 自行发出完成信号：
gt done  # （从 Polecat 会话内部）

# 只有显式 nuke 才销毁 Polecat：
gt polecat nuke Toast  # （销毁沙盒，身份保留）
```

Polecat 管理自己的会话生命周期。外部操作会绕过验证。

### 没有工作的沙盒（Idle 与 Stalled）

空闲的 Polecat 没有 Hook 也没有会话——这是**正常的**。它完成了工作，正在等待下一次 `gt sling`。

**Stalled** 的 Polecat 有 Hook 但没有会话——这是**故障**：
- 会话崩溃且未被 Nudge 恢复
- Hook 在崩溃中丢失
- 状态损坏

**Stalled 的恢复：**
```bash
# Witness 在已有沙盒中重新生成会话
# 或者，如果无法恢复：
gt polecat nuke Toast        # 清理 stalled 的 Polecat
gt sling gt-abc gastown      # 用新的 Polecat respawn
```

### 混淆会话与沙盒

**反模式：** 认为会话重启 = 丢失工作。

```bash
# 会话结束（handoff、崩溃、压缩）
# 工作并未丢失，因为：
# - Git 提交保留在沙盒中
# - 暂存的变更保留在沙盒中
# - Molecule 状态保留在 beads 中
# - Hook 在会话之间持续存在
```

新会话通过 `gt prime` 从旧会话停止的地方继续。

## 会话生命周期详情

会话因以下原因循环：

| 触发 | 动作 | 结果 |
|------|------|------|
| `gt handoff` | 自愿 | 干净地循环到全新上下文 |
| 上下文压缩 | 自动 | 由 Claude Code 强制 |
| 崩溃/超时 | 故障 | Witness respawn |
| `gt done` | 完成 | 会话退出，Polecat 转为 idle |

除 `gt done` 外都导致继续工作。只有 `gt done` 发出完成信号并将 Polecat 转为 idle。

## Witness 职责

Witness 监控 Polecat 但不会：
- 强制会话循环（Polecat 通过 handoff 自我管理）
- 中途打断步骤（除非真的卡住了）
- 在完成后 nuke Polecat（持久模型）

Witness 会：
- 检测并 Nudge stalled 的 Polecat（意外停止的会话）
- 清理 zombie 的 Polecat（`gt done` 失败的会话）
- Respawn 崩溃的会话
- 处理来自 stuck Polecat 的升级请求（明确求助的 Polecat）

## Polecat 身份

**关键洞察**：Polecat 的*身份*是永久的；会话是临时的，沙盒是持久的。

在 HOP 模型中，每个实体都有一个链（CV）追踪：
- 它们做过什么工作
- 成功/失败率
- 展示的技能
- 质量指标

Polecat 的*名称*（Toast、Shadow 等）是池中的一个 Slot——持续存在直到显式 nuke。作为该 Polecat 执行的*Agent 身份*在所有任务中积累工作历史。

```
POLECAT 身份（永久）            会话（临时）         沙盒（持久）
├── CV 链                      ├── Claude 实例      ├── Git worktree
├── 工作历史                    ├── 上下文窗口       ├── 分支
├── 展示的技能                  └── 在 handoff 时    └── 在复用时
└── 工作功劳                       或 gt done 时         被 gt sling
                                       终止                修复
```

这个区别对以下场景很重要：
- **归属** — 谁获得工作的功劳？
- **技能路由** — 哪个 Agent 最适合这个任务？
- **成本核算** — 谁为推理付费？
- **联邦** — Agent 在分布式世界中拥有自己的链

## 实现状态

截至 2026-03-07（gt-o8g8 审计），所有核心生命周期操作已**交付并在生产环境中运行**。参见 [design/polecat-lifecycle-patrol.md § 10](../design/polecat-lifecycle-patrol.md#10-implementation-status-gt-o8g8-audit-2026-03-07) 获取完整实现矩阵，以及 [design/persistent-polecat-pool.md](../design/persistent-polecat-pool.md) 获取分阶段交付状态。

关键文件：
- `internal/cmd/done.go` — 工作提交、沙盒同步、idle 转换
- `internal/cmd/sling.go` + `polecat_spawn.go` — idle 复用、仅分支修复
- `internal/cmd/handoff.go` — 所有角色的会话循环
- `internal/witness/handlers.go` — 清理管道、POLECAT_DONE 路由、zombie/orphan 检测
- `internal/polecat/manager.go` — stale 检测、idle 复用（`FindIdlePolecat`、`ReuseIdlePolecat`）、池管理

## 相关文档

- [Overview](../overview.md) - 角色分类与架构
- [Molecules](molecules.md) - Molecule 执行与 Polecat 工作流
- [Propulsion Principle](propulsion-principle.md) - 为什么工作触发立即执行
- [Polecat Lifecycle Patrol](../design/polecat-lifecycle-patrol.md) - 实现细节、清理阶段、巡逻协调
- [Persistent Polecat Pool](../design/persistent-polecat-pool.md) - 池管理设计与交付状态