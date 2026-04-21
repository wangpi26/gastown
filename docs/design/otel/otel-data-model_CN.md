# OpenTelemetry 数据模型

Gas Town 发射的所有遥测事件的完整模式。每个事件由以下组成：

1. **日志记录**（→ 任何 OTLP v1.x+ 后端，默认为 VictoriaLogs），带完整结构化属性
2. **指标计数器**（→ 任何 OTLP v1.x+ 后端，默认为 VictoriaMetrics），用于聚合

> **`run.id` 关联**：自动 `run.id` 注入到所有日志记录已在 PR #2199（`otel-p0-work-context`）中实现。每个日志记录包含 `run.id` 属性，无需每个事件类型中显式记录。

---

## 事件格式

### 日志记录结构

每个日志记录包含：

| 字段 | 值 | 描述 |
|-------|-------|-------------|
| `Timestamp` | 事件时间 | 事件发生的精确时间 |
| `ObservedTimestamp` | 收集时间 | OTel SDK 观察到事件的时间 |
| `SeverityNumber` | 变化 | INFO=9，WARN=13，ERROR=17 |
| `SeverityText` | 变化 | "INFO"、"WARN"、"ERROR" |
| `Body` | `event.name` 值 | 事件名称（如 `agent.started`） |
| `Attributes` | 事件特定 | 见下方每个事件类型 |

### 指标计数器结构

每个日志记录伴随一个计数器增量：

| 字段 | 值 | 描述 |
|-------|-------|-------------|
| `Name` | `gt_<event.name>_total` | 指标名称（下划线分隔，`_total` 后缀） |
| `Value` | `+1` | 单调递增计数器 |
| `Attributes` | 与日志相同 | 与日志记录共享的属性 |
| `Type` | `Counter` | 总是单调计数器 |

---

## 标准属性（所有事件）

这些属性存在于每个遥测事件中：

| 属性键 | 类型 | 来源 | 描述 |
|--------------|------|--------|-------------|
| `town.name` | string | 配置 | Gas Town 实例名称 |
| `rig.name` | string | 配置 | Rig 名称 |
| `run.id` | string | 自动 | 唯一运行标识符 |
| `agent.role` | string | 进程 | Agent 角色（mayor、witness、deacon、polecat、refinery） |
| `agent.pid` | int | 进程 | 进程 ID |
| `session.id` | string | 进程 | 会话 ID |
| `event.name` | string | 事件 | 事件类型标识符 |
| `event.category` | string | 事件 | 事件类别 |
| `event.timestamp` | int64 | 事件 | Unix 纳秒时间戳 |

---

## Agent 生命周期事件

### agent.started

Agent 进程启动。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `agent.version` | string | Gas Town 版本 |
| `agent.uptime_seconds` | int | 进程启动后的秒数 |
| `agent.restart_count` | int | 重启次数（来自 watchdog 链） |
| `daemon.watchdog` | string | 监控此 agent 的 watchdog（如 `boot`、`deacon`） |

**严重性**：INFO
**指标**：`gt_agent_started_total`

### agent.stopped

Agent 进程干净停止。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `agent.uptime_seconds` | int | 总运行时间秒数 |
| `agent.stop_reason` | string | 停止原因（`shutdown`、`upgrade`、`manual`） |
| `agent.tasks_completed` | int | 运行期间完成的任务数 |

**严重性**：INFO
**指标**：`gt_agent_stopped_total`

### agent.crashed

Agent 进程崩溃（非干净停止）。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `agent.uptime_seconds` | int | 崩溃前运行时间 |
| `agent.exit_code` | int | 进程退出码 |
| `agent.signal` | string | 接收到的信号（如 `SIGKILL`、`SIGSEGV`） |
| `agent.last_task` | string | 崩溃时正在处理的 bead ID |
| `agent.restart_count` | int | 之前的重启次数 |

**严重性**：ERROR
**指标**：`gt_agent_crashed_total`

### agent.restarted

Agent 被 watchdog 重启。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `agent.restart_count` | int | 重启后的重启次数 |
| `agent.crash_run_id` | string | 崩溃实例的 run.id |
| `daemon.watchdog` | string | 执行重启的 watchdog |
| `agent.restart_delay_seconds` | float64 | 崩溃和重启间秒数 |

**严重性**：WARN
**指标**：`gt_agent_restarted_total`

### agent.heartbeat

