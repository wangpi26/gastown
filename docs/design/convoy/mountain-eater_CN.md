# Mountain-Eater：自主 Epic 碾磨

> Convoy 驱动的 epic 执行判断层。
>
> **状态**：设计
> **依赖**：Convoy Milestone 0-2（ConvoyManager、stage-launch）
> **相关**：[roadmap.md](roadmap.md) | [spec.md](spec.md) | [swarm-architecture.md](../../../docs/swarm-architecture.md)

---

## 1. 问题陈述

Gas Town 拥有自主 epic 执行的所有组件：
- ConvoyManager 在阻塞依赖关闭时分派就绪 issue（事件驱动，5s）
- 搁浅扫描捕获遗漏的分派（周期性，30s）
- Stage-launch 验证 DAG 并计算波次（Kahn 算法）
- Polecat 执行单个 issue
- Witness 监控 polecat，Refinery 合并

然而用户报告大型 epic"卡住了"。他们创建了一堆 bead，启动 convoy，离开几个小时，回来发现 convoy 停在 40%，没有指示原因。

**根本原因**：ConvoyManager 是机械式的。它在 issue 关闭时分派下一个就绪 issue，但无法推理失败模式、做出跳过决策或智能升级。当 polecat 在同一 issue 上反复失败时，机械系统无限重新 sling。当 dep 图外存在微妙的阻塞条件时，无人注意。

Mountain-Eater 在现有机械喂养之上添加判断层 — agent 驱动的停滞检测、N 次失败后跳过、智能升级和完成通知。

---

## 2. 设计原则：没有 Agent 持有线程

单协调者方法失败的原因是**滞后性**。任何维护"我在驱动这个 epic"循环的 agent 会在压缩时丢失该线程。即使 epic 已 hooked，重新 prime 的 agent 也不记得协调上下文。

Mountain-Eater 完全规避了这一点：

- **Epic 就是线程。** Bead 就是状态。
- **没有 agent 需要记住任何东西。** 每次检查重新发现状态。
- **Dog 每次带来全新上下文。** 构造上零滞后。
- **标签触发巡逻行为。** 无需持久协调者。

这与 Gas Town 核心原则一致：
- **ZFC**：Agent 决策，Go 传输。ConvoyManager 是传输；Dog 做判断。
- **NDI**：任何 Dog 可以检查任何 mountain。不同 agent，相同结果。
- **发现而非跟踪**：`bd ready --epic=X` 和 convoy 状态派生状态。
- **浮于整数之上**：卡住的 issue 不会阻止 mountain — 工作绕过它流动。

---

## 3. 架构：四层碾磨

```
层 0：CONVOY MANAGER（机械式，Go daemon — 已构建）
    事件驱动分派 + 搁浅扫描
    处理快乐路径：issue 关闭 → 分派下一个就绪

层 1：WITNESS（响应式，按 rig — 增强）
    Mountain convoy issue 的 Polecat 失败跟踪
    同一 issue 失败 3+ 次 → 标记阻塞，跳过，分派下一个

层 2：DEACON DOG（周期性，跨 rig — 新增）
    "此 mountain 自上次检查以来有进展吗？"
    新 Dog 以完整上下文调查停滞
    做判断：跳过、重构、升级
    在停滞和完成时通知 Mayor

层 3：MAYOR（战略性，面向用户 — 增强）
    接收层 2 的停滞升级
    跨 rig 判断
    在完成或不可恢复停滞时通知用户
```

**层 0** 已存在，处理约 80% 的 convoy 执行。
**层 1-2** 是 Mountain-Eater — 它们处理卡住的 20%。
**层 3** 是约 2% 需要人类判断的升级路径。

### 为什么是四层？

冗余监控即弹性。如果 Witness 遗漏完成（崩溃、压缩），ConvoyManager 会捕获它（5s 事件轮询）。如果 ConvoyManager 反复分派有问题的 issue，Witness 会捕获失败模式。如果两者都遗漏停滞，Deacon Dog 在下一次巡逻周期会捕获它。每层独立运行，从 beads 发现状态。

---

## 4. `mountain` 标签

