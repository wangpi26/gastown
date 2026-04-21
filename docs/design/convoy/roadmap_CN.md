# Convoy 稳定性路线图

如何从当前位置到达目标 UX，同时保留现有工作流并修复用户实际遇到的可靠性问题。

---

## 当前状态

Milestone 0 完成 — 所有基础 PR 已合并。

---

## 要保留的工作流

### 工作流 A：手动 bead 创建 + 批量 sling

当今最常见的模式：

```
bd create --type=task "Fix auth timeout"       → sh-task-1
bd create --type=task "Add validation"         → sh-task-2
bd create --type=task "Integration tests"      → sh-task-3
bd dep add sh-task-2 sh-task-1 --type=blocks
gt sling sh-task-1 sh-task-2 sh-task-3 gastown
```

今天发生什么（PR [#1759](https://github.com/steveyegge/gastown/pull/1759)）：
- 批量 sling 创建**一个 convoy** 跟踪所有 3 个任务
- Rig 从 bead 前缀自动解析（显式 rig 已弃用）
- 任务按顺序 sling，间隔 2s，共享 1 个 convoy
- `blocks` 依赖由 daemon feeder 尊重 — sh-task-2 在 sh-task-1 关闭前不会被 daemon 分配
  （但初始分派无论依赖关系都发送所有任务）

人们期望的：
- 任务按依赖顺序分派
- 被阻塞的任务在阻塞者关闭前不被 sling
- 完成的任务通过 refinery 落到目标分支

### 工作流 B：design-to-beads + 手动 sling

```
/design-to-beads PRD.md
→ creates: root epic, sub-epics, leaf tasks
→ adds: parent-child deps (organizational hierarchy)
→ adds: blocks deps (execution ordering between tasks)
gt sling <task1> <task2> <task3> gastown
```

与工作流 A 结果相同：一个共享 convoy，blocks 依赖由 daemon feeder 尊重。Epic 和 sub-epic 结构存在于 beads 中，影响 daemon 驱动分派（epic 被 `IsSlingableType` 过滤，阻塞任务等待阻塞者关闭）。

### 工作流 C：手动 convoy 创建

```
gt convoy create "Auth overhaul" sh-task-1 sh-task-2 sh-task-3
gt sling sh-task-1 gastown
→ witness feeds sh-task-2 when sh-task-1 closes (serial)
→ witness feeds sh-task-3 when sh-task-2 closes (serial)
→ convoy auto-closes when all 3 are done
```

这在 upstream/main 上可用但是串行的（一次一个任务），且 witness feed 忽略 blocks 依赖、类型过滤和 rig 容量。

---

## 目标 UX

路线图结束时可达的理想体验：

```
/design-to-beads PRD.md
→ creates: root epic → sub-epics → leaf tasks
→ adds: parent-child (hierarchy) + blocks (ordering) deps
→ sub-epics get integration branches

gt convoy stage <epic-id>
→ walks DAG, validates structure, displays route plan (tree + waves)
→ creates staged convoy tracking all beads

gt convoy launch <convoy-id>
→ activates convoy, dispatches Wave 1 tasks
→ daemon feeds subsequent waves as tasks close
→ sub-epic status auto-managed (open → in_progress → closed)
→ when sub-epic closes: sling sub-epic with review formula
→ review formula examines accumulated changes on integration branch
→ on approval: integration branch lands to main/parent branch
→ convoy closes when root epic closes
```

---

## 用户实际报告的故障

最常见的投诉：**任务无法通过 refinery 落到目标分支。** 这不是 convoy 问题 — 这是 sling→done→refinery 管道可靠性问题。Convoy 系统叠加在此管道之上。

### 关键故障点（与 convoy 无关）

| # | 故障 | 位置 | 严重性 | 恢复 |
|---|---------|-------|----------|----------|
| 1 | ~~Dolt 分支合并失败~~ | ~~`done.go`~~ | 已解决 | 被 all-on-main 架构消除（无每 polecat Dolt 分支） |
| 2 | 推送失败（所有 3 层） | `done.go:531-572` | 关键 | 提交仅限本地。Worktree 保留。需手动恢复。 |
| 3 | MR bead 创建失败 | `done.go:744-752` | 高 | 分支已推送但无 MR。Witness 已通知。无自动恢复。 |
| 4 | Refinery 从未唤醒（agent 停滞） | Agent 级 | 高 | 心跳重启，但间隔可能数分钟。 |
| 5 | 合并冲突无限期阻塞 MR | `engineer.go:764-786` | 中 | 须分派 + 解决冲突任务。如果 rig 满载则停滞。 |
| 6 | 孤儿 MR（分支已删除，MR 仍开放） | `engineer.go:1086-1198` | 中 | 异常检测可发现。Agent 须行动。 |

这些故障影响**所有** polecat 工作，不只是 convoy 跟踪的工作。修复它们惠及整个系统。

### Convoy 特定故障点

| # | 故障 | 修复方案 | 状态 |
|---|---------|----------|--------|
| 7 | 被阻塞任务被 sling（blocks 依赖被忽略） | `isIssueBlocked` | PR [#1759](https://github.com/steveyegge/gastown/pull/1759)（开放） |
| 8 | Epic 被 sling 到 polecat（无类型过滤） | `IsSlingableType` | PR [#1759](https://github.com/steveyegge/gastown/pull/1759)（开放） |
| 9 | 跨 rig 关闭事件对 daemon 不可见 | 多 rig SDK 轮询 | **已合并** |
| 10 | Daemon 关闭后不分派下一个任务 | 继续分派 | **已合并** |
| 11 | Refinery convoy check 传递错误路径（从不生效） | 调用已移除 | **已合并** |
| 12 | 首次分派失败放弃整个 convoy | 分派失败迭代 | PR [#1759](https://github.com/steveyegge/gastown/pull/1759)（开放） |
| 13 | 搁浅扫描仅报告，不自动分派 | `feedFirstReady` | **已合并** |

---

## 分阶段计划

### Milestone 0：落地基础

**状态：完成。**

### Milestone 1：管道可靠性（与 convoy 无关）

**目标：** 修复导致"任务无法落地"投诉的 sling→done→refinery 管道故障。

这是用户报告问题中影响最大的工作。如果底层管道丢失任务，convoy 无法交付。

**工作项：**

| # | 问题 | 提议修复 | 复杂性 |
|---|---------|-------------|------------|
| 1a | ~~Dolt 分支合并失败~~ | 已解决 — all-on-main 消除每 polecat Dolt 分支 | N/A |
| 1b | ~~Dolt 分支上的搁浅 MR bead~~ | 已解决 — 无每 polecat Dolt 分支可搁浅 | N/A |
| 1c | Refinery agent 停滞 | 加强 refinery 心跳。添加 daemon 级 MR 队列监控，当 MR 超过阈值未处理时 nudge（或重启）refinery | 中 |
| 1d | 合并冲突无限期阻塞 | 跟踪冲突任务年龄。N 小时未解决则升级到 Mayor/owner 并提供具体冲突详情 | 低 |

**此 milestone 与 convoy 工作无关。** 可由不同贡献者并行完成，或在 Milestone 0 之后排序。

### Milestone 2：Stage 和 launch（`gt convoy stage`、`gt convoy launch`）

**目标：** 启用 `/design-to-beads → gt convoy stage → gt convoy launch` 工作流。

**依赖：** Milestone 0（feeder 必须尊重 blocks 依赖并过滤类型，staged convoy 才能正确工作）。

**交付内容（来自 Phase 2 PRD）：**
- `gt convoy stage <bead-id>` — DAG 遍历、验证、波次计算、树 + 波次路线计划显示
- `gt convoy launch <convoy-id>` — 激活 convoy，分派波次 1
- Epic 状态管理（open → in_progress → closed）
- 集成分支感知（缺失时警告）
- Staged 状态转换（staged_ready ↔ staged_warnings → open）

**已做出的关键设计决策：**
- `parent-child` 仅用于组织，永不阻塞（与 `bd ready` 和 beads SDK 一致）
- 执行排序通过显式 `blocks` 依赖
- 波次计算是信息性的（仅显示），运行时分派使用每周期 `isIssueBlocked` 检查
- 集成分支创建和落地保持手动（或 refinery 自动落地）

**对工作流 B 的启用的内容：**
```
/design-to-beads PRD.md
gt convoy stage <root-epic-id>
→ see tree view + wave view
→ see warnings (missing integration branch, parked rigs, etc.)
gt convoy launch <convoy-id>
→ Wave 1 tasks dispatched automatically
→ subsequent waves fed by daemon as tasks close
→ epic statuses update as children progress
→ convoy closes when root epic closes
```

**尚未启用的内容：**
- Sub-epic 审查 formula（见 Milestone 3）
- Epic slinging 的自动 formula 检测（Phase 3）
- 协调者 polecat（Phase 3）

### Milestone 3：Sub-epic 审查门控

**目标：** 当 sub-epic 下的所有任务完成并合并到 sub-epic 的集成分支时，在落地前自动触发对累积变更的全面审查。

这是"任务合并到集成分支"和"集成分支落地到 main"之间的缺失环节。

**当前状态：** 集成分支落地纯粹是机械式的 — 所有子项关闭 + 所有 MR 合并 = 准备落地。没有审查步骤检查合并的 diff。

**提议机制：**

1. **Sub-epic 完成触发**：当 convoy 的 epic 状态管理（Milestone 2 US-014）关闭 sub-epic 时，代替（或在）自动落地之前，用审查 formula sling sub-epic 本身。

2. **审查 Formula**：新 formula（如 `mol-integration-review` 或适配 `code-review.formula.toml`）：
   - 检出集成分支
   - 计算相对基础分支的完整 diff
   - 审查累积变更：
     - 跨任务一致性
     - API 契约违反
     - 组合功能缺失测试
     - 合并冲突残留
   - 生成审查报告
   - 如果批准：运行 `gt mq integration land <sub-epic-id>`
   - 如果拒绝：创建修复任务，在 sub-epic 上阻塞

3. **Convoy 感知**：Convoy 在审查运行期间保持开放。审查 polecat 的完成触发下一个 sub-epic（如果 root epic 的 sub-epic 之间有 `blocks` 依赖）或 root epic 关闭。

**集成点：**
- `internal/convoy/operations.go` — 关闭 epic 后，检查是否有集成分支。如有，用审查 formula sling 而非调用 `gt mq integration land`
- `internal/daemon/convoy_manager.go` — 事件轮询检测审查 polecat 的 bead 关闭，分派下一个 sub-epic 或关闭 root epic
- 新 formula：`mol-integration-review.formula.toml`

**design-to-beads 需要的变更：**
- 确保 sub-epic 获得集成分支（design-to-beads 创建，或 `gt convoy stage` 在 stage 时创建）
- 确保 sub-epic 间存在 `blocks` 依赖（如果需要顺序排序）

### Milestone 4：高级分派（Phase 3 PRD）

**目标：** 可插拔分派策略和协调者 polecat。

**交付内容：**
- `FeederStrategy` 接口
- 层级深度验证（可选）
- 从层级自动生成 `blocks` 依赖（`--infer-blocks`）
- `gt sling` 中的自动 formula 检测（epic → 协调者 formula）
- 协调者 polecat 策略
- 动态 DAG 分解

此 milestone 最远且最不紧急。默认分派策略（Phase 1 feeder 带 blocks 检查）覆盖常见情况。协调者 polecat 用于 AI 驱动的任务选择优于静态依赖排序的复杂 epic。

### Milestone 5：Mountain-Eater（自主 epic 碾磨）

**目标：** 在机械 ConvoyManager 之上叠加 agent 驱动的判断层，使大型 epic 自主碾磨到完成。

**依赖：** Milestone 2（stage-launch），用于 mountain 构建的 `gt convoy stage/launch` 管道。

**设计文档：** [mountain-eater.md](mountain-eater.md)

**交付内容：**

| 组件 | 描述 |
|-----------|-------------|
| `gt mountain <epic>` | CLI：验证 + stage + label + launch |
| `gt mountain status` | CLI：丰富进度视图（活跃、就绪、阻塞、跳过） |
| `gt mountain pause/resume/cancel` | CLI：生命周期管理 |
| Witness 失败跟踪 | 巡逻步骤：计算每 convoy issue 的 polecat 失败次数，3 次后自动跳过 |
| Deacon mountain-audit | 巡逻步骤：周期性进度检查，停滞时分派 Dog |
| `mol-mountain-dog` formula | Dog formula：调查停滞、sling 孤儿 issue、升级 |
| ConvoyManager skip-after-N | 全局：搁浅扫描停止重新 sling 反复失败的 issue |
| 增强 convoy 状态 | 全局：`gt convoy status` 显示活跃 polecat、就绪前沿、阻塞 issue |

**关键洞察：** 没有 agent 持有线程。Convoy 上的 `mountain` 标签触发 Witness（失败跟踪）和 Deacon（进度审计）中的巡逻行为。Dog 为停滞调查带来全新上下文。ConvoyManager 的机械喂养处理快乐路径；判断层处理卡住的 20%。

**全局改进（惠及所有 convoy）：**
- Polecat 失败跟踪（Witness）
- 搁浅扫描中的 skip-after-N-failures（ConvoyManager）
- 增强 `gt convoy status` 输出

---

## 依赖图

```
Milestone 0: Foundation  ← MERGED
  │
  ├──────────────────────────┐
  │                          │
  v                          v
Milestone 1: Pipeline    Milestone 2: Stage/Launch
  (done/refinery fixes)    (gt convoy stage/launch)
  │                          │
  │                          ├───────────────────────┐
  │                          v                       v
  │                      Milestone 3: Review gate  Milestone 5: Mountain-Eater
  │                          │                       │
  └──────────┬───────────────┘                       │
             │                                       │
             v                                       │
         Milestone 4: Advanced dispatch ◄────────────┘
```

Milestone 1 和 2 相互独立，可并行运行。
Milestone 3 依赖 Milestone 2（需要 epic 状态管理）。
Milestone 4 依赖 2 和 3 都稳定。
Milestone 5 依赖 Milestone 2（使用 stage-launch 管道）。
Milestone 3 和 5 相互独立，可并行运行。

---

## design-to-beads 需要的变更

当前 design-to-beads 插件创建了正确的结构（带 parent-child 依赖的 epic、带 blocks 依赖的 task）。对于 staged convoy 工作流，它需要：

| 变更 | 何时需要 | 谁 |
|--------|------------|-----|
| 在 sub-epic 之间创建 `blocks` 依赖（不只是任务之间） | Milestone 2 | design-to-beads 插件 |
| 为 sub-epic 创建集成分支 | Milestone 3 | design-to-beads 插件或 `gt convoy stage` |
| 输出 root epic ID 供 `gt convoy stage` 输入 | Milestone 2 | design-to-beads 插件 |

当前插件已在任务之间创建 blocks 依赖。缺口是 sub-epic 间排序：如果 Sub-Epic A 应在 Sub-Epic B 开始前完成，它们之间（或 A 最后一个任务和 B 第一个任务之间）必须存在 `blocks` 依赖。

如果 design-to-beads 不创建 sub-epic 间 blocks 依赖，`gt convoy stage` 将显示它们并行分派（Wave 1），这可能不理想。`--infer-blocks` 标志（Milestone 4）可以从创建顺序自动生成这些依赖，但 PRD 结构中的显式依赖更可靠。

---

## 总结：下一步

1. **现在：** 让 PR [#1759](https://github.com/steveyegge/gastown/pull/1759)（feeder 安全守卫）审查并合并，完成 Milestone 0。

2. **接下来：** 根据优先级启动 Milestone 1（管道可靠性）和/或 Milestone 2（stage/launch）。Milestone 1 影响更广（为所有人修复"任务无法落地"）。Milestone 2 启用 staged convoy UX。这两者可并行。

3. **M2 之后：** Milestone 3（sub-epic 审查门控）和 Milestone 5（Mountain-Eater）可并行。Milestone 5 是"去吃午饭"的自主碾磨功能。Milestone 3 是审查质量门控。

4. **稍后：** Milestone 4（高级分派），当常见情况稳定后。