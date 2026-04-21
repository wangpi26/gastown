+++
name = "compactor-dog"
description = "监控生产 Dolt 数据库的提交增长，在需要压缩时进行上报"
version = 1

[gate]
type = "cooldown"
duration = "30m"

[tracking]
labels = ["plugin:compactor-dog", "category:maintenance"]
digest = true

[execution]
timeout = "5m"
notify_on_failure = true
severity = "medium"
+++

# Compactor Dog

监控所有生产 Dolt 数据库的提交增长，当需要执行历史压缩或
flatten 操作时上报给 Mayor。这是一个需要判断的决策，
而非硬阈值触发。

**你是一个 Dog 代理（Claude）。收集下方数据，然后根据你的判断
决定是否需要维护。** 请考虑以下因素：

- 每个数据库的提交数（绝对数量）
- 增长速率（自上次检查以来每小时的提交数）
- 距上次 flatten 或压缩的时间
- 当前 swarm 活跃度（更多 Polecat = 更快增长）
- 增长是"正常繁忙"还是"失控"

## 配置

```bash
DOLT_HOST="127.0.0.1"
DOLT_PORT=3307
DOLT_USER="root"
DOLT_DATA_DIR="$HOME/gt/.dolt-data"
STATE_FILE="$HOME/gt/.dolt-data/.compactor-state.json"
```

## 步骤 1：发现生产数据库

查找 Dolt 服务器上所有活跃的生产数据库：

```bash
echo "=== Compactor Dog: Checking commit health ==="

PROD_DBS=$(dolt sql -q "SHOW DATABASES" \
  --host "$DOLT_HOST" --port "$DOLT_PORT" -u "$DOLT_USER" \
  --result-format csv 2>/dev/null \
  | tail -n +2 \
  | grep -v -E '^(information_schema|mysql|dolt_cluster|testdb_|beads_t|beads_pt|doctest_)' \
  | tr -d '\r')

if [ -z "$PROD_DBS" ]; then
  echo "SKIP: No production databases found (is Dolt running?)"
  exit 0
fi

echo "Production databases: $(echo "$PROD_DBS" | tr '\n' ' ')"
```

## 步骤 2：统计每个数据库的提交数

查询每个数据库的提交历史：

```bash
echo ""
echo "=== Commit Counts ==="

REPORT=""
TOTAL_COMMITS=0
NOW=$(date +%s)

while IFS= read -r DB; do
  [ -z "$DB" ] && continue

  # 总提交数
  COUNT=$(dolt sql -q "SELECT count(*) AS cnt FROM dolt_log" \
    --host "$DOLT_HOST" --port "$DOLT_PORT" -u "$DOLT_USER" \
    -d "$DB" --result-format csv 2>/dev/null \
    | tail -1 | tr -d '\r')

  # 最近一小时的提交数（增长率指标）
  RECENT=$(dolt sql -q "SELECT count(*) AS cnt FROM dolt_log WHERE date > DATE_SUB(NOW(), INTERVAL 1 HOUR)" \
    --host "$DOLT_HOST" --port "$DOLT_PORT" -u "$DOLT_USER" \
    -d "$DB" --result-format csv 2>/dev/null \
    | tail -1 | tr -d '\r')

  # 最近 24 小时的提交数
  DAILY=$(dolt sql -q "SELECT count(*) AS cnt FROM dolt_log WHERE date > DATE_SUB(NOW(), INTERVAL 24 HOUR)" \
    --host "$DOLT_HOST" --port "$DOLT_PORT" -u "$DOLT_USER" \
    -d "$DB" --result-format csv 2>/dev/null \
    | tail -1 | tr -d '\r')

  # 最早提交日期（近似判断上次 flatten 时间）
  OLDEST=$(dolt sql -q "SELECT MIN(date) AS oldest FROM dolt_log" \
    --host "$DOLT_HOST" --port "$DOLT_PORT" -u "$DOLT_USER" \
    -d "$DB" --result-format csv 2>/dev/null \
    | tail -1 | tr -d '\r')

  LINE="$DB: total=$COUNT, last_1h=$RECENT, last_24h=$DAILY, oldest_commit=$OLDEST"
  echo "  $LINE"
  REPORT="$REPORT\n$LINE"
  TOTAL_COMMITS=$((TOTAL_COMMITS + ${COUNT:-0}))
done <<< "$PROD_DBS"

echo ""
echo "Total commits across all DBs: $TOTAL_COMMITS"
```

## 步骤 3：检查 swarm 活跃度

统计活跃的 Polecat 和 Dog 数量，以评估预期的提交速率：

```bash
echo ""
echo "=== Swarm Activity ==="

# 统计活跃的 tmux 会话数（作为代理活跃度的代理指标）
POLECAT_SESSIONS=$(tmux list-sessions -F '#{session_name}' 2>/dev/null \
  | grep -c 'polecat\|pcat' || echo 0)
DOG_SESSIONS=$(tmux list-sessions -F '#{session_name}' 2>/dev/null \
  | grep -c 'dog' || echo 0)
TOTAL_SESSIONS=$(tmux list-sessions 2>/dev/null | wc -l | tr -d ' ')

echo "  Active polecats: $POLECAT_SESSIONS"
echo "  Active dogs: $DOG_SESSIONS"
echo "  Total sessions: $TOTAL_SESSIONS"
```

