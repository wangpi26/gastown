+++
name = "stuck-agent-dog"
description = "上下文感知的卡死/崩溃代理检测与重启，适用于 Polecat 和 Deacon"
version = 1

[gate]
type = "cooldown"
duration = "5m"

[tracking]
labels = ["plugin:stuck-agent-dog", "category:health"]
digest = true

[execution]
timeout = "5m"
notify_on_failure = true
severity = "high"
+++

# Stuck Agent Dog

通过检查 tmux 会话上下文来检测卡死或崩溃的 Polecat 和 Deacon，
而非盲目采取行动。与守护进程的盲杀重启方式不同，此插件
在重启前会检查代理是否真正无响应。

**设计原则**：守护进程永远不应杀死 Worker。它只检测和记录。
此插件（以拥有 AI 判断能力的 Dog 代理运行）在检查
tmux 面板输出的生命迹象后做出重启决策。

参考：WAR-ROOM-SERIAL-KILLER.md，提交 f3d47a96。

## 范围 — 可以和不可以操作的对象

**在范围内**（此插件只能检查或操作以下会话）：
- Polecat 会话（`<rig>-polecat-<name>`）
- Deacon 会话（`hq-deacon`）

**不在范围内 — 在任何情况下都绝不能触碰：**
- **Crew 会话**（`<rig>-crew-<name>`，例如 `gastown-crew-bear`）。Crew 的生命周期
  由 overseer（人类）管理，而非 Dog。Crew 成员是持久的、
  长生命周期的、用户管理的。看起来空闲的 Crew 会话并非
  卡死——它在等待其人类。杀死 Crew 会话会破坏 overseer 的
  活跃工作区，属于**严重事故**。
- **Mayor 会话**（`hq-mayor`）
- **Witness 会话**（`<rig>-witness`）
- **Refinery 会话**（`<rig>-refinery`）
- 任何未被步骤 1-3 的 bash 脚本明确枚举的会话

**此范围是绝对的。** 不要根据自己的判断扩展它。bash
脚本精确枚举了你应该检查的会话。如果一个会话没有出现在
`CRASHED[]` 或 `STUCK[]` 数组中，它对你来说就不存在。

## 步骤 1：枚举需要检查的代理

收集所有 Polecat 和 Deacon 会话。我们同时检查崩溃的会话
（会话已死，工作在 hook 上）和卡死的会话（会话存活但代理挂起）。

```bash
echo "=== Stuck Agent Dog: Checking agent health ==="

TOWN_ROOT="$HOME/gt"
RIGS_JSON_PATH="${TOWN_ROOT}/rigs.json"

# 回退：对于仍在 mayor/ 下暴露 rigs.json 的旧版/运行时复制布局
if [ ! -f "$RIGS_JSON_PATH" ] && [ -f "$TOWN_ROOT/mayor/rigs.json" ]; then
  RIGS_JSON_PATH="$TOWN_ROOT/mayor/rigs.json"
fi

# 读取 rigs.json 获取 rig 名称和 beads 前缀
# 关键：我们需要 rig 名称（用于文件系统路径如 $TOWN_ROOT/$RIG/polecats/）
# 和 beads 前缀（用于 tmux 会话名如 $PREFIX-polecat-$NAME）。
# 这两者可能不同——例如 rig "cfutons" 可能有前缀 "CF"。
if [ ! -f "$RIGS_JSON_PATH" ]; then
  echo "SKIP: rigs.json not found at $RIGS_JSON_PATH"
  exit 0
fi

if ! RIG_PREFIX_MAP=$(jq -r '
  if (.rigs | type) == "object" then
    .rigs | to_entries[] | "\(.key)|\(.value.beads.prefix // .key)"
  else
    empty
  end
' "$RIGS_JSON_PATH" 2>/dev/null); then
  echo "SKIP: could not parse rigs.json"
  exit 0
fi

# 过滤掉格式错误/空行，使部分注册表状态安全失败
RIG_PREFIX_MAP=$(printf '%s\n' "$RIG_PREFIX_MAP" | awk -F'|' 'NF >= 2 && $1 != "" && $2 != ""')
if [ -z "$RIG_PREFIX_MAP" ]; then
  echo "SKIP: no rigs found in rigs.json"
  exit 0
fi
```

## 步骤 2：检查 Polecat 健康状态

对每个 rig，枚举 Polecat 并检查其会话状态。
Polecat 在以下情况需要关注：
- 有挂起的工作（hook_bead 已设置）
- 其 tmux 会话已死或代理进程已死

