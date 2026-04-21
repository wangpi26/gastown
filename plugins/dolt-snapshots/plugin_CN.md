+++
name = "dolt-snapshots"
description = "在 Convoy 边界处为 Dolt 数据库打标签，用于审计、diff 和回滚"
version = 3

[gate]
type = "event"
on = "convoy.created"

[tracking]
labels = ["plugin:dolt-snapshots", "category:data-safety"]
digest = true

[execution]
timeout = "2m"
notify_on_failure = true
severity = "low"
+++

# Dolt Snapshots v3

在 Convoy 生命周期的关键节点为 Dolt 数据库创建快照，使用**标签**（不可变）
和可选的**分支**（可变，用于工作中的 diff）。

实现为独立的 Go 二进制文件，使用参数化 SQL——没有 shell
插值、没有子 shell bug、不会自动提交脏状态。

## 功能说明

**Convoy 审计** — 验证代理是否完成了预期的工作：
```sql
SELECT * FROM dolt_diff('staged/pi-rust-bug-fixes-hq-cv-xrwki', 'HEAD', 'issues')
SELECT * FROM dolt_diff_stat('staged/pi-rust-bug-fixes-hq-cv-xrwki', 'HEAD')
```

**Convoy 回滚** — 将数据库恢复到 Convoy 前的状态：
```sql
CALL DOLT_CHECKOUT('staged/pi-rust-bug-fixes-hq-cv-xrwki');          -- 整个数据库
CALL DOLT_CHECKOUT('staged/pi-rust-bug-fixes-hq-cv-xrwki', 'issues'); -- 单个表
```

**跨 Convoy 对比** — 跟踪不同运行间的进展：
```sql
SELECT * FROM dolt_diff('staged/pi-rust-bug-fixes-hq-cv-xrwki', 'staged/otel-dashboard-hq-cv-7q3vi', 'issues')
```

**数据丢失调查** — 当备份告警触发时，与上次快照做 diff：
```sql
SELECT * FROM dolt_diff('staged/pi-rust-bug-fixes-hq-cv-xrwki', 'HEAD', 'issues')
WHERE diff_type = 'removed'
```

## 分支的用途（可变沙箱）

分支是数据库在快照时刻的可写副本。与标签不同，
你可以向分支提交——这使它们适用于：

- **试运行 Convoy 工作** — 测试批量操作而不影响主分支
- **隔离的 Convoy 写入** — 代理写入分支，Refinery 合并
- **假设分析** — 无风险地验证想法
- **并行 Convoy 隔离** — 两个 Convoy 写入各自独立的分支

## 为什么选择标签而非分支

- 分支只是一个随新提交移动的指针——不是真正的快照
- 标签是不可变的：`staged/pi-rust-bug-fixes-hq-cv-xrwki` 始终指向
  Convoy 进入 staging 阶段时的精确状态
- 标签在分支清理后仍然存在，且长期保留成本更低
- `dolt diff staged/convoy-A staged/convoy-B` 适用于标签

## 触发条件

这是三个共享同一个 Go 二进制文件的事件门控 Plugin 之一：

| Plugin | 事件 | 快照 |
|--------|-------|----------|
| `dolt-snapshots` | `convoy.created` | `open/` 标签（工作前基线） |
| `dolt-snapshots-staged` | `convoy.staged` | `staged/` 标签 + 分支（staging 基线） |
| `dolt-snapshots-launched` | `convoy.launched` | `staged/` 标签 + 分支（launch 基线） |

每个在其特定事件上触发。二进制文件是幂等的——它检查所有
Convoy 并创建缺失的标签/分支。

## 步骤 1：构建并启动快照监听器

Go 二进制文件使用参数化 SQL 处理所有 Dolt 操作。
它使用 gastown 标准 Dolt 配置连接（127.0.0.1:3307, root, 无密码）
并读取 routes.jsonl 来发现 rig 数据库。

在 `--watch` 模式下，二进制文件跟踪 `~/.events.jsonl`，当检测到
Convoy 事件时立即（<1s）运行快照周期。这比约 60s 的
deacon 巡检轮询方式快得多——对于 `convoy.launched` 事件尤为关键，
因为代理会立即开始写入数据库。

```bash
PLUGIN_DIR="$(dirname "$0")"
PIDFILE="$PLUGIN_DIR/.snapshot.pid"

# 如果监听器已在运行，跳过
if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
  echo "Snapshot watcher already running (PID $(cat "$PIDFILE"))"
  exit 0
fi

# 如果二进制文件缺失或源码更新则构建
if [ ! -f "$PLUGIN_DIR/snapshot" ] || [ "$PLUGIN_DIR/main.go" -nt "$PLUGIN_DIR/snapshot" ]; then
  echo "Building dolt-snapshots binary..."
  cd "$PLUGIN_DIR" && go build -o snapshot . 2>&1
  if [ $? -ne 0 ]; then
    echo "FATAL: Go build failed"
    exit 1
  fi
fi

# 先运行一次性快照以补上监听器停机期间遗漏的内容
"$PLUGIN_DIR/snapshot" --cleanup --routes "$HOME/gt/.beads/routes.jsonl"
SNAPSHOT_EXIT=$?

if [ $SNAPSHOT_EXIT -ne 0 ]; then
  echo "Snapshot catch-up exited with code $SNAPSHOT_EXIT"
fi

# 在后台启动监听器（对 Convoy 事件亚秒级响应）
nohup "$PLUGIN_DIR/snapshot" --watch --routes "$HOME/gt/.beads/routes.jsonl" \
  >> "$PLUGIN_DIR/.snapshot.log" 2>&1 &
echo $! > "$PIDFILE"
echo "Snapshot watcher started (PID $!)"
```

## 步骤 2：记录结果

```bash
RESULT="success"
if [ $SNAPSHOT_EXIT -ne 0 ]; then
  RESULT="failure"
fi

bd create "dolt-snapshots: $RESULT" -t chore --ephemeral \
  -l type:plugin-run,plugin:dolt-snapshots,result:$RESULT \
  -d "dolt-snapshots plugin completed with exit code $SNAPSHOT_EXIT. Watcher started." --silent 2>/dev/null || true
```