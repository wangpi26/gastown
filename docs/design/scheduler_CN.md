# Scheduler 架构

> 配置驱动的容量控制 Polecat 分派。

## 快速入门

启用延迟分派并调度一些工作：

```bash
# 1. 启用延迟分派（配置驱动，无每命令标志）
gt config set scheduler.max_polecats 5

# 2. 通过 gt sling 调度工作（当 max_polecats > 0 时自动延迟）
gt sling gt-abc gastown              # 单个任务 bead
gt sling gt-abc gt-def gt-ghi gastown  # 批量任务 bead
gt sling hq-cv-abc                   # Convoy（调度所有跟踪的 issue）
gt sling gt-epic-123                 # Epic（调度所有子项）

# 3. 查看调度状态
gt scheduler status
gt scheduler list

# 4. 手动分派（或让 daemon 处理）
gt scheduler run
gt scheduler run --dry-run    # 先预览
```

### 分派模式

`scheduler.max_polecats` 配置值控制分派行为：

| 值 | 模式 | 行为 |
|-------|------|----------|
| `-1`（默认） | 直接分派 | `gt sling` 立即分派，近乎零开销 |
| `0` | 直接分派 | 与 `-1` 相同 — `gt sling` 立即分派 |
| `N > 0` | 延迟分派 | `gt sling` 创建 sling 上下文 bead，daemon 分派 |

无需每调用标志。相同的 `gt sling` 命令自动适配。

### 常用 CLI

| 命令 | 描述 |
|---------|-------------|
| `gt sling <bead> <rig>` | Sling bead（直接或延迟，取决于配置） |
| `gt sling <bead>... <rig>` | 批量 sling/调度多个 bead |
| `gt sling <convoy-id>` | Sling/调度 convoy 中所有跟踪的 issue |
| `gt sling <epic-id>` | Sling/调度 epic 的所有子项 |
| `gt scheduler status` | 显示 scheduler 状态和容量 |
| `gt scheduler list` | 按 rig 列出所有已调度的 bead |
| `gt scheduler run` | 手动触发分派 |
| `gt scheduler pause` | 暂停 town 范围内所有分派 |
| `gt scheduler resume` | 恢复分派 |
| `gt scheduler clear` | 从 scheduler 移除 bead |

### 最简示例

```bash
gt config set scheduler.max_polecats 5
gt sling gt-abc gastown              # 延迟：创建 sling 上下文 bead
gt scheduler status                  # "Queued: 1 total, 1 ready"
gt scheduler run                     # 分派 → 生成 polecat → 关闭上下文
```

---

## 概述

Scheduler 解决批量 polecat 分派中的**反压**和**容量控制**问题。

没有 scheduler 时，slinging N 个 bead 会同时生成 N 个 polecat，耗尽 API 速率限制、内存和 CPU。Scheduler 引入了一个调节器：bead 进入等待状态，daemon 增量分派它们，遵守可配置的并发上限。

Scheduler 集成到 daemon 心跳中作为**步骤 14** — 在所有 agent 健康检查、生命周期处理和分支修剪之后。这确保系统在生成新工作之前是健康的。

```
Daemon 心跳（每 3 分钟）
    |
    +- 步骤 0-13：健康检查、agent 恢复、清理
    |
    +- 步骤 14：gt scheduler run（容量控制分派）
         |
         +- flock（排他锁）
         +- 检查暂停状态
         +- 加载配置（max_polecats、batch_size）
         +- 计算活跃 polecat（tmux）
         +- 查询 sling 上下文（bd list --label=gt:sling-context）
         +- 与 bd ready 连接以确定未阻塞的 bead
         +- DispatchCycle.Run() — 计划 + 执行 + 报告
         |    +- PlanDispatch(availableCapacity, batchSize, ready)
         |    +- 对每个计划的 bead：Execute → OnSuccess/OnFailure
         +- 唤醒 rig agent（witness、refinery）
         +- 保存分派状态
```

---

## Sling 上下文 Bead

调度状态存储在称为 sling 上下文的**独立临时 bead** 上。工作 bead 永远不会被 scheduler 修改。