周期性心跳事件（daemon 特定）。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `daemon.step` | string | 心跳步骤名称 |
| `daemon.step_number` | int | 步骤号（0-14） |
| `daemon.duration_ms` | int | 步骤持续时间（毫秒） |
| `daemon.step_status` | string | 步骤结果（`success`、`warning`、`error`、`skipped`） |

**严重性**：INFO
**指标**：`gt_agent_heartbeat_total`

### agent.idle

Agent 完成工作并变为空闲。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `agent.idle_reason` | string | 空闲原因（`no_work`、`all_blocked`、`cooldown`） |
| `agent.tasks_completed` | int | 此活跃期间完成的任务数 |
| `agent.active_duration_seconds` | float64 | 从首次任务分派到空闲的秒数 |

**严重性**：INFO
**指标**：`gt_agent_idle_total`

---

## 分派事件

### dispatch.bead.slung

Bead 被 sling 到 polecat（直接分派）。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `bead.id` | string | Bead ID（如 `gt-abc123`） |
| `bead.type` | string | Bead 类型（task、bug、feature） |
| `bead.title` | string | Bead 标题（截断到 200 字符） |
| `convoy.id` | string | Convoy bead ID（如适用） |
| `dispatch.mode` | string | `direct` 或 `deferred` |
| `dispatch.target_rig` | string | 目标 rig 名称 |
| `dispatch.polecat_name` | string | 分配的 polecat 名称 |

**严重性**：INFO
**指标**：`gt_dispatch_bead_slung_total`

### dispatch.bead.scheduled

Bead 被 scheduler 调度（延迟分派）。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `bead.id` | string | Bead ID |
| `bead.type` | string | Bead 类型 |
| `bead.title` | string | Bead 标题（截断到 200 字符） |
| `convoy.id` | string | Convoy bead ID（如适用） |
| `dispatch.mode` | string | `deferred` |
| `dispatch.target_rig` | string | 目标 rig 名称 |
| `scheduler.capacity` | int | 调度时的可用容量 |
| `scheduler.queue_depth` | int | 调度时的队列深度 |

**严重性**：INFO
**指标**：`gt_dispatch_bead_scheduled_total`

### dispatch.bead.dispatched

调度 bead 被 daemon 分派。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `bead.id` | string | Bead ID |
| `dispatch.target_rig` | string | 目标 rig 名称 |
| `dispatch.polecat_name` | string | 分配的 polecat 名称 |
| `scheduler.wait_seconds` | float64 | 从调度到分派的等待时间秒数 |
| `scheduler.batch_size` | int | 此分派周期的批量大小 |
| `scheduler.capacity` | int | 分派时剩余容量 |

**严重性**：INFO
**指标**：`gt_dispatch_bead_dispatched_total`

### dispatch.bead.failed

Bead 分派失败。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `bead.id` | string | Bead ID |
| `dispatch.target_rig` | string | 目标 rig 名称 |
| `dispatch.error` | string | 错误消息（截断到 500 字符） |
| `dispatch.failure_type` | string | 故障类型（`rig_unavailable`、`sling_error`、`tmux_error`、`timeout`） |
| `dispatch.retry_count` | int | 失败尝试次数 |
| `convoy.id` | string | Convoy bead ID（如适用） |

**严重性**：ERROR
**指标**：`gt_dispatch_bead_failed_total`

### dispatch.bead.skipped

Bead 被 scheduler 跳过（断路器或容量）。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `bead.id` | string | Bead ID |
| `dispatch.skip_reason` | string | 跳过原因（`circuit_broken`、`no_capacity`、`already_dispatched`） |
| `dispatch.failure_count` | int | 连续失败计数（如适用） |
| `convoy.id` | string | Convoy bead ID（如适用） |

**严重性**：WARN
**指标**：`gt_dispatch_bead_skipped_total`

---

## Convoy 事件

### convoy.created

创建新 convoy。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `convoy.id` | string | Convoy bead ID |
| `convoy.title` | string | Convoy 标题 |
| `convoy.tracked_count` | int | 跟踪的 issue 数量 |
| `convoy.source` | string | 创建触发器（`sling`、`stage`、`manual`、`formula`） |
| `convoy.epic_id` | string | 源 epic bead ID（如适用） |

**严重性**：INFO
**指标**：`gt_convoy_created_total`

### convoy.issue_closed

跟踪的 issue 在 convoy 内关闭。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `convoy.id` | string | Convoy bead ID |
| `bead.id` | string | 已关闭的 issue bead ID |
| `convoy.progress` | string | 进度作为分数（如 `23/35`） |
| `convoy.progress_pct` | float64 | 进度百分比 |
| `convoy.wave` | int | Issue 所属的当前波次 |

