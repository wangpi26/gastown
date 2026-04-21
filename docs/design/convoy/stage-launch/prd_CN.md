# PRD：Convoy Stage 与 Launch（`gt convoy stage`、`gt convoy launch`）

## 问题

**1. 分派工作前无预飞行验证。** `gt sling` 立即分派任务，无结构分析。运行 `gt sling task1 task2 task3` 的用户无法提前知道 task2 有循环依赖、task3 的 rig 不存在，或三个任务在应该串行时会尝试并行运行。问题仅在 polecat 开始失败后才出现。

**2. 无依赖感知分派。** `gt sling` 不尊重 `blocks` 依赖。如果 task2 被 task1 阻塞，两者仍被立即 sling。阻塞者关闭后重新分派由 daemon 事件驱动的 feeder（ConvoyManager）处理，但初始分派顺序是"一次性全部发射"。

**3. 无分派计划可见性。** 用户无法在提交前预览分派计划。无法看到：
   - 任务将按什么顺序分派
   - 哪些任务将并行 vs 串行运行
   - 是否存在结构性问题（循环、缺失 rig、无效类型）
   - 整体工作范围和预估时间线

**4. 无分阶段分派。** 所有任务一次性分派。对于大型 epic（20+ 任务），这意味着同时生成 20+ polecat，耗尽 API 速率限制、token 预算和可用 rig 容量。

**5. Epic 无状态管理。** 当子任务执行时，epic bead 保留 `open` 状态。完成传播纯粹是手动的 — 没有东西在所有子任务完成时将 epic 标记为 `in_progress` 或关闭它。

---

## 解决方案：Stage 和 Launch

两步分派：先 **stage**（验证、计划、预览），再 **launch**（提交分派）。

```bash
# 步骤 1：Stage — 验证结构，显示分派计划
gt convoy stage <epic-or-bead-id>

# 步骤 2：审查计划，修复任何问题

# 步骤 3：Launch — 提交分派
gt convoy launch <convoy-id>
```

### Stage 做什么

1. **遍历 DAG** 从 bead ID 递归解析所有后代（子项、孙项等）。
2. **验证结构** 检测循环依赖、无效 rig、不可 sling 类型。
3. **计算波次** 使用 Kahn 算法确定执行层级（波次 1 无依赖，波次 2 依赖波次 1 等）。
4. **显示路线计划** 带波次信息的树视图。
5. **创建 convoy** 跟踪所有解析的 bead。

### Launch 做什么

1. **激活 convoy** 将 staged convoy 移至活跃分派。
2. **分派波次 1** 仅执行无未满足依赖的任务。
3. **依赖队列** 后续波次由 daemon 事件驱动的 feeder（ConvoyManager）在阻塞者关闭时分派。
4. **Epic 状态管理** 随子项进展自动推进 epic 状态。

---

## 用户故事

### US-001：Stage 从 epic 创建 convoy

**作为**用户，**我希望** `gt convoy stage <epic-id>` 创建跟踪所有后代的 convoy，**以便**我可以一起管理相关工作。

**验收标准**：
- [x] `gt convoy stage <epic-id>` 解析所有后代 bead（子项、孙项、叶任务）
- [x] 创建带有 `tracks` 依赖到每个解析 bead 的 convoy
- [x] Convoy 标题为 "Mountain: <epic-title>"（如果输入是 epic 类型）或 "Staged: <bead-title>"
- [x] 不分派 polecat（仅验证和跟踪）
- [x] 重复 stage 是幂等的（如果 convoy 已存在并跟踪相同 bead）

### US-002：Stage 显示路线计划

**作为**用户，**我希望**看到我分派工作的树视图，**以便**我可以在提交前理解范围和依赖。

**验收标准**：
- [x] 显示从 epic root 到叶任务的层级树
- [x] 每个节点显示：bead ID、标题、类型、rig（从前缀解析）
- [x] 波次注释：每个任务以其计算波次号显示
- [x] 警告和错误在底部显示摘要

### US-003：Stage 检测循环依赖

**作为**用户，**我希望** stage 在分派前检测循环依赖，**以便**我的工作不会在无限循环中卡住。

**验收标准**：
- [x] 如果任何解析的 bead 之间检测到循环，stage 报错
- [x] 错误消息识别循环中的 bead
- [x] Stage 不创建 convoy（无法修复循环）

### US-004：Stage 验证 rig 存在性

