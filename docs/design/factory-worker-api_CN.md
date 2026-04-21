# Factory Worker API

Gas Town 与 AI Agent 运行时之间 API 边界的设计。

参考: gt-5zs8

## 问题

Gas Town 与 AI agent 之间没有稳定的接口。每个集成都是 hack：

- **提示词投递**：tmux send-keys 配合 512 字节分块、ESC+600ms readline
  舞步、Enter 重试、SIGWINCH 唤醒
- **空闲检测**：对 Claude Code 的 `❯` 提示符前缀（带 NBSP
  标准化）和 `⏵⏵` 状态栏解析的正则匹配
- **遥测**：文件系统抓取 `~/.claude/projects/<slug>/<session>.jsonl`
- **成本追踪**：从 JSONL 记录解析 token 用量，使用硬编码定价
- **账户轮换**：仅 macOS 的 keychain token 交换
- **存活检测**：通过 `pane_current_command` + `pgrep` 遍历 tmux 进程树
- **Guard 脚本**：PreToolUse hook 的退出码 2 来拦截命令
- **权限绕过**：10 种不同的 `--dangerously-*` 标志，每种 agent 供应商一个

已编目 28+ 个触点。全部依赖 Claude Code、tmux、macOS Keychain
或可能随时变更的文件系统约定的实现细节。

## 设计原则

1. **推送，而非抓取。** Agent 报告自身状态；GT 不从终端输出猜测。
2. **结构化，而非字符串匹配。** JSON 消息，而非窗格内容的正则。
3. **Agent 无关。** 一个 API，无论 worker 是 Claude、Gemini、
   Codex 还是自定义运行时。
4. **设计即关联。** 单个 `run_id` 贯穿从生成到消亡的每个事件。
5. **默认安全关闭。** 未知状态 = 不发送工作，不杀死会话。

## API 接口

### 1. 生命周期

运行时报告生命周期转换。GT 不推断它们。

```
POST /lifecycle
{
  "event": "started" | "ready" | "busy" | "idle" | "stopping" | "stopped",
  "run_id": "uuid",
  "session_id": "gt-crew-max",
  "timestamp": "2026-03-01T15:00:00Z",
  "metadata": {}           // 事件特定的（例如 "stopped" 的 exit_code）
}
```

替代：提示符前缀匹配、状态栏解析、`pane_current_command`、
`IsAgentAlive()`、`GetSessionActivity()`、心跳文件、`WaitForIdle()` 轮询、
`WaitForRuntimeReady()` 轮询。

**关键事件：**

| 事件 | 替代 | 时机 |
|------|------|------|
| `started` | 会话创建检测 | Agent 进程开始 |
| `ready` | `WaitForRuntimeReady()` 轮询 | Agent 准备好接收首个提示词 |
| `idle` | 提示符前缀 + 状态栏检测 | 回合完成，等待输入 |
| `busy` | "esc to interrupt" 检测 | 正在处理提示词 |
| `stopping` | done-intent 文件检测 | Agent 发起关闭 |
| `stopped` | 进程树检查 | Agent 进程退出 |

### 2. 提示词提交

GT 发送结构化消息。无终端注入。

```
POST /prompt
{
  "run_id": "uuid",
  "content": "审查 PR #2068...",
  "priority": "normal" | "urgent" | "system",
  "source": "nudge" | "mail" | "sling" | "prime",
  "metadata": {
    "from": "gastown/crew/tom",
    "bead_id": "gt-abc12"
  }
}

响应:
{
  "accepted": true,
  "queued": false,          // 如果 agent 忙碌则排队
  "position": 0             // 排队时的队列位置
}
```

替代：`NudgeSession()`（8 步 tmux send-keys 协议）、512 字节分块、
ESC+readline 舞步、去抖定时器、nudge 队列 JSON 文件、`UserPromptSubmit`
hook 排空、大提示词临时文件变通。

**优先级语义：**
- `system`：在回合边界注入（替代 `<system-reminder>` 块）
- `urgent`：中断当前工作（替代即时 nudge）
- `normal`：空闲时投递（替代 wait-idle + 队列回退）

### 3. 上下文注入（Priming）

会话启动和压缩时的结构化上下文投递。

```
POST /context
{
  "run_id": "uuid",
  "sections": [
    {"type": "role", "content": "你是一个 polecat worker..."},
    {"type": "work", "content": "AUTONOMOUS WORK MODE: gt-abc12..."},
    {"type": "mail", "content": "2 条未读消息..."},
    {"type": "checkpoint", "content": "前一会话状态..."},
    {"type": "directive", "content": "执行你的被 sling 的工作。"}
  ],
  "mode": "full" | "compact" | "resume"
}
```

替代：`gt prime` 管道（10 段输出）、`SessionStart` hook、
`PreCompact` hook、beacon 注入、非 hook agent 的启动 nudge 回退、
角色模板渲染到 stdout。

### 4. 工具授权

运行时在执行工具前询问 GT。GT 决定。

```
POST /authorize
{
  "run_id": "uuid",
  "tool": "Bash",
  "input": {"command": "git push --force"},
  "context": {
    "role": "polecat",
    "rig": "gastown",
    "bead_id": "gt-abc12"
  }
}

响应:
{
  "allowed": false,
  "reason": "force push 被 dangerous-command guard 拦截"
}
```

