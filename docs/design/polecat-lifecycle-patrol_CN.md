# Polecat 生命周期与巡逻协调

> **Bead:** gt-t6muy
> **日期:** 2026-02-20
> **作者:** capable（gastown polecat）
> **状态:** 已实现 — 核心生命周期已发布，分支清理已发布，通知 mayor 待定
> **更新:** 2026-03-07（gt-o8g8 实现审计，由 bear 执行）
> **相关:** gt-dtw9u（Witness 监控）、gt-qpwv4（完成检测）、
> gt-6qyt1（Refinery 队列）、gt-budeb（自动 nuke）、gt-5j3ia（Swarm 聚合）、
> gt-1dbcp（Polecat 自动启动）、w-gt-004（荒地生命周期项）

---

## 1. 概述

本文档形式化了 Deacon、Witness、Refinery 和 Polecat 如何协调工作，使其通过 Gas Town 推进系统。它捕获了每步一会话模型，定义了两个清理阶段，设计了按 rig 划分的生命周期通道，并解决了关于步骤粒度、回收和生成的开放设计问题。

**核心洞察：** Polecat 不会端到端完成复杂的 Molecule。相反，每个 Molecule 步骤获得一个 polecat 会话。沙箱（分支、worktree）跨会话持久存在。会话是活塞；沙箱是气缸。

---

## 2. 每步一会话模型

### 2.1 接力赛

参见 [concepts/polecat-lifecycle.md](../concepts/polecat-lifecycle.md) 了解接力赛模型。

### 2.2 会话轮换 vs 步骤轮换

这是不同的概念：

| 概念 | 触发 | 什么改变 | 什么持久 |
|------|------|----------|----------|
| **会话轮换** | Handoff、compaction、崩溃 | Claude 上下文窗口 | 分支、worktree、Molecule 状态 |
| **步骤轮换** | 步骤 bead 关闭 | 当前步骤焦点 | 分支、worktree、剩余步骤 |

单个步骤可能跨越多个会话轮换（如果步骤复杂或发生 compaction）。多个步骤可能容纳在单个会话中（如果步骤较小且上下文允许）。每步一会话模型是设计目标，而非硬性约束。

### 2.3 会话何时轮换

| 触发 | 谁发起 | 发生什么 |
|------|--------|----------|
| 步骤完成 | Polecat | `bd close <step>` 然后 `gt handoff` 进入下一步 |
| 上下文填满 | Claude Code | 自动 compaction；PreCompact hook 保存状态 |
| 崩溃/超时 | 基础设施 | Witness 检测，重新生成会话 |
| `gt done` | Polecat | 最终步骤；提交到合并队列，进入空闲（沙箱保留） |

### 2.4 状态连续性

在会话之间，状态通过以下方式保留：

- **Git 状态：** 提交、暂存变更、分支位置
- **Beads 状态：** Molecule 进度（哪些步骤已关闭）
- **Hook 状态：** agent bead 上的 `hook_bead` 跨会话持久
- **Agent bead：** `agent_state`、`cleanup_status`、`hook_bead` 字段

新会话通过以下方式发现其位置：

```bash
gt prime --hook    # 加载角色上下文，读取 hook
bd mol current     # 发现下一步是哪个步骤
bd show <step-id>  # 读取步骤指令
```

不需要显式的 "handoff 负载"。Beads 状态本身就是 handoff。

---

## 3. 两个清理阶段

### 3.1 步骤清理（会话消亡，沙箱存活）

当步骤完成但 Molecule 中还有更多步骤时触发。

| 操作 | 结果 |
|------|------|
| 关闭步骤 bead | `bd close <step-id>` |
| 会话轮换 | `gt handoff`（主动）或崩溃恢复 |
| 沙箱持久 | 分支、worktree、未提交的工作全部存活 |
| Molecule 持久 | 剩余步骤仍然开放，hook 仍然设置 |
| 身份持久 | Agent bead 不变，CV 累积 |

**谁处理：**
- Polecat 通过 `gt handoff` 发起
- Witness 在崩溃时重新生成（通过 `SessionManager.Start`）
- Daemon 在会话死亡时触发（`LIFECYCLE:Shutdown` → witness）

