# Dog 基础设施：看门狗链与池架构

> Gas Town 中的自主健康监控、恢复和并发关闭舞。

## 概述

Gas Town 使用三层看门狗链进行自主健康监控：

```
Daemon（Go 进程）            <- 机械传输，3 分钟心跳
    |
    +-> Boot（AI agent）     <- 智能分诊，每次 tick 全新
            |
            +-> Deacon（AI agent）  <- 持续巡逻，长时运行
                    |
                    +-> Witnesses & Refineries  <- 按 rig 的 agent
```

**关键洞察**：Daemon 是机械的（不能推理），但健康决策需要
智能（agent 是卡住了还是只是在思考？）。Boot 弥补了这个差距。

## 设计理据：为什么是两个 Agent？

### 问题

Daemon 需要确保 Deacon 健康，但：

1. **Daemon 不能推理** — 它是遵循 ZFC 原则的 Go 代码（不对其他 agent
   推理）。它能检查"会话是否存活？"但不能检查"agent 是否卡住了？"

2. **唤醒消耗上下文** — 每次生成 AI agent，都会消耗上下文
   token。在空闲的 town 中，每 3 分钟唤醒 Deacon 浪费资源。

3. **观察需要智能** — 区分"agent 正在组合大型工件"与
   "agent 在工具提示处挂起"需要推理。

### 解决方案：Boot 作为分诊

Boot 是一个狭窄的、临时性的 AI agent：
- 每个 daemon tick 全新运行（无累积上下文债务）
- 做出单一决策：Deacon 是否应该被唤醒？
- 决定后立即退出

这使我们在不持续运行完整 AI 的情况下获得智能分诊。

### 为什么不把 Boot 合并到 Deacon 中？

我们可以让 Deacon 处理自己的"我是否应该被唤醒？"逻辑，但：

1. **Deacon 无法观察自身** — 挂起的 Deacon 无法检测到自己挂了
2. **上下文累积** — Deacon 持续运行；Boot 每次重新启动
3. **空闲 town 的成本** — Boot 仅在运行时消耗 token；Deacon
   如果保持存活则持续消耗 token

## 会话所有权

| Agent | 会话名 | 位置 | 生命周期 |
|-------|--------|------|----------|
| Daemon | （Go 进程） | `~/gt/daemon/` | 持久，自动重启 |
| Boot | `gt-boot` | `~/gt/deacon/dogs/boot/` | 临时，每次 tick 全新 |
| Deacon | `hq-deacon` | `~/gt/deacon/` | 长时运行，handoff 循环 |

**关键**：Boot 在 `gt-boot` 中运行，不是 `hq-deacon`。这防止 Boot
与正在运行的 Deacon 会话冲突。

## 心跳机制

### Daemon 心跳（3 分钟）

Daemon 每 3 分钟运行一次心跳 tick：

```go
func (d *Daemon) heartbeatTick() {
    d.ensureBootRunning()           // 1. 生成 Boot 进行分诊
    d.checkDeaconHeartbeat()        // 2. 双保险回退
    d.ensureWitnessesRunning()      // 3. Witness 健康（直接检查 tmux）
    d.ensureRefineriesRunning()     // 4. Refinery 健康（直接检查 tmux）
    d.processLifecycleRequests()    // 5. 循环/重启请求
    // Agent 状态从 tmux 推导，不记录在 beads 中（gt-zecmc）
}
```

### Deacon 心跳（持续）

Deacon 在每个巡逻周期开始时更新 `~/gt/deacon/heartbeat.json`：

```json
{
  "timestamp": "2026-01-02T18:30:00Z",
  "cycle": 42,
  "last_action": "health-scan",
  "healthy_agents": 3,
  "unhealthy_agents": 0
}
```

### 心跳新鲜度

| 年龄 | 状态 | Boot 动作 |
|------|------|-----------|
| < 5 分钟 | 新鲜 | 无事（Deacon 活跃） |
| 5-15 分钟 | 过时 | 如有待处理邮件则催促 |
| > 15 分钟 | 非常过时 | 唤醒（Deacon 可能卡住） |

## Boot 决策矩阵

Boot 运行时，它会观察：
- Deacon 会话是否存活？
- Deacon 的心跳有多旧？
- 是否有待处理的 Deacon 邮件？
- Deacon 的 tmux 窗格里有什么？

然后决策：

