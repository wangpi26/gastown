# 属性层：多级配置

> Gas Town 配置系统的实现指南。
> 创建日期：2025-01-06

## 概述

Gas Town 使用分层属性系统进行配置。属性通过多个层进行查找，较早的层覆盖较晚的层。这同时实现了本地控制和全局协调。

## 四个层

```
┌─────────────────────────────────────────────────────────────┐
│ 1. WISP 层（临时，town 本地）                                │
│    位置：<rig>/.beads-wisp/config/                           │
│    同步：从不                                                 │
│    用途：临时本地覆盖                                         │
└─────────────────────────────┬───────────────────────────────┘
                              │ 如果缺失
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. RIG BEAD 层（持久，全局同步）                             │
│    位置：<rig>/.beads/（rig 身份 bead 标签）                  │
│    同步：通过 git（所有克隆可见）                              │
│    用途：项目级运营状态                                        │
└─────────────────────────────┬───────────────────────────────┘
                              │ 如果缺失
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. TOWN 默认值                                               │
│    位置：~/gt/config.json 或 ~/gt/.beads/                   │
│    同步：不适用（每个 town）                                   │
│    用途：Town 级策略                                          │
└─────────────────────────────┬───────────────────────────────┘
                              │ 如果缺失
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. 系统默认值（编译内置）                                     │
│    用途：无其他指定时的回退                                    │
└─────────────────────────────────────────────────────────────┘
```

## 查找行为

### 覆盖语义（默认）

对于大多数属性，第一个非 nil 值胜出：

```go
func GetConfig(key string) interface{} {
    if val := wisp.Get(key); val != nil {
        if val == Blocked { return nil }
        return val
    }
    if val := rigBead.GetLabel(key); val != nil {
        return val
    }
    if val := townDefaults.Get(key); val != nil {
        return val
    }
    return systemDefaults[key]
}
```

### 堆叠语义（整数）

对于整数属性，wisp 和 bead 层的值**累加**到基数上：

```go
func GetIntConfig(key string) int {
    base := getBaseDefault(key)    // Town 或系统默认值
    beadAdj := rigBead.GetInt(key) // 0 如果缺失
    wispAdj := wisp.GetInt(key)    // 0 如果缺失
    return base + beadAdj + wispAdj
}
```

这允许临时调整而不改变基础值。

### 阻断继承

你可以显式阻断属性的继承：

```bash
gt rig config set gastown auto_restart --block
```

这在 wisp 层创建一个"blocked"标记。即使 rig bead 或默认值说 `auto_restart: true`，查找也返回 nil。

## Rig 身份 Bead

每个 rig 都有一个身份 bead 用于运营状态：

```yaml
id: gt-rig-gastown
type: rig
name: gastown
repo: git@github.com:steveyegge/gastown.git
prefix: gt

labels:
  - status:operational
  - priority:normal
```

这些 bead 通过 git 同步，因此 rig 的所有克隆看到相同的状态。

## 两级 Rig 控制

### 级别 1：Park（本地，临时）

```bash
gt rig park gastown      # 停止服务，daemon 不会重启
gt rig unpark gastown    # 允许服务运行
```

- 存储在 wisp 层（`.beads-wisp/config/`）
- 仅影响此 town
- 清理时消失
- 用途：本地维护、调试

### 级别 2：Dock（全局，持久）

```bash
gt rig dock gastown      # 在 rig bead 上设置 status:docked 标签
gt rig undock gastown    # 移除标签
```

- 存储在 rig 身份 bead 上
- 通过 git 同步到所有克隆
- 永久有效直到显式更改
- 用途：项目级维护、协调停机

### Daemon 行为

Daemon 在自动重启前检查两个级别：

```go
func shouldAutoRestart(rig *Rig) bool {
    status := rig.GetConfig("status")
    if status == "parked" || status == "docked" {
        return false
    }
    return true
}
```

## 配置键

| 键 | 类型 | 行为 | 描述 |
|----|------|------|------|
| `status` | string | 覆盖 | operational/parked/docked |
| `auto_restart` | bool | 覆盖 | Daemon 自动重启行为 |
| `max_polecats` | int | 覆盖 | 最大并发 polecat 数 |
| `priority_adjustment` | int | **堆叠** | 调度优先级修饰符 |
| `maintenance_window` | string | 覆盖 | 允许维护的时间窗口 |
| `dnd` | bool | 覆盖 | 免打扰模式 |

## 命令

### 查看配置

```bash
gt rig config show gastown           # 显示有效配置（所有层）
gt rig config show gastown --layer   # 显示每个值来自哪一层
```

### 设置配置

```bash
# 在 wisp 层设置（本地，临时）
gt rig config set gastown key value

# 在 bead 层设置（全局，持久）
gt rig config set gastown key value --global

# 阻断继承
gt rig config set gastown key --block

# 从 wisp 层清除
gt rig config unset gastown key
```

### Rig 生命周期

```bash
gt rig park gastown          # 本地：停止 + 阻止重启
gt rig unpark gastown        # 本地：允许重启

gt rig dock gastown          # 全局：标记为离线
gt rig undock gastown        # 全局：标记为运营

gt rig status gastown        # 显示当前状态
```

## 示例

### 临时优先级提升

```bash
# 基础优先级：0（来自默认值）
# 给这个 rig 临时优先级提升用于紧急工作

gt rig config set gastown priority_adjustment 10

# 有效优先级：0 + 10 = 10
# 完成后，清除它：

gt rig config unset gastown priority_adjustment
```

### 本地维护

```bash
# 我正在升级本地克隆，不要重启服务
gt rig park gastown

# ... 做维护 ...

gt rig unpark gastown
```

### 项目级维护

