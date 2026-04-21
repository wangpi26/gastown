# Polecat 自管理完成

> **Bead:** gt-0wkk
> **日期:** 2026-02-28
> **作者:** rictus（gastown polecat）
> **状态:** 设计提案
> **相关:** gt-4ac（持久化 polecat 模型）、gt-a6gp（基于邮件的 nudge）、
> gt-6a9d（nuke 安全）、gt-w0br（基于 bead 的发现）

---

## 1. 问题陈述

Polecat 当前依赖 witness 完成其生命周期。当 polecat 运行 `gt done` 时，它执行大部分工作（推送分支、创建 MR bead、写入完成元数据、nudge witness），但随后**停下来等待 witness**：

1. 发现完成（通过巡逻扫描 agent bead）
2. 将 polecat 从 `agent_state=done` 转换到 `agent_state=idle`
3. 创建清理 wisp 来跟踪待处理的 MR
4. 向 Refinery 发送 `MERGE_READY`

Witness 是单线程的（一次一个巡逻周期），因此在高吞吐量时它成为瓶颈。僵尸 polecat 在 `done` 状态堆积，等待 witness 处理。这是对原始模型的回归——原始模型中 polecat 是完全自包含的。

### 用数字说明瓶颈

当 N 个 polecat 同时完成时：
- 每次 witness 巡逻周期需要 30-90 秒
- `survey-workers` 步骤顺序扫描所有 agent bead
- 每个周期只处理一个完成（创建 wisp、nudge refinery）
- N 个完成排队，需要 N * 巡逻周期时间 来处理

### 我们如何走到这一步

Witness 依赖通过两个好意的改动逐渐渗入：

1. **持久化 polecat 模型（gt-4ac）：** 保留沙箱以便复用，需要有人管理 idle→reuse 生命周期。Witness 成为那个人，因为它已经在监控 polecat。

2. **基于邮件的 nudge（gt-a6gp）：** 将完成发现从 polecat 发送的邮件转移到 witness 扫描 agent bead。这减少了 Dolt 压力（nudge 免费 vs 邮件创建 bead），但将发现集中在 witness 巡逻循环中。

两个改动都没有错。但它们共同创建了串行瓶颈，witness 成为每次完成的必需检查点。

---

## 2. 当前流程（今天发生什么）

```
Polecat 运行 gt done
    │
    ├── 1. 验证干净状态（无未提交的变更）
    ├── 2. 推送分支到 origin
    ├── 3. 创建 MR bead（类型：merge-request，标签：gt:merge-request）
    ├── 4. 写入完成元数据到 agent bead：
    │      exit_type, mr_id, branch, mr_failed, completion_time
    ├── 5. 设置 agent_state = "done"（不是 idle）
    ├── 6. 清除 hook_bead
    ├── 7. 通过 tmux nudge witness
    ├── 8. 同步 worktree 到 main，删除旧分支
    └── 9. 会话进入空闲（沙箱保留）
         │
         ▼
    ┌─── 等待 ─────────────────────────────────────────┐
    │ Polecat 处于 "done" 状态。                          │
    │ 在 witness 处理前不能接受新工作。                    │
    │ 如果 witness 忙碌：polecat 空闲等待数分钟。         │
    └───────────────────────────────────────────────────┘
         │
         ▼ （下一次 witness 巡逻周期）
Witness survey-workers 步骤
    │
    ├── 扫描所有 polecat agent bead
    ├── 发现 exit_type + completion_time 已设置
    ├── 如果有待处理 MR：
    │   ├── 创建清理 wisp（merge-requested 状态）
    │   ├── 向 refinery 发送 MERGE_READY
    │   └── 清除完成元数据
    ├── 转换 agent_state：done → idle
    └── Polecat 现在可用于新工作
```

**在 "done" 状态的时间：** 30 秒到数分钟，取决于 witness 巡逻周期时机和有多少其他 polecat 同时完成。

---

## 3. 提议的流程（自管理完成）

```
Polecat 运行 gt done
    │
    ├── 1. 验证干净状态（无未提交的变更）
    ├── 2. 推送分支到 origin
    ├── 3. 创建 MR bead（类型：merge-request，标签：gt:merge-request）
    ├── 4. 写入完成元数据到 agent bead（用于审计）
    ├── 5. 直接 nudge refinery："MERGE_READY <mr-id>"      ← 新增
    ├── 6. 设置 agent_state = "idle"                        ← 变更
    ├── 7. 清除 hook_bead
    ├── 8. 同步 worktree 到 main，删除旧分支
    └── 9. 会话进入空闲（沙箱保留）
              │
              └── Polecat 立即可用于新工作
```