## 步骤 4：加载上次状态

检查上次压缩或 flatten 的时间：

```bash
echo ""
echo "=== Previous State ==="

if [ -f "$STATE_FILE" ]; then
  LAST_CHECK=$(cat "$STATE_FILE" 2>/dev/null)
  echo "  Last state: $LAST_CHECK"
else
  echo "  No previous state found (first run)"
  LAST_CHECK="{}"
fi

# 检查 beads 中最近的 compactor-dog 或 flatten 运行记录
RECENT_RUNS=$(bd list --label plugin:compactor-dog --status closed --json 2>/dev/null \
  | jq -r '.[0].created_at // "never"' 2>/dev/null || echo "unknown")
echo "  Last compactor run: $RECENT_RUNS"

# 检查 flatten 迹象（单提交历史 = 最近被 flatten 过）
FLATTEN_CANDIDATES=""
while IFS= read -r DB; do
  [ -z "$DB" ] && continue
  COUNT=$(dolt sql -q "SELECT count(*) AS cnt FROM dolt_log" \
    --host "$DOLT_HOST" --port "$DOLT_PORT" -u "$DOLT_USER" \
    -d "$DB" --result-format csv 2>/dev/null \
    | tail -1 | tr -d '\r')
  if [ "${COUNT:-0}" -le 5 ]; then
    FLATTEN_CANDIDATES="$FLATTEN_CANDIDATES $DB(${COUNT})"
  fi
done <<< "$PROD_DBS"

if [ -n "$FLATTEN_CANDIDATES" ]; then
  echo "  Recently flattened DBs:$FLATTEN_CANDIDATES"
fi
```

## 步骤 5：保存当前状态

记录本次检查的数据，供下次运行比较增长率：

```bash
# 保存状态供下次运行使用
cat > "$STATE_FILE" << STATEOF
{
  "checked_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "total_commits": $TOTAL_COMMITS,
  "active_polecats": $POLECAT_SESSIONS,
  "active_dogs": $DOG_SESSIONS
}
STATEOF

echo ""
echo "State saved to $STATE_FILE"
```

## 步骤 6：做出判断

**这是你（Dog 代理）运用判断的地方。** 回顾上面收集的所有数据，
决定是否需要上报。

**判断参考指南**（不是硬性规则——上下文很重要）：

| 指标 | 正常 | 需要关注 | 需要上报 |
|--------|------------|--------------|----------|
| 总提交数（每个数据库） | <200 | 200-500 | >500 |
| 每小时增长率 | <10/hr | 10-30/hr | >30/hr |
| 每日增长率 | <100/day | 100-300/day | >300/day |
| 距上次 flatten 时间 | <2 周 | 2-4 周 | >4 周 |

**但如果有上下文依据，可以推翻表格的结论：**
- 10 个 Polecat 的 swarm 产生 400 次提交 = 正常，会自行稳定
- 没有活跃 swarm 但 200 次提交以 50/hr 增长 = 出问题了
- 任何超过 1000 次提交的数据库 = 无论何种情况都需上报

**如果你判断需要维护：**

```bash
gt escalate "Dolt compaction recommended" \
  -s MEDIUM \
  --reason "Commit growth analysis:
$REPORT

Total: $TOTAL_COMMITS commits across all DBs
Active polecats: $POLECAT_SESSIONS
Recommendation: Run compaction on databases exceeding comfort threshold.
See dolt-storage.md for procedure."
```

**如果一切正常，只需记录结果：**

```bash
echo "All databases within comfortable commit ranges. No action needed."
```

## 记录结果

```bash
SUMMARY="Compactor check: $TOTAL_COMMITS total commits across $(echo "$PROD_DBS" | wc -l | tr -d ' ') DBs, $POLECAT_SESSIONS active polecats"
echo "=== $SUMMARY ==="
```

成功时（无需上报）：
```bash
bd create "compactor-dog: $SUMMARY" -t chore --ephemeral \
  -l type:plugin-run,plugin:compactor-dog,result:success \
  -d "$SUMMARY" --silent 2>/dev/null || true
```

需要上报时：
```bash
bd create "compactor-dog: ESCALATED - $SUMMARY" -t chore --ephemeral \
  -l type:plugin-run,plugin:compactor-dog,result:warning \
  -d "Escalated to Mayor for compaction. $SUMMARY" --silent 2>/dev/null || true
```

失败时：
```bash
bd create "compactor-dog: FAILED" -t chore --ephemeral \
  -l type:plugin-run,plugin:compactor-dog,result:failure \
  -d "Compactor check failed: $ERROR" --silent 2>/dev/null || true

gt escalate "Plugin FAILED: compactor-dog" \
  --severity medium \
  --reason "$ERROR"
```