### 3.2 Molecule 清理（Polecat 进入空闲）

当 Molecule 的最终步骤完成且工作被提交时触发。

| 操作 | 结果 |
|------|------|
| Polecat 运行 `gt done` | 推送分支，提交 MR，设置 `cleanup_status=clean` |
| Polecat 设置 agent 状态 | `agent_state=idle`，`hook_bead` 清除 |
| Polecat 终止会话 | 会话终止，沙箱保留 |
| Witness 收到 `POLECAT_DONE` | 确认空闲转换 |
| Refinery 合并 | Squash-merge 到 main，关闭 MR 和源 issue |
| 身份存活 | Agent bead 仍然存在；CV 链有新条目；polecat 准备复用 |

```
步骤清理（中间）                 Molecule 清理（最终）
┌────────────────────┐               ┌────────────────────────────┐
│ 步骤 bead: 已关闭  │               │ 所有步骤 bead: 已关闭       │
│ 会话: 已终止       │               │ 会话: 已终止               │
│ 沙箱: 存活         │               │ 沙箱: 保留（空闲）          │
│ Molecule: 活跃     │               │ Molecule: 已压缩            │
│ Hook: 已设置       │               │ Hook: 已清除                │
│ Agent bead: working│               │ Agent bead: 已 nuke         │
│ 分支: 存活         │               │ 分支: 已推送（空闲）         │
└────────────────────┘               └────────────────────────────┘
```

### 3.3 清理管道

清理管道是交接链，而非单一操作：

```
Polecat 调用 gt done
    │
    ├── 在 agent bead 上设置 cleanup_status=clean
    ├── 推送分支到 origin
    ├── 创建 MR bead（标签：gt:merge-request）
    ├── 发送 POLECAT_DONE 邮件给 witness
    └── 会话退出
         │
         ▼
Witness 收到 POLECAT_DONE
    │
    ├── 检查 cleanup_status（ZFC：信任 polecat 自报告）
    ├── 如果 clean → 发送 MERGE_READY 给 refinery
    ├── 如果 dirty → 创建清理 wisp（不能自动 nuke）
    └── 通知 refinery 会话
         │
         ▼
Refinery 处理 MERGE_READY
    │
    ├── 认领 MR（设置 assignee）
    ├── 获取合并槽位（序列化推送锁）
    ├── 运行质量门控
    ├── Squash-merge 到 main
    ├── 关闭 MR bead 和源 issue
    ├── 发送 MERGED 邮件给 witness
    └── 释放合并槽位
         │
         ▼
Witness 收到 MERGED
    │
    ├── 验证提交在 main 上（所有远程）
    ├── 检查 cleanup_status
    ├── 确认合并（polecat 已空闲，沙箱已保留）
    └── 如果 dirty → 警告（合并后不应该发生）
```

### 3.4 清理管道中的故障恢复

每个阶段可能独立失败。恢复由下一次巡逻周期处理：

| 故障 | 检测 | 恢复 |
|------|------|------|
| `gt done` 中途执行失败 | 僵尸状态：会话存活，done-intent 标签 | Witness `DetectZombiePolecats()` 发现 stuck-in-done，恢复 |
| `POLECAT_DONE` 邮件丢失 | Witness 巡逻：发现死会话带有 `hook_bead` | `DetectZombiePolecats()` 检测到 agent-dead-in-session |
| 合并冲突 | Refinery `doMerge()` 检测到 | 创建冲突解决任务，阻塞 MR |
| `MERGED` 邮件丢失 | Refinery 已关闭 bead；witness 巡逻发现关闭的 bead 仍有活跃会话 | `DetectZombiePolecats()` bead-closed-still-running |
| Nuke 失败 | 尝试 kill 后会话仍在运行 | 下次巡逻检测到僵尸，重试 nuke |

---

## 4. 按 Rig 划分的 Polecat 通道

### 4.1 设计决策：基于邮件的通道

按 rig 划分的 polecat 通道使用现有的 `gt mail` 系统实现。选择此方案而非基于 beads 的队列或状态文件，原因如下：