**关键变更：**
1. Polecat 直接设置 `agent_state=idle`（不是 `done`）
2. Polecat 直接 nudge refinery（不经 witness 中继）
3. 不需要清理 wisp（见第 5 节）
4. Witness 不在关键路径上

### Witness 仍然做什么

Witness 角色**回归为观察者**——它巡逻发现异常，仅在有问题时干预：

| Witness 操作 | 时机 | 原因 |
|-------------|------|------|
| 僵尸检测 | 巡逻扫描 | 会话死亡但 agent_state=running |
| 卡住检测 | 巡逻扫描 | Hook 已设置但 30+ 分钟无进展 |
| 脏状态恢复 | 巡逻扫描 | 空闲 polecat 中有未提交的变更 |
| MR 失败恢复 | 巡逻扫描 | MR bead 有错误状态，无重试 |
| 升级中继 | 发现时 | 超出 polecat 自修复能力的问题 |

Witness 不需要：
- 处理每个成功的完成
- 向 Refinery 中继 MERGE_READY
- 为常规完成创建清理 wisp
- 将 agent_state 从 done→idle 转换

---

## 4. 详细设计

### 4.1 Polecat 自转换

当前，agent 状态转换在 polecat 和 witness 之间分配：

| 转换 | 当前拥有者 | 提议拥有者 |
|------|-----------|-----------|
| → working | Polecat（gt sling） | Polecat（无变更） |
| → done | Polecat（gt done） | **已移除**（跳到 idle） |
| done → idle | Witness（巡逻） | Polecat（gt done） |
| → stuck | Polecat（gt done --status=ESCALATED） | Polecat（无变更） |
| → running | Witness（重启） | Witness（无变更 — 安全网） |

**消除 "done" 状态：** 中间的 `done` 状态仅作为给 witness 的交接信号存在。有了自管理完成，polecat 直接从 `working` 转换到 `idle`。完成元数据（exit_type、mr_id 等）仍然保留在 agent bead 上用于审计目的。

### 4.2 直接 Refinery 通知

当前，witness 在发现完成时创建清理 wisp 并 nudge refinery。Polecat 可以直接做这件事：

```go
// 在 gt done 中，创建 MR bead 之后：
if mrID != "" {
    // 直接 nudge refinery（已实现，但当前
    // 仅作为 witness 通知旁边的备用方案）
    nudgeRefinery(rigName, fmt.Sprintf("MERGE_READY %s", mrID))
}
```

Refinery 已经通过**轮询 bead**发现 MR，查找开放的 merge-request issue（`ListReadyMRs()`）。Nudge 只是唤醒信号——即使错过，refinery 也会在下次巡逻周期找到 MR。这使得通知幂等且容损。

**Refinery 不依赖 witness 发现 MR。** 从 `engineer.go:1194-1252`，`ListReadyMRs()` 直接查询 bead：
```go
issues, err := e.beads.List(beads.ListOptions{
    Status:   "open",
    Label:    "gt:merge-request",
    Priority: -1,
})
```

所以 witness 中继一直是冗余的——refinery 自身的轮询才是真正的发现机制。Witness nudge 只是减少了延迟。

### 4.3 清理 Wisp 消除

清理 wisp（`merge-requested` 状态）是为了让 witness 能跟踪待处理 MR 并检测失败而引入的。有了自管理完成，这种跟踪变得不必要，因为：

1. **MR bead 是自跟踪的。** MR bead 有状态（open/closed）、retry_count、错误状态。Refinery 在处理时更新这些。

2. **失败检测转移到 refinery。** 如果合并失败，refinery 已经创建冲突解决任务。Witness 不需要 wisp 来发现这些。

3. **Witness 仍然可以检测异常** 通过扫描过期 MR bead（开放的 merge-request 比阈值老且无 refinery assignee）。这是基于发现的——不需要 wisp。

**迁移：** 现有清理 wisp 可以自然排空。Witness 巡逻的 `process-cleanups` 步骤变为无操作，迁移后可移除。

### 4.4 完成元数据保留

Agent bead 完成元数据（exit_type、mr_id、branch、completion_time）仍由 polecat 写入。这有两个目的：

1. **审计轨迹：** 账本精确显示每个 polecat 做了什么。
2. **异常检测：** Witness 可以在巡逻期间扫描异常模式（重复升级、MR 失败等）。

元数据不再用作交接信号。Witness 在巡逻中读取它是为了可观察性，而非用于操作路由。