**严重性**：INFO
**指标**：`gt_convoy_issue_closed_total`

### convoy.completed

所有跟踪 issue 完成时 Convoy 关闭。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `convoy.id` | string | Convoy bead ID |
| `convoy.title` | string | Convoy 标题 |
| `convoy.tracked_count` | int | 跟踪的 issue 总数 |
| `convoy.closed_count` | int | 已关闭的 issue 数量 |
| `convoy.skipped_count` | int | 跳过的 issue 数量 |
| `convoy.duration_seconds` | float64 | 从创建到完成的秒数 |
| `convoy.wave_count` | int | 总波次数 |

**严重性**：INFO
**指标**：`gt_convoy_completed_total`

### convoy.stalled

Convoy 检测到停滞（自上次检查以来无进展）。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `convoy.id` | string | Convoy bead ID |
| `convoy.stall_duration_seconds` | float64 | 无进展的秒数 |
| `convoy.active_polecats` | int | 活跃 polecat 数量 |
| `convoy.ready_issues` | int | 准备分派的 issue 数量 |
| `convoy.blocked_issues` | int | 被依赖阻塞的 issue 数量 |
| `convoy.skipped_issues` | int | 跳过的 issue 数量 |

**严重性**：WARN
**指标**：`gt_convoy_stalled_total`

---

## Refinery 事件

### refinery.merge.started

Refinery 开始处理 MR。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `mr.bead_id` | string | MR bead ID |
| `mr.branch` | string | 源分支名称 |
| `mr.target_branch` | string | 目标分支名称 |
| `mr.convoy_id` | string | 关联的 convoy（如适用） |
| `mr.bead_age_seconds` | float64 | MR bead 创建以来的秒数 |

**严重性**：INFO
**指标**：`gt_refinery_merge_started_total`

### refinery.merge.completed

MR 成功合并。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `mr.bead_id` | string | MR bead ID |
| `mr.branch` | string | 源分支名称 |
| `mr.target_branch` | string | 目标分支名称 |
| `mr.merge_duration_seconds` | float64 | 合并操作持续时间 |
| `mr.conflicts_resolved` | int | 解决的冲突数量 |
| `mr.convoy_id` | string | 关联的 convoy（如适用） |

**严重性**：INFO
**指标**：`gt_refinery_merge_completed_total`

### refinery.merge.conflict

合并期间检测到冲突。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `mr.bead_id` | string | MR bead ID |
| `mr.branch` | string | 源分支名称 |
| `mr.conflict_files` | int | 有冲突的文件数量 |
| `mr.conflict_type` | string | 冲突类型（`content`、`delete_modify`、`rename`） |
| `mr.convoy_id` | string | 关联的 convoy（如适用） |

**严重性**：WARN
**指标**：`gt_refinery_merge_conflict_total`

### refinery.merge.failed

合并失败（冲突后或异常）。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `mr.bead_id` | string | MR bead ID |
| `mr.branch` | string | 源分支名称 |
| `mr.error` | string | 错误消息（截断到 500 字符） |
| `mr.failure_type` | string | 故障类型（`conflict`、`build_failure`、`test_failure`、`timeout`） |
| `mr.retry_count` | int | 重试尝试次数 |
| `mr.convoy_id` | string | 关联的 convoy（如适用） |

**严重性**：ERROR
**指标**：`gt_refinery_merge_failed_total`

---

## Daemon 事件

### daemon.heartbeat

Daemon 心跳步骤执行。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `daemon.step` | string | 步骤名称（如 `health_check`、`agent_recovery`、`scheduled_dispatch`） |
| `daemon.step_number` | int | 步骤号（0-14） |
| `daemon.duration_ms` | int | 步骤持续时间（毫秒） |
| `daemon.step_status` | string | 步骤结果（`success`、`warning`、`error`、`skipped`） |
| `daemon.active_agents` | int | 活跃 agent 数量 |

**严重性**：INFO
**指标**：`gt_daemon_heartbeat_total`

### daemon.agent.recovered

Daemon 恢复崩溃的 agent。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `agent.role` | string | 恢复的 agent 角色 |
| `agent.crash_run_id` | string | 崩溃实例的 run.id |
| `agent.new_run_id` | string | 新实例的 run.id |
| `daemon.recovery_type` | string | 恢复类型（`restart`、`reattach`、`replace`） |
| `daemon.recovery_delay_seconds` | float64 | 崩溃和恢复间秒数 |