每个 sling 上下文 bead：
- 通过 `bd create --ephemeral` 创建，带有标签 `gt:sling-context`
- 具有指向工作 bead 的 `tracks` 依赖
- 将所有调度参数作为 JSON 存储在其描述中
- 当分派成功、bead 被清除或断路器跳闸时关闭

### 为什么使用独立 Bead？

之前的方法将调度元数据存储在工作 bead 的描述中（分隔块），并使用标签（`gt:queued`）作为状态信号。这需要：
- 带回滚的两步写入（先元数据后标签）
- 描述清理以避免分隔符冲突
- 三步分派清理（剥离元数据 + 交换标签 + 重试）
- 自定义键值格式/解析/剥离函数（约 250 行）

Sling 上下文 bead 消除了所有这些：
- **单次原子创建** — `bd create --ephemeral` 是一个操作
- **JSON 格式** — `json.Marshal`/`json.Unmarshal` 替代自定义解析器
- **工作 bead 保持原样** — 无描述修改，无标签操作
- **清晰的生命周期** — 开放上下文 = 已调度，关闭上下文 = 完成

### 上下文字段（JSON）

| 字段 | 类型 | 描述 |
|-------|------|-------------|
| `version` | int | 模式版本（当前为 1） |
| `work_bead_id` | string | 被调度的实际工作 bead |
| `target_rig` | string | 目标 rig 名称 |
| `formula` | string | 分派时应用的 Formula（如 `mol-polecat-work`） |
| `args` | string | 执行者的自然语言指令 |
| `vars` | string | 换行分隔的 Formula 变量（`key=value`） |
| `enqueued_at` | RFC3339 | 调度时间戳 |
| `merge` | string | 合并策略：`direct`、`mr`、`local` |
| `convoy` | string | Convoy bead ID（自动 convoy 创建后设置） |
| `base_branch` | string | 覆盖 polecat worktree 的基础分支 |
| `no_merge` | bool | 完成时跳过合并队列 |
| `account` | string | Claude Code 账户句柄 |
| `agent` | string | Agent/运行时覆盖 |
| `hook_raw_bead` | bool | 无默认 Formula 的 Hook |
| `owned` | bool | 调用者管理的 convoy 生命周期 |
| `mode` | string | 执行模式：`ralph`（每步全新上下文） |
| `dispatch_failures` | int | 连续失败计数（断路器） |
| `last_failure` | string | 最近分派错误消息 |

---

## Bead 状态机

Sling 上下文经历以下状态转换：

```
                                  +------------------+
                                  |                  |
                                  v                  |
          +----------+    分派成功     +--------+ |
 调度 |  CONTEXT  | ----------------> | CLOSED | |
--------> |   OPEN    |                   | (完成) | |
          +----------+                    +--------+ |
                |                                    |
                +-- 3 次失败 --> CLOSED (断路跳闸)
                |
                +-- gt scheduler clear --> CLOSED (已清除)
```

| 状态 | 表示 | 触发 |
|-------|---------------|---------|
| **SCHEDULED** | 开放的 sling 上下文 bead | `scheduleBead()` |
| **DISPATCHED** | 关闭的 sling 上下文（原因："dispatched"） | `dispatchSingleBead()` 成功 |
| **CIRCUIT-BROKEN** | 关闭的 sling 上下文（原因："circuit-broken"） | `dispatch_failures >= 3` |
| **CLEARED** | 关闭的 sling 上下文（原因："cleared"） | `gt scheduler clear` |

关键不变式：工作 bead **永远不被** scheduler 修改。所有状态都在 sling 上下文 bead 上。

---

## 入口点

### CLI 入口点

`gt sling` 自动检测配置和 ID 类型的分派模式：

| 命令 | 直接模式（max_polecats=-1） | 延迟模式（max_polecats>0） |
|---------|-------------------------------|-------------------------------|
| `gt sling <bead> <rig>` | 立即分派 | 调度以供稍后分派 |
| `gt sling <bead>... <rig>` | 批量立即分派 | 批量调度 |
| `gt sling <epic-id>` | `runEpicSlingByID()` — 分派所有子项 | `runEpicScheduleByID()` — 调度所有子项 |
| `gt sling <convoy-id>` | `runConvoySlingByID()` — 分派所有跟踪项 | `runConvoyScheduleByID()` — 调度所有跟踪项 |