1. **一致性：** 邮件已经是所有 Gas Town agent 的协调原语
2. **持久性：** 消息在进程崩溃和会话轮换后存活
3. **路由：** 邮件地址（`gastown/witness`）已映射到 rig 级 agent
4. **审计轨迹：** 邮件创建 beads 条目（可观察、可发现）
5. **无需新基础设施：** 不需要新的 Dolt 表，不需要基于文件的队列

### 4.2 通道地址

每个 rig 通过现有邮件路由拥有隐式生命周期通道：

| 通道 | 地址 | 用途 | 服务者 |
|------|------|------|--------|
| Polecat 生命周期 | `<rig>/witness` | 回收、nuke、健康请求 | Witness 巡逻 |
| 合并队列 | `<rig>/refinery` | MERGE_READY、冲突报告 | Refinery 巡逻 |
| Rig 协调 | `<rig>/witness` | 生成请求、升级 | Witness |
| Town 协调 | `mayor/` | 跨 rig、战略性 | Mayor |

### 4.3 生命周期消息协议

Polecat 生命周期通道中的消息遵循现有的 witness 协议（`protocol.go`）：

| 主题模式 | 类型 | 发送者 | 操作 |
|----------|------|--------|------|
| `POLECAT_DONE <name>` | 完成 | Polecat | 验证干净，转发给 refinery |
| `LIFECYCLE:Shutdown <name>` | 外部关闭 | Daemon | 自动 nuke 或清理 wisp |
| `LIFECYCLE:Cycle <name>` | 会话重启 | Daemon | 终止并重启会话 |
| `HELP: <topic>` | 升级 | Polecat | Witness 评估，必要时转发 |
| `MERGED <id>` | 合并后 | Refinery | Nuke polecat 沙箱 |
| `MERGE_FAILED <id>` | 合并失败 | Refinery | 通知 polecat，需要返工 |
| `RECOVERED_BEAD <id>` | 孤儿恢复 | Witness | Deacon 重新分派工作 |
| `GUPP_VIOLATION: <name>` | 停滞检测 | Daemon | Witness 调查 |
| `ORPHANED_WORK: <name>` | 死会话 + 工作 | Daemon | Witness 恢复或 nuke |

### 4.4 通道处理

Witness 在巡逻周期中处理其通道。处理在每个周期内按先到先服务顺序。巡逻模式：

```
Witness 巡逻周期：
    │
    ├── 1. 检查收件箱（gt mail inbox）
    │   └── 按顺序处理生命周期消息
    │
    ├── 2. 检测僵尸 polecat
    │   └── 对每个僵尸：nuke 或升级
    │
    ├── 3. 检测孤儿 bead
    │   └── 对每个孤儿：重置状态，邮件通知 deacon
    │
    ├── 4. 检测停滞的 polecat
    │   └── 对每个停滞：nudge 或升级
    │
    ├── 5. 检查待处理的生成
    │   └── 处理来自 daemon 的生成请求
    │
    └── 6. 写入巡逻回执
        └── 机器可读的发现摘要
```

### 4.5 谁服务通道

Witness 是主要消费者，但设计支持其他巡逻 agent 的机会性服务：

| Agent | 何时服务 | 能做什么 |
|-------|----------|----------|
| **Witness** | 每次巡逻周期 | 完整生命周期：生成、nuke、升级 |
| **Deacon** | Rig 级巡逻期间 | 检测未服务的请求，nudge witness |
| **Daemon** | 每次心跳 tick | 检测死会话，发送 LIFECYCLE 消息 |
| **Refinery** | 合并处理期间 | 发送 MERGED/MERGE_FAILED 给 witness |

这创造了冗余监控：如果 witness 遗漏了一条消息，deacon 或 daemon 会检测到结果状态（死会话、孤儿 bead）并直接处理或 nudge witness。

---

## 5. GUPP + 固定工作 = 完成保证

### 5.1 完成不变式

只要三个条件成立，一个 Molecule 最终将完成：

1. **工作已固定**（agent bead 上的 `hook_bead` 已设置）
2. **沙箱持久**（分支 + worktree 存在）
3. **有人持续生成会话**（崩溃时 witness 重启）

GUPP 确保当会话以 hook 启动时，它会执行。Hook 跨会话轮换持久。沙箱提供连续性。Witness 提供复活。这些一起保证了最终完成。

