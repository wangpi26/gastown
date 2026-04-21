# OpenTelemetry 架构

## 概述

Gas Town 使用 OpenTelemetry (OTel) 对所有 agent 操作进行结构化可观测。遥测通过标准 OTLP HTTP 发射到任何兼容后端（指标、日志）。

**后端无关设计**：系统发射标准 OpenTelemetry Protocol (OTLP) — 任何 OTLP v1.x+ 兼容后端可以消费它。您**不必**使用 VictoriaMetrics/VictoriaLogs；这些只是开发默认值。其他选项包括 Grafana Cloud、Datadog、Honeycomb、SigNoz 或任何 OTLP 接收器。

生产部署指南见 [DEPLOYMENT.md](../../deployment/DEPLOYMENT.md)。

---

## 架构图

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                                Gas Town Town                                     │
│                                                                                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐    │
│  │  Mayor   │  │  Deacon  │  │ Witness  │  │ Polecats │  │    Refinery      │    │
│  │ (daemon) │  │ (daemon) │  │(witness) │  │(workers) │  │   (daemon)      │    │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────────┬─────────┘    │
│       │              │              │              │                 │               │
│       └──────────────┴──────────────┴──────────────┴─────────────────┘               │
│                                     │                                             │
│                                     ▼                                             │
│                        ┌────────────────────────┐                                 │
│                        │   GT Telemetry SDK     │                                 │
│                        │  (Go SDK wrapper)      │                                 │
│                        └───────────┬────────────┘                                 │
│                                    │                                              │
│                            ┌───────┴───────┐                                      │
│                            │               │                                      │
│                            ▼               ▼                                      │
│                   ┌──────────────┐ ┌──────────────┐                              │
│                   │  OTLP Logs   │ │ OTLP Metrics  │                              │
│                   │  Exporter    │ │  Exporter      │                              │
│                   └──────┬───────┘ └──────┬───────┘                              │
│                          │                │                                       │
└──────────────────────────┼────────────────┼───────────────────────────────────────┘
                           │                │
                    ┌──────┴────────────────┴──────┐
                    │        OTLP HTTP             │
                    │   (localhost:9431/metrics,    │
                    │    localhost:9432/logs)       │
                    └──────┬────────────────┬──────┘
                           │                │
                           ▼                ▼
                 ┌──────────────┐  ┌──────────────┐
                 │  Victoria    │  │  Victoria     │
                 │  Metrics     │  │  Logs         │
                 │  (dev 默认)  │  │  (dev 默认)   │
                 └──────────────┘  └──────────────┘
```

### 组件说明

| 组件 | 角色 | 技术 |
|-----------|------|------------|
| **GT Telemetry SDK** | Go SDK 包装器，为 GT 遥测事件提供类型安全 API | `internal/otel/` |
| **OTLP 日志导出器** | 发送日志记录到 OTLP 接收器 | 标准 OTel Go SDK |
| **OTLP 指标导出器** | 发送指标计数器到 OTLP 接收器 | 标准 OTel Go SDK |
| **OTLP HTTP** | 开放网络传输协议 | HTTP/JSON protobuf |
| **VictoriaMetrics** | 指标存储（开发默认） | 开源，与 Prometheus 兼容 |
| **VictoriaLogs** | 日志存储（开发默认） | 开源，与 OTLP 兼容 |

---

## GT Telemetry SDK

### 设计目标

GT Telemetry SDK 封装标准 OTel Go SDK 以提供：
1. Gas Town 特定的类型安全事件发射
2. 自动 `run.id` 注入到所有日志记录
3. 一致的属性命名和值格式
4. 配置驱动的导出器设置
5. 零依赖导入（`internal/` 包，无外部 Go 模块依赖）

### 包结构

```
internal/otel/
├── sdk.go              # 初始化，导出器设置
├── events.go           # 事件类型和发射函数
├── attributes.go       # 属性键定义和辅助函数
├── config.go           # 配置加载
└── otel_test.go        # 测试
```

### 初始化

```go
// 启动时
func Init(cfg Config) error {
    // 1. 从属性层创建 OTel 资源
    // 2. 设置 OTLP 导出器（日志 + 指标）
    // 3. 注册全局 provider
    // 4. 启动后台指标收集
}

// 关闭时
func Shutdown() error {
    // 1. 刷新待处理遥测
    // 2. 关闭导出器
    // 3. 清理资源
}
```

### 配置

配置来自 Gas Town 属性层（见 [property-layers.md](../property-layers.md)）：

| 键 | 默认值 | 描述 |
|-----|---------|-------------|
| `otel.enabled` | `false` | 启用/禁用 OTel 遥测 |
| `otel.metrics_endpoint` | `http://localhost:9431/opentelemetry/v1/metrics` | OTLP 指标端点 |
| `otel.logs_endpoint` | `http://localhost:9432/opentelemetry/v1/logs` | OTLP 日志端点 |
| `otel.export_timeout` | `10s` | 单次导出最大持续时间 |
| `otel.batch_size` | `512` | 批处理前最大记录数 |
| `otel.export_interval` | `5s` | 最大导出间隔 |
| `otel.town_name` | *(当前目录名)* | 用于 `town.name` 属性的 Town 名称 |
| `otel.rig_name` | *(来自 rig 配置)* | 用于 `rig.name` 属性的 Rig 名称 |