```bash
CRASHED=()
STUCK=()
HEALTHY=0

while IFS='|' read -r RIG PREFIX; do
  [ -z "$RIG" ] && continue
  # 列出 polecat 目录
  POLECAT_DIR="$TOWN_ROOT/$RIG/polecats"
  [ -d "$POLECAT_DIR" ] || continue

  for PCAT_PATH in "$POLECAT_DIR"/*/; do
    [ -d "$PCAT_PATH" ] || continue
    PCAT_NAME=$(basename "$PCAT_PATH")
    # 使用 beads 前缀（而非 rig 名称）作为 tmux 会话名
    SESSION_NAME="${PREFIX}-polecat-${PCAT_NAME}"

    # 检查会话是否存在
    if ! tmux has-session -t "$SESSION_NAME" 2>/dev/null; then
      # 会话已死——检查是否有挂起的工作
      HOOK_BEAD=$(bd show "$RIG/polecats/$PCAT_NAME" --json 2>/dev/null \
        | jq -r '.hook_bead // empty' 2>/dev/null)

      if [ -n "$HOOK_BEAD" ]; then
        # 检查 agent_state 以避免对主动关闭产生误报
        AGENT_STATE=$(bd show "$RIG/polecats/$PCAT_NAME" --json 2>/dev/null \
          | jq -r '.agent_state // empty' 2>/dev/null)
        if [ "$AGENT_STATE" = "spawning" ]; then
          echo "  SKIP $SESSION_NAME: agent_state=spawning (sling in progress)"
          continue
        fi
        if [ "$AGENT_STATE" = "done" ] || [ "$AGENT_STATE" = "nuked" ]; then
          echo "  SKIP $SESSION_NAME: agent_state=$AGENT_STATE (intentional shutdown, not a crash)"
          continue
        fi
        CRASHED+=("$SESSION_NAME|$RIG|$PCAT_NAME|$HOOK_BEAD")
        echo "  CRASHED: $SESSION_NAME (hook=$HOOK_BEAD)"
      fi
    else
      # 会话存活——检查代理进程是否还活着
      # 捕获面板最近 5 行输出以检查生命迹象
      PANE_OUTPUT=$(tmux capture-pane -t "$SESSION_NAME" -p -S -5 2>/dev/null || echo "")

      # 检查会话中是否有代理进程在运行
      PANE_PID=$(tmux list-panes -t "$SESSION_NAME" -F '#{pane_pid}' 2>/dev/null | head -1)
      if [ -n "$PANE_PID" ]; then
        # 检查 Claude 或其他代理进程是否为子进程
        AGENT_ALIVE=$(pgrep -P "$PANE_PID" -f 'claude|node|anthropic' 2>/dev/null | head -1)
        if [ -z "$AGENT_ALIVE" ]; then
          # 代理进程已死但会话存活——僵尸会话
          HOOK_BEAD=$(bd show "$RIG/polecats/$PCAT_NAME" --json 2>/dev/null \
            | jq -r '.hook_bead // empty' 2>/dev/null)
          if [ -n "$HOOK_BEAD" ]; then
            STUCK+=("$SESSION_NAME|$RIG|$PCAT_NAME|$HOOK_BEAD|agent_dead")
            echo "  ZOMBIE: $SESSION_NAME (agent dead, session alive, hook=$HOOK_BEAD)"
          fi
        else
          HEALTHY=$((HEALTHY + 1))
        fi
      else
        HEALTHY=$((HEALTHY + 1))
      fi
    fi
  done
done <<< "$RIG_PREFIX_MAP"

echo ""
echo "Health summary: ${#CRASHED[@]} crashed, ${#STUCK[@]} stuck, $HEALTHY healthy"
```

## 步骤 3：检查 Deacon 健康状态

Deacon 会话名为 `hq-deacon`。检查心跳是否过期。

```bash
echo ""
echo "=== Deacon Health ==="

DEACON_SESSION="hq-deacon"
DEACON_ISSUE=""

if ! tmux has-session -t "$DEACON_SESSION" 2>/dev/null; then
  echo "  CRASHED: Deacon session is dead"
  DEACON_ISSUE="crashed"
else
  # 检查 deacon 心跳文件
  HEARTBEAT_FILE="$TOWN_ROOT/deacon/heartbeat.json"
  if [ -f "$HEARTBEAT_FILE" ]; then
    HEARTBEAT_TIME=$(jq -r '(.timestamp // empty) | sub("\\.[0-9]+Z$"; "Z") | fromdateiso8601? // empty' "$HEARTBEAT_FILE" 2>/dev/null)
    if [ -n "$HEARTBEAT_TIME" ]; then
      NOW=$(date +%s)
      HEARTBEAT_AGE=$(( NOW - HEARTBEAT_TIME ))

      if [ "$HEARTBEAT_AGE" -gt 900 ]; then
        echo "  STUCK: Deacon heartbeat stale (${HEARTBEAT_AGE}s old, >15m threshold)"
        DEACON_ISSUE="stuck_heartbeat_${HEARTBEAT_AGE}s"
      else
        echo "  OK: Deacon heartbeat ${HEARTBEAT_AGE}s old"
      fi
    else
      echo "  WARN: Could not parse heartbeat timestamp from $HEARTBEAT_FILE"
    fi
  else
    echo "  WARN: No heartbeat file found at $HEARTBEAT_FILE"
  fi
fi
```

## 步骤 4：操作前检查上下文（AI 判断）

**这是与守护进程盲杀方式的关键区别。** 对每个崩溃或卡死的
代理，检查 tmux 面板上下文以确定是否适合重启。