### 5.2 完成循环

```
┌─────────────────────────────────────────────┐
│              完成循环                         │
│                                              │
│   会话生成 → gt prime → 发现 hook            │
│        │                                     │
│        ▼                                     │
│   GUPP 触发 → 执行当前步骤                    │
│        │                                     │
│        ▼                                     │
│   步骤完成 → bd close → handoff              │
│        │                                     │
│        ▼                                     │
│   还有更多步骤？ ──是──▶ 重新生成会话 ──┐     │
│        │                                 │     │
│        否                                │     │
│        │                                 │     │
│        ▼                                 │     │
│   gt done → 合并 → nuke                   │     │
│                                          │     │
│   会话崩溃？ ──▶ Witness 重新生成 ─────────┘     │
│                                              │
└─────────────────────────────────────────────┘
```

### 5.3 什么会破坏保证

| 故障 | 影响 | 恢复 |
|------|------|------|
| Witness 宕机 | 崩溃时无重启 | Deacon 检测，重启 witness |
| 沙箱损坏 | 分支或 worktree 损坏 | `RepairWorktree()` 或 nuke 并重新生成 |
| Hook 意外清除 | GUPP 不触发 | Witness `DetectOrphanedBeads()` 发现进行中的 bead，重置以重新分派 |
| Dolt 服务器宕机 | 无法读取 beads 状态 | Daemon 自动重启 Dolt；polecat 重试 |
| 崩溃循环（3+ 次崩溃） | 同一步骤不断失败 | Witness 升级到 mayor；提交为 bug |

### 5.4 活跃性 vs 安全性

系统优先考虑 **活跃性**（工作最终完成）而非严格安全性（无重复工作）。这意味着：

- **重复检测是尽力而为。** 如果两个会话以某种方式运行同一步骤，git 分支序列化写入，其中一个将推送失败。
- **优先选择幂等操作。** 关闭已关闭的 bead 是无操作。推送已推送的分支是安全的。
- **崩溃恢复可能重新执行部分工作。** 中途崩溃的步骤将从头重新执行。Git 状态有帮助：如果已提交，新会话能看到它们。

---

## 6. 巡逻协调

### 6.1 四个巡逻 Agent

Gas Town 有四个执行巡逻（周期性健康监控）的 agent：

| Agent | 范围 | 频率 | 关键检查 |
|-------|------|------|----------|
| **Daemon** | Town 级 | 3 分钟心跳 | 会话活跃性、GUPP 违规、孤儿工作 |
| **Boot/Deacon** | Town 级 | 每次 daemon tick | Deacon 健康、witness 健康、跨 rig 问题 |
| **Witness** | 按 rig | 持续 | Polecat 健康、僵尸检测、完成处理 |
| **Refinery** | 按 rig | 按需 | 合并队列处理、冲突检测 |

### 6.2 巡逻重叠作为弹性

多个 agent 观察重叠状态是有意的冗余：

```
               Daemon                          Deacon
           （机械式）                      （智能式）
                │                               │
    ┌───────────┼───────────┐       ┌──────────┼──────────┐
    │           │           │       │          │          │
 会话        GUPP         孤儿    Witness   Refinery    跨 rig
 活跃性      违规         工作    健康      健康        convoy
    │           │           │       │          │
    └───────────┤           │       │          │
                │           │       │          │
                ▼           ▼       ▼          ▼
              Witness               Witness    Refinery
           （按 rig 巡逻）      （响应）    （响应）
                │
    ┌───────────┼───────────┐
    │           │           │
 僵尸        孤儿         停滞的
 检测        bead         polecat
```

**关键属性：** 如果任何单个巡逻 agent 失败，其他 agent 会检测到结果的状态退化并进行补偿。Daemon 检测死会话。Deacon 检测死 witness。Witness 检测死 polecat。

### 6.3 巡逻 Agent 之间的信息流