`runSling` 中的**检测链**：
1. `shouldDeferDispatch()` — 检查 `scheduler.max_polecats` 配置
2. 批量（3+ 参数，最后一个是 rig）— `runBatchSchedule()` 或 `runBatchSling()`
3. 设置了 `--on` 标志 — formula-on-bead 模式
4. 2 个参数 + 最后一个是 rig — `scheduleBead()` 或内联分派
5. 1 个参数，自动检测类型：epic/convoy/task

所有调度路径通过 `internal/cmd/sling_schedule.go` 中的 `scheduleBead()`。
所有分派通过 `internal/cmd/capacity_dispatch.go` 中的 `dispatchScheduledWork()`。

### Daemon 入口点

Daemon 在每次心跳（步骤 14）调用 `gt scheduler run` 作为子进程：

```go
// internal/daemon/daemon.go
func (d *Daemon) dispatchScheduledWork() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()
    cmd := exec.CommandContext(ctx, "gt", "scheduler", "run")
    cmd.Env = append(os.Environ(), "GT_DAEMON=1", "BD_DOLT_AUTO_COMMIT=off")
    // ...
}
```

| 属性 | 值 |
|----------|-------|
| 超时 | 5 分钟 |
| 环境变量 | `GT_DAEMON=1`（标识 daemon 分派） |
| 门控 | `scheduler.max_polecats > 0`（延迟模式） |

---

## 调度路径

`scheduleBead()` 按顺序执行以下步骤：

1. **验证** bead 存在，rig 存在
2. **跨 rig 守卫** — 拒绝 bead 前缀与目标 rig 不匹配的情况（除非 `--force`）
3. **幂等性** — 跳过已存在该工作 bead 的开放 sling 上下文
4. **状态守卫** — 拒绝 bead 处于 hooked/in_progress 状态（除非 `--force`）
5. **验证 Formula** — 验证 Formula 存在（轻量级，无副作用）
6. **烹饪 Formula** — `bd cook` 在 daemon 分派循环前捕获错误 proto
7. **构建上下文字段** — `SlingContextFields` 结构体，包含所有 sling 参数
8. **创建 sling 上下文** — `bd create --ephemeral` + `bd dep add --type=tracks`（原子）
9. **自动 convoy** — 如果尚未跟踪则创建 convoy，在上下文字段中存储 convoy ID
10. **记录事件** — 为仪表板可见性提供事件

创建是**单次原子操作** — 无两步写入，无需回滚。

---

## 分派引擎

### DispatchCycle

分派循环是一个带有注入回调的通用编排器：

```go
type DispatchCycle struct {
    AvailableCapacity func() (int, error)        // 空闲分派槽位（0=无限）
    QueryPending      func() ([]PendingBead, error) // 符合分派条件的工作项
    Execute           func(PendingBead) error     // 分派单个项
    OnSuccess         func(PendingBead) error     // 分派后清理
    OnFailure         func(PendingBead, error)    // 失败处理
    BatchSize         int
    SpawnDelay        time.Duration
}
```

`Run()` 内部调用 `PlanDispatch(availableCapacity, batchSize, ready)` 以确定分派内容，然后用回调执行每个计划项。

### 分派流程

```
DispatchCycle.Run()
    |
    +- AvailableCapacity() → capacity = maxPolecats - activePolecats
    |
    +- QueryPending() → getReadySlingContexts():
    |    +- bd list --label=gt:sling-context --status=open（所有 rig DB）
    |    +- 从每个上下文 bead 描述解析 SlingContextFields
    |    +- bd ready --json --limit=0（所有 rig DB）→ readyWorkIDs 集合
    |    +- 过滤：WorkBeadID 在 readyWorkIDs 中的上下文 bead
    |    +- 跳过断路跳闸的（dispatch_failures >= 阈值）
    |
    +- PlanDispatch(capacity, batchSize, ready)
    |    +- 返回 DispatchPlan{ToDispatch, Skipped, Reason}
    |
    +- 对每个计划的 bead：
         +- Execute: ReconstructFromContext(fields) → executeSling(params)
         +- OnSuccess: CloseSlingContext(contextID, "dispatched")
         +- OnFailure: 增加 dispatch_failures，更新上下文，可能关闭
         +- sleep(SpawnDelay)
```