Mountain 是带有 `mountain` 标签的 convoy。无新实体类型，无新数据库模式。标签就是层 1-2 的选择加入。

```bash
# 在 epic 上激活 Mountain-Eater
gt mountain <epic-id>

# 内部执行：
#   1. gt convoy stage <epic-id>          ← 验证 DAG，计算波次
#   2. bd update <convoy> --add-label mountain  ← 触发判断层
#   3. gt convoy launch <convoy-id>       ← 分派波次 1，ConvoyManager 接管

# 检查进度
gt mountain status [epic-id|convoy-id]

# 暂停/恢复（保留标签，停止/开始分派）
gt mountain pause <epic-id|convoy-id>
gt mountain resume <epic-id|convoy-id>

# 取消（移除标签，保留 convoy 供手动管理）
gt mountain cancel <epic-id|convoy-id>
```

常规 convoy（无 `mountain` 标签）继续像今天一样工作。`mountain` 标签让 convoy 选择加入增强的停滞检测、N 次失败后跳过和活跃进度监控。

### 何时使用 Mountain vs 常规 Convoy

| 场景 | 使用 |
|----------|-----|
| 3-5 个任务的批量 sling | 常规 convoy（ConvoyManager 足够） |
| 带有 DAG 依赖的大型 epic（10+ 任务） | Mountain |
| 跨 rig epic | Mountain（需要 Dog 的跨 rig 可见性） |
| "去吃午饭回来发现它完成了" | Mountain |
| 快速并行任务，无依赖 | 常规 convoy |

---

## 5. 层 1：Witness 失败跟踪

### 问题

当 polecat 在 mountain issue 上失败时，ConvoyManager 的搁浅扫描重新 sling 它。如果 issue 存在根本性问题（糟糕的描述、不可能的任务、缺失上下文），这创建了无限 sling-失败 循环。

### 增强

Witness 已监控 polecat 完成。为属于 mountain convoy 的 issue 添加失败跟踪：

```
WITNESS 巡逻 — mountain 失败跟踪：

对每个未完成 issue 退出的 polecat：
  issue = polecat 的 hooked bead
  convoy = 此 issue 的跟踪 convoy（如有）
  如果 convoy 有 "mountain" 标签：
    增加此 issue 的失败计数（存储为 issue note 或标签）
    如果 failure_count >= 3：
      bd update <issue> --status=blocked --add-label mountain:skipped
      bd update <issue> --notes "Skipped by Mountain-Eater after 3 polecat failures"
      log: "Mountain: skipped <issue> after 3 failures"
      # ConvoyManager 的下一次分派将跳过此 issue（blocked 状态）
      # 而是分派下一个就绪 issue
```

**失败计数存储**：在 issue 上使用 `mountain:failures:3` 这样的标签。标签廉价、可查询、在 `bd show` 中可见。无需新模式。

**为什么是 Witness 而不是 ConvoyManager？** Witness 已经观察 polecat 生命周期。它知道 polecat 是成功完成还是崩溃。ConvoyManager 只看到 issue 状态变更 — 它无法区分"polecat 失败"和"polecat 仍在工作"。

### 跳过语义

被跳过的 issue（`mountain:skipped` 标签、`blocked` 状态）：
- 从就绪前沿排除（blocked 状态）
- 在 `gt mountain status` 输出中可见
- 由层 2（Deacon Dog）升级到 Mayor
- 可恢复：`bd update <issue> --status=open --remove-label mountain:skipped`

Mountain 继续绕过被跳过的 issue 碾磨。如果被跳过的 issue 在 DAG 中阻塞其他工作，那些依赖者保持阻塞。Dog 在其停滞诊断中报告此情况。

---

## 6. 层 2：Deacon Dog Mountain 审计

### 核心循环

Deacon 的巡逻 formula 获得 `mountain-audit` 步骤：