**作为**用户，**我希望** stage 检测不存在的 rig，**以便**我不会将任务 sling 到死胡同。

**验收标准**：
- [x] Stage 通过 `routes.jsonl` 将 bead 前缀映射到 rig
- [x] 前缀未映射到 rig 的 bead 标记为警告
- [x] 带有警告仍创建 convoy，但 affected bead 不被分派
- [x] 所有 bead 缺失 rig 为错误（无 convoy 创建）

### US-005：Stage 验证可 sling 类型

**作为**用户，**我希望** stage 检测不可执行类型，**以便**我不会将 epic 或注释 sling 到 polecat。

**验收标准**：
- [x] `IsSlingableType` 过滤非任务类型（epic、注释、MR bead 等）
- [x] 不可 sling bead 在路线计划中显示为警告
- [x] 不可 sling bead 不计入波次计算
- [x] 仅可 sling bead 被分派

### US-006：Stage 计算波次

**作为**用户，**我希望**看到哪些任务并行运行哪些串行运行，**以便**我可以预测时间线。

**验收标准**：
- [x] 使用 Kahn 算法计算波次
- [x] 波次 1 = 无 `blocks` 依赖的任务
- [x] 波次 N = 仅当所有 `blocks` 依赖在波次 < N 完成时可执行的任务
- [x] `parent-child` 依赖不阻塞（仅 `blocks` 阻塞）
- [x] 波次号在路线计划树视图中显示

### US-007：Launch 分派波次 1

**作为**用户，**我希望** `gt convoy launch <convoy-id>` 分派波次 1 任务，**以便**工作立即开始但尊重依赖。

**验收标准**：
- [x] `gt convoy launch <convoy-id>` 分派波次 1（无未满足依赖的任务）中的所有任务
- [x] 每个分派生成一个 polecat
- [x] 波次 2+ 任务不由 launch 分派（由 daemon feeder 处理）
- [x] Launch 将 convoy 设置为活跃（设置标签 `gt:convoy:launched`）
- [x] 重复 launch 是幂等的（已分派的 bead 不重新分派）

### US-008：Daemon feeder 尊重依赖

**作为**用户，**我希望**后续波次在阻塞者完成时分派，**以便**任务按依赖顺序执行。

**验收标准**：
- [x] 当波次 N 任务关闭时，daemon feeder 评估波次 N+1 任务
- [x] 仅分派所有 `blocks` 依赖已关闭的任务
- [x] 仍被阻塞的任务保持排队
- [x] 已由 feeder 处理（`isIssueBlocked`、`IsSlingableType`）

### US-009：Epic 状态自动管理

**作为**用户，**我希望** epic 状态随子项进展自动更新，**以便**我无需手动跟踪进度。

**验收标准**：
- [x] Epic 在首个子任务开始时从 `open` → `in_progress`
- [x] Epic 在所有子任务完成时从 `in_progress` → `closed`
- [x] 仅在子 bead 上存在 `parent-child` 依赖时适用
- [x] 由 daemon 事件驱动的 feeder 处理

### US-010：Sub-epic 集成分支感知

**作为**用户，**我希望** stage 警告我缺少集成分支，**以便**我知道合并策略可能受影响。

**验收标准**：
- [x] Stage 检查 sub-epic 是否有关联的集成分支（`<branch-name>-integration`）
- [x] 缺失集成分支在路线计划中显示为警告
- [x] 警告不阻止 staging
- [x] 为未来：stage 可自动创建集成分支（当前仅警告）

### US-011：Stage 交互模式

**作为**用户，**我希望** stage 默认交互确认，**以便**我可以在启动前审查分派计划。

**验收标准**：
- [x] `gt convoy stage <id>` 显示路线计划并提示确认
- [x] 提示提供选项：继续 / 取消
- [x] `--yes` 标志跳过确认（CI/自动化用）
- [x] `--dry-run` 标志显示计划但不创建 convoy

### US-012：Staged convoy 状态

**作为**用户，**我希望**在 convoy 列表中看到 staged convoy，**以便**我知道有工作等待启动。

**验收标准**：
- [x] Staged convoy 在 `gt convoy list` 中可见
- [x] Staged convoy 有独特状态（`staged` vs `open`）
- [x] `gt convoy status <id>` 显示波次信息和分派准备情况

### US-013：从 bead ID Stage（非 epic）

