# 账面导出触发器

> **状态**：设计 — 对应 gt-ayk
> **作者**：mel（crew）
> **日期**：2026-02-07
> **相关**：dolt-storage.md（三个数据平面）、WISP-COMPACTION-POLICY.md（Level 0-1）、
> PRIMING.md（身份与 CV 模型、技能推导）

---

## 问题陈述

dolt-storage 架构定义了三个数据平面（操作面、账面、设计面）
并引用了 Level 0-3 的保真度模型。Wisp 压缩设计处理了
Level 0（临时）到 Level 1（提升/持久操作）的转换。缺少的是
工作从**操作面（Level 1）**移动到**账面（Level 2-3）**的
触发器规范。

没有定义的触发器，账面保持空白。没有永久记录积累。
没有技能推导。CV 不增长。HOP 经济无法启动。

## 保真度级别参考

| 级别 | 平面 | 内容 | 持久性 | 可见性 |
|------|------|------|--------|--------|
| 0 | 操作面 | 临时 wisps（心跳、巡逻） | 基于 TTL | 本地 |
| 1 | 操作面 | 持久操作记录（打开/活跃的 bead） | 天-周 | 本地 |
| 2 | 账面 | 压缩的完成记录 | 永久 | 联邦 |
| 3 | 账面 | 完整保真度的基本事实 | 永久 | 联邦 |

Level 0 → 1 由 wisp 压缩策略处理（基于已证明价值的提升）。
**本文档定义 Level 1 → 2 和 Level 1 → 3 的触发器。**

---

## 设计原则

### 1. 导出是单向且仅追加的

账面记录永远不被更新。如果 bead 在导出后被重新打开，
会创建新的账面条目（更正记录），而非修改旧条目。
这保留了审计跟踪，符合 Plane 2 的仅追加属性。

### 2. 触发器是边界，不是时钟

账面导出发生在有意义的工作边界，而非定时器。凌晨 3 点关闭的
bead 在凌晨 3 点被导出。批量处理可用于提高效率（每 N 分钟
导出），但概念上的触发器始终是边界事件。

### 3. 级别选择基于 HOP 价值，而非重要性

Level 2 与 3 的区别不在于工作有多"重要"。而在于 HOP
需要什么来推导技能。常规 bug 修复进入 Level 2（完成的事实
就是技能信号）。新颖的调试方法进入 Level 3（解决问题的方式
才是技能信号）。

### 4. 导出安全失败

如果触发器触发但导出失败（服务器宕机、schema 不匹配），
操作记录不受影响。下次触发器扫描时重试导出。
不会丢失数据；账面只是延迟。

---

## Level 2 触发器：压缩完成记录

Level 2 捕获**做了什么** — 完成的事实、元数据、
结果。它丢弃操作噪音（状态变更、中间评论、
agent 心跳）。可以将其理解为工作记录的"git squash"。

### 触发器 1：Bead 关闭

**时机**：Bead 转换为 `status: closed`（通过 `bd close <id>`）。

**导出内容**：

| 字段 | 来源 | 备注 |
|------|------|------|
| `id` | bead.id | 稳定标识符 |
| `type` | bead.type | task、bug、feature 等 |
| `title` | bead.title | 关闭时的标题 |
| `outcome` | bead.description | 最终描述（非历史） |
| `priority` | bead.priority | 分配时的优先级 |
| `owner` | bead.owner | 谁拥有此工作 |
| `assignee` | bead.assignee | 谁做了此工作 |
| `labels` | bead.labels | 分类标签 |
| `created_at` | bead.created_at | 工作被设想的时间 |
| `closed_at` | bead.closed_at | 工作完成的时间 |
| `duration_days` | 计算得出 | closed_at - created_at |
| `parent` | bead.parent | Convoy/epic 关联 |
| `rig` | 上下文 | 属于哪个 rig |
| `commit_refs` | git log | 关联的 git commit（如有） |
| `files_touched` | git diff | 变更的文件路径（用于技能推导） |
| `lines_changed` | git diff | +/- 行数 |