| 条件 | 动作 | 命令 |
|------|------|------|
| 会话已死 | START | 退出；daemon 调用 `ensureDeaconRunning()` |
| 心跳 > 15 分钟 | WAKE | `gt nudge deacon "Boot wake: check your inbox"` |
| 心跳 5-15 分钟 + 有邮件 | NUDGE | `gt nudge deacon "Boot check-in: pending work"` |
| 心跳新鲜 | NOTHING | 静默退出 |

## Handoff 流程

### Deacon Handoff

Deacon 运行持续的巡逻周期。在 N 个周期或高上下文后：

```
巡逻周期结束：
    |
    +- 将 wisp 压缩为 digest（临时 → 持久）
    +- 写入摘要到 molecule 状态
    +- gt handoff -s "Routine cycle" -m "Details"
        |
        +- 为下一会话创建邮件
```

下一次 daemon tick：
```
Daemon -> ensureDeaconRunning()
    |
    +- 在 gt-deacon 中生成全新 Deacon
        |
        +- SessionStart hook: gt mail check --inject
            |
            +- 前一次 handoff 邮件被注入
                |
                +- Deacon 读取并继续
```

### Boot Handoff（罕见）

Boot 是临时性的 — 每次 tick 后退出。不需要持久 handoff。

不过，Boot 使用标记文件防止重复生成：
- 标记：`~/gt/deacon/dogs/boot/.boot-running`（TTL：5 分钟）
- 状态：`~/gt/deacon/dogs/boot/.boot-status.json`（上次动作/结果）

如果标记存在且最近，daemon 跳过该 tick 的 Boot 生成。

## 降级模式

当 tmux 不可用时，Gas Town 进入降级模式：

| 能力 | 正常 | 降级 |
|------|------|------|
| Boot 运行 | tmux 中作为 AI | 作为 Go 代码（机械） |
| 观察窗格 | 是 | 否 |
| 催促 agent | 是 | 否 |
| 启动 agent | tmux 会话 | 直接生成 |

降级 Boot 分诊纯粹是机械的：
- 会话已死 → 启动
- 心跳过时 → 重启
- 没有推理，只有阈值

## 回退链

多层确保恢复：

1. **Boot 分诊** — 智能观察，第一道防线
2. **Daemon checkDeaconHeartbeat()** — 双保险，如果 Boot 失败
3. **基于 Tmux 的发现** — Daemon 直接检查 tmux 会话（无 bead 状态）
4. **人类升级** — 向 overseer 发送邮件处理不可恢复的状态

---

## Dog 池架构

当多个死亡令状被发出时，Boot 需要并发运行多个关闭舞 molecule。
所有令状需要并发追踪、独立超时和独立结果。

### 设计决策：轻量级状态机

关闭舞不需要 Claude 会话。关闭舞是一个确定性状态机：

```
WARRANT -> INTERROGATE -> EVALUATE -> PARDON|EXECUTE
```

每一步都是机械的：
1. 发送 tmux 消息（不需要 LLM）
2. 等待超时或响应（定时器）
3. 检查 tmux 输出中的 ALIVE 关键词（字符串匹配）
4. 重复或终止

**决策**：Dog 是轻量级 Go 协程，不是 Claude 会话。

### 架构概览

```
+-----------------------------------------------------------------+
|                             BOOT                                |
|                     （tmux 中的 Claude 会话）                    |
|                                                                 |
|  +-----------------------------------------------------------+ |
|  |                      Dog Manager                           | |
|  |                                                            | |
|  |   Pool: [Dog1, Dog2, Dog3, ...]  （goroutine + 状态）    | |
|  |                                                            | |
|  |   allocate() -> Dog                                        | |
|  |   release(Dog)                                             | |
|  |   status() -> []DogStatus                                  | |
|  +-----------------------------------------------------------+ |
|                                                                 |
|  Boot 的职责：                                                  |
|  - 监视令状（文件或事件）                                       |
|  - 从池中分配 dog                                              |
|  - 监控 dog 进度                                               |
|  - 处理 dog 完成/失败                                          |
|  - 报告结果                                                    |
+-----------------------------------------------------------------+
```

### Dog 结构