```
Daemon ───LIFECYCLE:──────▶ Witness 收件箱
Daemon ───GUPP_VIOLATION:─▶ Witness 收件箱
Daemon ───ORPHANED_WORK:──▶ Witness 收件箱

Deacon ◀──heartbeat.json──── Daemon
Deacon ───nudge────────────▶ Witness（如果过期）
Deacon ───nudge────────────▶ Refinery（如果过期）

Witness ──MERGE_READY:────▶ Refinery 收件箱
Witness ──RECOVERED_BEAD:─▶ Deacon（用于重新分派）
Witness ──巡逻回执──────────▶ Beads（审计轨迹）

Refinery ─MERGED:─────────▶ Witness 收件箱
Refinery ─MERGE_FAILED:───▶ Witness 收件箱
Refinery ─convoy check─────▶ Deacon（用于搁浅的 convoy）
```

### 6.4 收敛状态

所有巡逻 agent 收敛到相同的可观察状态：beads（通过 Dolt）、git（通过分支和 worktree）以及 tmux（通过会话活跃性）。没有 agent 维护其他 agent 依赖的私有状态。这是"发现而非跟踪"原则在监控中的应用。

如果状态分化（例如，消息丢失），下一次巡逻周期从可观察量重新派生状态并自我修复。

---

## 7. 已解决的设计问题

### Q1：喂养式与步骤粒度

**问题：** 每个物理 Molecule 步骤有多少逻辑步骤？每个 polecat 会话有多少步骤？

**答案：** 使用 Formula 定义粒度，让上下文压力决定会话边界。

**步骤粒度指南：**

| 步骤类型 | 粒度 | 示例 |
|----------|------|------|
| 设置/清理 | 一个物理步骤 | "设置工作分支" |
| 实现 | 每个逻辑单元一个 | "实现解决方案"（可能跨会话） |
| 验证 | 每种检查类型一个 | "运行质量检查"、"自查" |
| 交接 | 每个生命周期事件一个 | "提交变更"、"提交工作" |

`mol-polecat-work` Formula 目前使用 10 个步骤。这对大多数工作是合适的，因为：

- 每个步骤有清晰的进入/退出标准
- 步骤可独立恢复（步骤中途崩溃最多丢失一个步骤的工作）
- 上下文保持聚焦（一个步骤的指令，不是整个 Molecule）

**每步一会话是指导原则，而非规则。** 如果上下文允许，Polecat 可能在一次会话中完成多个步骤。关键约束是每个步骤单独关闭（不批量关闭——批量关闭异端）。

**反模式：**
- 步骤太小，仅仅是 `git add` 命令（开销超过价值）
- 步骤太大，耗尽上下文（实现+测试+审查在一个步骤中）
- 步骤不能独立恢复（步骤 3 需要步骤 2 的上下文窗口）

### Q2：机械式 vs Agent 驱动的回收

**问题：** 何时适合机械干预（daemon 驱动）vs agent 驱动（polecat 请求自己的回收）？

**答案：** 优先选择显式自回收。仅将机械干预用作安全网。

**光谱：**

```
AGENT 驱动（优先）                    机械式（安全网）
├── gt done（polecat 进入空闲）       ├── Daemon 检测死会话
├── gt handoff（polecat 自轮换）     ├── Daemon 检测 GUPP 违规
├── gt escalate（polecat 请求帮助）   ├── Witness 僵尸扫描
└── HELP 邮件（polecat 发信号）       └── Deacon 在心跳过期时重启
```

**设计原则：** Polecat 是自身状态的权威。外部干预仅应在 polecat 无法为自己发声时发生（死会话、挂起进程、stuck-in-done）。

**具体阈值（agent 确定，非硬编码）：**

Daemon 使用宽泛的阈值进行安全网检测：
- **GUPP 违规：** 30 分钟有 `hook_bead` 但无进展
- **挂起会话：** 30 分钟无 tmux 输出（`HungSessionThresholdMinutes`）
- **Stuck-in-done：** 60 秒带 `done-intent` 标签

这些阈值有意宽泛。目标是捕获真正卡住的 polecat，而非深度思考中的 polecat。误报（"Deacon 屠杀潮" bug）比慢检测更糟糕。

**屠杀潮教训：** 机械式检测"卡住"是脆弱的，因为区分"深度思考"与"挂起"需要智能。这就是 Boot 存在的原因（智能分诊），也是 Daemon 阈值保守的原因。只有 Witness（AI agent）应该对 polecat 是否真正卡住做出判断。