**丢弃内容**：状态变更历史、中间评论（除非标记为 Level 3）、
agent 分配变更、心跳/巡逻关联。

**排除项**：Wisps（`wisp: true`）从不导出到 Level 2。
它们要么被 TTL 删除，要么提升到 Level 1（持久操作）。
提升的 wisp 可以在关闭时像其他 bead 一样触发 Level 2 导出。

### 触发器 2：Convoy 完成

**时机**：Convoy 中所有 bead 达到 `closed` 状态。

**导出内容**：除个别 bead 记录（通过触发器 1 导出）外的
Convoy 级摘要记录。

| 字段 | 来源 | 备注 |
|------|------|------|
| `convoy_id` | convoy bead id | 协调单元 |
| `title` | convoy title | 协调了什么 |
| `bead_count` | 子项计数 | 工作规模 |
| `agents_involved` | 唯一 assignee | 谁参与了 |
| `rigs_involved` | 唯一 rig 上下文 | 跨 rig 广度 |
| `created_at` | convoy 创建时间 | 协调开始的时间 |
| `completed_at` | 最后子项关闭时间 | 所有工作落地的时间 |
| `duration_days` | 计算得出 | 总经过时间 |

**为什么与触发器 1 分开**：Convoy 记录捕获协调模式 —
多 agent 工作、跨 rig 广度、并行性。这些是个别 bead 记录
无法捕获的独立技能信号。

### 触发器 3：Refinery 合并

**时机**：Refinery 成功将 polecat 的工作合并到 main。

**导出内容**：带验证元数据的增强版 bead 关闭记录。

| 字段 | 来源 | 备注 |
|------|------|------|
| `merge_id` | refinery record | 合并队列条目 |
| `bead_id` | 关联 bead | 链接到 bead 记录 |
| `branch` | polecat 分支 | 工作来源 |
| `merged_by` | refinery agent | 验证者身份 |
| `merge_result` | pass/fail/conflict | 结果 |
| `test_results` | CI 输出 | 如果测试运行了 |
| `conflict_resolution` | 合并策略 | 冲突如何处理 |

**为什么对 HOP 重要**：Refinery 合并是外部验证。
Bead 可以被 assignee 自行关闭，但合并证明工作经过了代码审查
（即使是自动化的）。这是更强的技能信号。

### 触发器 4：里程碑/Sprint 边界

**时机**：达到基于时间或计数的边界（可配置）。

**导出内容**：汇总该期间的汇总/摘要记录。

| 字段 | 来源 | 备注 |
|------|------|------|
| `period` | 配置 | "daily"、"weekly" 或自定义 |
| `period_start` | 时间戳 | 窗口开始 |
| `period_end` | 时间戳 | 窗口结束 |
| `beads_closed` | 计数 | 数量 |
| `beads_opened` | 计数 | 进入速率 |
| `agents_active` | 唯一 assignee | 工作力规模 |
| `top_labels` | 标签频率 | 什么类型的工作占主导 |
| `anomalies` | 启发式 | 异常模式 |

**目的**：聚合个别 bead 无法捕获的信号。
单个 bug 修复说明不了什么；一周 47 个 bug 修复说明"调试冲刺"。
这些模式在更高层次上为 HOP 技能推导提供输入。

---

## Level 3 触发器：完整保真度基本事实

Level 3 捕获**工作如何完成** — 推理、决策、
解决问题的方法。这是 HOP 技能推导的原材料。
Level 3 记录比 Level 2 更大也更少。

### 触发器 5：设计决策

**时机**：Bead 关闭时带有指示设计结果的标签：
- `label: design-decision`
- `label: architecture`
- `label: rfc`
- `type: design`（bead 类型）

或者：设计文档被提交到仓库（通过文件路径
模式检测，如 `docs/design/*.md`、`**/DESIGN.md`、`**/RFC-*.md`）。

**导出内容**：