### dispatchSingleBead

大幅简化 — 上下文字段已解析：

1. `ReconstructFromContext(b.Context)` → `DispatchParams`，其中 `BeadID = b.WorkBeadID`
2. 调用 `executeSling(params)` — 就这样

分派后清理由回调处理：
- **OnSuccess**：`CloseSlingContext(b.ID, "dispatched")`
- **OnFailure**：增加 `dispatch_failures`，更新上下文 bead，如果断路跳闸则关闭

---

## 容量管理

### 配置

| 键 | 类型 | 默认值 | 描述 |
|-----|------|---------|-------------|
| `scheduler.max_polecats` | *int | `-1` | 最大并发 polecat（-1=直接，0=禁用，N=延迟） |
| `scheduler.batch_size` | *int | `1` | 每心跳 tick 分派的 bead 数 |
| `scheduler.spawn_delay` | string | `"0s"` | 生成间延迟（Dolt 锁争用） |

通过 `gt config set` 设置：

```bash
gt config set scheduler.max_polecats 5    # 启用延迟分派
gt config set scheduler.max_polecats -1   # 直接分派（默认）
gt config set scheduler.batch_size 2
gt config set scheduler.spawn_delay 3s
```

### 分派计数公式

```
toDispatch = min(capacity, batchSize, readyCount)

其中：
  capacity   = maxPolecats - activePolecats（正数 = 那么多槽位，0 或负数 = 无容量）
  batchSize  = scheduler.batch_size（默认 1）
  readyCount = 工作 bead 出现在 bd ready 中的 sling 上下文
```

### 活跃 Polecat 计数

活跃 polecat 通过扫描 tmux 会话并通过 `session.ParseSessionName()` 匹配角色来计数。这计算**所有** polecat（包括 scheduler 分派的和直接 sling 的），因为 API 速率限制、内存和 CPU 是共享资源。

---

## 断路器

断路器防止永久失败的 bead 导致无限重试循环。

| 属性 | 值 |
|----------|-------|
| 阈值 | `maxDispatchFailures = 3` |
| 计数器 | sling 上下文 JSON 中的 `dispatch_failures` 字段 |
| 跳闸动作 | 关闭 sling 上下文（原因："circuit-broken"） |
| 重置 | 无自动重置（需要手动干预） |

### 流程

```
分派尝试失败
    |
    +- 在上下文 bead 中增加 dispatch_failures
    +- 存储 last_failure 错误消息
    |
    +- dispatch_failures >= 3?
         +- 是 -> CloseSlingContext(contextID, "circuit-broken")
         |         （上下文 bead 关闭，工作 bead 不受影响）
         +- 否 -> bead 保持已调度状态，下一周期重试
```

---

## Scheduler 控制

### 暂停 / 恢复

暂停停止 town 范围内所有分派。状态存储在 `.runtime/scheduler-state.json` 中。

```bash
gt scheduler pause    # 设置 paused=true，记录操作者和时间戳
gt scheduler resume   # 清除暂停状态
```

写入是原子的（临时文件 + 重命名），以防止并发写入导致损坏。

### 清除

关闭 sling 上下文 bead，从 scheduler 中移除 bead：

```bash
gt scheduler clear              # 关闭所有 sling 上下文
gt scheduler clear --bead gt-abc  # 关闭特定 bead 的上下文
```

### 状态 / 列表

```bash
gt scheduler status         # 摘要：暂停、排队数、活跃 polecat
gt scheduler status --json  # JSON 输出

gt scheduler list           # 按目标 rig 分组的 bead，带阻塞指示
gt scheduler list --json    # JSON 输出
```

`list` 将 sling 上下文（所有已调度）与 `bd ready`（未阻塞的工作 bead）对账，以标记阻塞的 bead。

---

## Scheduler 与 Convoy 集成

Convoy 和 scheduler 是互补但不同的机制。Convoy 跟踪相关 bead 的完成；scheduler 控制分派容量。分派 convoy 工作存在两条路径：

### 分派路径