### Q3：通道实现

**问题：** 基于邮件、基于 beads、还是状态文件？

**答案：** 基于邮件。完整设计见[第 4 节](#4-per-rig-polecat-channel)。

**为什么不是基于 beads（特殊 issue 类型）？**
- Beads issue 是持久的工作产物。生命周期请求是临时信号。
- 为"回收我"创建/关闭 beads 会增加不必要的 Dolt 写入压力。
- 邮件已是协调原语，具有正确的生命周期（读取 → 处理 → 删除）。

**为什么不是状态文件（rig/polecat-queue.json）？**
- 状态文件需要显式锁定以支持并发访问。
- 无审计轨迹（文件被覆盖）。
- 不与现有巡逻模式集成（agent 已经检查邮件）。
- 崩溃后恢复更难（部分写入的 JSON）。

### Q4：谁生成下一步？

**问题：** Polecat 完成步骤并交接后，谁生成下一个会话来继续 Molecule？

**答案：** Witness，由交接检测或 daemon 生命周期请求触发。

**生成链：**

```
Polecat 完成步骤
    │
    ├── 关闭步骤 bead
    ├── 调用 gt handoff（创建 handoff 邮件）
    └── 会话退出
         │
         ▼
Daemon 心跳 tick
    │
    ├── 检测死 polecat 会话
    ├── 发现 hook_bead 仍然设置（工作未完成）
    └── 触发会话重启
         │
         ▼
SessionManager.Start()
    │
    ├── 在现有 worktree 中创建新 tmux 会话
    ├── 注入环境变量（GT_POLECAT, GT_RIG）
    ├── SessionStart hook 触发：gt prime --hook
    └── 新会话通过 bd mol current 发现下一步
```

**当前实现：** Daemon 的 `processLifecycleRequests()` 处理此流程。当会话死亡但 hook 仍然设置时，daemon 要么发送 `LIFECYCLE:` 消息给 witness，要么直接重启会话（取决于配置）。Polecat 启动由 GUPP/beacon 流程端到端处理（SessionManager → StartupNudge → BuildStartupPrompt → SessionStart hook → gt prime）。

**未来（AT 集成）：** Witness 通过 `Teammate({ operation: "spawn" })` 直接生成替代队友。SubagentStop hook 检测队友死亡并触发重新生成。详情见 `docs/design/witness-at-team-lead.md`。

---

## 8. 边缘情况与故障模式

### 8.1 Stuck-in-Done 僵尸

Polecat 运行 `gt done` 但会话在清理完成前挂起。

**检测：** Witness `DetectZombiePolecats()` 检查超过 60 秒的 `done-intent` 标签且有活跃会话。

**恢复：** Witness 终止会话并继续清理管道（验证 `cleanup_status`，如果 MR 存在则转发给 refinery）。

### 8.2 孤儿沙箱

Polecat 目录存在但没有 tmux 会话且没有 `hook_bead`。

**检测：** `Manager.ReconcilePool()` 发现没有会话的目录。`DetectStalePolecats()` 识别远落后于 main 且无工作的沙箱。

**恢复：** 如果没有未提交的工作且没有活跃 MR，nuke 沙箱。如果有未提交的工作，升级（需要有人决定工作是否重要）。

### 8.3 脑裂合并

Refinery 开始合并而 Polecat 仍在推送。

**预防：** Agent bead 上的 `cleanup_status=clean` 字段对此进行序列化。Witness 仅在验证 polecat 已退出且分支干净后发送 `MERGE_READY`。合并槽位提供额外的序列化。

### 8.4 无限循环

某步骤不断失败，会话不断重启。

**检测：** 跟踪每个 polecat 的崩溃计数（通过 `ReconcilePool` 或临时状态）。同一步骤崩溃三次触发升级。

**恢复：** Witness 停止重新生成，创建 bug bead，邮件通知 mayor。Molecule 保持当前状态（bug 修复后可恢复）。

### 8.5 同一 Issue 上的并发 Polecat

不应该发生，因为 hook 是独占的（每个 agent bead 一个 `hook_bead`，每个 polecat 名称一个 agent bead）。但如果发生了：

**预防：** Git 分支命名包含唯一后缀（`@<timestamp>`）。`DetectZombiePolecats()` 中的 TOCTOU 守卫（记录 `detectedAt`，在破坏性操作前重新验证）防止检测与操作之间的竞争。

**恢复：** 第二个会话推送失败（分支分化）并升级。

---

## 9. 未来：AT 集成影响

Agent Teams (AT) 集成（见 `docs/design/witness-at-team-lead.md`）改变传输层但保留生命周期模型：

| 方面 | 当前（tmux） | 未来（AT） |
|------|-------------|-----------|
| 会话管理 | tmux 会话 | AT 队友 |
| 生成 | `SessionManager.Start()` | `Teammate({ operation: "spawn" })` |
| 健康监控 | tmux 活跃性 + pane 输出 | AT 生命周期 hook（SubagentStop） |
| 消息传递 | `gt nudge`（tmux send-keys） | AT 消息 |
| 清理 | 会话终止（沙箱保留） | `Teammate({ operation: "requestShutdown" })`（沙箱保留） |

**保持不变：**
- Beads 作为持久账本
- Molecule 作为工作流模板
- `gt done` 作为 polecat 空闲信号
- 两阶段清理（步骤 vs Molecule）
- 邮件用于跨 rig 通信
- 完成保证（GUPP + 固定工作 + 重启）

**变化：**
- Witness 成为 AT 团队负责人（委托模式）
- 僵尸检测变为结构化（hook vs 轮询）
- Polecat 间隔离由 hook 强制执行，非 tmux 强制
- 实时协调从 tmux 迁移到 AT（临时），减少 Dolt 压力

---

## 10. 实现状态（gt-o8g8 审计，2026-03-07）

### 已发布

所有核心生命周期操作已实现并在生产环境运行：

| 操作 | 命令/组件 | 关键实现 |
|------|-----------|----------|
| 生成/分配 | `gt sling` | `sling.go`、`polecat_spawn.go` — 找到空闲 polecat 或分配新槽位 |
| 工作执行 | `gt prime --hook` | 会话通过 `bd mol current` 发现 hook，GUPP 触发 |
| 会话轮换 | `gt handoff` | `handoff.go` — 所有角色，保留沙箱和身份 |
| 步骤完成 | `bd close` + `gt handoff` | 步骤清理：会话消亡，沙箱存活 |
| 工作提交 | `gt done` | `done.go` — 推送、MR、沙箱同步、设为空闲 |
| 空闲 polecat 复用 | `gt sling` | `polecat/manager.go`：`FindIdlePolecat()` + `ReuseIdlePolecat()` — 仅分支修复 |
| 僵尸检测 | Witness 巡逻 | `witness/handlers.go`：`DetectZombiePolecats()` — 重启优先，不自动 nuke |
| 过期检测 | Witness 巡逻 | `polecat/manager.go`：`DetectStalePolecats()` — 基于 tmux，保护暂停状态 |
| 孤儿恢复 | Witness 巡逻 | `witness/handlers.go`：`DetectOrphanedBeads()` — 重置并重新分派 |
| 清理管道 | 基于邮件 | POLECAT_DONE → Witness → MERGE_READY → Refinery → MERGED |
| 合并队列 | Refinery | Squash-merge，关闭 MR 和 issue，convoy check |

### 待定

| 特性 | 描述 | 影响 |
|------|------|------|
| Refinery 合并后通知 mayor | PR #2436/#2437 已关闭；分支清理已发布，通知 mayor 尚未 | 解锁依赖工作分派 |

### 已推迟（仅设计）

| 特性 | 推迟原因 |
|------|----------|
| 池大小强制 | 按需分配有效；固定池是优化，非正确性 |
| `gt polecat pool init` | Polecat 由第一次 `gt sling` 自然创建；预分配不必要 |
| `ReconcilePool()` | Witness 巡逻已通过僵尸/过期/孤儿检查检测状态漂移 |

---

## 11. 总结

参见 [concepts/polecat-lifecycle.md](../concepts/polecat-lifecycle.md) 了解完整的生命周期模型（三层、四状态、持久化 polecat 设计）。本文档覆盖实现细节：清理阶段、邮件通道、巡逻协调和边缘情况处理。