```
DEACON 巡逻 — mountain-audit 步骤：

mountains = bd list --label mountain --status=open --type=convoy
对每个 mountain：
  dog_needed = false

  # 进度检查（与上次审计比较）
  current_closed = 此 convoy 中已关闭 issue 计数
  last_closed = 从 deacon bead 上的 mountain:audit:<convoy-id> 标签读取

  如果 current_closed > last_closed：
    # 有进展 — 更新审计标记，继续
    update mountain:audit:<convoy-id> = current_closed

  否则如果 current_closed == total_issues：
    # 完成 — 分派 Dog 进行清理 + 通知
    dog_needed = true
    dog_task = "complete"

  否则：
    # 自上次检查无进展 — 分派 Dog 调查
    dog_needed = true
    dog_task = "stall"

  如果 dog_needed：
    sling mountain-dog formula 到 Dog，带 convoy-id 和任务类型
```

### Mountain Dog Formula

`mol-mountain-dog.formula.toml` — 用于调查 mountain 进度的短命 Dog formula：

```toml
[formula]
name = "mountain-dog"
description = "Investigate mountain convoy progress"
type = "worker"

[formula.variables]
convoy_id = { required = true }
task = { required = true }  # "stall" 或 "complete"

[[formula.steps]]
name = "investigate"
description = """
You are a Mountain Dog investigating a mountain convoy.

Convoy: {{convoy_id}}
Task: {{task}}

If task is "stall":
  1. Run: gt convoy status {{convoy_id}}
  2. Identify why no progress:
     - Are there skipped issues (mountain:skipped label)?
     - Are all remaining issues blocked? By what?
     - Are polecats active but slow?
     - Is the refinery backed up?
  3. If there are ready issues with no polecats: sling them
  4. If all remaining issues are skipped/blocked:
     Mail Mayor: "Mountain {{convoy_id}} stalled: N skipped, M blocked.
     Remaining DAG cannot progress without intervention."
  5. If polecats are active: this is fine, no action needed

If task is "complete":
  1. Run: gt convoy status {{convoy_id}}
  2. Verify all tracked issues are closed
  3. If any skipped issues remain:
     Mail Mayor: "Mountain {{convoy_id}} finished with N skipped issues.
     Review skipped work: [list issue IDs]"
  4. If all clean:
     Mail Mayor: "Mountain {{convoy_id}} complete. N issues closed in Xh Ym."
  5. Run: gt convoy close {{convoy_id}}
"""
```

### Dog 的特性使此工作

- **全新上下文**：Dog 以零状态启动。它从头读取 convoy 和 bead。无前次会话的滞后。
- **狭窄范围**：一个 convoy，一个问题（"停滞？" 或 "完成？"）。轻松装入单个上下文窗口。
- **临时性**：完成工作、报告、消亡。无长时间运行的协调。
- **跨 rig 可见性**：Dog 有多个 rig 的 worktree。它们可以跨 rig 检查 beads 状态用于跨 rig convoy。

### 审计频率

Deacon 巡逻周期决定 mountain 被审计的频率。当前 Deacon 巡逻在分派驱动 + 心跳模型上运行。对于 mountain，相关问题为："mountain 停滞多久后才有人注意到？"

- **目标**：10-15 分钟内检测到停滞
- **机制**：Deacon 的心跳间隔（daemon 根据活跃度每 5-10 分钟 poke Deacon 一次）。每次心跳运行包含 mountain-audit 步骤的巡逻 formula。
- **成本**：每个巡逻周期一次 `bd list --label mountain` 查询（廉价），加上每个停滞 mountain 一次 Dog 生成（仅在需要时）。

---

## 7. 层 3：Mayor 通知

Mayor 从 Dog 接收两种类型的 mountain mail：

### 停滞通知

```
Subject: Mountain stalled: <convoy-title>
Body:
  Convoy: hq-cv-abc "Rebuild auth system"
  Progress: 23/35 closed (65%)
  Stalled for: 15 minutes

  Skipped issues (polecat failure):
    gt-xyz "Migrate session store" (failed 3 times)
    gt-abc "Update JWT validation" (failed 3 times)

  Blocked issues (DAG):
    gt-def "Integration tests" (blocked by gt-xyz)
    gt-ghi "E2E tests" (blocked by gt-def)

  Active polecats: 0
  Ready issues: 0

  Action needed: Review skipped issues. Possible fixes:
    bd update gt-xyz --status=open --remove-label mountain:skipped  (retry)
    bd close gt-xyz --reason="Descoped"  (skip permanently, unblocks dependents)
```