**严重性**：WARN
**指标**：`gt_daemon_agent_recovered_total`

### daemon.branches.pruned

Daemon 修剪过期的 polecat 分支。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `daemon.branches_pruned` | int | 修剪的分支数量 |
| `daemon.branches_checked` | int | 检查的分支数量 |
| `daemon.oldest_branch_age_days` | int | 最老修剪分支的天数 |

**严重性**：INFO
**指标**：`gt_daemon_branches_pruned_total`

---

## Git 事件

### git.push.succeeded

Git push 成功。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `git.branch` | string | 推送的分支 |
| `git.commits_pushed` | int | 推送的提交数量 |
| `git.remote` | string | 远程名称 |
| `bead.id` | string | 关联的 bead ID |

**严重性**：INFO
**指标**：`gt_git_push_succeeded_total`

### git.push.failed

Git push 失败。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `git.branch` | string | 推送的分支 |
| `git.error` | string | 错误消息 |
| `git.failure_type` | string | 故障类型（`auth`、`network`、`rejected`、`timeout`） |
| `git.retry_count` | int | 重试尝试次数 |
| `bead.id` | string | 关联的 bead ID |

**严重性**：ERROR
**指标**：`gt_git_push_failed_total`

### git.merge.succeeded

Git merge 成功。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `git.source_branch` | string | 源分支 |
| `git.target_branch` | string | 目标分支 |
| `git.merge_strategy` | string | 策略（`merge`、`squash`、`rebase`） |
| `bead.id` | string | 关联的 bead ID |

**严重性**：INFO
**指标**：`gt_git_merge_succeeded_total`

### git.merge.conflict

合并冲突检测到。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `git.source_branch` | string | 源分支 |
| `git.target_branch` | string | 目标分支 |
| `git.conflict_files` | int | 有冲突的文件数量 |
| `bead.id` | string | 关联的 bead ID |

**严重性**：WARN
**指标**：`gt_git_merge_conflict_total`

---

## Mail 事件

### mail.sent

邮件消息发送。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `mail.from` | string | 发送者 agent |
| `mail.to` | string | 收件者 agent |
| `mail.subject` | string | 主题行（截断到 200 字符） |
| `mail.category` | string | 类别（`notification`、`escalation`、`coordination`） |
| `convoy.id` | string | 关联的 convoy（如适用） |

**严重性**：INFO
**指标**：`gt_mail_sent_total`

### mail.escalated

邮件消息标记为升级。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `mail.from` | string | 发送者 agent |
| `mail.to` | string | 收件者 agent |
| `mail.escalation_reason` | string | 升级原因 |
| `mail.escalation_level` | int | 升级级别（1=直接，2=管理者，3=用户） |
| `convoy.id` | string | 关联的 convoy（如适用） |

**严重性**：WARN
**指标**：`gt_mail_escalated_total`

---

## Scheduler 事件

### scheduler.bead.scheduled

Bead 被 scheduler 调度。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `bead.id` | string | Bead ID |
| `scheduler.max_polecats` | int | 配置的最大 polecat |
| `scheduler.queue_depth` | int | 调度后队列深度 |
| `scheduler.capacity` | int | 调度时的可用容量 |
| `convoy.id` | string | 关联的 convoy（如适用） |

**严重性**：INFO
**指标**：`gt_scheduler_bead_scheduled_total`

### scheduler.bead.dispatched

调度 bead 被 daemon 分派。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `bead.id` | string | Bead ID |
| `scheduler.wait_seconds` | float64 | 等待时间 |
| `scheduler.batch_size` | int | 此周期的批量大小 |
| `scheduler.capacity_remaining` | int | 分派后剩余容量 |
| `convoy.id` | string | 关联的 convoy（如适用） |

**严重性**：INFO
**指标**：`gt_scheduler_bead_dispatched_total`

### scheduler.cycle.completed

分派周期完成。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `scheduler.dispatched_count` | int | 本周期分派的 bead 数量 |
| `scheduler.skipped_count` | int | 跳过的 bead 数量 |
| `scheduler.available_capacity` | int | 期初可用容量 |
| `scheduler.queue_depth` | int | 期末队列深度 |
| `scheduler.cycle_duration_ms` | int | 周期持续时间（毫秒） |

**严重性**：INFO
**指标**：`gt_scheduler_cycle_completed_total`