### 4.5 `gt done` 中有什么变更

```diff
 func runDone(ctx context.Context, exitType ExitType, ...) error {
     // ... 验证、推送、MR 创建 ...

     if mrID != "" {
-        // Nudge witness（witness 中继到 refinery）
-        nudgeWitness(rigName, fmt.Sprintf("POLECAT_DONE %s exit=%s", name, exitType))
+        // 直接 nudge refinery（witness 不在关键路径上）
+        nudgeRefinery(rigName, fmt.Sprintf("MERGE_READY %s", mrID))
     }

-    // 设置 agent_state 为 "done"（witness 将转换为 idle）
-    setAgentState(agentBeadID, "done")
+    // 直接设置 agent_state 为 "idle"（自管理）
+    setAgentState(agentBeadID, "idle")

     // ... 清除 hook，同步 worktree ...
 }
```

### 4.6 Witness 巡逻中有什么变更

`survey-workers` 步骤简化：

```diff
 func surveyWorkers() {
     for _, polecat := range allPolecats {
-        // 检查完成（done 状态）
-        if polecat.AgentState == "done" && polecat.CompletionTime != "" {
-            handleDiscoveredCompletion(polecat)
-        }

         // 检查僵尸（死会话，agent 显示运行中）
         if polecat.AgentState == "running" && !isSessionAlive(polecat) {
             handleZombie(polecat)
         }

+        // 检查卡住的空闲 polecat（空闲但沙箱脏）
+        if polecat.AgentState == "idle" && hasDirtyState(polecat) {
+            handleDirtyIdle(polecat)
+        }
+
+        // 检查过期 MR（开放的 MR bead 无 refinery 认领）
+        if polecat.MRID != "" && isMRStale(polecat.MRID) {
+            handleStaleMR(polecat)
+        }
     }
 }
```

Witness 巡逻获得新的异常检测检查，但失去完成处理责任。净效果：更快的巡逻周期（无 wisp 创建、无 refinery nudging）与更好的异常覆盖。

---

## 5. 边缘情况与故障模式

### 5.1 Polecat 在 `gt done` 期间崩溃

**当前：** Witness 检测到 `done-intent` 标签 + 活跃会话 = stuck-in-done。Witness 终止会话并继续清理管道。

**提议：** 相同机制。`done-intent` 标签在 `gt done` 开始时设置（在任何状态变更之前）。如果 polecat 在 mid-done 崩溃：
- Agent 状态仍为 `working`（尚未转换为 idle）
- `done-intent` 标签已设置
- Witness 僵尸检测发现：死会话 + done-intent = 在 done 中崩溃
- Witness 重启会话（重启优先策略，gt-dsgp）
- 新会话发现 done-intent，恢复 `gt done`

**无需变更。** done-intent 安全机制独立于谁管理 idle 转换。

### 5.2 Polecat 设置 Idle 但推送失败

**当前：** 不可能——推送在 witness 处理之前发生。

**提议：** 相同。推送发生在 `gt done` 早期，在 idle 转换之前。如果推送失败，`gt done` 报错，polecat 保持 `working` 状态。Witness 将此检测为僵尸（死会话但 agent_state=working）并重启。

### 5.3 Refinery 错过 Nudge

**当前：** Refinery 独立轮询 MR。Nudge 是延迟优化。

**提议：** 相同。无论 nudge 来自 witness 还是 polecat，refinery 的轮询（`ListReadyMRs`）是可靠的发现机制。错过的 nudge 最多增加一个巡逻周期的延迟。

### 5.4 两个 Polecat 同时完成

**当前：** Witness 顺序处理（串行瓶颈）。

**提议：** 每个 polecat 独立转换到 idle 并 nudge refinery。无序列化。Refinery 从其队列处理 MR（已由合并槽位序列化）。这是主要的吞吐量改进。

### 5.5 Witness 宕机

**当前：** 完成排队为 `done` 状态 polecat。当 witness 恢复时，它排空队列。Polecat 在中断期间不可用。

**提议：** Polecat 自转换到 idle 并直接 nudge refinery。Witness 宕机对常规完成**零影响**。Witness 仅在异常恢复（僵尸、脏状态）时需要，这些可以等待。

---

## 6. 迁移策略

### 阶段 1：双信号（低风险）

在 `gt done` 中添加直接 refinery nudge，与现有 witness 通知并存。Polecat 仍设置 `agent_state=done`（witness 仍处理）。