替代：退出码 2 的 `PreToolUse` hook、PR-workflow guard、dangerous-command
guard、patrol-formula guard、每 agent 的 `--dangerously-*` 标志。

**权限模型：**
- 按角色的权限集（polecat：完整、witness：只读、crew：可配置）
- Guard 规则为数据，非 shell 脚本
- 安全关闭：如果 GT 不可达，拦截工具调用

### 5. 遥测与成本报告

运行时推送结构化事件。无文件系统抓取。

```
POST /telemetry
{
  "run_id": "uuid",
  "events": [
    {
      "type": "turn_complete",
      "timestamp": "2026-03-01T15:01:00Z",
      "usage": {
        "input_tokens": 12000,
        "output_tokens": 3500,
        "cache_read_tokens": 8000,
        "cache_creation_tokens": 0,
        "model": "claude-opus-4-6",
        "cost_usd": 0.2325
      },
      "tools_called": [
        {"name": "Bash", "success": true, "duration_ms": 1200},
        {"name": "Read", "success": true, "duration_ms": 50}
      ]
    }
  ]
}
```

替代：JSONL 记录抓取、`extractCostFromWorkDir()`、硬编码定价
表、`agentlog` 包（Claude Code JSONL tailing）、`RecordPaneRead`、Stop hook
`gt costs record`、6 个独立日志文件无关联。

**流动内容：**
- 每回合的 token 用量（而非每内容块 — 避免重复计数）
- 成本在源头计算（运行时知道模型和定价）
- 工具调用结果含成功/失败和计时
- 所有事件携带 `run_id` 用于关联

### 6. 身份与凭证

GT 分配身份；运行时认证。

```
POST /identity
{
  "run_id": "uuid",
  "role": "polecat",
  "rig": "gastown",
  "agent_name": "alpha",
  "session_id": "gt-gastown-alpha",
  "credentials": {
    "type": "api_key" | "oauth" | "token",
    "value": "sk-ant-...",
    "expires_at": "2026-03-02T00:00:00Z"
  },
  "env": {
    "GT_ROLE": "gastown/polecats/alpha",
    "BD_ACTOR": "gastown/polecats/alpha",
    "GT_ROOT": "/Users/stevey/gt"
  }
}
```

替代：`AgentEnv()`（30+ 个通过 tmux `SetEnvironment` + `PrependEnv` 的环境变量）、
macOS keychain token 交换、`CLAUDE_CONFIG_DIR` 隔离、账户切换
符号链接、`GT_QUOTA_ACCOUNT` 环境变量、凭证透传白名单。

**凭证轮换：**
- GT 在轮换时推送新凭证；运行时无需重启即可应用
- 无 keychain 依赖 — 适用于任何操作系统
- 运行时报告凭证过期；GT 在过期前主动轮换

### 7. 健康与存活

双向健康检查。

```
GET /health
响应:
{
  "status": "healthy" | "degraded" | "unhealthy",
  "run_id": "uuid",
  "uptime_seconds": 3600,
  "current_state": "idle" | "busy" | "stopping",
  "last_activity": "2026-03-01T15:00:00Z",
  "context_usage": 0.73,       // 上下文窗口已用比例
  "error": null
}
```

替代：`CheckSessionHealth()`（3 级 tmux 检查）、`IsAgentAlive()`（进程
树遍历）、`GetSessionActivity()`（tmux 活动时间戳）、心跳文件、
`TouchSessionHeartbeat()`、僵尸检测启发式、spawn 风暴检测。

**上下文窗口压力**是新的信号 — 运行时知道其上下文有多满。
GT 可以使用此信号在 agent 退化前触发压缩/handoff。

## 传输

API 仅限本地。两种选择：

**Unix domain socket**（首选）：`$GT_ROOT/.runtime/worker.sock`
- 无网络暴露
- 基于文件权限的访问控制
- GT 是服务器；agent 运行时是客户端
- Agent 启动时连接，维持持久连接

**嵌入式 HTTP**：localhost 随机端口，写入已知文件。
- 不支持 Unix socket 的运行时的回退方案
- 端口文件位于 `$GT_ROOT/.runtime/worker-<session>.port`

## 迁移

Factory Worker API 不需要替换 Claude Code。它可以实现为
**sidecar**：

1. 在同一 tmux 会话中与 Claude Code 并行运行
2. 在 API 和 Claude Code 现有机制（hooks、JSONL、send-keys）
   之间转换
3. 随着 sidecar 成熟逐步替代 tmux 中介的交互

这使我们在构建完整 GT 原生运行时之前验证 API 设计。

## 非目标

- **多机编排。** 这是同一机器上 GT 和 worker 之间的本地 API。
- **Agent 智能。** API 是管道，不是策略。Agent 用提示词
  *做什么*是它自己的事。
- **与每个 agent 供应商的向后兼容。** Sidecar 处理
  转换。API 为 GT 原生运行时设计。

## 开放问题

1. 提示词提交应该是同步的（阻塞直到接受）还是即发即弃
   并带状态回调？
2. 工具授权应该是按调用（延迟成本）还是按会话带预协商
   能力集？
3. 压缩/handoff 如何工作？GT 告诉运行时"立即压缩"还是
   运行时报告上下文压力由 GT 决定？
4. 运行时应该暴露其对话历史，还是遥测流对 GT 的需求
   已足够？