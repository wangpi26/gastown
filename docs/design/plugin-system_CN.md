# 插件系统设计

> **状态：设计提案 — 尚未实现**
>
> Gas Town 插件系统的设计文档。
> 编写于 2026-01-11，crew/george 会话。

## 问题陈述

Gas Town 需要可扩展的、项目特定的自动化功能，在 Deacon 巡逻周期中运行。直接用例是重建过期的二进制文件（gt、bd、wv），但该模式可推广到任何周期性维护任务。

当前状态：
- 插件基础设施概念上存在（巡逻步骤提及它）
- `~/gt/plugins/` 目录存在，附有 README
- 生产环境中没有实际运行的插件
- 没有形式化的执行模型

## 应用的设计原则

### 发现而非跟踪
> 现实即真理，状态是派生的。

插件状态（上次运行时间、运行次数、结果）以 wisp 形式存在于账本上，而非影子状态文件中。门控评估直接查询账本。

### ZFC：零框架认知
> Agent 决策，Go 传输。

Deacon（agent）评估门控并决定是否分派。Go 代码提供传输（`gt dog dispatch`），但不做决策。

### MEOW 栈集成

| 层 | 插件类比 |
|----|----------|
| **M**olecule | `plugin.md` — 带有 TOML frontmatter 的工作模板 |
| **E**phemeral | 插件运行 wisp — 高吞吐、可消化 |
| **O**bservable | 插件运行出现在 `bd activity` 动态中 |
| **W**orkflow | 门控 → 分派 → 执行 → 记录 → 摘要 |

---

## 架构

### 插件位置

```
~/gt/
├── plugins/                      # Town 级插件（通用）
│   └── README.md
├── gastown/
│   └── plugins/                  # Rig 级插件
│       └── rebuild-gt/
│           └── plugin.md
├── beads/
│   └── plugins/
│       └── rebuild-bd/
│           └── plugin.md
└── wyvern/
    └── plugins/
        └── rebuild-wv/
            └── plugin.md
```

**Town 级**（`~/gt/plugins/`）：到处适用的通用插件。
**Rig 级**（`<rig>/plugins/`）：项目特定插件。

Deacon 在巡逻期间扫描这两个位置。

### 执行模型：Dog 分派

**关键洞察：** 插件执行不应阻塞 Deacon 巡逻。

Dog 是为基础设施任务设计的可复用工作者。插件执行被分派给 Dog：

```
Deacon 巡逻                    Dog 工作者
─────────────────               ─────────────────
1. 扫描插件
2. 评估门控
3. 对于开放的门控：
   └─ gt dog dispatch plugin     ──→ 4. 执行插件
      （非阻塞）                      5. 创建结果 wisp
                                      6. 发送 DOG_DONE
4. 继续巡逻
   ...
5. 处理 DOG_DONE              ←── （下一周期）
```

优势：
- Deacon 保持响应
- 多个插件可并发运行（不同的 Dog）
- 插件故障不会阻塞巡逻
- 与 Dog 的用途一致（基础设施工作）

### 状态跟踪：账本上的 Wisp

每次插件运行创建一个 wisp：

```bash
bd create --wisp-type patrol \
  --labels type:plugin-run,plugin:rebuild-gt,rig:gastown,result:success \
  --description "Rebuilt gt: abc123 → def456 (5 commits)" \
  "Plugin: rebuild-gt [success]"
```

**门控评估**查询 wisp 而非状态文件：

```bash
# 冷却检查：过去一小时内是否有运行？
bd list --wisp-type patrol --label plugin:rebuild-gt --created-after 1h -n 1
```

**派生状态**（不需要 state.json）：

| 查询 | 命令 |
|------|------|
| 上次运行时间 | `bd list --label=plugin:X --limit=1 --json` |
| 运行次数 | `bd list --label=plugin:X --json \| jq length` |
| 上次结果 | 从最新 wisp 解析 `result:` 标签 |
| 失败率 | 统计 `result:failure` vs 总数 |

### 摘要模式

与成本摘要类似，插件 wisp 累积并每天被压缩：

```bash
gt plugin digest --yesterday
```

创建：`Plugin Digest 2026-01-10` bead 附带摘要
删除：当天的各个 plugin-run wisp

这保持账本整洁，同时保留审计历史。

---

## 插件格式规范

### 文件结构