```go
// Dog 代表一个关闭舞执行器
type Dog struct {
    ID        string            // 唯一 ID（例如 "dog-1704567890123"）
    Warrant   *Warrant          // 正在处理的死亡令状
    State     ShutdownDanceState
    Attempt   int               // 当前审问尝试（1-3）
    StartedAt time.Time
    StateFile string            // 持久状态：~/gt/deacon/dogs/active/<id>.json
}

type ShutdownDanceState string

const (
    StateIdle          ShutdownDanceState = "idle"
    StateInterrogating ShutdownDanceState = "interrogating"  // 已发送消息，等待中
    StateEvaluating    ShutdownDanceState = "evaluating"     // 检查响应
    StatePardoned      ShutdownDanceState = "pardoned"       // 会话已响应
    StateExecuting     ShutdownDanceState = "executing"      // 正在杀死会话
    StateComplete      ShutdownDanceState = "complete"       // 完成，准备清理
    StateFailed        ShutdownDanceState = "failed"         // Dog 崩溃/出错
)

type Warrant struct {
    ID        string    // 令状的 Bead ID
    Target    string    // 要审问的会话（例如 "gt-gastown-Toast"）
    Reason    string    // 令状发布原因
    Requester string    // 谁提交了令状
    FiledAt   time.Time
}
```

### 池设计

**决策**：固定 5 个 dog 的池，可通过环境变量配置（`GT_DOG_POOL_SIZE`）。

理据：
- 动态调整大小增加复杂性但没有明确收益
- 5 个并发关闭舞可处理最坏情况
- 如果池耗尽，令状排队（优于无限 dog 生成）
- 内存占用可忽略（goroutine + 小状态文件）

```go
const (
    DefaultPoolSize = 5
    MaxPoolSize     = 20
)

type DogPool struct {
    mu       sync.Mutex
    dogs     []*Dog           // 池中所有 dog
    idle     chan *Dog        // 可用 dog 的通道
    active   map[string]*Dog  // ID -> Dog，活跃的 dog
    stateDir string           // ~/gt/deacon/dogs/active/
}
```

### 关闭舞状态机

```
                    +------------------------------------------+
                    |                                          |
                    v                                          |
    +----------------------------+                            |
    |     INTERROGATING          |                            |
    |                            |                            |
    |  1. 发送健康检查           |                            |
    |  2. 启动超时定时器         |                            |
    +-------------+--------------+                            |
                  |                                            |
                  | 超时或响应                                 |
                  v                                            |
    +----------------------------+                            |
    |      EVALUATING            |                            |
    |                            |                            |
    |  检查 tmux 输出中的        |                            |
    |  ALIVE 关键词              |                            |
    +-------------+--------------+                            |
                  |                                            |
          +-------+-------+                                   |
          |               |                                   |
          v               v                                   |
     [找到 ALIVE]     [未找到 ALIVE]                           |
          |               |                                   |
          |               | attempt < 3?                       |
          |               +-----------------------------------+
          |               | 是: attempt++, 更长超时
          |               |
          |               | 否: attempt == 3
          v               v
      +---------+    +-----------+
      | PARDONED|    | EXECUTING |
      |         |    |           |
      | 取消    |    | 杀死 tmux |
      | 令状    |    | 会话      |
      +----+----+    +-----+-----+
           |               |
           +-------+-------+
                   |
                   v
          +----------------+
          |    COMPLETE    |
          |                |
          |  写入结果      |
          |  释放 dog      |
          +----------------+
```

### 超时门控

| 尝试 | 超时 | 累计等待 |
|------|------|----------|
| 1    | 60s  | 60s      |
| 2    | 120s | 180s（3 分钟） |
| 3    | 240s | 420s（7 分钟） |

### 健康检查消息

```
[DOG] HEALTH CHECK: 会话 {target}，在 {timeout}s 内响应 ALIVE，否则将被终止。
令状原因：{reason}
提交者：{requester}
尝试：{attempt}/3
```

### 与现有 Dog 的集成

现有的 `dog` 包（`internal/dog/`）管理 Deacon 的多 rig 辅助 dog。
这些与关闭舞 dog 是不同的：

| 方面          | 辅助 Dog（现有）            | 舞蹈 Dog（新增）            |
|---------------|---------------------------|---------------------------|
| 用途          | 跨 rig 基础设施            | 关闭舞执行                |
| 会话          | Claude 会话               | Goroutine（无 Claude）    |
| Worktree      | 每个 rig 一个              | 无                        |
| 生命周期      | 长寿命，可复用              | 每个令状临时              |
| 状态          | idle/working              | 舞蹈状态机               |