---

## 事件发射

### 事件结构

每个遥测事件由以下组成：

1. **日志记录** — 带有完整结构化属性的详细事件记录
2. **指标计数器** — 用于聚合的计数器增量

两者从同一事件函数发射，确保日志和指标之间的一致性。

### 事件类别

| 类别 | 前缀 | 描述 |
|----------|--------|-------------|
| **Agent 生命周期** | `agent.` | Agent 启动、停止、崩溃、重启 |
| **分派** | `dispatch.` | Bead 分派、sling 执行 |
| **Convoy** | `convoy.` | Convoy 创建、完成、停滞 |
| **Refinery** | `refinery.` | 合并队列、MR 处理 |
| **Daemon** | `daemon.` | 心跳、巡逻步骤、观察者 |
| **Git** | `git.` | 推送、合并、冲突 |
| **Mail** | `mail.` | 邮件发送、投递、升级 |
| **Scheduler** | `scheduler.` | 调度、分派循环、断路器 |

### 标准属性

所有事件共享这些标准属性：

| 属性 | 来源 | 描述 |
|-----------|--------|-------------|
| `town.name` | 配置 | Town 名称 |
| `rig.name` | 配置 | Rig 名称 |
| `run.id` | 自动 | 唯一运行标识符（注入所有记录） |
| `agent.role` | 进程 | Agent 角色（mayor、witness 等） |
| `agent.pid` | 进程 | 进程 ID |
| `session.id` | 进程 | 会话 ID |

---

## Run ID 关联

### 概述

`run.id` 是在 agent 启动时生成的唯一标识符，注入到该进程发射的所有遥测记录中。这实现了：
- 跟踪特定 agent 运行的所有活动
- 将日志和指标关联到同一执行上下文
- 通过 `run.id` 过滤在日志后端查询 agent 历史

### 实现

PR #2199（`otel-p0-work-context`）中实现：

| 组件 | 机制 |
|-----------|-----------|
| **生成** | agent 启动时 `uuid.New()` |
| **存储** | 进程全局 `sync.Once` 单例 |
| **注入** | OTel 日志处理器将 `run.id` 添加到所有记录属性 |
| **传播** | 对同一进程自动（全局状态） |

### 查询示例

```logql
# VictoriaLogs：查找特定 agent 运行的所有事件
{run.id="550e8400-e29b-41d4-a716-446655440000"}

# VictoriaLogs：查找给定 town 的所有 agent 运行
{town.name="gastown"} | select run.id, agent.role

# VictoriaMetrics：按 run.id 计数事件
count by (run.id) ({__name__=~"gt_.*_total"})
```

---

## 数据保留

| 数据类型 | 保留期 | 存储 | 原因 |
|-----------|-----------|--------|--------|
| 日志记录 | 30 天 | VictoriaLogs | 操作调试，事件调查 |
| 指标计数器 | 90 天 | VictoriaMetrics | 趋势分析，容量规划 |
| 聚合指标 | 365 天 | VictoriaMetrics | 长期趋势，年度回顾 |

保留期通过 VictoriaMetrics `retentionPeriod` 和 VictoriaLogs `retentionPeriod` 配置标志配置。

---

## 部署架构

### 开发（默认）

```
Agent 进程 → OTLP HTTP → VictoriaMetrics + VictoriaLogs (localhost)
```

开发环境的 Docker Compose：
```yaml
# docker-compose.otel.yml
services:
  victoriametrics:
    image: victoriametrics/victoriametrics:latest
    ports:
      - "9431:8428"
    command:
      - "--opentelemetry.forceTemporalityPreference=delta"
      - "--retentionPeriod=30d"

  victorialogs:
    image: victoriametrics/victoria-logs:latest
    ports:
      - "9432:9428"
    command:
      - "--retentionPeriod=30d"
```

### 生产（替代后端）

```
Agent 进程 → OTLP HTTP → OTel Collector → [任何 OTLP 兼容后端]
```

OTel Collector 提供路由、批处理和与后端无关的交付。

### 后端替换

用任何 OTLP v1.x+ 兼容后端替换 VictoriaMetrics/VictoriaLogs：