**作为**用户，**我希望** `gt convoy stage <task-id>` 即使 bead 是单个任务也能工作，**以便**我可以在任何工作上使用 stage/launch 工作流。

**验收标准**：
- [x] 单任务 bead：创建跟踪单个任务的 convoy
- [x] Convoy 标题为 "Staged: <task-title>"
- [x] 波次 1 包含单个任务，无后续波次
- [x] 仍适用验证（rig 存在、类型有效）

### US-014：Epic 完成触发 convoy 检查

**作为**系统，**我希望**当 root epic 关闭时，convoy 被自动检查以关闭，**以便**当所有工作完成时 convoy 生命周期终止。

**验收标准**：
- [x] 当所有跟踪 bead 关闭时，convoy 自动关闭
- [x] Root epic 关闭是跟踪 bead 关闭的信号
- [x] 由 daemon 事件驱动完成检测处理

### US-015：Parked rig 感知

**作为**用户，**我希望** stage 检测目标 rig 是否已停驻，**以便**我不会将任务 sling 到不活跃的 rig。

**验收标准**：
- [x] Stage 检查每个解析 rig 的停驻状态
- [x] 停驻 rig 在路线计划中显示为警告
- [x] 停驻 rig 上的任务不被分派（由 feeder 跳过）
- [x] 所有目标 rig 停驻为错误（无 convoy 创建）

### US-016：Launch `--wave` 标志

**作为**用户，**我希望**覆盖默认波次 1 分派，**以便**我可以从特定波次恢复或重新开始。

**验收标准**：
- [x] `gt convoy launch <id> --wave 3` 从波次 3 开始分派
- [x] 波次 1-2 任务标记为跳过（假设已完成）
- [x] 用于恢复中断的分派

### US-017：路线计划输出格式

**作为**用户，**我希望**路线计划支持多种输出格式，**以便**我可以在终端中使用或在脚本中解析。

**验收标准**：
- [x] 默认：人类可读的树视图（彩色终端）
- [x] `--json`：路线计划数据的 JSON 输出
- [x] `--quiet`：仅 convoy ID（用于脚本：`gt convoy stage <id> --quiet`）

### US-018：Stage 保留现有 convoy

**作为**用户，**我希望** bead 被另一个 convoy 跟踪时，stage 告知我，**以便**我不会创建重复跟踪。

**验收标准**：
- [x] Stage 检查每个 bead 是否已被活跃 convoy 跟踪
- [x] 已跟踪 bead 在路线计划中显示为警告
- [x] Stage 仍创建 convoy 但跳过已跟踪 bead
- [x] `--force` 标志覆盖（将 bead 添加到新 convoy 无论如何）

---

## 波次计算算法

### Kahn 算法

波次计算使用 Kahn 算法对 DAG 拓扑排序，将节点分配到层级：

```
1. 对每个可 sling bead：
     计算入度 = 仅来自其他可 sling bead 的 blocks 依赖计数
2. 初始化：波次 1 = 入度为 0 的所有 bead（无阻塞者）
3. 当波次 N 有 bead：
     对每个 bead 在波次 N 中：
       减少其每个依赖者的入度
     入度达到 0 的依赖者加入波次 N+1
4. 如果任何 bead 剩余未入波次 → 循环依赖检测到
```

### 关键细节

- **仅 `blocks` 依赖**：`parent-child` 依赖不增加入度。Parent-child 是组织性的，非执行性的。
- **仅可 sling bead**：Epic、注释和其他不可 sling 类型不进入波次计算。它们被过滤掉，但其 `blocks` 依赖仍对可 sling 子项考虑。
- **孤立项**：无任何依赖的任务进入波次 1。
- **循环**：如果算法在并非所有 bead 都入波次时终止，剩余 bead 形成循环。

### 示例

```
Epic: gt-epic-auth
  ├── Task: gt-task-1 (无依赖)
  ├── Task: gt-task-2 (被 gt-task-1 阻塞)
  ├── Task: gt-task-3 (被 gt-task-1 阻塞)
  ├── Task: gt-task-4 (被 gt-task-2、gt-task-3 阻塞)
  └── Epic: gt-sub-epic (不进入波次计算)
       ├── Task: gt-task-5 (无依赖)
       └── Task: gt-task-6 (被 gt-task-5 阻塞)

波次：
  Wave 1: gt-task-1, gt-task-5    (无阻塞者)
  Wave 2: gt-task-2, gt-task-3, gt-task-6  (阻塞者在 Wave 1)
  Wave 3: gt-task-4               (阻塞者在 Wave 2)
```