```bash
# 重大重构进行中，所有克隆应暂停
gt rig dock gastown

# 通过 git 同步 - 其他 town 看到该 rig 为 docked
bd sync

# 完成后：
gt rig undock gastown
bd sync
```

### 本地阻断自动重启

```bash
# Rig bead 说 auto_restart: true
# 但我正在调试，不希望在这里启用

gt rig config set gastown auto_restart --block

# 现在仅对此 town auto_restart 返回 nil
```

## 实现说明

### Wisp 存储

Wisp 配置存储在 `.beads-wisp/config/<rig>.json`：

```json
{
  "rig": "gastown",
  "values": {
    "status": "parked",
    "priority_adjustment": 10
  },
  "blocked": ["auto_restart"]
}
```

### Rig Bead 标签

Rig 运营状态存储为 rig 身份 bead 上的标签：

```bash
bd label add gt-rig-gastown status:docked
bd label remove gt-rig-gastown status:docked
```

### Daemon 集成

Daemon 的生命周期管理器在启动服务前检查配置：

```go
func (d *Daemon) maybeStartRigServices(rig string) {
    r := d.getRig(rig)

    status := r.GetConfig("status")
    if status == "parked" || status == "docked" {
        log.Info("Rig %s is offline, skipping auto-start", rig)
        return
    }

    d.ensureWitness(rig)
    d.ensureRefinery(rig)
}
```

## 运营状态事件

运营状态变更以事件 bead 形式跟踪，提供不可变的审计轨迹。标签缓存当前状态用于快速查询。

### 事件类型

| 事件类型 | 描述 | 载荷 |
|----------|------|------|
| `patrol.muted` | 巡逻周期已禁用 | `{reason, until?}` |
| `patrol.unmuted` | 巡逻周期已重新启用 | `{reason?}` |
| `agent.started` | Agent 会话已开始 | `{session_id?}` |
| `agent.stopped` | Agent 会话已结束 | `{reason, outcome?}` |
| `mode.degraded` | 系统进入降级模式 | `{reason}` |
| `mode.normal` | 系统恢复正常 | `{}` |

### 创建和查询事件

```bash
# 创建运营事件
bd create --type=event --event-type=patrol.muted \
  --actor=human:overseer --target=agent:deacon \
  --payload='{"reason":"fixing convoy deadlock","until":"gt-abc1"}'

# 查询 agent 的最近事件
bd list --type=event --target=agent:deacon --limit=10

# 通过标签查询当前状态
bd list --type=role --label=patrol:muted
```

### 标签即状态模式

事件捕获完整历史。标签缓存当前状态：

- `patrol:muted` / `patrol:active`
- `mode:degraded` / `mode:normal`
- `status:idle` / `status:working`

状态变更流程：创建事件 bead（不可变），然后更新角色 bead 标签（缓存）。

```bash
# 静音巡逻
bd create --type=event --event-type=patrol.muted ...
bd update role-deacon --add-label=patrol:muted --remove-label=patrol:active
```

### 配置 vs 状态

| 类型 | 存储 | 示例 |
|------|------|------|
| **静态配置** | TOML 文件 | Daemon tick 间隔 |
| **角色指令** | Markdown 文件 | 每个角色的操作员行为策略 |
| **Formula 覆盖** | TOML 文件 | 每步骤 Formula 修改 |
| **运营状态** | Bead（事件 + 标签） | 巡逻已静音 |
| **运行时标记** | 标记文件 | `.deacon-disabled` |

*事件是真相来源。标签是缓存。*

关于 Boot 分诊和降级模式详情，参见 [Watchdog Chain](watchdog-chain.md)。

## 角色指令与 Formula 覆盖

指令和覆盖将属性层模型扩展到 agent 行为。它们遵循与其他配置相同的 rig > town > system 优先级。

### 指令（行为策略）

按角色的 Markdown 文件，在 prime 时修改 agent 行为：

```
系统层：   嵌入的角色模板（编译内置）
                        │ 如果指令存在
                        ▼
TOWN 层：     ~/gt/directives/<role>.md
                        │ 连接
                        ▼
RIG 层：      ~/gt/<rig>/directives/<role>.md
```

Town 和 rig 指令都连接。Rig 内容出现在最后，在冲突中获胜（与 CSS 特异性相同 — 后规则覆盖前规则）。

### 覆盖（Formula 修改）

按 Formula 的 TOML 文件，修改单个步骤：

```
系统层：   嵌入的 Formula（编译内置）
                        │ 如果覆盖存在
                        ▼
TOWN 层：     ~/gt/formula-overlays/<formula>.toml
                        │ rig 完全替换 town
                        ▼
RIG 层：      ~/gt/<rig>/formula-overlays/<formula>.toml
```

与指令不同，覆盖在 rig 级别使用**完全替换** — 如果存在 rig 覆盖，town 覆盖完全被忽略。这防止冲突的步骤修改不可预测地合并。

### 优先级总结

| 配置类型 | Town + Rig 交互 | 理由 |
|----------|----------------|------|
| Rig 属性 | 第一个非 nil 胜出（覆盖） | 标准配置查找 |
| 整数属性 | 值堆叠（累加） | 允许调整 |
| 角色指令 | 连接（rig 在最后） | 累加策略；rig 有最终决定权 |
| Formula 覆盖 | Rig 替换 town | 步骤修改可能冲突；完全替换更安全 |

完整参考（TOML 格式、示例和 `gt doctor` 集成）参见 [directives-and-overlays.md](directives-and-overlays.md)。

## 相关文档

- `~/gt/docs/hop/PROPERTY-LAYERS.md` - 战略架构
- `wisp-architecture.md` - Wisp 系统设计
- `agent-as-bead.md` - Agent 身份 bead（类似模式）
- [directives-and-overlays.md](directives-and-overlays.md) - 完整参考