| 后端 | 指标 | 日志 | 配置 |
|----------|--------|------|--------|
| Grafana Cloud | 是 | 是 | 设置 `otel.metrics_endpoint` 和 `otel.logs_endpoint` 为 Grafana OTLP 端点 |
| Datadog | 是 | 是 | 使用 OTel Collector 带 Datadog 导出器 |
| Honeycomb | 是 | 是 | 设置端点为 Honeycomb OTLP API |
| SigNoz | 是 | 是 | 设置端点为 SigNoz OTLP 接收器 |
| New Relic | 是 | 是 | 设置端点为 New Relic OTLP 端点 |

---

## 日志查询模式

### 常见查询

```logql
# 所有 witness 活动
{agent.role="witness"}

# 特定 bead 的事件
{bead.id="gt-abc123"}

# 特定 convoy 的事件
{convoy.id="hq-cv-abc"}

# 特定 agent 运行的事件
{run.id="550e8400-e29b-41d4-a716-446655440000"}

# 过去 1 小时的分派失败
{event.name="dispatch.bead.failed"} | after=1h

# 按 convoy 的合并冲突
{event.name="git.merge.conflict"} | select convoy.id, bead.id
```

### 指标查询

```metricsql
# 每分钟分派速率
rate(gt_dispatch_bead_total[5m])

# 按角色的活跃 agent
gt_agent_active_count by (agent.role)

# 按 convoy 的完成率
gt_convoy_completed_total / gt_convoy_created_total by (convoy.id)

# 按步骤的 daemon 心跳
rate(gt_daemon_heartbeat_total[5m]) by (daemon.step)
```

---

## 安全考量

### 数据分类

| 数据类别 | 示例 | 处理 |
|----------------|-----------|---------------|
| **操作元数据** | Agent 角色、rig 名称、bead ID | 安全记录，标准保留 |
| **命令输出** | Git 状态、bd 输出 | 清理 PII，记录摘要 |
| **错误消息** | 失败描述、堆栈跟踪 | 记录以进行调试，30 天保留 |
| **用户内容** | 代码 diff、PR 描述 | **从不记录** — 代理可见，遥测不可见 |

### PII 处理

- 遥测不包含用户内容（代码、PR 描述、issue 文本）
- 错误消息可能包含文件路径 — 在生产环境进行路径清理
- Agent PID 和会话 ID 是操作标识符，非 PII
- Town 和 rig 名称是配置值，非 PII

### 访问控制

- 开发：遥测端点仅 localhost
- 生产：OTLP 端点受网络策略保护
- 日志后端：只读访问用于调试，管理访问用于保留配置

---

## 故障模式

| 故障 | 影响 | 检测 | 恢复 |
|---------|--------|-----------|----------|
| OTLP 端点不可达 | 丢失遥测，agent 继续 | 导出器错误日志 | 端点恢复时自动重新连接 |
| 后端存储满 | 丢失新遥测 | 后端健康检查 | 增加存储或缩短保留 |
| 高延迟导出 | 延迟遥测交付 | 导出超时 | 增加超时或批处理大小 |
| 配置错误 | 无遥测发射 | `gt doctor --check otel` | 修复配置值 |

**关键原则**：遥测故障**绝不**阻塞 agent 操作。所有导出器以 "best-effort" 模式运行 — 导出失败记录但不会停止工作。

---

## 与现有系统的集成

### Daemon 心跳

Daemon 心跳已经发出结构化日志。OTel 增强添加：
- 每个心跳步骤的指标计数器
- 标准化属性（town.name、rig.name、run.id）
- OTLP 导出而非仅本地日志

### Witness 监控

Witness 巡逻循环发出 agent 生命周期事件。OTel 增强：
- 标准化事件格式
- 指标计数器用于 polecat 启动/停止/崩溃
- OTLP 导出用于跨 rig 可见性

### Refinery 合并队列

Refinery 已经记录合并操作。OTel 增强：
- 带属性的合并结果的结构化事件
- 合并延迟、冲突率和队列深度的指标
- OTLP 导出用于管道可见性

---

## 实现阶段

| 阶段 | 内容 | 状态 |
|-------|--------|--------|
| **P0** | SDK 初始化、run.id 注入、核心事件类型 | **已合并**（PR #2199） |
| **P1** | 所有 agent 生命周期事件、daemon 心跳指标 | 计划中 |
| **P2** | Convoy 事件、refinery 指标、mail 指标 | 计划中 |
| **P3** | 自定义仪表板、告警规则、SLI/SLO | 计划中 |

---

## 另见

- [otel-data-model.md](otel-data-model.md) — 完整数据模型，所有事件类型和属性
- [DEPLOYMENT.md](../../deployment/DEPLOYMENT.md) — 生产部署指南
- [property-layers.md](../property-layers.md) — OTel 配置属性层
- [watchdog-chain.md](../watchdog-chain.md) — 使用 OTel 进行心跳监控的 Daemon heartbeat chain