| 字段 | 来源 | 备注 |
|------|------|------|
| 所有 Level 2 字段 | bead | 基础记录 |
| `full_description` | bead.description | 完整文本，非摘要 |
| `comments` | 所有评论 | 完整讨论线程 |
| `decision_context` | 提取 | 考虑了哪些替代方案 |
| `design_doc_path` | git | 关联设计文档路径 |
| `design_doc_content` | 文件 | 关闭时的完整文档内容 |

**为什么需要完整保真度**：设计决策编码了*判断力* — 为什么选 A 而非 B，
衡量了哪些权衡，存在什么约束。这是 HOP 技能推导的
最高价值信号。未来的 agent 学习"如何设计存储系统"需要推理，
而不仅仅是结果。

### 触发器 6：新颖问题解决

**时机**：启发式检测非常规工作：
- Bead 在关闭后被重新打开（需要返工）
- Bead 有超过 N 条评论（重要讨论）
- Bead 被重新分配（初始 assignee 无法解决）
- Bead 有 `label: investigation` 或 `label: debugging`
- Bead 时长超过其类型滚动平均值的 3 倍
- Bead 的 commit diff 涉及超过 M 个文件（广影响变更）

**可配置阈值**（在 rig 导出配置中）：
```json
{
  "level3_heuristics": {
    "comment_threshold": 5,
    "reassignment_count": 2,
    "duration_multiplier": 3.0,
    "file_touch_threshold": 15,
    "reopen_triggers_level3": true
  }
}
```

**导出内容**：所有 Level 2 字段加上：

| 字段 | 来源 | 备注 |
|------|------|------|
| `comments` | 所有评论 | 完整讨论 |
| `status_history` | dolt_history | 每次状态转换 |
| `assignee_history` | dolt_history | 重新分配链 |
| `reopen_count` | 计算得出 | 被重新打开的次数 |
| `commit_diffs` | git | 实际代码变更（摘要） |
| `trigger_reason` | 启发式 | 为什么被标记为 Level 3 |

**为什么对 HOP 重要**：常规完成（Level 2）证明 agent *能*
做某事。新颖问题解决证明 agent 能*想出*
新事物的解决方案。后者是根本不同且更有价值的技能信号。

### 触发器 7：跨 Rig 协调

**时机**：Bead 或 convoy 涉及跨多个 rig 的工作（通过
convoy 成员、跨 rig 引用或 worktree 使用检测）。

**导出内容**：所有 Level 2 字段加上：

| 字段 | 来源 | 备注 |
|------|------|------|
| `rigs_involved` | bead 引用 | 涉及了哪些 rig |
| `worktrees_used` | gt worktree | 跨 rig 工作会话 |
| `coordination_pattern` | 分析 | 串行 vs 并行、委托 vs 直接 |
| `mail_thread` | gt mail | 此工作的 agent 间通信 |
| `convoy_structure` | bead 图 | 工作如何分解 |

**为什么对 HOP 重要**：跨 rig 工作展示了架构理解 —
知道代码在哪里、系统如何交互、何时委托 vs 直接做。
这是技能向量的"广度"维度。

### 触发器 8：显式完整保真度标志

**时机**：人类或 agent 显式标记 bead 为 Level 3 导出：
- `bd update <id> --label ledger-full`
- 包含 `@ledger-full` 或 `#ground-truth` 的评论

**导出内容**：所有可用内容 — 完整 bead 状态、所有评论、
所有历史、关联的 commit、关联的设计文档。

**为什么存在**：启发式会遗漏内容。当人类识别出
某个教学时刻或关键决策时，他们应该能够显式标记。
这是"手动提升"的逃生舱。

---

## 有意义的边界

不是每个触发器都独立触发。它们在自然工作边界处聚集。
导出系统应识别这些边界并批量导出以提高效率：

### 边界 1：任务完成

单个 bead 关闭。最常见的边界。始终触发触发器 1，
可能触发触发器 5-8（如果满足 Level 3 条件）。

```
bead 关闭 → Level 2 导出（始终）
          → Level 3 导出（如果是设计/新颖/跨 rig/被标记）
```