```go
// gt done 发送两个信号
nudgeWitness(rigName, fmt.Sprintf("POLECAT_DONE %s", name))
nudgeRefinery(rigName, fmt.Sprintf("MERGE_READY %s", mrID))  // 新增
```

**验证：** 验证 refinery 从两个信号源处理 MR。无行为变更——只是冗余。

### 阶段 2：自转换（中等风险）

Polecat 直接设置 `agent_state=idle`。Witness 巡逻跳过完成处理（没有 `done` 状态需要发现）。Witness nudge 变为可选。

```go
// gt done：自管理
setAgentState(agentBeadID, "idle")
nudgeRefinery(rigName, fmt.Sprintf("MERGE_READY %s", mrID))
// Witness nudge：可选，仅用于可观察性
```

**验证：** 验证 polecat 立即可用于新工作。验证 witness 巡逻在没有 `done` 状态 polecat 时不中断。

### 阶段 3：清理（低风险）

移除 witness 完成处理代码：
- 移除 `DiscoverCompletions()` 函数
- 移除 `handleDiscoveredCompletion()` 函数
- 移除常规完成的清理 wisp 创建
- 移除 `process-cleanups` 巡逻步骤（或重新用于异常 wisp）
- 更新 `mol-witness-patrol.formula.toml` 移除完成引用

**验证：** 完整巡逻周期测试。验证僵尸检测仍然有效。

### 回滚

在每个阶段，回滚是简单的：
- 阶段 1：移除额外的 nudge 行
- 阶段 2：恢复 `agent_state=done` 并重新启用 witness 处理
- 阶段 3：重新添加 witness 完成代码

---

## 7. 影响评估

### 吞吐量

| 指标 | 当前 | 提议 |
|------|------|------|
| 完成延迟 | 30s-3min（witness 周期） | ~0s（立即） |
| 并发完成 | 串行（每周期 1 个） | 并行（无限） |
| Witness 巡逻时间 | 30-90s（处理完成） | 10-30s（仅异常扫描） |
| Polecat 空闲时间 | 等待数分钟 | 零等待 |

### Dolt 压力

无变更 — 两个流程都使用 nudge（免费）和直接 bead 写入。

### 健壮性

**改进：** 从关键路径移除单点故障（witness）。即使 witness 宕机、重启或缓慢，常规完成也能成功。

**保留：** Witness 仍为边缘情况（僵尸、脏状态、过期 MR）提供安全网。"发现而非跟踪"原则得到维护。

### 复杂性

**降低：** 消除了清理 wisp、完成发现代码和 witness 中的 done→idle 状态机。`gt done` 命令成为完成生命周期的单一权威来源。

---

## 8. 与设计原则的一致性

| 原则 | 此设计如何一致 |
|------|---------------|
| **GUPP** | Polecat 更快地可用于新工作 → 更高吞吐量 |
| **ZFC** | Polecat 自报告空闲（已经做了 cleanup_status）。Witness 按异常验证 |
| **发现而非跟踪** | Witness 通过扫描状态发现异常，而非通过处理事件 |
| **优先自回收** | 来自 polecat-lifecycle-patrol.md Q2："优先显式自回收。仅将机械干预用作安全网。"此设计实现了该声明 |
| **持久化 polecat 模型** | 完全兼容 — 沙箱保留和身份持久不变 |

### gt-4ac 被遗漏的含义

持久化 polecat 模型（gt-4ac）旨在让 polecat 存活并复用。但 witness 被插入为 idle 转换的守门人，抵消了部分好处。一个完成工作但因为 witness 尚未处理而 3 分钟不能接受新工作的 polecat，实际上是闲置产能。

此设计完成了 gt-4ac 的承诺：自管理完整生命周期的持久化 polecat，Witness 作为安全网而非必需的检查点。

---

## 9. 总结

**核心洞察：** 常规完成的 witness 中继是冗余的。Refinery 已经通过轮询 bead 发现 MR。Polecat 已经写入所有元数据。Witness 仅在异常检测时需要——它可以扫描状态来完成，而非处理每次完成。

**三个变更：**
1. Polecat 直接设置 `agent_state=idle`（跳过 `done` 中间态）
2. Polecat 直接 nudge refinery（跳过 witness 中继）
3. Witness 移除完成处理代码（巡逻聚焦于异常）

**结果：** 完成延迟从分钟降至零。Witness 回归其设计的观察者角色。系统随 polecat 数量线性扩展，而非受限于单线程巡逻循环。

---

*"优先自回收。机械干预是安全网，而非主要机制。" — polecat-lifecycle-patrol.md，Q2*