```
rebuild-gt/
└── plugin.md      # 带有 TOML frontmatter 的定义
```

### plugin.md 格式

```markdown
+++
name = "rebuild-gt"
description = "Rebuild stale gt binary from source"
version = 1

[gate]
type = "cooldown"
duration = "1h"

[tracking]
labels = ["plugin:rebuild-gt", "rig:gastown", "category:maintenance"]
digest = true

[execution]
timeout = "5m"
notify_on_failure = true
+++

# 重建 gt 二进制文件

Dog 工作者执行的指令...
```

### TOML Frontmatter 模式

```toml
# 必填
name = "string"           # 唯一插件标识符
description = "string"    # 人类可读描述
version = 1               # 模式版本（用于未来演进）

[gate]
type = "cooldown|cron|condition|event|manual"
# 类型特定字段：
duration = "1h"           # 用于 cooldown
schedule = "0 9 * * *"    # 用于 cron
check = "gt stale -q"     # 用于 condition（退出 0 = 运行）
on = "startup"            # 用于 event

[tracking]
labels = ["label:value", ...]  # 执行 wisp 的标签
digest = true|false            # 包含在每日摘要中

[execution]
timeout = "5m"            # 最大执行时间
notify_on_failure = true  # 失败时升级
severity = "low"          # 失败时的升级严重程度
```

### 门控类型

| 类型 | 配置 | 行为 |
|------|------|------|
| `cooldown` | `duration = "1h"` | 查询 wisp，在窗口内无运行则执行 |
| `cron` | `schedule = "0 9 * * *"` | 按 cron 计划运行 |
| `condition` | `check = "cmd"` | 运行检查命令，退出 0 则运行 |
| `event` | `on = "startup"` | 在 Deacon 启动时运行 |
| `manual` | （无 gate 部分） | 永不自动运行，显式分派 |

### 指令部分

frontmatter 之后的 markdown 正文包含 agent 可执行的指令。Dog 工作者读取并执行这些步骤。

标准部分：
- **检测**：检查是否需要执行操作
- **动作**：实际工作
- **记录结果**：创建执行 wisp
- **通知**：成功/失败时

---

## 需要的新命令

- **`gt stale`** — 暴露二进制过期检查（人类可读，`--json`，`--quiet` 退出码）
- **`gt dog dispatch --plugin <name>`** — 将插件执行分派给空闲 Dog（非阻塞）
- **`gt plugin list|show|run|digest|history`** — 插件管理和执行历史

---

## 实现计划

### 阶段 1：基础

1. **`gt stale` 命令** — 通过 CLI 暴露 CheckStaleBinary()
2. **插件格式规范** — 最终确定 TOML 模式
3. **插件扫描** — Deacon 扫描 town + rig 插件目录

### 阶段 2：执行

4. **`gt dog dispatch --plugin`** — 形式化的 Dog 分派
5. **Dog 中插件执行** — Dog 读取 plugin.md 并执行
6. **Wisp 创建** — 在账本上记录结果

### 阶段 3：门控与状态

7. **门控评估** — 通过 wisp 查询实现冷却
8. **其他门控类型** — Cron、condition、event
9. **插件摘要** — 每日压缩插件 wisp

### 阶段 4：升级

10. **`gt escalate` 命令** — 统一升级 API
11. **升级路由** — 配置驱动的多通道
12. **过期升级巡逻** — 检查未确认项

### 阶段 5：首个插件

13. **`rebuild-gt` 插件** — 实际的 gastown 插件
14. **文档** — 让 Beads/Wyvern 可以创建自己的插件

---

## 待解决问题

1. **多克隆中的插件发现**：如果 gastown 有 crew/george、crew/max、crew/joe — 哪个克隆的 plugins/ 目录是权威的？可能：扫描所有，按名称去重，如果存在则偏好 rig-root。

2. **Dog 分配**：特定插件是否应该偏好特定 Dog？还是任何空闲 Dog？

3. **插件依赖**：插件能否依赖其他插件？v1 中可能不支持。

4. **插件禁用/启用**：如何临时禁用插件而不删除它？在插件 bead 上加标签？frontmatter 中 `enabled = false`？

---

## 参考

- PRIMING.md - 核心设计原则
- mol-deacon-patrol.formula.toml - 巡逻步骤 plugin-run
- ~/gt/plugins/README.md - 当前插件存根