### 边界 2：Convoy 落地

Convoy 中所有 bead 关闭。在所有个别触发器 1 导出后
触发触发器 2（convoy 摘要）。这是自然的"项目完成"边界。

```
最后一批 convoy bead 关闭 → 所有触发器 1 导出（如尚未完成）
                        → 触发器 2 convoy 摘要
                        → 触发器 7（如果是多 rig）
```

### 边界 3：合并验证

Refinery 合并代码。触发触发器 3。通常紧随触发器 1
（bead 关闭，然后代码合并），因此这些应在账面中关联。

```
refinery 合并 → 触发器 3（增强现有 Level 2 记录）
```

### 边界 4：会话 Handoff

Agent 通过 `gt handoff` 循环。不是直接导出触发器，而是
**检查点机会**：在会话结束前应刷新之前触发器的待处理导出。

```
gt handoff → 刷新待处理导出
           → 如果跨越周期边界则触发触发器 4
```

### 边界 5：设计具化

设计文档被提交且关联 bead 关闭。这是想法离开设计面
进入账面面的自然点。

```
设计 bead 关闭 → 触发器 1（基础记录）
               → 触发器 5（带文档内容的完整保真度）
```

---

## HOP 技能推导：完整保真度应捕获什么

HOP 从工作证据中推导技能。导出触发器的问题是：
HOP 需要什么证据，以什么保真度？

### Level 2 的技能信号（压缩）

这些仅从元数据推导 — 无需完整内容：

| 信号 | 推导来源 | 技能类别 |
|------|----------|----------|
| 语言熟练度 | `files_touched` 扩展名 | 技术/语言 |
| 领域专业知识 | `labels`、`rig` 上下文 | 领域 |
| 完成速度 | `duration_days` | 效率 |
| 工作量 | Level 2 记录计数 | 容量 |
| 广度 | 唯一 rig、唯一标签集 | 多面性 |
| 可靠性 | 关闭/重新打开比率 | 质量 |

### Level 3 的技能信号（完整保真度）

这些需要推理内容 — 仅 Level 2 元数据不够：

| 信号 | 推导来源 | 技能类别 |
|------|----------|----------|
| 架构判断 | 设计决策推理 | 设计/架构 |
| 调试方法论 | 问题解决评论 | 问题解决 |
| 沟通质量 | 评论清晰度、线程连贯性 | 协作 |
| 权衡分析 | 设计文档"考虑的替代方案" | 决策 |
| 系统级思维 | 跨 rig 协调模式 | 架构 |
| 新颖模式识别 | 非常规问题的处理方式 | 创新 |
| 教学/指导 | 解释性评论、文档质量 | 领导力 |

### "HOP 能否从元数据中学到这个？" 测试

在决定 Level 2 与 3 时，问题是：

> 未来的 agent 能否仅从压缩记录中学会复制这项工作？

- **能** → Level 2。"修复了 README 中的拼写错误" — 知道它发生了就足够了。
- **不能** → Level 3。"将存储重新设计为三个数据平面" — 推理
  是技能，而非结果。

这是应该指导自动启发式和手动 `@ledger-full` 标记的实用测试。

---

## 导出机制

### Schema：账面记录