### scheduler.circuit_broken

断路器对 bead 跳闸。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `bead.id` | string | Bead ID |
| `scheduler.failure_count` | int | 连续失败计数 |
| `scheduler.last_failure` | string | 最后一次失败错误消息 |
| `convoy.id` | string | 关联的 convoy（如适用） |

**严重性**：ERROR
**指标**：`gt_scheduler_circuit_broken_total`

### scheduler.paused

Scheduler 暂停。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `scheduler.paused_by` | string | 暂停调度器的 agent |
| `scheduler.queue_depth` | string | 暂停时的队列深度 |

**严重性**：WARN
**指标**：`gt_scheduler_paused_total`

### scheduler.resumed

Scheduler 恢复。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `scheduler.resumed_by` | string | 恢复调度器的 agent |
| `scheduler.paused_duration_seconds` | float64 | 暂停的秒数 |
| `scheduler.queue_depth` | int | 恢复时的队列深度 |

**严重性**：INFO
**指标**：`gt_scheduler_resumed_total`

---

## 运营状态事件

以下事件使用标签模式来传达运营状态。这与 [property-layers.md](../property-layers.md) 中描述的"标签作为状态"模式一致。

### 运营状态标签

| 标签 | 值 | 含义 |
|-------|-------|---------|
| `gt:convoy:launched` | *(存在)* | Convoy 已启动且活跃 |
| `gt:convoy:staged` | *(存在)* | Convoy 已 staged 但未启动 |
| `gt:sling-context` | *(存在)* | Bead 是 sling 上下文（scheduler 调度状态） |
| `gt:mountain` | *(存在)* | Convoy 是 mountain（agent 驱动的停滞检测） |
| `gt:parked` | *(存在)* | Rig 已停驻（无分派） |

这些标签不是遥测事件 — 它们是 bead 元数据。然而，标签的**变更**可以触发遥测事件：

### state.rig.parked

Rig 停驻或取消停驻。

| 属性 | 类型 | 描述 |
|-----------|------|-------------|
| `rig.name` | string | Rig 名称 |
| `rig.parked` | bool | 新的停驻状态 |
| `rig.parked_by` | string | 执行变更的 agent |
| `rig.active_polecats` | int | 变更时的活跃 polecat 数量 |

**严重性**：WARN
**指标**：`gt_state_rig_parked_total`

---

## 属性命名约定

| 规则 | 示例 | 描述 |
|------|---------|-------------|
| 小写，点分隔 | `bead.id`、`convoy.title` | 与 OTel 语义约定一致 |
| 实体前缀 | `agent.*`、`bead.*`、`convoy.*` | 按实体分组 |
| 有限字符串值 | 截断到 500 字符 | 防止超大属性 |
| 非空值 | 错误时使用 `"unknown"` | 防止空字符串属性 |
| 持续时间以秒为单位 | `merge_duration_seconds` | OTel 持续时间的标准单位 |
| 计数是无后缀整数 | `tracked_count`、`commits_pushed` | OTel 计数的标准命名 |

---

## 版本控制

事件模式遵循语义化版本控制：

| 变更 | 版本冲击 |
|--------|---------------|
| 新增事件类型 | MINOR（向后兼容） |
| 将属性添加到现有事件 | MINOR（向后兼容） |
| 重命名事件或属性 | MAJOR（破坏性） |
| 移除事件类型 | MAJOR（破坏性） |
| 更改属性类型 | MAJOR（破坏性） |
| 更改属性语义 | MAJOR（破坏性） |

当前模式版本：`1.0.0`

模式版本包含在每个日志记录的 `event.schema_version` 属性中。

---

## 实现优先级

| 优先级 | 事件类型 | 理由 |
|----------|-------------|-----------|
| **P0** | `agent.started`、`agent.stopped`、`agent.crashed` | Agent 生命周期可见性 |
| **P0** | `dispatch.bead.slung`、`dispatch.bead.failed` | 分派管道可见性 |
| **P1** | `daemon.heartbeat`、`daemon.agent.recovered` | Daemon 健康监控 |
| **P1** | `convoy.created`、`convoy.completed`、`convoy.stalled` | Convoy 生命周期跟踪 |
| **P2** | `refinery.merge.*` | 合并管道可见性 |
| **P2** | `scheduler.*` | 调度器操作可见性 |
| **P3** | `git.*`、`mail.*` | 辅助可见性 |
| **P3** | `state.*` | 运营状态变更跟踪 |