**建议**：使用不同的包以避免混淆：
- `internal/dog/` - 现有辅助 dog
- `internal/shutdown/` - 关闭舞池

## 故障处理

### Dog 在舞蹈中崩溃

如果 dog 崩溃（Boot 进程重启、系统崩溃）：

1. 状态文件持久存在于 `~/gt/deacon/dogs/active/`
2. Boot 重启时，扫描孤立的状态文件
3. 根据状态恢复或重启：

| 状态            | 恢复动作                        |
|-----------------|---------------------------------|
| interrogating   | 从当前尝试重启                  |
| evaluating      | 检查响应，继续                  |
| executing       | 验证杀死，标记完成              |
| pardoned/complete| 已完成，清理                   |

```go
func (p *DogPool) RecoverOrphans() error {
    files, _ := filepath.Glob(p.stateDir + "/*.json")
    for _, f := range files {
        state := loadDogState(f)
        if state.State != StateComplete && state.State != StatePardoned {
            dog := p.allocateForRecovery(state)
            go dog.Resume()
        }
    }
    return nil
}
```

### 处理池耗尽

如果所有 dog 都在忙碌时新令状到达，令状排队等待
稍后处理。当一个 dog 完成并被释放时，检查队列
中是否有待处理的令状。

## 目录结构

```
~/gt/
├── daemon/
│   ├── daemon.log              # Daemon 活动日志
│   └── daemon.pid              # Daemon 进程 ID
├── deacon/
│   ├── heartbeat.json          # Deacon 新鲜度（每个巡逻周期更新）
│   ├── health-check-state.json # Agent 健康追踪（gt deacon health-check）
│   └── dogs/
│       ├── boot/               # Boot 的工作目录
│       │   ├── CLAUDE.md       # Boot 上下文
│       │   ├── .boot-running   # Boot 进行中标记（TTL：5 分钟）
│       │   └── .boot-status.json # Boot 上次动作/结果
│       ├── active/             # 活跃 dog 状态文件
│       │   ├── dog-123.json
│       │   └── ...
│       ├── completed/          # 已完成舞蹈记录（用于审计）
│       │   └── dog-789.json
│       └── warrants/           # 待处理令状队列
│           └── warrant-abc.json
```

## 调试

```bash
# 检查 Deacon 心跳
cat ~/gt/deacon/heartbeat.json | jq .

# 检查 Boot 状态
cat ~/gt/deacon/dogs/boot/.boot-status.json | jq .

# 查看 daemon 日志
tail -f ~/gt/daemon/daemon.log

# 手动 Boot 运行
gt boot triage

# 手动 Deacon 健康检查
gt deacon health-check

# Dog 池状态
gt dog pool status

# 查看活跃关闭舞
gt dog dances

# 查看令状队列
gt dog warrants
```

## 常见问题

### Boot 在错误的会话中生成

**症状**：Boot 在 `hq-deacon` 而非 `gt-boot` 中运行
**原因**：生成代码中的会话名混淆
**修复**：确保 `gt boot triage` 指定 `--session=gt-boot`

### 僵尸会话阻止重启

**症状**：tmux 会话存在但 Claude 已死
**原因**：Daemon 检查会话存在，不检查进程健康
**修复**：在重新创建前杀死僵尸会话：`gt session kill hq-deacon`

### 状态显示错误

**症状**：`gt status` 显示 agent 的错误状态
**原因**：之前 bead 状态和 tmux 状态可能分歧
**修复**：自 gt-zecmc 起，状态从 tmux 直接推导（可观察条件
如 running/stopped 不再使用 bead 状态）。不可观察的状态（stuck、awaiting-gate）
仍然存储在 beads 中。

## 摘要

看门狗链提供自主恢复：

- **Daemon**：机械心跳，生成 Boot
- **Boot**：智能分诊，决定 Deacon 命运
- **Deacon**：持续巡逻，监控 worker

Boot 之所以存在，是因为 daemon 不能推理而 Deacon 无法观察自身。
这种分离增加了复杂性但实现了：

1. **智能分诊**，无持续 AI 成本
2. 每次分诊决策的**全新上下文**
3. tmux 不可用时的**优雅降级**
4. **多层回退**保证可靠性

Dog 池通过并发关闭舞扩展了这一架构 — 轻量级
Go 状态机执行令状，无需消耗 Claude 会话。