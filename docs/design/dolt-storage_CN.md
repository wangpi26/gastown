# Dolt 存储架构

> **状态**：Gas Town agent 的当前参考
> **更新日期**：2026-02-28
> **背景**：Dolt 是 Beads 和 Gas Town 的唯一存储后端

---

## 概述

Gas Town 使用 [Dolt](https://github.com/dolthub/dolt)，一个具有 Git 式版本控制的开源
SQL 数据库（Apache 2.0）。每个 town 的一个 Dolt SQL server 通过 MySQL 协议
在 3307 端口提供所有数据库。没有嵌入式模式和 SQLite。JSONL 仅用于灾难恢复
备份（JSONL Dog 每 15 分钟将清洗后的快照导出到 git 备份的存档），
而非作为主要存储格式。

`gt daemon` 管理服务器生命周期（自动启动、每 30 秒健康检查、
带指数退避的崩溃重启）。

## 服务器架构

```
Dolt SQL Server（每个 town 一个，端口 3307）
├── hq/       town 级 beads  (hq-* 前缀)
├── gastown/  rig beads     (gt-* 前缀)
├── beads/    rig beads     (bd-* 前缀)
├── wyvern/   rig beads     (wy-* 前缀)
└── sky/      rig beads     (sky-* 前缀)
```

**数据目录**：`~/gt/.dolt-data/` — 每个子目录是通过 SQL `USE <name>` 访问的数据库。

**连接**：`root@tcp(<host>:3307)/<database>`（无密码）。

## 环境变量

gt 和 bd 使用独立的环境变量连接 Dolt。gt 在生成 agent 时自动
将其变量转换为 bd 的等价形式。

| gt（Gas Town） | bd（Beads） | 用途 |
|---------------|------------|------|
| `GT_DOLT_HOST` | `BEADS_DOLT_SERVER_HOST` | 服务器主机（bd 未设置时默认为 `127.0.0.1`） |
| `GT_DOLT_PORT` | `BEADS_DOLT_PORT` | 服务器端口（默认：`3307`） |

**远程 Dolt 服务器**：如果 Dolt 运行在不同的机器上（例如通过 Tailscale），
在环境中设置 `GT_DOLT_HOST`。gt 将其作为 `BEADS_DOLT_SERVER_HOST`
传播到所有 bd 子进程，覆盖 bd 硬编码的 `127.0.0.1` 默认值。
否则，每个新 rig/worktree/polecat 会静默连接到 localhost 并失败。

按工作空间覆盖：在 rig 的 `.beads/config.yaml` 中设置 `dolt.host`。
这对特定工作空间优先于环境变量。

## 命令

```bash
# Daemon 管理服务器生命周期（首选）
gt daemon start

# 手动管理
gt dolt start          # 启动服务器
gt dolt stop           # 停止服务器
gt dolt status         # 健康检查，列出数据库
gt dolt logs           # 查看服务器日志
gt dolt sql            # 打开 SQL shell
gt dolt init-rig <X>   # 创建新的 rig 数据库
gt dolt list           # 列出所有数据库
```

如果服务器未运行，`bd` 会快速失败并显示指向 `gt dolt start` 的明确消息。

## 写入并发：全在 Main

所有 agent — polecats、crew、witness、refinery、deacon — 直接写入
`main`。并发通过事务纪律管理：每次写入以 `BEGIN` / `DOLT_COMMIT` / `COMMIT`
原子操作包装。

```
bd update <bead> --status=in_progress
  → BEGIN
  → UPDATE issues SET status='in_progress' ...
  → CALL DOLT_COMMIT('-Am', 'update status')
  → COMMIT
```

这消除了之前的每 worker 一个分支策略（BD_BRANCH、
每 polecat 的 Dolt 分支、完成时合并）。所有写入对所有 agent
立即可见 — 无跨 agent 可见性间隙。

多语句的 `bd` 命令在单个事务中批量写入以保持原子性。

## Schema

Schema 版本 6。完整 schema 位于 `beads/.../storage/dolt/schema.go`。
下表为关键表；索引和完整列列表参见源码。

```sql
-- 核心：每个 bead 是 issues 表中的一行（task、message、agent、gate 等）
CREATE TABLE issues (
    id VARCHAR(255) PRIMARY KEY,
    title VARCHAR(500) NOT NULL,
    description TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'open',
    priority INT NOT NULL DEFAULT 2,
    issue_type VARCHAR(32) NOT NULL DEFAULT 'task',
    assignee VARCHAR(255),
    owner VARCHAR(255) DEFAULT '',
    sender VARCHAR(255) DEFAULT '',          -- 消息
    mol_type VARCHAR(32) DEFAULT '',         -- molecule 类型
    work_type VARCHAR(32) DEFAULT 'mutex',   -- mutex vs open_competition
    hook_bead VARCHAR(255) DEFAULT '',       -- agent hook
    role_bead VARCHAR(255) DEFAULT '',       -- agent 角色
    agent_state VARCHAR(32) DEFAULT '',      -- agent 生命周期
    wisp_type VARCHAR(32) DEFAULT '',        -- 基于 TTL 的压缩类别
    metadata JSON DEFAULT (JSON_OBJECT()),   -- 可扩展元数据
    created_at DATETIME, updated_at DATETIME, closed_at DATETIME
    -- ... 以及约 20 个其他列（见 schema.go）
);

-- bead 之间的关系
CREATE TABLE dependencies (
    issue_id VARCHAR(255) NOT NULL,
    depends_on_id VARCHAR(255) NOT NULL,
    type VARCHAR(32) NOT NULL DEFAULT 'blocks',   -- blocks、parent-child、thread
    PRIMARY KEY (issue_id, depends_on_id)
);

-- 标签（多对多）
CREATE TABLE labels (
    issue_id VARCHAR(255) NOT NULL,
    label VARCHAR(255) NOT NULL,
    PRIMARY KEY (issue_id, label)
);

-- 审计跟踪
CREATE TABLE comments (id BIGINT AUTO_INCREMENT PRIMARY KEY, issue_id, author, text, created_at);
CREATE TABLE events   (id BIGINT AUTO_INCREMENT PRIMARY KEY, issue_id, event_type, actor, old_value, new_value, created_at);

-- Agent 交互日志
CREATE TABLE interactions (id, kind, actor, issue_id, model, prompt, response, created_at);

-- 基础设施
CREATE TABLE config          (key PRIMARY KEY, value);       -- 运行时配置旋钮
CREATE TABLE metadata        (key PRIMARY KEY, value);       -- schema 版本等
CREATE TABLE routes          (prefix PRIMARY KEY, path);     -- 前缀→数据库路由
CREATE TABLE issue_counter   (prefix PRIMARY KEY, last_id);  -- 顺序 ID 生成
CREATE TABLE child_counters  (parent_id PRIMARY KEY, last_child);
CREATE TABLE federation_peers (name PRIMARY KEY, remote_url, sovereignty, last_sync);

-- 压缩
CREATE TABLE issue_snapshots     (id, issue_id, compaction_level, original_content, ...);
CREATE TABLE compaction_snapshots (id, issue_id, compaction_level, snapshot_json, ...);
CREATE TABLE repo_mtimes         (repo_path PRIMARY KEY, mtime_ns, last_checked);
```

**Wisps**（临时巡逻数据）复用相同的 `issues` 表，设置了 `wisp_type`。
它们被 Dolt 忽略（`dolt_ignore` 表），因此 wisp 变更不会生成
Dolt commit — 只有 ignore 配置本身的结构变更会被提交。

**Mail** 实现为 issues 表中 `issue_type='message'` 的 bead —
没有单独的邮件表。`sender` 字段和 `dependencies`（type='thread'）
提供线程功能。

## Dolt 特有功能

以下功能可通过 SQL 供 agent 使用，并在 Gas Town 中广泛使用：

| 功能 | 用途 |
|------|------|
| `dolt_history_*` 表 | 完整的行级历史，可通过 SQL 查询 |
| `AS OF` 查询 | 时间旅行："昨天这看起来是什么样的？" |
| `dolt_diff()` | "这两个时间点之间发生了什么变化？" |
| `DOLT_COMMIT` | 带消息的显式提交（默认为自动提交） |
| `DOLT_MERGE` | 合并分支（集成分支、联邦） |
| `dolt_conflicts` 表 | 合并后的编程式冲突解决 |
| `DOLT_BRANCH` | 创建/删除分支（集成分支） |

**自动提交**默认开启：每次写入获得一个 Dolt commit。Agent
可以通过临时禁用自动提交来批量写入。

**冲突解决**默认：`newest`（最新 `updated_at` 胜出）。
数组（labels）：`union` 合并。计数器：`max`。

## 三个数据平面

Beads 数据落入三个具有不同特征的平面：

| 平面 | 内容 | 变更频率 | 持久性 | 传输 | 状态 |
|------|------|----------|--------|------|------|
| **操作面** | 进行中的工作、状态、分配、心跳 | 高（秒级） | 天–周 | Dolt SQL server（本地） | **活跃** |
| **账面** | 完成的工作、永久记录 | 低（完成边界） | 永久 | JSONL 导出 → git push 到 GitHub | **活跃** |
| **设计** | Epic、RFC、规格 — 尚未被认领的想法 | 对话式 | 直到具化 | DoltHub 公共空间（共享） | **计划中** |

操作面完全存在于本地 Dolt server 中。账面目前由 JSONL Dog 提供服务，
每 15 分钟将清洗后的快照导出到 git 备份的存档 — 这是经受住
灾难的持久记录（在 Clown Show #13 中得到验证）。设计面将通过
DoltHub 作为 Wasteland 公共空间的一部分进行联邦（计划中，
尚未进入活跃开发）。

## 数据生命周期：像 Git 一样思考，而非 SQL（关键）

Dolt 底层是 git。**提交图就是存储成本，而不是行。**
每次 `bd create`、`bd update`、`bd close` 都会生成一个 Dolt commit。
DELETE 一行，但写入它的 commit 仍然存在于历史中。`dolt gc`
回收未引用的块，但提交图本身永远增长。

这是来自 Tim Sehn（Dolt 创始人，2026-02-27）的关键洞察：

> "你的 Beads 数据库很小，但你的提交历史很大。"
>
> "如果你删除一个 bead，你应该对写入它的 commit 进行 rebase，
> 这样它在历史中就不再存在了。"

**Rebase**（`CALL DOLT_REBASE()`，自 v1.81.2 起可用）重写
提交图 — 它是真正的清理机制。DELETE + gc 是必要但不充分的。
DELETE + rebase + gc 是完整管道。

**关键更新**（Tim Sehn，2026-02-28）：所有压缩操作 —
`DOLT_RESET --soft`、`DOLT_REBASE()`、`dolt_gc()` — 在运行中的
服务器上是**安全的**。不需要停机时间或维护窗口。自 Dolt 1.75.0 起
自动 GC 默认开启。Flatten 操作极其廉价（指针移动，而非数据写入）。
可以每天或更频繁地运行。

参考：https://www.dolthub.com/blog/2026-01-28-everybody-rebase/

### 六阶段生命周期

```
CREATE → LIVE → CLOSE → DECAY → COMPACT → FLATTEN
  │        │       │        │        │          │
  Dolt   active   done   DELETE   REBASE     SQUASH
  commit  work    bead    rows    commits    all history
                         >7-30d  together   to 1 commit
```

| 阶段 | 拥有者 | 频率 | 机制 |
|------|--------|------|------|
| CREATE | 任何 agent | 持续 | `bd create`、`bd mol wisp create` |
| CLOSE | Agent 或巡逻 | 每任务 | `bd close`、`gt done` |
| DECAY | Reaper Dog | 每天 | `DELETE FROM wisps WHERE status='closed' AND age > 7d` |
| COMPACT | Compactor Dog | 每天 | `DOLT_RESET --soft` + `DOLT_COMMIT`（在运行中的服务器上安全） |
| FLATTEN | Compactor Dog | 每天 | 与 COMPACT 相同 — 无停机时间，无维护窗口 |

所有六个阶段都已在代码中实现。DECAY 在 Reaper Dog 中运行
（wisp_reaper.go），COMPACT/FLATTEN 在 Compactor Dog 中运行
（compactor_dog.go）。所有生命周期定时器默认通过
`EnsureLifecycleDefaults()`（lifecycle_defaults.go）启用，
该函数在 `gt init` 或 `gt up` 时自动填充 daemon.json 的合理
默认值。显式禁用的巡逻会被保留。

### 两条数据流

```
临时（wisps、巡逻数据）              持久（issues、molecules、agents）
  CREATE                                CREATE
  → work                                → work
  → CLOSE（>24h）                       → CLOSE
  → DELETE 行（Reaper）                 → JSONL 导出（清洗后）
  → REBASE 历史（Compactor）            → git push 到 GitHub
  → gc 未引用块（Compactor）            → COMPACT/FLATTEN 每天（无停机）
```

**临时数据**（wisps、wisp_events、wisp_labels、wisp_deps）是
高容量的巡逻副产品。实时有价值，24 小时后无价值。
Reaper Dog DELETE 行。Compactor Dog 将写入它们的
commit 从历史中 flatten 掉。两者缺一不可，否则存储无限制增长。

**持久数据**（issues、molecules、agents、dependencies、labels）是
账面。即使持久数据也受益于历史压缩 — 一个被创建、更新 5 次、
然后关闭的 bead 产生 7 个 commit，可以 rebase 为 1 个。
数据存活；中间历史不保留。

### 历史压缩操作

**每日压缩**（Compactor Dog 或 Dolt 定时事件）：

所有压缩操作在运行中的服务器上安全 — 不需要停机。
也可以配置为 Dolt 定时事件（MySQL 风格的 cron）：
https://www.dolthub.com/blog/2023-10-02-scheduled-events/

```sql
-- 简单每日 flatten（将比工作集更旧的所有内容压缩）
SET @init = (SELECT commit_hash FROM dolt_log ORDER BY date ASC LIMIT 1);
CALL DOLT_RESET('--soft', @init);
CALL DOLT_COMMIT('-Am', 'daily compaction');
```

**Flatten**（在运行中的服务器上安全 — 不需要停机）：

Flatten 极其廉价。`dolt_reset --soft` 不写入任何数据 —
它将工作集的父指针移动到引用的 commit。
随后的 commit 写入一个新 commit 和两个小指针写入。
可以每天甚至更频繁地运行（Tim Sehn，2026-02-28）。

```sql
-- 在运行中的服务器上通过 SQL flatten（首选）
-- 找到初始 commit
SET @init = (SELECT commit_hash FROM dolt_log ORDER BY date ASC LIMIT 1);
CALL DOLT_RESET('--soft', @init);
CALL DOLT_COMMIT('-Am', 'flatten: squash history');
-- GC 在日志超过 50MB 时自动运行
```

Flatten 期间的并发写入是安全的 — merge base 变为初始 commit，
但 diff 只是事务写入的内容，因此合并成功。

**精细压缩**通过交互式 rebase（压缩旧的，保留最近的）：

与 flatten（压缩一切）不同，交互式 rebase 让你保留
最近的单个 commit，同时压缩旧历史。在运行中的
服务器上执行。基于 Jason Fulghum 的 rebase 实现。

**并发写入风险**：DOLT_REBASE与并发写入不安全
（Tim Sehn，2026-02-28）。如果 agent 在 rebase 期间向数据库提交，
Dolt 会检测到图变更并报错。Compactor Dog 在此类
错误上重试一次。Flatten 模式（DOLT_RESET --soft）不受影响 —
并发写入在那里安全，因为 merge base 偏移但 diff 只是 txn。

```sql
-- 1. 在初始 commit 处创建分支（rebase 的 "upstream"）
SET @init = (SELECT commit_hash FROM dolt_log ORDER BY date ASC LIMIT 1);
CALL DOLT_BRANCH('compact-base', @init);

-- 2. 从 main 创建工作分支（永远不要直接 rebase main）
CALL DOLT_BRANCH('compact-work', 'main');
CALL DOLT_CHECKOUT('compact-work');

-- 3. 启动交互式 rebase — 填充 dolt_rebase 系统表
--    compact-base 和 compact-work 之间的所有 commit 进入计划
CALL DOLT_REBASE('--interactive', 'compact-base');

-- 4. 修改计划：压缩旧 commit，保留最近
--    第一个 commit 必须保持 'pick'（squash 需要一个父级来折叠入）。
--    保留最后 N 个 commit 为 'pick'，其他全部 squash。
SET @keep_recent = 50;  -- 保留最近 50 个独立 commit
UPDATE dolt_rebase SET action = 'squash'
WHERE rebase_order > (SELECT MIN(rebase_order) FROM dolt_rebase)
  AND rebase_order <= (SELECT MAX(rebase_order) FROM dolt_rebase) - @keep_recent;

-- 5. 执行 rebase 计划
CALL DOLT_REBASE('--continue');

-- 6. 交换分支：使 compact-work 成为新 main
CALL DOLT_CHECKOUT('compact-work');
CALL DOLT_BRANCH('-D', 'main');
CALL DOLT_BRANCH('-m', 'compact-work', 'main');
CALL DOLT_BRANCH('-D', 'compact-base');
CALL DOLT_CHECKOUT('main');
-- GC 在日志超过 50MB 时自动运行
```

**Rebase 动作**（来自 `dolt_rebase` 表）：
- `pick` — 保持 commit 原样
- `squash` — 折叠入前一个 commit，拼接消息
- `fixup` — 折叠入前一个 commit，丢弃消息
- `drop` — 完全移除 commit
- `reword` — 保持 commit，更改消息

**注意事项**：rebase 期间的冲突导致自动中止（尚无手动
解决方案）。简单的 flatten 对日常使用更可靠；
精细 rebase 适用于需要保留部分历史的场景。

参考：https://www.dolthub.com/blog/2024-01-03-announcing-dolt-rebase/

### Dolt GC

`dolt gc` 在 rebase 从图中移除 commit 之后压缩旧的块数据。
在 rebase 之后运行 gc，而不是替代它。顺序很重要：先 rebase，
再 gc。

**自动 GC 自 Dolt 1.75.0 起默认开启**（2025 年 10 月）。
当日志文件（`.dolt/noms/vvvv...`）达到 50MB 时触发。
无需手动 gc 或停止服务器 — 服务器自行处理。

```sql
-- 手动 gc（在运行中的服务器上安全，无需停止）
CALL dolt_gc();
```

GC 消耗内存但我们的数据库很小，所以无需担心（Tim Sehn，
2026-02-28）。

### Dolt 定时事件（探测结果，2026-02-28）

Dolt 支持 MySQL 风格的 `CREATE EVENT` 用于服务器维护的 cron job。
参考：https://www.dolthub.com/blog/2023-10-02-scheduled-events/

**在 Dolt 1.82.6 上测试：**
- `CREATE EVENT ... EVERY 1 DAY DO CALL dolt_gc()` — 可用
- 事件持久存储在 `dolt_schemas` 表中 — 服务器重启后保留
- 事件仅在 `main` 分支上触发
- 存储过程可用（`CREATE PROCEDURE`，含 DECLARE、BEGIN...END）
- 事件可以调用存储过程
- 最小间隔：30 秒（Dolt 强制此下限）

**定时事件能否替代 Compactor Dog？**

**不能。** Compactor Dog 的 10 步 flatten 算法需要 SQL 事件
无法提供的安全特性：
- 阈值检查（仅在 commit 数量超过 N 时压缩）
- 完整性验证（压缩前后的行数比较）
- 并发中止（检测压缩期间 main HEAD 是否移动）
- 错误升级（失败时通知 Mayor）
- 跨数据库迭代（单次巡逻处理所有 DB）
- Daemon 级别的日志和可观察性

存储过程可以实现原始的 flatten SQL，但缺少升级、
可观察性和与 daemon 生命周期的集成。

**定时事件能做的：**
- 用显式的 `dolt_gc()` 调度来补充 Compactor Dog
- 但自 Dolt 1.75.0 起自动 GC 已默认开启，使这变得多余

**建议：** 保留 Compactor Dog 进行 flatten。自动 GC 处理块回收。
定时事件不增加我们已有的价值。

### 污染防护

污染通过四个向量进入 Dolt：

1. **提交图增长**：每次变更 = 一个 commit。Rebase 压缩。
2. **邮件污染**：Agent 过度使用 `gt mail send` 进行日常通信。
   改用 `gt nudge`（临时、零 Dolt 成本）。参见 mail-protocol.md。
3. **测试制品**：测试代码在生产服务器上创建 issue。
   store.go 中的防火墙拒绝在 3307 端口上带测试前缀的 CREATE DATABASE。
4. **僵尸进程**：比测试生命周期更长的测试 dolt-server 进程。
   Doctor Dog 杀死这些。2026-02-27 发现并杀死了 45 个僵尸（7GB RAM）。

防护是分层的：
- **提示**：Agent 优先使用 `gt nudge` 而非 `gt mail send`（零 commit）
- **防火墙**（store.go）：拒绝在 3307 端口上带测试前缀的 CREATE DATABASE
- **Reaper Dog**：DELETE 关闭的 wisps，auto-close 过期的 issue
- **Compactor Dog**：flatten 旧 commit 以压缩历史，之后运行 gc
- **Doctor Dog**：杀死僵尸服务器，检测孤立 DB，监控健康
- **JSONL Dog**：清洗导出，拒绝污染，commit 前进行 spike 检测

所有 Dog 默认通过 `EnsureLifecycleDefaults()`（在
lifecycle_defaults.go 中）启用。Daemon 在启动时
（`gt init` / `gt up`）自动填充 daemon.json 中缺失的巡逻条目。
要禁用特定 Dog，在其 daemon.json 部分设置 `"enabled": false` —
自动填充逻辑保留显式配置的条目。

### 通信卫生（减少 Commit 数量）

每次 `gt mail send` 创建一个 bead + Dolt commit。每次 `gt nudge`
什么也不创建。规则：

**默认使用 `gt nudge`。仅在消息必须在接收方会话死亡后仍然存活时
才使用 `gt mail send`。**

| 角色 | 邮件预算 | 其他一切用 Nudge |
|------|----------|------------------|
| Polecat | 每会话 0-1（仅 HELP） | 状态、问题、更新 |
| Witness | 仅协议消息 | 健康检查、polecat 催促 |
| Refinery | 仅协议消息 | 向 Witness 的状态更新 |
| Deacon | 仅升级 | 定时器回调、健康催促 |
| Dogs | 零（不发邮件） | 通过 nudge 向 Deacon 报告 DOG_DONE |

## 独立 Beads 说明

`bd` CLI 保留了嵌入式 Dolt 选项用于独立使用（Gas Town 之外）。
仅服务器模式专用于 Gas Town — 独立用户可能没有运行中的 Dolt 服务器。

Dolt 团队正在改进嵌入式模式，用于单进程用例如独立 Beads。
这将为独立的 `bd` 用户提供零配置体验（无需管理服务器），
同时保留 Dolt 的版本控制能力。

## 远程推送（Git 协议）

Gas Town 通过 `gt dolt sync` 将 Dolt 数据库推送到 GitHub 远程。
这些使用 git SSH 协议（`git+ssh://git@github.com/...`），而非
DoltHub 的原生协议。

### Git 远程缓存

Dolt 在 `~/gt/.dolt-data/<db>/.dolt/git-remote-cache/` 维护一个缓存，
存储从 Dolt 内部格式构建的 git 对象。根据 Dolt 团队
（Dustin Brown，2026-02-26）：

- **缓存是必要的** — Dolt 使用它为 push/pull 构建 git 对象
- **积累垃圾**（孤立引用），且不会自动清理
- **安全删除** — 在两次推送之间可以删除，但下次推送时会全量重建
  （beads：约 20 分钟重建，gastown：更长）
- **孤立引用** 可以在不删除整个缓存的情况下修剪 — 更好的平衡
- **随时间增长** — 随着数据库增长而增长 — git 协议远程的固有特性

**指导**：不要常规性地删除缓存。优先修剪孤立引用。
仅在磁盘压力关键且可接受长时间重建时进行完整删除。

### 同步过程

`gt dolt sync` 停放所有 rig（停止 witness/refinery），停止 Dolt
服务器，为每个有配置远程的数据库运行 `dolt push`，然后
重启服务器并解除 rig 停放。停放防止 witness 检测到服务器
中断并在推送过程中重启它。

### 强制推送

数据恢复后（如 Clown Show #13），本地和远程历史
分歧。第一次推送使用 `gt dolt sync --force` 用本地状态
覆盖远程。后续推送应无需 `--force` 即可工作。

### 已知限制

- **慢**：Git 协议远程比 DoltHub 原生远程慢几个数量级。
  71MB 数据库约 90 秒；更大的数据库需要 20+ 分钟。
- **缓存增长**：无自动垃圾回收。孤立引用修剪待定。
- **服务器停机**：推送需要对数据目录的独占访问，
  因此推送期间必须停止服务器。这产生了维护窗口。

### DoltHub 远程（计划中）

DoltHub 的原生协议（`https://doltremoteapi.dolthub.com/...`）完全
避免 git-remote-cache，且速度快得多。基于 DoltHub 的联邦
计划作为 Wasteland 公共空间的一部分 — 这将替代
设计和账面的 git 协议远程。迁移需要 DoltHub 账户和
使用 `dolt remote set-url` 重新配置远程。目前不在活跃开发中。

## 文件布局

```
~/gt/                            Town 根目录
├── .dolt-data/                  集中式 Dolt 数据目录
│   ├── hq/                      Town beads（hq-*）
│   ├── gastown/                 Gastown rig（gt-*）
│   ├── beads/                   Beads rig（bd-*）
│   ├── wyvern/                  Wyvern rig（wy-*）
│   └── sky/                     Sky rig（sky-*）
├── daemon/
│   ├── dolt.pid                 服务器 PID（daemon 管理）
│   ├── dolt.log                 服务器日志
│   └── dolt-state.json          服务器状态
└── mayor/
    └── daemon.json              Daemon 配置（dolt_server 部分）
```