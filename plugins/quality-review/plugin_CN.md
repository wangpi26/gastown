+++
name = "quality-review"
description = "审查合并质量并跟踪每个 Worker 的趋势"
version = 1

[gate]
type = "cooldown"
duration = "6h"

[tracking]
labels = ["plugin:quality-review", "category:quality"]
digest = true

[execution]
timeout = "5m"
notify_on_failure = true
severity = "medium"
+++

# Quality Review — 趋势分析

此插件在 Deacon 巡检期间每 6 小时运行一次。它分析 Refinery 在合并时
记录的 quality-review 结果 wisp，计算每个 Worker 的趋势，
并在质量违规时发出告警。

## 步骤 1：查询近期的 quality-review 结果

获取最近 24 小时内的所有 quality-review 结果 wisp：

```bash
bd list --json --all -l type:plugin-run,plugin:quality-review-result --created-after=-24h
```

如果没有找到结果，记录一次运行 wisp 并停止：

```bash
bd create "quality-review: No results in last 24h" -t chore --ephemeral \
  -l type:plugin-run,plugin:quality-review,result:success \
  -d "No quality-review results in last 24h. Nothing to analyze." \
  --silent 2>/dev/null || true
```

## 步骤 2：计算每个 Worker 的趋势

解析 wisp 标签以提取每个 Worker 的数据。每个结果 wisp 包含以下标签：
- `worker:<polecat-name>`
- `rig:<rig-name>`
- `score:<0.0-1.0>`
- `recommendation:<approve|request_changes>`

对每个 Worker，计算：
- **平均分数**：时间窗口内所有结果的均值
- **驳回率**：`recommendation:request_changes` 的计数 / 总数
- **趋势方向**：比较时间窗口前半段与后半段的平均分
  - 差值 > 0.05：`improving`（改善中）
  - 差值 < -0.05：`declining`（下降中）
  - 否则：`stable`（稳定）

## 步骤 3：对 Worker 状态分类

对每个 Worker 的平均分应用阈值：
- **OK**：平均值 >= 0.60
- **WARN**：0.45 <= 平均值 < 0.60
- **BREACH**：平均值 < 0.45

## 步骤 4：对违规发出告警

对每个处于 BREACH 状态的 Worker，发送告警：

```bash
gt mail send mayor/ -s "Quality BREACH: <worker>" -m "Worker: <worker>
Rig: <rig>
Avg Score: <avg>
Reviews: <count>
Rejection Rate: <rate>%
Trend: <improving|stable|declining>

Action: Review recent merges from this worker for quality issues."
```

同时进行上报：

```bash
gt escalate "Quality BREACH: <worker> (avg: <avg>)" \
  --severity medium \
  --reason "Worker <worker> in rig <rig> has avg quality score <avg> over <count> reviews"
```

## 步骤 5：记录运行结果

为本次插件运行记录一个汇总 wisp：

```bash
bd create "quality-review: Analyzed <N> workers over <M> reviews" -t chore --ephemeral \
  -l type:plugin-run,plugin:quality-review,result:success \
  -d "Analyzed <N> workers over <M> reviews. <B> breaches, <W> warnings." \
  --silent 2>/dev/null || true
```

如果任何步骤意外失败，记录失败 wisp 并上报：

```bash
bd create "quality-review: FAILED" -t chore --ephemeral \
  -l type:plugin-run,plugin:quality-review,result:failure \
  -d "<error description>" \
  --silent 2>/dev/null || true

gt escalate "Plugin FAILED: quality-review" \
  --severity medium \
  --reason "$ERROR"
```

---

## 分数是如何记录的（参考）

此插件本身不记录分数。Refinery 在合并期间通过
`quality-review` 配方步骤记录结果 wisp。每次合并产生一个如下 wisp：

```bash
bd create "quality-review: Score 0.85, approve" -t chore --ephemeral \
  -l type:plugin-run,plugin:quality-review-result,worker:<polecat-name>,rig:<rig-name>,score:0.85,recommendation:approve,result:success \
  -d "Score: 0.85, approve. Issues: 1 minor (style)" \
  --silent 2>/dev/null || true
```

这些数据供步骤 1 查询使用。