### 完成通知

```
Subject: Mountain complete: <convoy-title>
Body:
  Convoy: hq-cv-abc "Rebuild auth system"
  Result: 33/35 closed, 2 skipped
  Elapsed: 3h 42m

  Skipped issues:
    gt-xyz "Migrate session store" (failed 3 times — needs manual review)
    gt-abc "Update JWT validation" (failed 3 times — needs manual review)
```

### Mayor 的角色

Mayor **不是**碾磨循环的一部分。它接收通知并可以采取行动，但 mountain 无需 Mayor 参与即自主碾磨。Mayor 的行动：

- **重试被跳过的 issue**：`bd update <id> --status=open --remove-label mountain:skipped`
- **永久跳过**：`bd close <id> --reason="Descoped"`（解除依赖者阻塞）
- **通知用户**：转发停滞/完成通知
- **重构 DAG**：移除或添加依赖以绕过阻塞

---

## 8. 用户体验

### 启动 Mountain

```bash
$ gt mountain gt-epic-auth-rebuild

Validating epic structure...
  Epic: gt-epic-auth-rebuild "Rebuild auth system"
  Tasks: 35 (31 slingable, 4 epics)
  Waves: 6 (computed from blocking deps)
  Max parallelism: 4

  Warnings:
    gt-migrate-sessions has no description (may cause polecat confusion)

  Errors: none

Creating convoy...
  Convoy: hq-cv-m7x "Mountain: Rebuild auth system"
  Label: mountain

Launching Wave 1 (4 tasks)...
  Slung gt-foundation-types → gastown
  Slung gt-config-schema → gastown
  Slung gt-test-fixtures → gastown
  Slung gt-error-types → gastown

Mountain active. ConvoyManager will feed subsequent waves.
Deacon will audit progress every ~10 minutes.
Check status: gt mountain status hq-cv-m7x
```

### 检查状态

```bash
$ gt mountain status

Active Mountains:
  hq-cv-m7x "Rebuild auth system"
    Progress: ████████████░░░░░░░░ 23/35 (65%)
    Active: 3 polecats working
    Ready: 1 issue waiting for polecat
    Blocked: 6 issues (DAG deps)
    Skipped: 2 issues (polecat failures)
    Elapsed: 1h 47m

  hq-cv-n9y "Migrate database layer"
    Progress: ██████████████████░░ 18/20 (90%)
    Active: 2 polecats working
    Elapsed: 52m
```

### 详细状态

```bash
$ gt mountain status hq-cv-m7x

Mountain: hq-cv-m7x "Rebuild auth system"
Epic: gt-epic-auth-rebuild

Progress: 23/35 closed (65%)
Elapsed: 1h 47m
Wave: 4 of 6

Completed (23):
  ✓ gt-foundation-types, gt-config-schema, gt-test-fixtures, ...

Active (3):
  ⟳ gt-session-handler (polecat: gastown/nux, 12m)
  ⟳ gt-middleware-chain (polecat: gastown/furiosa, 8m)
  ⟳ gt-rate-limiter (polecat: gastown/max, 3m)

Ready (1):
  ○ gt-cache-layer (unblocked, waiting for polecat)

Skipped (2):
  ⊘ gt-migrate-sessions (failed 3 times — no description)
  ⊘ gt-jwt-validation (failed 3 times — test dependency missing)

Blocked (6):
  ◌ gt-auth-integration (needs: gt-session-handler, gt-jwt-validation⊘)
  ◌ gt-e2e-auth-tests (needs: gt-auth-integration)
  ...

Stall risk: gt-jwt-validation⊘ blocks 4 downstream issues.
  Fix: bd update gt-jwt-validation --status=open --remove-label mountain:skipped
  Or:  bd close gt-jwt-validation --reason="Descoped"
```

---

## 9. 全局改进（所有 Convoy）

Mountain-Eater 设计揭示了惠及**所有** convoy（不只是 mountain）的改进。这些应全局应用：

### 9.1 Polecat 失败跟踪

