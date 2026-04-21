---
name: convoy
description: 使用 gastown convoy 系统的权威指南 -- 批量工作追踪、事件驱动喂养、stage-launch 工作流和派发安全防护。适用于编写 convoy 代码、调试 convoy 行为、添加 convoy 功能、测试 convoy 变更或回答关于 convoy 如何工作的问题。触发词：convoy, convoy manager, convoy feeding, dispatch, stranded convoy, feedFirstReady, feedNextReadyIssue, IsSlingableType, isIssueBlocked, CheckConvoysForIssue, gt convoy, gt sling, stage, launch, staged, wave。
---

# Gastown Convoy 系统

Convoy 系统跨 Rig 追踪批量工作。Convoy 是一个通过依赖 `tracks` 其他 Bead 的 Bead。守护进程监控关闭事件，当一个完成时喂养下一个就绪的 issue。

## 架构

```
+================================ CREATION =================================+
|                                                                            |
|   gt sling <beads>      gt convoy create ...     gt convoy stage <epic>    |
|        |  (auto-convoy)       |  (explicit)            |  (validated)     |
|        v                      v                        v                  |
|   +-----------+          +-----------+         +----------------+         |
|   |  status:  |          |  status:  |         |    status:     |         |
|   |   open    |          |   open    |         | staged:ready   |         |
|   +-----------+          +-----------+         | staged:warnings|         |
|                                                +----------------+         |
|                                                        |                  |
|                                              gt convoy launch             |
|                                                        |                  |
|                                                        v                  |
|                                                +----------------+         |
|                                                |    status:     |         |
|                                                |     open       |         |
|                                                | (Wave 1 slung) |         |
|                                                +----------------+         |
|                                                                            |
|   All paths produce: CONVOY (hq-cv-*)                                      |
|                      tracks: issue1, issue2, ...                           |
+============================================================================+
              |                              |
              v                              v
+= EVENT-DRIVEN FEEDER (5s) =+   +=== STRANDED SCAN (30s) ===+
|                              |   |                            |
|   GetAllEventsSince (SDK)    |   |   findStranded             |
|     |                        |   |     |                      |
|     v                        |   |     v                      |
|   close event detected       |   |   convoy has ready issues  |
|     |                        |   |   but no active workers    |
|     v                        |   |     |                      |
|   CheckConvoysForIssue       |   |     v                      |
|     |                        |   |   feedFirstReady           |
|     v                        |   |   (iterates all ready)     |
|   feedNextReadyIssue         |   |     |                      |
|   (iterates all ready)       |   |     v                      |
|     |                        |   |   gt sling <next-bead>     |
|     v                        |   |   or closeEmptyConvoy     |
|   gt sling <next-bead>       |   |                            |
|                              |   +============================+
+==============================+
```

三种创建路径（sling、create、stage），两种喂养路径，相同的安全防护：
- **事件驱动**（`operations.go`）：每约 5 秒轮询 Beads 存储获取关闭事件。调用 `feedNextReadyIssue`，在派发前检查 `IsSlingableType` + `isIssueBlocked`。**跳过已 stage 的 convoy**（`isConvoyStaged` 检查）。
- **Stranded 扫描**（`convoy_manager.go`）：每 30 秒运行。`feedFirstReady` 迭代所有就绪 issue。就绪列表在 `findStrandedConvoys`（cmd/convoy.go）中由 `IsSlingableType` 预过滤。**只看到开放的 convoy** — 已 stage 的 convoy 不会出现。

## 安全防护（三条规则）

这些防止事件驱动喂养器派发不该派发的工作：

### 1. 类型过滤（`IsSlingableType`）

只有叶子工作项可以被派发。定义在 `operations.go`：

```go
var slingableTypes = map[string]bool{
    "task": true, "bug": true, "feature": true, "chore": true,
    "": true, // empty defaults to task
}
```

Epic、子 Epic、convoy、decision — 全部跳过。在 `feedNextReadyIssue`（事件路径）和 `findStrandedConvoys`（stranded 路径）中均应用。

### 2. 阻塞依赖检查（`isIssueBlocked`）

有未关闭的 `blocks`、`conditional-blocks` 或 `waits-for` 依赖的 issue 被跳过。`parent-child` **不是**阻塞 — 子任务即使其父 Epic 仍开放也会派发。这与 `bd ready` 和 Molecule 步骤行为一致。

存储错误时 fail-open（假设未阻塞），以避免在临时 Dolt 问题上停滞 convoy。