| 路径 | 触发 | 容量控制 | 用例 |
|------|---------|-----------------|----------|
| **直接分派** | `gt sling <convoy-id>`（max_polecats=-1） | 无（立即发射） | 默认模式 — 所有 issue 立即分派 |
| **延迟分派** | `gt sling <convoy-id>`（max_polecats>0） | 有（daemon 心跳、max_polecats、batch_size） | 容量控制 — 批量带反压 |

**直接分派**（max_polecats=-1）：`gt sling <convoy-id>` 调用 `runConvoySlingByID()`，通过 `executeSling()` 立即分派所有开放的跟踪 issue。每个 issue 的 rig 从其 bead ID 前缀自动解析。无容量控制 — 所有 issue 立即分派。

**延迟分派**（max_polecats>0）：`gt sling <convoy-id>` 调用 `runConvoyScheduleByID()`，调度所有开放的跟踪 issue（创建 sling 上下文 bead）。Daemon 通过 `gt scheduler run` 增量分派，遵守 `max_polecats` 和 `batch_size`。当大批量同时分派会耗尽资源时使用此方式。

### 何时使用哪种

- **小型 convoy（< 5 个 issue）**：直接分派（默认，max_polecats=-1）
- **大型批量（5+ 个 issue）**：设置 `scheduler.max_polecats` 进行容量控制分派
- **Epic**：相同逻辑 — `gt sling <epic-id>` 从配置自动解析模式

### Rig 解析

`gt sling <convoy-id>` 和 `gt sling <epic-id>` 使用 `beads.ExtractPrefix()` + `beads.GetRigNameForPrefix()` 从每个 bead 的 ID 前缀自动解析目标 rig。Town-root bead（`hq-*`）被跳过并发出警告，因为它们是协调产物，不是可分派的工作。

---

## 安全属性

| 属性 | 机制 |
|----------|-----------|
| **调度幂等性** | 跳过已存在工作 bead 的开放 sling 上下文 |
| **工作 bead 保持原样** | Scheduler 永不修改工作 bead 的描述或标签 |
| **跨 rig 守卫** | 拒绝 bead 前缀与目标 rig 不匹配（除非 `--force`） |
| **分派序列化** | `flock(scheduler-dispatch.lock)` 防止双重分派 |
| **原子调度** | 单次 `bd create --ephemeral` — 无两步写入，无需回滚 |
| **Formula 预烹饪** | 调度时 `bd cook` 在 daemon 分派循环前捕获错误 proto |
| **保存时刷新状态** | 分派在保存前重新读取状态，以避免覆盖并发暂停 |

---

## 代码布局

| 路径 | 用途 |
|------|---------|
| `internal/scheduler/capacity/config.go` | `SchedulerConfig` 类型、默认值、`IsDeferred()` |
| `internal/scheduler/capacity/pipeline.go` | `PendingBead`、`SlingContextFields`、`PlanDispatch()`、`ReconstructFromContext()` |
| `internal/scheduler/capacity/dispatch.go` | `DispatchCycle` 类型 — 通用分派编排器 |
| `internal/scheduler/capacity/state.go` | `SchedulerState` 持久化 |
| `internal/beads/beads_sling_context.go` | Sling 上下文 CRUD（创建、查找、列表、关闭、更新） |
| `internal/cmd/sling.go` | CLI 入口，配置驱动路由 |
| `internal/cmd/sling_schedule.go` | `scheduleBead()`、`shouldDeferDispatch()`、`isScheduled()` |
| `internal/cmd/scheduler.go` | `gt scheduler` 命令树 |
| `internal/cmd/scheduler_epic.go` | Epic 调度/sling 处理器 |
| `internal/cmd/scheduler_convoy.go` | Convoy 调度/sling 处理器 |
| `internal/cmd/capacity_dispatch.go` | `dispatchScheduledWork()`、分派回调连接 |
| `internal/daemon/daemon.go` | 心跳集成（`gt scheduler run`） |

---

## 另见

- [Watchdog Chain](watchdog-chain.md) — Daemon 心跳，scheduler 分派作为步骤 14 运行
- [Convoys](../concepts/convoy.md) — Convoy 跟踪，调度时的自动 convoy
- [Property Layers](property-layers.md) — Scheduler 标签使用的标签即状态模式（见运营状态事件部分）