```sql
-- Level 2: 压缩完成记录
CREATE TABLE ledger_completions (
    id VARCHAR(64) PRIMARY KEY,      -- 与源 bead ID 相同
    bead_type VARCHAR(32),
    title TEXT,
    outcome TEXT,                     -- 最终描述
    priority INT,
    owner VARCHAR(255),
    assignee VARCHAR(255),
    labels JSON,
    rig VARCHAR(64),
    created_at TIMESTAMP,
    closed_at TIMESTAMP,
    duration_days FLOAT,
    parent VARCHAR(64),              -- convoy 关联
    commit_refs JSON,                -- 关联的 git commit
    files_touched JSON,              -- 文件路径（用于技能推导）
    lines_changed JSON,              -- {added: N, removed: M}
    fidelity_level INT DEFAULT 2,    -- 2 或 3
    exported_at TIMESTAMP,
    export_trigger VARCHAR(32)       -- 哪个触发器导致了导出
);

-- Level 2: Convoy 摘要记录
CREATE TABLE ledger_convoys (
    convoy_id VARCHAR(64) PRIMARY KEY,
    title TEXT,
    bead_count INT,
    agents_involved JSON,
    rigs_involved JSON,
    created_at TIMESTAMP,
    completed_at TIMESTAMP,
    duration_days FLOAT,
    exported_at TIMESTAMP
);

-- Level 2: 合并验证记录
CREATE TABLE ledger_merges (
    merge_id VARCHAR(64) PRIMARY KEY,
    bead_id VARCHAR(64),
    branch VARCHAR(255),
    merged_by VARCHAR(255),
    merge_result VARCHAR(32),
    conflict_resolution VARCHAR(64),
    exported_at TIMESTAMP
);

-- Level 3: 完整保真度扩展（关联 ledger_completions）
CREATE TABLE ledger_ground_truth (
    bead_id VARCHAR(64) PRIMARY KEY,
    full_description TEXT,
    comments JSON,                    -- 完整评论线程
    status_history JSON,              -- 所有状态转换
    assignee_history JSON,            -- 重新分配链
    design_doc_path VARCHAR(512),
    design_doc_content TEXT,
    commit_diffs JSON,                -- 代码变更摘要
    trigger_reasons JSON,             -- 为什么触发了 Level 3
    coordination_pattern VARCHAR(64), -- 跨 rig 工作
    mail_thread JSON,                 -- agent 间通信
    exported_at TIMESTAMP,
    FOREIGN KEY (bead_id) REFERENCES ledger_completions(id)
);

-- Level 2: 周期性汇总记录
CREATE TABLE ledger_rollups (
    id VARCHAR(64) PRIMARY KEY,
    period VARCHAR(32),
    period_start TIMESTAMP,
    period_end TIMESTAMP,
    rig VARCHAR(64),
    beads_closed INT,
    beads_opened INT,
    agents_active JSON,
    top_labels JSON,
    anomalies JSON,
    exported_at TIMESTAMP
);
```

### 导出过程

```
1. 触发器触发（bead 关闭、合并等）
2. 从操作 Dolt 表收集源数据
3. 评估 Level 2 与 Level 3（启发式 + 显式标志）
4. 写入账面表（同一 Dolt 服务器，不同 schema/命名空间）
5. Dolt commit: "ledger: export <bead-id> at level <N>"
6. 标记源 bead 为已导出（添加标签: "ledger-exported-L<N>"）
```

当 dolt-in-git 交付时，账面表将包含在 git 追踪的
二进制中 — 使它们自动联邦化。在那之前，账面表
与操作表并列存在于同一 Dolt 服务器中。

### 重试和幂等性

导出是幂等的：以相同级别重新导出同一 bead 产生
相同的账面记录（基于 bead_id 的 INSERT OR REPLACE）。
`exported_at` 时间戳会更新，但内容是稳定的。

失败的导出通过 `export_queue` 表追踪：

```sql
CREATE TABLE export_queue (
    bead_id VARCHAR(64),
    trigger VARCHAR(32),
    triggered_at TIMESTAMP,
    attempts INT DEFAULT 0,
    last_error TEXT,
    next_retry_at TIMESTAMP,
    PRIMARY KEY (bead_id, trigger)
);
```

---

## 配置

每 rig 的导出配置在 `.beads/config/ledger-export.json`：

```json
{
  "enabled": true,
  "auto_export_on_close": true,
  "batch_interval_minutes": 5,
  "level3_heuristics": {
    "comment_threshold": 5,
    "reassignment_count": 2,
    "duration_multiplier": 3.0,
    "file_touch_threshold": 15,
    "reopen_triggers_level3": true
  },
  "level3_labels": [
    "design-decision",
    "architecture",
    "rfc",
    "investigation",
    "debugging",
    "ledger-full"
  ],
  "level3_bead_types": [
    "design"
  ],
  "exclude_labels": [
    "wip",
    "draft"
  ],
  "rollup_period": "daily"
}
```