### 3. 派发失败迭代

两种喂养路径都迭代跳过失败而非放弃：
- `feedNextReadyIssue`：派发失败时 `continue`，尝试下一个就绪 issue
- `feedFirstReady`：`for range ReadyIssues`，在跳过/失败时 `continue`，首次成功时 `return`

## CLI 命令

### Stage 和 launch（验证式创建）

```bash
gt convoy stage <epic-id>            # 分析依赖，构建 DAG，计算波次，创建已 stage 的 convoy
gt convoy stage gt-task1 gt-task2    # 从显式任务列表 stage
gt convoy stage hq-cv-abc            # 重新 stage 已有的已 stage convoy
gt convoy stage <epic-id> --json     # 机器可读输出
gt convoy stage <epic-id> --launch   # stage + 如无错误则立即 launch
gt convoy launch hq-cv-abc           # 从 staged 转为 open，派发 Wave 1
gt convoy launch <epic-id>           # 一步完成 stage + launch（委托给 stage --launch）
```

### 创建和管理

```bash
gt convoy create "Auth overhaul" gt-task1 gt-task2 gt-task3
gt convoy add hq-cv-abc gt-task4
```

### 检查和监控

```bash
gt convoy check hq-cv-abc       # 如所有追踪的 issue 已完成则自动关闭
gt convoy check                  # 检查所有开放的 convoy
gt convoy status hq-cv-abc       # 单个 convoy 详情
gt convoy list                   # 所有 convoy
gt convoy list --all             # 包括已关闭的
```

### 查找 stranded 工作

```bash
gt convoy stranded               # 有就绪工作但没有活跃工作者的
gt convoy stranded --json        # 机器可读
```

### 关闭和着陆

```bash
gt convoy close hq-cv-abc --reason "done"
gt convoy land hq-cv-abc         # 清理 worktree + 关闭
```

### 交互式 TUI

```bash
gt convoy -i                     # 打开交互式 convoy 浏览器
gt convoy --interactive          # 完整形式
```

## 批量 Sling 行为

`gt sling <bead1> <bead2> <bead3>` 创建**一个 convoy** 追踪所有 Bead。Rig 从 Bead 的前缀自动解析（通过 `routes.jsonl`）。Convoy 标题为 `"Batch: N beads to <rig>"`。每个 Bead 获得自己的 Polecat，但它们共享一个 convoy 进行追踪。

Convoy ID 和合并策略存储在每个 Bead 上，所以 `gt done` 可以通过快速路径（`getConvoyInfoFromIssue`）找到 convoy。

### Rig 解析

- **自动解析（首选）：** `gt sling gt-task1 gt-task2 gt-task3` — 从 `gt-` 前缀解析 rig。所有 Bead 必须解析到同一个 rig。
- **显式 Rig（已弃用）：** `gt sling gt-task1 gt-task2 gt-task3 myrig` — 仍然可用，打印弃用警告。如果任何 Bead 的前缀不匹配显式 rig，则报错并建议操作。
- **混合前缀：** 如果 Bead 解析到不同的 rig，报错并列出每个 Bead 解析的 rig 和建议操作（分开 sling 或 `--force`）。
- **未映射前缀：** 如果前缀没有路由，报错并给出诊断信息（`cat .beads/routes.jsonl | grep <prefix>`）。

### 冲突处理

如果任何 Bead 已被另一个 convoy 追踪，批量 sling **报错**并附带详细冲突信息（哪个 convoy、其中所有 Bead 的状态和 4 个建议操作）。这防止意外的双重追踪。

```bash
# 自动解析：一个 convoy，三个 Polecat（首选）
gt sling gt-task1 gt-task2 gt-task3
# -> Created convoy hq-cv-xxxxx tracking 3 beads

# 显式 rig 仍可用但打印弃用警告
gt sling gt-task1 gt-task2 gt-task3 gastown
# -> Deprecation: gt sling now auto-resolves the rig from bead prefixes.
# -> Created convoy hq-cv-xxxxx tracking 3 beads
```

## Stage-Launch 工作流