**范围提醒：你只能操作步骤 2-3 填充的 `CRASHED[]` 和 `STUCK[]`
数组中的条目。这些数组仅包含 Polecat 和 Deacon。
不要检查、评估或操作任何其他会话（Crew、Mayor、Witness、
Refinery）。如果你发现自己在考虑不在此数组中的会话，请停止。**

**你（Dog 代理）必须评估每种情况：**

对于 CRASHED 代理（会话已死，工作在 hook 上）：
- 这几乎总是需要重启的真正崩溃
- 例外：如果 Polecat 刚刚执行了 `gt done` 而 hook 尚未清除
- 检查 Bead 状态：如果根 wisp 已关闭，说明 Polecat 正常完成了

对于 STUCK 代理（会话存活，代理已死）：
- 先杀死僵尸会话，然后重启
- 例外：如果面板输出显示代理正在执行长时间运行的构建/测试

对于 DEACON 卡死（心跳过期）：
- 捕获面板输出：`tmux capture-pane -t hq-deacon -p -S -20`
- 如果输出显示活跃工作（近期时间戳、命令输出），心跳
  文件可能只是过期了——先轻推而非杀死
- 如果输出显示近期无活动，重启是合理的

**决策框架：**
1. 如果代理明确已死（无进程、无输出） → 重启
2. 如果代理在面板中显示近期活动 → 先轻推，下个周期再检查
3. 如果代理已卡死超过 15 分钟且无面板活动 → 重启
4. 如果检测到大规模宕机（同一周期 >3 个崩溃） → 上报，不要重启

## 步骤 5：采取行动

对每个需要重启的代理：

```bash
# 对于崩溃的 Polecat——通知 Witness 处理重启
for ENTRY in "${CRASHED[@]}"; do
  IFS='|' read -r SESSION RIG PCAT HOOK <<< "$ENTRY"

  echo "Requesting restart for $RIG/polecats/$PCAT (hook=$HOOK)"

  gt mail send "$RIG/witness" \
    -s "RESTART_POLECAT: $RIG/$PCAT" \
    --stdin <<BODY
Polecat $PCAT crash confirmed by stuck-agent-dog plugin.
Context-aware inspection completed — agent is genuinely dead.

hook_bead: $HOOK
action: restart requested

Please restart this polecat session.
BODY

done

# 对于僵尸 Polecat——先杀死僵尸会话，再请求重启
for ENTRY in "${STUCK[@]}"; do
  IFS='|' read -r SESSION RIG PCAT HOOK REASON <<< "$ENTRY"

  echo "Killing zombie session $SESSION and requesting restart"
  tmux kill-session -t "$SESSION" 2>/dev/null || true

  gt mail send "$RIG/witness" \
    -s "RESTART_POLECAT: $RIG/$PCAT (zombie cleared)" \
    --stdin <<BODY
Polecat $PCAT zombie session cleared by stuck-agent-dog plugin.
Session was alive but agent process was dead.

hook_bead: $HOOK
reason: $REASON
action: restart requested

Please restart this polecat session.
BODY

done

# 对于 Deacon 问题
if [ -n "$DEACON_ISSUE" ]; then
  echo "Escalating deacon issue: $DEACON_ISSUE"
  gt escalate "Deacon $DEACON_ISSUE detected by stuck-agent-dog" \
    -s HIGH \
    --reason "Deacon issue: $DEACON_ISSUE. Context inspection completed."
fi
```

## 步骤 6：大规模宕机检查

如果多个代理在同一周期崩溃，可能表明存在系统性
问题（Dolt 宕机、OOM 等）。此时应上报而非盲目重启所有代理。

```bash
TOTAL_ISSUES=$(( ${#CRASHED[@]} + ${#STUCK[@]} ))
if [ "$TOTAL_ISSUES" -ge 3 ]; then
  echo "MASS DEATH: $TOTAL_ISSUES agents down in same cycle — escalating"
  gt escalate "Mass agent death: $TOTAL_ISSUES agents down" \
    -s CRITICAL \
    --reason "stuck-agent-dog detected $TOTAL_ISSUES agents down simultaneously.
Crashed: ${CRASHED[*]}
Stuck: ${STUCK[*]}
This may indicate a systemic issue (Dolt, OOM, infra). Investigate before mass restart."
fi
```

## 记录结果

```bash
SUMMARY="Agent health check: ${#CRASHED[@]} crashed, ${#STUCK[@]} stuck, $HEALTHY healthy"
if [ -n "$DEACON_ISSUE" ]; then
  SUMMARY="$SUMMARY, deacon=$DEACON_ISSUE"
fi
echo "=== $SUMMARY ==="
```

成功时（无问题或问题已处理）：
```bash
bd create "stuck-agent-dog: $SUMMARY" -t chore --ephemeral \
  -l type:plugin-run,plugin:stuck-agent-dog,result:success \
  -d "$SUMMARY" --silent 2>/dev/null || true
```

失败时：
```bash
bd create "stuck-agent-dog: FAILED" -t chore --ephemeral \
  -l type:plugin-run,plugin:stuck-agent-dog,result:failure \
  -d "Agent health check failed: $ERROR" --silent 2>/dev/null || true

gt escalate "Plugin FAILED: stuck-agent-dog" \
  --severity high \
  --reason "$ERROR"
```