覆盖优先级：rig 配置 > town 默认值 > 硬编码默认值。

---

## 集成点

### 与 Wisp 压缩（WISP-COMPACTION-POLICY.md）

Wisp 压缩处理 Level 0 → Level 1 提升。一旦 wisp 被提升
（成为持久操作 bead），它在关闭时进入正常的 Level 1 → 2/3
导出管道。两个系统互补：

```
Level 0（临时） ──[wisp TTL/提升]──> Level 1（操作面）
Level 1（操作面） ──[本设计]──> Level 2/3（账面）
```

### 与 Refinery

Refinery 合并事件触发触发器 3。实现：Refinery 的合并
完成 hook 调用 `bd ledger export <bead-id> --trigger merge`。

### 与 `gt handoff`

Handoff 刷新待处理导出。实现：将导出刷新添加到
handoff 检查清单（git push 之后，会话结束之前）。

### 与 HOP 技能推导（未来）

账面表是 HOP 技能查询的输入。示例：

```sql
-- "Agent X 做了哪些 Go 工作？"
SELECT lc.title, lc.files_touched, lc.duration_days
FROM ledger_completions lc
WHERE lc.assignee = 'gastown/crew/mel'
  AND JSON_CONTAINS(lc.files_touched, '"*.go"')
ORDER BY lc.closed_at DESC;

-- "展示存储架构的设计决策"
SELECT lc.title, lgt.full_description, lgt.design_doc_content
FROM ledger_completions lc
JOIN ledger_ground_truth lgt ON lc.id = lgt.bead_id
WHERE JSON_CONTAINS(lc.labels, '"architecture"')
  AND lc.fidelity_level = 3;
```

---

## 实现路线图

### 阶段 1：Level 2 核心（触发器 1 + Schema）

- 将账面表添加到 Dolt schema
- 实现 bead-关闭导出（触发器 1）
- 成功导出时添加 `ledger-exported-L2` 标签
- `bd ledger export <id>` 命令用于手动触发
- `bd ledger status` 命令检查导出状态

### 阶段 2：Convoy + 合并（触发器 2-3）

- Convoy 完成检测和摘要导出
- Refinery 合并 hook 集成
- 将合并记录链接到完成记录

### 阶段 3：Level 3 启发式（触发器 5-8）

- 实现 Level 3 选择启发式
- 设计决策检测（标签 + 文件路径）
- 新颖问题检测（评论、重新分配、时长）
- `@ledger-full` 标志支持
- 完整保真度数据收集（评论、历史、diff）

### 阶段 4：汇总 + 联邦（触发器 4 + dolt-in-git）

- 周期性汇总生成
- 异常检测启发式
- dolt-in-git 集成用于联邦账面访问
- 跨 town 技能查询

---

## 开放问题

1. **账面表位置**：与操作表同在同一 Dolt 数据库
   （更简单）还是独立数据库（更清晰的分离）？建议：
   同一数据库使用 `ledger_` 表前缀，当 dolt-in-git 交付
   且联邦需求需要时迁移到独立数据库。

2. **回溯导出**：是否应该为所有当前已关闭的 bead
   回填 Level 2 记录？建议：是，作为一次性迁移。
   数据存在于 Dolt 历史中；我们只需将其投影到
   账面 schema 中。

3. **git commit 的导出粒度**：在 `commit_refs` 和
   `files_touched` 中捕获多少 commit 细节？完整 diff 很大。
   建议：Level 2 仅文件路径和行数；Level 3 使用摘要 diff。

4. **隐私/脱敏**：是否应将某些 bead 从联邦
   账面中排除？（例如，带 `label: private` 或在私有
   rig 中的 bead）。建议：是，添加 `exclude_from_federation` 标志，
   将记录保留在本地账面中，但从 dolt-in-git 导出中省略。