> 实现于 [PR #1820](https://github.com/steveyegge/gastown/pull/1820)。依赖于 [PR #1759](https://github.com/steveyegge/gastown/pull/1759) 的喂养器安全防护。设计文档：`docs/design/convoy/stage-launch/prd.md`、`docs/design/convoy/stage-launch/testing.md`。

Stage-launch 工作流是一种两阶段的 convoy 创建路径，在**任何工作被派发之前**验证依赖并计算波次派发顺序。这是 Epic 交付的首选路径。

### 输入类型

`gt convoy stage` 接受三种互斥的输入类型：

| 输入 | 示例 | 行为 |
|------|------|------|
| Epic ID | `gt convoy stage bcc-nxk2o` | BFS 遍历整个父子树，收集所有后代 |
| 任务列表 | `gt convoy stage gt-t1 gt-t2 gt-t3` | 精确分析这些任务 |
| Convoy ID | `gt convoy stage hq-cv-abc` | 从已有 staged convoy 重新读取追踪的 Bead（重新 stage） |

混合类型（如 epic + task 一起）会报错。多个 Epic 或多个 Convoy 也会报错。

### 处理流水线

```
1. validateStageArgs     — 拒绝空/标志类参数
2. bdShow each arg       — 解析 Bead 类型
3. resolveInputKind      — 分类 Epic / Tasks / Convoy
4. collectBeads          — 收集 BeadInfo + DepInfo（Epic 用 BFS，tasks 直接）
5. buildConvoyDAG        — 构建内存中的 DAG（节点 + 边）
6. detectErrors          — 循环检测 + 缺少 rig 检查
7. detectWarnings        — 孤立、parked rig、跨 rig、容量、缺少分支
8. categorizeFindings    — 分为 errors / warnings
9. chooseStatus          — staged:ready, staged:warnings, 或有 errors 时中止
10. computeWaves         — Kahn 算法（仅在无 errors 时）
11. renderDAGTree        — 打印 ASCII 依赖树
12. renderWaveTable      — 打印波次派发计划
13. createStagedConvoy   — bd create --type=convoy --status=<staged-status>
```

### 波次计算（Kahn 算法）

只有可 Sling 的类型参与波次：`task`、`bug`、`feature`、`chore`。Epic 被排除。

执行边（创建波次排序）：
- `blocks`
- `conditional-blocks`
- `waits-for`

非执行边（波次排序时忽略）：
- `parent-child` — 仅层级关系
- `related`、`tracks`、`discovered-from`

**算法：**
1. 过滤仅可 Sling 的节点
2. 计算每个节点的入度（计算指向其他可 Sling 节点的 BlockedBy 边数）
3. 剥离循环：收集所有入度为 0 的节点 → Wave N；移除它们；递减邻居入度；重复
4. 每个波次内按字母顺序排序以保证确定性

输出示例：
```
  Wave   ID              Title                     Rig       Blocked By
  ──────────────────────────────────────────────────────────────────────
  1      bcc-nxk2o.1.1   Init scaffolding          bcc       —
  2      bcc-nxk2o.1.2   Shared types              bcc       bcc-nxk2o.1.1
  3      bcc-nxk2o.1.3   CLI wrapper               bcc       bcc-nxk2o.1.2

  3 tasks across 3 waves (max parallelism: 1 in wave 1)
```

### Convoy 状态模型

四种状态，有定义的转换：

| 状态 | 含义 |
|------|------|
| `staged:ready` | 已验证，无错误或警告，准备 launch |
| `staged:warnings` | 已验证，无错误但有警告。修复并重新 stage，或忽略警告 launch |
| `open` | 活跃 — 守护进程在 Bead 关闭时喂养工作 |
| `closed` | 完成或取消 |

有效转换：

| 从 → 到 | 允许？ |
|-----------|----------|
| `staged:ready` → `open` | 是（launch） |
| `staged:warnings` → `open` | 是（launch） |
| `staged:*` → `closed` | 是（取消） |
| `staged:ready` ↔ `staged:warnings` | 是（重新 stage） |
| `open` → `closed` | 是 |
| `closed` → `open` | 是（重新打开） |
| `open` → `staged:*` | **否** |
| `closed` → `staged:*` | **否** |

### Error 与 Warning 分类

**Error**（致命 — 阻止 convoy 创建）：

| 类别 | 触发 | 修复 |
|----------|---------|-----|
| `cycle` | 在执行边中检测到循环 | 移除循环中的一个阻塞依赖 |
| `no-rig` | 可 Sling 的 Bead 没有 rig（前缀不在 routes.jsonl 中） | 添加 routes.jsonl 条目 |

**Warning**（非致命 — convoy 创建为 `staged:warnings`）：

| 类别 | 触发 |
|----------|---------|
| `orphan` | 没有任意方向的阻塞依赖的可 Sling 任务（仅 Epic 输入） |
| `blocked-rig` | Bead 目标为 parked 或 docked 的 rig |
| `cross-rig` | Bead 在不同于多数的 rig 上 |
| `capacity` | 某波次超过 5 个任务 |
| `missing-branch` | 有子项但没有 integration branch 的子 Epic |

### Launch 行为

`gt convoy launch <convoy-id>` 将已 stage 的 convoy 转为 open 并派发 Wave 1：

1. 验证 convoy 存在且已 stage
2. 将状态转为 `open`
3. 重新读取追踪的 Bead，重建 DAG，重新计算波次
5. 通过 `gt sling <beadID> <rig>` 派发 Wave 1 中的每个任务
6. 单个 sling 失败不会中止其余派发
7. 打印派发结果（每个任务 checkmark/X）
8. 后续波次由守护进程自动处理

如果 `gt convoy launch` 收到 Epic 或任务列表（非已 stage 的 convoy），它委托给 `gt convoy stage --launch` 以一步完成 stage-then-launch。

### 已 stage Convoy 的守护进程安全

**已 stage 的 convoy 对守护进程完全惰性。** 两种喂养路径都不处理它们：

- **事件驱动喂养器：** `CheckConvoysForIssue` 中的 `isConvoyStaged` 检查跳过任何 `staged:*` 状态的 convoy。读取错误时 fail-open（假设未 stage → 处理，安全因为对不存在的 convoy 读取不做任何事）。
- **Stranded 扫描：** `gt convoy stranded` 只返回开放的 convoy。已 stage 的 convoy 不会出现。

这意味着你可以 stage 一个 convoy，审查波次计划，准备好时再 launch — 没有提前派发的风险。

### 重新 Staging

对已有的已 stage convoy 运行 `gt convoy stage <convoy-id>` 会重新分析和更新：
- 从 convoy 的 `tracks` 依赖重新读取追踪的 Bead
- 重建 DAG，重新检测 errors/warnings，重新计算波次
- 通过 `bd update` 更新状态（如警告已解决则 `staged:warnings` → `staged:ready`）
- 不创建新的 convoy 或重新添加追踪依赖

## 测试 Convoy 变更

### 运行测试

```bash
# 完整 convoy 套件（所有包）
go test ./internal/convoy/... ./internal/daemon/... ./internal/cmd/... -count=1

# 按领域：
go test ./internal/convoy/... -v -count=1                       # 喂养逻辑
go test ./internal/daemon/... -v -count=1 -run TestConvoy       # ConvoyManager
go test ./internal/daemon/... -v -count=1 -run TestFeedFirstReady
go test ./internal/cmd/... -v -count=1 -run TestCreateBatchConvoy  # 批量 sling
go test ./internal/cmd/... -v -count=1 -run TestBatchSling
go test ./internal/cmd/... -v -count=1 -run TestResolveRig      # rig 解析
go test ./internal/daemon/... -v -count=1 -run Integration      # 真实 beads 存储

# Stage-launch：
go test ./internal/cmd/... -v -count=1 -run TestConvoyStage     # staging 逻辑
go test ./internal/cmd/... -v -count=1 -run TestConvoyLaunch    # launch + Wave 1 派发
go test ./internal/cmd/... -v -count=1 -run TestDetectCycles    # 循环检测
go test ./internal/cmd/... -v -count=1 -run TestComputeWaves    # 波次计算
go test ./internal/cmd/... -v -count=1 -run TestBuildConvoyDAG  # DAG 构建
```

### 关键测试不变量

- `feedFirstReady` 每次调用只派发 1 个 issue（首次成功即停止）
- `feedFirstReady` 迭代跳过失败（sling 退出 1 → 尝试下一个）
- 事件轮询和 feedFirstReady 中都跳过 parked rig
- 即使 `isRigParked` 对所有都返回 true，hq 存储也不会被跳过
- 高水位标记防止跨轮询周期的事件重复处理
- 首个轮询周期仅用于预热（播种标记，不处理）
- `IsSlingableType("epic") == false`，`IsSlingableType("task") == true`，`IsSlingableType("") == true`
- `isIssueBlocked` 是 fail-open（存储错误 → 未阻塞）
- `parent-child` 依赖不是阻塞的
- 批量 sling 为 N 个 Bead 创建恰好 1 个 convoy（不是 N 个）
- `resolveRigFromBeadIDs` 在混合前缀、未映射前缀、town 级前缀时报错
- 阻塞依赖中的循环阻止已 stage convoy 创建（非零退出，无副作用）
- Wave 1 仅包含在可 Sling 节点中零未满足阻塞依赖的任务
- Epic 和非可 Sling 类型永远不会放入波次
- 守护进程不会从 `staged:*` convoy 喂养 issue（两种喂养路径都跳过）
- `staged:warnings` convoy 仍然可以被 launch（警告是信息性的）
- 重新 staging convoy 不会创建重复（原地更新）
- Launch 仅派发 Wave 1，不派发后续波次
- 波次计算是确定性的（相同输入 → 相同输出，波次内按字母排序）

### 更深入的测试工程

参见 `docs/design/convoy/stage-launch/testing.md` 获取完整的 stage-launch 测试计划（跨单元、集成、快照和属性层级的 105 个测试）。

参见 `docs/design/convoy/testing.md` 获取涵盖失败模式、覆盖缺口、harness 评分卡、测试矩阵和推荐测试策略的通用 convoy 测试计划。

## 常见陷阱

- **`parent-child` 永远不阻塞。** 这是刻意的设计选择，不是 bug。与 `bd ready`、beads SDK 和 Molecule 步骤行为一致。
- **批量 sling 在已追踪的 Bead 上报错。** 如果任何 Bead 已在 convoy 中，整个批量 sling 失败并附冲突详情。用户必须在继续前解决冲突。
- **Stranded 扫描有自己的阻塞检查。** cmd/convoy.go 中的 `isReadyIssue` 从 issue 详情读取 `t.Blocked`。operations.go 中的 `isIssueBlocked` 覆盖事件驱动路径。不了解两条路径就不要合并它们。
- **空的 IssueType 是可 Sling 的。** Beads 在 IssueType 未设置时默认为"task"类型。将空视为不可 Sling 会破坏所有遗留 Bead。
- **`isIssueBlocked` 是 fail-open 的。** 存储错误时假设未阻塞。临时 Dolt 错误不应永久停滞 convoy — 下一个喂养周期以新状态重试。
- **批量 sling 中的显式 rig 已弃用。** `gt sling beads... rig` 仍可用但打印警告。首选 `gt sling beads...` 自动解析。
- **已 stage 的 convoy 是惰性的。** 守护进程完全忽略它们。不要期望在你 `gt convoy launch` 之前自动喂养。
- **Launch 前审查 `staged:warnings`。** 警告是信息性的 — 尽可能修复并重新 stage，或如果可接受就忽略警告 launch。
- **对非 staged 输入执行 `gt convoy launch` 会委托给 stage。** 如果给 launch 传 Epic 或任务列表，它内部运行 `stage --launch`。只有已 stage 的 convoy 获得快速路径。
- **波次计算是信息性的。** 波次在 stage 时计算用于显示。运行时派发使用守护进程每个周期的 `isIssueBlocked` 检查，这更动态。
- **你不能将开放的 convoy un-stage。** 一旦 launched，convoy 不能回到 staged 状态。`open → staged:*` 转换被拒绝。

## 关键源文件

| 文件 | 功能 |
|------|------|
| `internal/convoy/operations.go` | 核心喂养：`CheckConvoysForIssue`、`feedNextReadyIssue`、`IsSlingableType`、`isIssueBlocked` |
| `internal/daemon/convoy_manager.go` | `ConvoyManager` goroutines：`runEventPoll`（5s）、`runStrandedScan`（30s）、`feedFirstReady` |
| `internal/cmd/convoy.go` | 所有 `gt convoy` 子命令 + `findStrandedConvoys` 类型过滤 |
| `internal/cmd/sling.go` | 约 242 行的批量检测、自动 rig 解析、弃用警告 |
| `internal/cmd/sling_batch.go` | `runBatchSling`、`resolveRigFromBeadIDs`、`allBeadIDs`、跨 rig 防护 |
| `internal/cmd/sling_convoy.go` | `createAutoConvoy`、`createBatchConvoy`、`printConvoyConflict` |
| `internal/cmd/convoy_stage.go` | `gt convoy stage`：DAG 遍历、波次计算、error/warning 检测、已 stage convoy 创建 |
| `internal/cmd/convoy_launch.go` | `gt convoy launch`：状态转换、通过 `dispatchWave1` 的 Wave 1 派发 |
| `internal/daemon/daemon.go` | 守护进程启动 — 约第 237 行创建 `ConvoyManager` |