即使非 mountain convoy 也受益于知道"此 issue 已失败 3 次"。Witness 应为所有 convoy 跟踪 issue 跟踪失败计数，不只是 mountain 的。区别：mountain 在 3 次失败后自动跳过；常规 convoy 仅记录警告。

### 9.2 搁浅扫描中的停滞检测

ConvoyManager 的搁浅扫描当前分派第一个就绪 issue。增加：如果同一 issue 已被 sling 3+ 次且持续作为搁浅出现，停止重新 sling 并记录警告。这为所有 convoy 防止无限 sling-失败 循环。

### 9.3 进度可见性

`gt convoy status` 应显示与 `gt mountain status` 相同的丰富信息 — 活跃 polecat、就绪前沿、阻塞 issue、跳过 issue。这对所有 convoy 有用，不只是 mountain。

---

## 10. 与 Swarm 架构的关系

[swarm 架构文档](../../../docs/swarm-architecture.md)描述了一种设计，其中 swarm 是由专用 agent 协调的持久 molecule。Mountain-Eater 通过不同机制实现相同目标：

| Swarm 架构 | Mountain-Eater |
|--------------------|----------------|
| 专用协调 agent | 无协调者 — 巡逻步骤 + Dog |
| Swarm molecule 跟踪状态 | 标签触发巡逻行为 |
| 协调者通过 molecule 存活 | Dog 带来全新上下文（无需存活） |
| Ready Front 由协调者计算 | Ready Front 由 ConvoyManager + Dog 计算 |
| 通过 molecule resume 恢复 | 通过 beads 状态发现恢复 |

Mountain-Eater 是 swarm 架构目标的实现路径。Swarm 文档的"ready front"模型、"gate issue"和"批量管理"概念直接适用。区别在于机制：巡逻驱动碾磨而非协调者驱动碾磨。

Swarm 架构文档应更新以引用 Mountain-Eater 作为具体实现。

---

## 11. 实现计划

见 [roadmap.md](roadmap.md) Milestone 5 了解分阶段实现。

### 变更摘要

| 组件 | 变更 | 范围 |
|-----------|--------|-------|
| `gt mountain` CLI | 新命令（stage + label + launch） | ~200 行 |
| `gt mountain status` | 新命令（查询 + 格式化） | ~300 行 |
| `gt mountain pause/resume/cancel` | 标签管理 | ~100 行 |
| Witness 巡逻 formula | Convoy issue 的失败跟踪 | Formula 步骤 |
| Deacon 巡逻 formula | Mountain 审计步骤 | Formula 步骤 |
| `mol-mountain-dog.formula.toml` | 用于停滞调查的 Dog formula | 新 formula |
| ConvoyManager 搁浅扫描 | N 次失败后跳过（全局） | ~30 行 |
| `gt convoy status` | 增强输出（活跃、就绪、阻塞） | ~100 行 |

### 不变的内容

- Convoy 数据模型（仍是带 `tracks` 依赖的 `hq-cv-*` bead）
- ConvoyManager 事件轮询（仍为 5s，关闭时分派）
- ConvoyManager 搁浅扫描（仍为 30s，增强了跳过逻辑）
- Stage-launch 工作流（mountain 直接使用）
- Polecat 生命周期（不变）
- Refinery（不变）

---

## 12. 开放问题

1. **`gt mountain` 是否应自动解除停驻已停驻的 rig？** 如果 epic 的 issue 路由到已停驻的 rig，mountain 是否应自动解除其停驻？当前想法：否 — 要求 rig 处于活跃状态。Mountain 仅碾磨活跃 rig。

2. **每个 mountain 的最大并发 polecat。** Mountain 是否应有可配置的并发上限？ConvoyManager 每个关闭事件分派一个 issue。对于 mountain，我们可能希望在波次过渡时分派多个就绪 issue（如波次 1 完成，波次 2 有 8 个就绪 issue — 分派所有 8 个，而非一次一个）。

3. **Mountain 间依赖。** 一个 mountain 能否依赖另一个？初期可能不需要 — mountain 间依赖只是 DAG 中的跨 issue 依赖。

4. **通知通道。** Mayor mail 是当前通知路径。Mountain 是否也应支持用户 webhook/Slack 通知？推迟到未来工作。