---

## 命令接口

### `gt convoy stage`

```
gt convoy stage <bead-id> [--dry-run] [--yes] [--json] [--quiet] [--force]

选项：
  --dry-run    显示路线计划但不创建 convoy
  --yes        跳过交互确认
  --json       JSON 输出路线计划
  --quiet      仅输出 convoy ID
  --force      在新 convoy 中包含已跟踪 bead
```

### `gt convoy launch`

```
gt convoy launch <convoy-id> [--wave N]

选项：
  --wave N     从波次 N 开始分派（默认：1）
```

### `gt convoy status`（增强）

```
gt convoy status <convoy-id>

增强：为 staged/launched convoy 显示波次信息
```

---

## 状态转换

```
                    ┌─────────────────────────────┐
                    │                             │
                    v                             │
  ┌─────────┐  stage  ┌──────────────┐  launch  ┌───────┐
  │  (无)    │ ──────► │ staged_ready │ ────────► │ open  │
  └─────────┘        └──────────────┘          └───────┘
                         │     ▲                     │
                    警告  │     │  修复               │ 全部关闭
                         v     │                     v
                    ┌──────────────┐            ┌───────┐
                    │staged_warning│ ─────────► │closed │
                    └──────────────┘  警告清除   └───────┘
```

**staged_ready**：验证通过，无阻塞问题。准备 launch。
**staged_warning**：验证通过但存在警告（缺失集成分支、已停驻 rig、已跟踪 bead）。仍可 launch。
**open**：已启动，工作正在执行中。
**closed**：所有跟踪 issue 已关闭。

修复警告（如取消停驻 rig）自动转换 `staged_warning` → `staged_ready`（在下次状态检查时）。

---

## 依赖交互

### Stage 与 ConvoyManager 的交互

Stage 创建 convoy 并设置跟踪依赖。Launch 分派波次 1。ConvoyManager 在波次 1 issue 关闭后处理后续波次。

### Stage 与 design-to-beads 的交互

`/design-to-beads` 创建 epics、sub-epics 和带正确依赖的任务。Stage 使用这些依赖计算波次和验证。无需对 design-to-beads 进行更改即可使 stage 工作。

### Stage 与现有 `gt sling` 的交互

- `gt sling` 继续用于即时分派（不改变现有工作流）
- `gt convoy stage` + `gt convoy launch` 是新可选工作流
- 两种工作流创建相同的 convoy 结构
- 关键区别：stage/launch 尊重依赖并提供预分派验证

---

## 非目标（本 PRD）

- **Sub-epic 审查 formula**：当 sub-epic 下的所有任务完成并合并时，在落地前自动触发全面审查。未来 milestone。
- **协调者 polecat**：驱动跨任务协调的长期 polecat。未来 milestone。
- **自动 `blocks` 依赖推断**：从层级结构自动生成 `blocks` 依赖（`--infer-blocks`）。未来 milestone。
- **时间线估计**：基于历史 polecat 速度估计总 convoy 持续时间。未来 milestone。

---

## 实现优先级

1. **US-001 到 US-007**：核心 stage/launch 功能
2. **US-009**：Epic 状态管理（影响最大 — 当前完全手动）
3. **US-003、US-004、US-005**：验证（防止坏分派）
4. **US-006**：波次计算（路线计划可见性）
5. **US-008**：Daemon feeder 增强（已部分实现）
6. **US-010 到 US-018**：辅助功能（集成分支感知、交互模式等）

---

## 测试策略

见 [testing.md](testing.md) 了解全面测试计划。

---

## 开放问题

1. **sub-epic 间 blocks 依赖**。design-to-beads 是否在 sub-epic 之间创建 `blocks` 依赖（不只是子任务之间）？如果不，sub-epic 将并行分派而非按顺序。建议：是的，design-to-beads 应在 sub-epic 间创建 blocks 依赖以启用顺序分派。

2. **波次信息是否持久化？** 波次分配是否存储在 convoy 或 bead 上？还是每次从 DAG 实时计算？建议：实时计算（无额外状态）。`gt convoy status` 每次调用时计算波次。

3. **Staged convoy 过期。** 应该有 staged convoy 的 TTL（存在时间）吗？例如，staged convoy 在 24 小时后自动清除？建议：否 — 让用户手动管理。`gt convoy close --force` 清除不需要的 staged convoy。