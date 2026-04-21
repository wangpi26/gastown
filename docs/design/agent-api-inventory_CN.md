# Agent API 触点清单

GT 与 Agent 之间所有集成点的完整清单，映射到源代码及拟议的 Factory Worker API 端点。

参考: gt-5zs8 | 配套文档: [factory-worker-api.md](factory-worker-api.md)

---

## 如何阅读本文档

每个触点列出以下内容：
- **What（是什么）**: GT 通过此触点做了什么
- **Code（代码）**: 源文件和关键函数（行号在编辑后为近似值）
- **Flow（流向）**: 信息的流动方向（GT→Agent 或 Agent→GT）
- **Fragility（脆弱性）**: 什么会坏以及为什么
- **API mapping（API 映射）**: 哪个 Factory Worker API 端点替代它

---

## 1. 提示词投递（tmux send-keys）

**What**: GT 通过 tmux 终端注入将文本发送到 agent 会话。

**Code**:
- `internal/tmux/tmux.go` — `NudgeSession()`（约第 1300 行）：8 步协议
  （序列化 → 查找窗格 → 退出复制模式 → 清洗 → 按 512 字节分块 →
  500ms 去抖 → ESC + 600ms readline 舞步 → 带重试的 Enter → SIGWINCH 唤醒）
- `internal/tmux/tmux.go` — `sendMessageToTarget()`（约第 1210 行）：按 512 字节分割，
  分块间延迟 10ms
- `internal/tmux/tmux.go` — `sendKeysLiteralWithRetry()`（约第 1253 行）：启动竞态的
  指数退避（500ms→2s 上限）
- `internal/tmux/tmux.go` — `sanitizeNudgeMessage()`（约第 1179 行）：剥离 ESC、CR、BS、
  DEL；将 TAB 替换为空格
- `internal/tmux/tmux.go` — `SendKeys()`、`SendKeysDebounced()`、`SendKeysRaw()`、
  `SendKeysReplace()`、`SendKeysDelayed()` — 变体入口
- `internal/cmd/nudge.go` — `runNudge()`（约第 196 行）、`deliverNudge()`（约第 129 行）：
  CLI 入口，按模式路由（immediate/queue/wait-idle）

**Flow**: GT→Agent。文本字符串输入，无结构化响应。

**Fragility**:
- 600ms ESC 延迟必须超过 bash readline 的 500ms keyseq-timeout；否则
  ESC+Enter 会变成 M-Enter（meta-return）= 无法提交
- 512 字节分块大小是经验值；tmux send-keys 有未文档化的限制
- 清洗会去除控制字符，但无法处理所有边界情况
- 无投递确认 — GT 无法知道 agent 是否收到了消息
- 每会话的通道信号量（30s 超时）使并发的 nudge 串行化

**API mapping**: `POST /prompt` — 结构化 JSON 投递，带 accepted/queued 响应

---

## 2. 三种投递模式（immediate、wait-idle、queue）

**What**: GT 根据紧急程度通过三种模式路由提示词投递。

**Code**:
- `internal/cmd/nudge.go` — 模式常量：`NudgeModeImmediate`、`NudgeModeQueue`、
  `NudgeModeWaitIdle`（约第 38-44 行）
- `internal/nudge/queue.go` — `Enqueue()`（约第 86 行）：将 JSON 文件写入
  `.runtime/nudge_queue/<session>/`，使用纳秒时间戳的原子命名
- `internal/nudge/queue.go` — `Drain()`（约第 143 行）：通过重命名为
  `.claimed` 进行原子认领，对超过 5 分钟的废弃认领进行孤立恢复，过期过滤
- `internal/nudge/queue.go` — `FormatForInjection()`（约第 277 行）：将排队的 nudge
  格式化为 `<system-reminder>` 块，供 Claude Code hook 注入
- `internal/cmd/mail_check.go` — `runMailCheck()`（约第 16 行）：UserPromptSubmit hook
  排空队列 + 检查邮件，输出注入块
- `internal/mail/router.go` — `NotifyRecipient()`（约第 1568 行）：wait-idle 优先
  策略，3s 超时，队列回退

**Flow**: GT→Agent。Immediate：终端注入。Queue：文件→hook→注入。

**Fragility**:
- 队列排空依赖 UserPromptSubmit hook — 非 Claude agent 永远不会排空
- TTL 硬编码（normal：30 分钟，urgent：2 小时，最大深度：50）
- 空闲 agent 不会调用 Drain()，因此排队的 nudge 可能过期而未被看到
- Witness 向 Refinery 的 nudge 仅使用 immediate 模式（handlers.go 约第 639 行）

**API mapping**: `POST /prompt` 带有 `priority` 字段（system/urgent/normal）

---

## 3. 空闲检测（提示符前缀 + 状态栏）

**What**: GT 判断 agent 是否处于空闲状态（等待输入）或忙碌状态。

**Code**:
- `internal/tmux/tmux.go` — `matchesPromptPrefix()`（约第 2261 行）：NBSP 标准化
  （U+00A0→空格），匹配 `DefaultReadyPromptPrefix = "❯ "`（U+276F）
- `internal/tmux/tmux.go` — `IsIdle()`（约第 2386 行）：状态栏解析，检测 `⏵⏵`
  （U+23F5），忙碌 = 存在 "esc to interrupt"
- `internal/tmux/tmux.go` — `WaitForIdle()`（约第 2321 行）：200ms 间隔轮询，
  捕获 5 行窗格内容，返回 `ErrIdleTimeout`
- `internal/tmux/tmux.go` — `IsAtPrompt()`（约第 2359 行）：非阻塞的即时检查
- `internal/tmux/tmux.go` — `promptSuffixes`（约第 1478 行）：
  `[">", "$", "%", "#", "❯"]` 用于对话框检测

**Flow**: Agent→GT（推断的）。GT 抓取终端输出；agent 不知情。

**Fragility**:
- 提示符前缀 `❯` 是 Claude Code 的 UI 字符串 — 任何变更都会破坏检测
- 状态栏 `⏵⏵` 和 "esc to interrupt" 是未文档化的 Claude Code 内部实现
- NBSP 标准化是一个 bug 修复（issues/1387），针对 Claude Code 渲染变更
- 不同 agent 有不同的提示符 — 没有通用检测方法
- 即时检查：检查与状态变化之间存在竞态

**API mapping**: `POST /lifecycle` 带有 `event: "idle" | "busy"`

---

## 4. 速率限制检测（窗格内容扫描）

**What**: GT 扫描终端输出以检测触发账户轮换的速率受限会话。

**Code**:
- `internal/quota/scan.go` — `Scanner` 结构体、`ScanAll()`（约第 77 行）、
  `scanSession()`（约第 99 行）：捕获 30 行窗格内容，检查底部 20 行
  是否匹配速率限制正则表达式模式
- `internal/constants/constants.go` — `DefaultRateLimitPatterns`：速率限制消息的
  正则表达式模式

**Flow**: Agent→GT（推断的）。GT 读取窗格；agent 不参与。

**Fragility**:
- 正则表达式模式必须精确匹配速率限制错误消息
- 消息可能在不同 Claude Code 版本中变化
- 仅捕获 30 行中的底部 20 行 — 速率限制消息必须是最近的
- 没有 agent 端的结构化信号表明其被速率限制

**API mapping**: `POST /lifecycle` 带有 `event: "degraded"` + 速率限制元数据，
或 `POST /telemetry` 带有速率限制事件

---

## 5. 账户/配额管理（keychain token 交换）

**What**: 当账户触发速率限制时，GT 跨会话轮换 API 凭证。

**Code**:
- `internal/quota/keychain.go` — 仅 Darwin（289 行）：
  `KeychainServiceName()`（约第 35 行）：配置目录的 SHA-256 哈希，
  `SwapKeychainCredential()`（约第 78 行）：备份目标 → 读取源 → 写入目标，
  `SwapOAuthAccount()`（约第 121 行）：交换 `.claude.json` 的 oauthAccount 字段，
  `ValidateKeychainToken()`（约第 203 行）：检查过期时间（JSON、JWT、opaque）
- `internal/quota/scan.go` — `ScanAll()`（约第 77 行）：扫描速率受限的会话
- `internal/quota/rotate.go` — `PlanRotation()`（约第 42 行）：4 阶段管道
  （扫描 → 状态管理器 → 规划器 → 执行器）
- `internal/quota/executor.go` — `Rotator.Execute()`（约第 81 行）：使用 flock 的
  原子执行，独立会话上并发执行

**Flow**: GT→Agent。GT 交换凭证；agent 使用新 token 重启。

**Fragility**:
- 仅限 macOS — 整个 keychain 子系统仅支持 darwin，不支持 Linux/Windows
- 凭证交换需要会话重启（杀死进程 → 重启窗格）
- OAuth 账户字段在 `.claude.json` 中的位置未文档化
- SHA-256 键值假定 Claude Code 的 keychain 服务命名约定
- 没有 agent 端的凭证刷新 — 始终是完全重启

**API mapping**: `POST /identity` 带有 `credentials` 字段 — 运行时无需重启即可应用

---

## 6. 会话生命周期（创建、重启、销毁）

**What**: GT 创建、重启和销毁 agent 的 tmux 会话。

**Code**:
- `internal/session/lifecycle.go` — `StartSession()`（约第 121 行）：13 步统一
  生命周期（解析配置 → 设置 → 命令 → 会话 → 环境变量 → 主题 → 等待 →
  对话框 → 延迟 → 验证 → 重启 → PID 追踪）
- `internal/polecat/session_manager.go` — `Start()`（约第 186 行）：polecat 专用
  会话，含僵尸杀死、worktree、beacon、env 注入、pane-died hook
- `internal/witness/manager.go` — `Start()`（约第 107 行）：witness 会话，含
  僵尸宽限期、角色配置、主题、pane-died hook
- `internal/dog/session_manager.go` — `Start()`（约第 85 行）：dog 会话，通过
  统一的 `session.StartSession()`
- `internal/tmux/tmux.go` — `NewSessionWithCommand()`：单命令会话创建，
  `SetAutoRespawnHook()`（约第 3126 行）：带 3s 去抖的 pane-died 自动重启
- `internal/tmux/tmux.go` — `KillSessionWithProcesses()`（约第 499 行）：8 步销毁
  （进程组 → 树遍历 → SIGTERM → 2s 宽限 → SIGKILL → 窗格 → 会话）

**Flow**: GT→Agent。GT 控制整个生命周期；agent 是被动的。

**Fragility**:
- 13 步创建有很多故障点（tmux、对话框、就绪状态）
- 通过 pane-died hook 的自动重启依赖 tmux 的 hook 机制
- Kill 序列必须处理重新父化的进程（PPID=1 检查）
- 僵尸清理存在 TOCTOU 间隙（杀前重新验证）
- 会话创建耗时 5-60 秒，取决于 agent 启动时间

**API mapping**: `POST /lifecycle`（agent 报告状态转换），
`POST /identity`（GT 在创建时分配身份）

---

## 7. 启动准入控制

**What**: GT 通过健康检查和容量限制来控制 polecat 的创建。

**Code**:
- `internal/cmd/polecat_spawn.go` — `SpawnPolecatForSling()`（约第 62 行）：
  Dolt 健康检查、连接容量、polecat 数量上限（25）、每个 bead 的
  重启断路器、每个 rig 的目录上限（30）、空闲 polecat 复用
- `internal/polecat/manager.go` — `CheckDoltHealth()`（约第 223 行）：带
  指数退避 + 抖动的重试；`CheckDoltServerCapacity()`（约第 276 行）：
  连接数准入门控
- `internal/witness/spawn_count.go` — `ShouldBlockRespawn()`（约第 74 行）：
  同一 bead 重启 3 次后触发断路器，`RecordBeadRespawn()`（约第 104 行）：
  使用 flock 的跨进程计数器

**Flow**: GT 内部。准入决策不涉及 agent。

**Fragility**:
- Polecat 上限（25）和目录上限（30）是硬编码的
- 断路器状态存储在 JSON 文件中（`bead-respawn-counts.json`）
- Dolt 健康检查给每次启动增加延迟

**API mapping**: GT 编排内部 — 不属于面向 agent 的 API

---

## 8. Agent 身份（环境变量 + 预设注册表）

**What**: GT 通过环境变量和预设注册表为 agent 分配身份。

**Code**:
- `internal/config/env.go` — `AgentEnv()`（约第 65 行）：生成 30+ 个环境变量，
  包括 GT_ROLE、GT_RIG、GT_POLECAT、GT_CREW、BD_ACTOR、GIT_AUTHOR_NAME、
  GT_ROOT、GT_AGENT、GT_SESSION，以及 OTEL 和凭证透传
- `internal/config/agents.go` — `builtinPresets`（约第 164 行）：10 个 agent 预设
  （Claude、Gemini、Codex、Cursor、Auggie、AMP、OpenCode、Copilot、Pi、OMP），
  每个含 21 个字段（Command、Args、ProcessNames、SessionIDEnv 等）
- `internal/session/identity.go` — `ParseSessionName()`（约第 84 行）、
  `ParseAddress()`（约第 30 行）、`SessionName()`（约第 163 行）：身份解析
  和格式化
- `internal/constants/constants.go` — 角色常量（约第 196-215 行）：
  `RoleMayor`、`RoleDeacon`、`RoleWitness`、`RoleRefinery`、`RolePolecat`、`RoleCrew`

**Flow**: GT→Agent。GT 设置环境变量；agent 读取它们。

**Fragility**:
- 30+ 个环境变量必须在 tmux SetEnvironment 和 exec-env 之间保持同步
- 三种传播机制（tmux SetEnvironment、PrependEnv 内联、cmd.Env）可能分歧
- Agent 预设发现依赖 GT_AGENT 或 GT_PROCESS_NAMES 环境变量
- 角色检测层级（env → CWD → 回退）可能产生不匹配

**API mapping**: `POST /identity` — 包含所有字段的结构化身份分配

---

## 9. 上下文注入（Priming）

**What**: GT 在会话启动时注入角色上下文、工作分配和系统状态。

**Code**:
- `internal/cmd/prime.go` — `runPrime()`（约第 101 行）：完整 prime 或 compact/resume 路径
- `internal/cmd/prime_output.go` — `outputPrimeContext()`（约第 22 行）：角色特定
  上下文渲染；角色函数：`outputMayorContext()`、`outputWitnessContext()`、
  `outputRefineryContext()`、`outputPolecatContext()`、`outputCrewContext()` 等
- `internal/cmd/prime_session.go` — `handlePrimeHookMode()`（约第 266 行）：
  SessionStart hook 集成，从 stdin JSON 读取会话 ID，持久化到磁盘
- `internal/cmd/prime_session.go` — `detectSessionState()`（约第 202 行）：
  返回 "normal" | "post-handoff" | "crash-recovery" | "autonomous"
- `internal/cmd/prime.go` — `checkSlungWork()`（约第 421 行）：检测被 sling 的工作，
  `outputAutonomousDirective()`（约第 542 行）："AUTONOMOUS WORK MODE" 输出
- `internal/cmd/prime_molecule.go` — `outputMoleculeContext()`（约第 182 行）：
  molecule 进度和步骤显示

**Flow**: GT→Agent。10 段输出：beacon、handoff 警告、角色上下文、
CONTEXT.md、handoff 内容、附件状态、自主指令、molecule 上下文、检查点、启动指令。

**Fragility**:
- 没有 hook 的非 Claude agent 完全失去自动 priming
- Compact/resume 路径必须更轻量，以防止重新初始化循环
- 会话状态检测依赖 handoff 标记文件
- 角色模板渲染使用 Go text/template — 错误静默

**API mapping**: `POST /context` 带有 sections 数组和模式（full/compact/resume）

---

## 10. Hooks（settings.json 安装）

**What**: GT 将 hook 配置安装到 agent 运行时的设置文件中。

**Code**:
- `internal/hooks/config.go` — `HooksConfig`（约第 28 行）：8 种事件类型
  （PreToolUse、PostToolUse、SessionStart、Stop、PreCompact、UserPromptSubmit、
  WorktreeCreate、WorktreeRemove）
- `internal/hooks/config.go` — `DefaultBase()`（约第 711 行）：基础 hook，包括
  PR-workflow guard、dangerous-command guard、SessionStart → `gt prime --hook`、
  UserPromptSubmit → `gt mail check --inject`、Stop → `gt costs record`
- `internal/hooks/config.go` — `DefaultOverrides()`（约第 199 行）：角色特定
  覆盖（crew PreCompact → handoff 循环，witness/deacon/refinery 巡逻 guard）
- `internal/hooks/merge.go` — `MergeHooks()`（约第 24 行）：按特异性顺序应用覆盖
- `internal/cmd/hooks_install.go` — `runHooksInstall()`（约第 48 行）：从注册表
  安装 hook 到 worktree，`installHookTo()`（约第 245 行）：加载、合并、写入
  settings.json
- `internal/hooks/config.go` — `DiscoverTargets()`（约第 382 行）：查找所有设置
  文件（mayor、deacon、crew、polecats、witness、refinery，按 rig）
- `internal/runtime/runtime.go` — 6 个提供方的 hook 安装器注册：
  claude、gemini、opencode、copilot、omp、pi

**Flow**: GT→Agent（安装时）。Agent 读取 settings.json；GT 写入的。

**Fragility**:
- 每个 agent 供应商有不同的 hook 格式（settings.json、plugins、extensions）
- 6 种不同的 hook 提供方，各有不同的文件位置
- 无 hook 框架的 agent 完全无法获得 hook
- Hook 合并逻辑（base → role → rig+role）很复杂

**API mapping**: `POST /authorize`（替代 PreToolUse guard）、
`POST /context`（替代 SessionStart/PreCompact priming）、
`POST /telemetry`（替代 Stop 成本记录）

---

## 11. Guard 脚本（命令拦截）

**What**: GT 通过 PreToolUse hook 拦截危险或违反策略的命令。

**Code**:
- `internal/cmd/tap_guard.go` — `runTapGuardPRWorkflow()`（约第 34 行）：在
  Gas Town agent 上下文中拦截 `gh pr create`、`git checkout -b`、`git switch -c`；
  `isGasTownAgentContext()`（约第 103 行）检查 GT_* 环境变量和 CWD 路径
- `internal/cmd/tap_guard_dangerous.go` — `runTapGuardDangerous()`（约第 66 行）：
  拦截 5 种模式：`rm -rf /`、`git push --force`、`git push -f`、
  `git reset --hard`、`git clean -f`；`extractCommand()`（约第 104 行）解析
  Claude Code 的 JSON hook 输入
- 退出码约定：2 = 拦截

**Flow**: Agent→GT→Agent。Agent 调用 hook → GT 评估 → 退出 0（允许）或 2（拦截）。

**Fragility**:
- Guard 从 stdin 读取 Claude Code JSON 格式的 hook 输入 — 格式变更会破坏
- 模式匹配基于子串 — 可能遗漏变体
- Guard 在 stdin 错误时 fail-open（无法解析 = 允许）
- 仅 3 个 guard 脚本；覆盖不完整

**API mapping**: `POST /authorize` — GT 带完整上下文评估工具调用，
返回 allow/deny 及原因

---

## 12. 对话日志访问（JSONL 抓取）

**What**: GT 读取 Claude Code 的对话记录以获取成本和会话数据。

**Code**:
- `internal/cmd/costs.go` — `getClaudeProjectDir()`（约第 704 行）：将工作目录
  映射到 `~/.claude/projects/{slug}/`；`findLatestTranscript()`（约第 717 行）：
  查找最近的 `.jsonl`；`parseTranscriptUsage()`（约第 751 行）：逐行 JSONL 扫描
  累加 token 用量
- `internal/cmd/seance.go` — 从 `.events.jsonl` 发现会话（约第 61 行），
  回退扫描 `~/.claude/projects/`（约第 513 行），`sessions-index.json`（约第 674 行）
- 数据结构：`TranscriptMessage`，含 `Type`、`SessionId`、`Message.Model`、
  `Message.Usage.{InputTokens, CacheCreationInputTokens, CacheReadInputTokens,
  OutputTokens}`

**Flow**: Agent→GT（推断的）。Claude Code 写入 JSONL；GT 抓取文件系统。

**Fragility**:
- 路径编码约定（斜杠→短横线）是未文档化的 Claude Code 内部实现
- JSONL 消息格式、用量字段嵌套可能在无通知的情况下变更
- 三个独立的 JSONL 解析器（agentlog、costs.go、seance）— 无共享代码
- `sessions-index.json` 格式是 Claude Code 内部实现
- 非 Claude agent 不产生 JSONL 记录

**API mapping**: `POST /telemetry` — agent 推送结构化用量事件

---

## 13. Token 用量与成本追踪

**What**: GT 从记录的 token 数和硬编码定价计算会话成本。

**Code**:
- `internal/cmd/costs.go` — 共 1516 行：
  `calculateCost()`（约第 801 行）：使用 `modelPricing` 映射将 token→USD，
  `extractCostFromWorkDir()`（约第 823 行）：从 Claude 记录提取，
  `runCostsRecord()`（约第 956 行）：Stop hook 追加到 `~/.gt/costs.jsonl`，
  `runCostsDigest()`（约第 1155 行）：从 costs.jsonl 生成每日摘要 bead
- `internal/cmd/costs.go` — `modelPricing`（约第 222 行）：硬编码定价表
  （Opus：$15/$75，Sonnet：$3/$15，Haiku：$1/$5 每百万 token，
  缓存读取 90% 折扣，缓存创建 25% 加价）
- `internal/config/cost_tier.go` — `CostTierRoleAgents()`（约第 44 行）：
  按成本层级（standard/economy/budget）将角色映射到模型

**Flow**: Agent→GT（推断的）。GT 在会话结束时读取记录。

**Fragility**:
- 定价表是硬编码的 — Anthropic 变更定价时必须更新
- 成本在会话结束时通过 Stop hook 计算，非实时
- 无按 bead 的成本归因
- 模型 ID 匹配脆弱（对模型名称进行子串匹配）
- 非 Claude agent 没有成本追踪

**API mapping**: `POST /telemetry` 带有 `usage.cost_usd` — 运行时在源头报告成本

---

## 14. 进程存活检测

**What**: GT 检查 agent 进程是否真正在 tmux 会话中运行。

**Code**:
- `internal/tmux/tmux.go` — `IsAgentAlive()`（约第 2157 行）：首选方法，
  委托给 `IsRuntimeRunning()`（约第 2091 行），使用会话进程名称
- `internal/tmux/tmux.go` — `resolveSessionProcessNames()`（约第 2164 行）：
  优先级 GT_PROCESS_NAMES env → GT_AGENT env → 配置回退
- `internal/tmux/tmux.go` — `GetPaneCommand()`（约第 1579 行）：通过
  tmux 格式获取 `#{pane_current_command}`
- `internal/tmux/tmux.go` — `hasDescendantWithNames()`（约第 1823 行）：
  递归 `pgrep -P <pid> -l` 树遍历（maxDepth=10）
- `internal/tmux/tmux.go` — `processMatchesNames()`（约第 1800 行）：
  `ps -p <pid> -o comm=`
- `internal/tmux/tmux.go` — `getAllDescendants()`（约第 681 行）：
  最深优先的进程树，用于安全清理
- `internal/tmux/process_group_unix.go` — `getProcessGroupMembers()`（约第 38 行）、
  `getParentPID()`（约第 20 行）、`getProcessGroupID()`（约第 30 行）
- `internal/config/agents.go` — 每个预设的 `ProcessNames`：Claude=`["node","claude"]`、
  Gemini=`["gemini"]` 等

**Flow**: Agent→GT（推断的）。GT 遍历进程树；agent 不知情。

**Fragility**:
- 进程名检测依赖精确的二进制名称
- Shell 封装器（如 c2claude）需要后代树遍历
- `pgrep` 和 `ps` 输出解析依赖平台
- 进程可能在检查和操作之间退出（TOCTOU）

**API mapping**: `GET /health` — agent 自行报告存活状态

---

## 15. 三级健康检查

**What**: GT 对 agent 会话执行 3 级健康评估。

**Code**:
- `internal/tmux/tmux.go` — `CheckSessionHealth()`（约第 1771 行）：
  Level 1：`HasSession()`（tmux 会话是否存在？）、
  Level 2：`IsAgentAlive()`（agent 进程是否运行？）、
  Level 3：`GetSessionActivity()`（在 maxInactivity 内有活动？）
- `internal/tmux/tmux.go` — `ZombieStatus`（约第 1723 行）：枚举，含
  `SessionHealthy`、`SessionDead`、`AgentDead`、`AgentHung`；
  `IsZombie()` 对 AgentDead 或 AgentHung 返回 true

**Flow**: GT→GT（内部健康评估）。

**Fragility**:
- HungSessionThreshold = 30 分钟（硬编码默认值）
- 活动时间戳来自 tmux `#{session_activity}` — 衡量任何终端活动，
  而非有意义的 agent 工作
- 休眠中无输出的 agent 即使健康也会看起来像挂起

**API mapping**: `GET /health` — agent 报告状态、context_usage、last_activity

---

## 16. 心跳文件

**What**: GT 在 tmux 之外使用心跳文件进行存活检测。

**Code**:
- `internal/polecat/heartbeat.go` — `TouchSessionHeartbeat()`（约第 34 行）：
  将 JSON 写入 `.runtime/heartbeats/<session>.json`，`IsSessionHeartbeatStale()`
  （约第 74 行）：3 分钟阈值，`ReadSessionHeartbeat()`（约第 54 行）、
  `RemoveSessionHeartbeat()`
- `internal/deacon/heartbeat.go` — `WriteHeartbeat()`（约第 52 行）：
  deacon 心跳在 `deacon/heartbeat.json`，含循环计数、健康统计；
  `IsFresh()`（<5 分钟）、`IsStale()`（5-15 分钟）、`IsVeryStale()`（>15 分钟）

**Flow**: Agent→GT（隐式的）。Agent 命令写入文件；GT 读取。

**Fragility**:
- 基于文件 — 写入无通知，必须轮询
- 过期阈值（3 分钟）是经验值
- 心跳触发依赖 GT 命令被调用（非 agent 主动发起）

**API mapping**: `GET /health` — agent 直接报告存活；无需文件

---

## 17. 工作目录检测

**What**: GT 通过多种方法确定 agent 的工作目录。

**Code**:
- `internal/tmux/tmux.go` — `GetPaneWorkDir()`（约第 1676 行）：
  通过 tmux 获取 `#{pane_current_path}`
- `internal/workspace/find.go` — `Find()`（约第 29 行）：从 CWD 向上查找
  `mayor/town.json` 标记；处理 worktree 路径（polecats/、crew/）；
  `FindFromCwdWithFallback()`（约第 113 行）：已删除 worktree 的
  GT_TOWN_ROOT env 回退
- `internal/config/env.go` — GT_ROOT 环境变量在 `AgentEnv()` 中设置

**Flow**: GT→GT（检测）和 GT→Agent（环境变量）。

**Fragility**:
- 5 种检测方法可能不一致（tmux CWD、环境变量、路径解析、git worktree）
- Worktree 删除使 agent 没有有效 CWD
- GT_TOWN_ROOT 回退之所以存在，正是因为 worktree 清理会破坏 CWD

**API mapping**: 属于 `POST /identity` 的一部分 — GT 分配工作目录

---

## 18. 权限绕过（YOLO 标志）

**What**: GT 使用供应商特定的权限绕过标志启动所有 agent。

**Code**:
- `internal/config/agents.go` — 每个预设的 Args：
  - Claude：`--dangerously-skip-permissions`
  - Gemini：`--approval-mode yolo`
  - Codex：`--dangerously-bypass-approvals-and-sandbox`
  - Cursor：`-f`
  - Auggie：`--allow-indexing`
  - AMP：`--dangerously-allow-all --no-ide`
  - OpenCode：env `OPENCODE_PERMISSION={"*":"allow"}`
  - Copilot：`--yolo`
- `internal/tmux/tmux.go` — `AcceptBypassPermissionsWarning()`（约第 1509 行）：
  轮询 "Bypass Permissions mode" 对话框，发送 Down+Enter；
  `DismissStartupDialogsBlind()`（约第 1558 行）：盲按键序列回退

**Flow**: GT→Agent（启动时）。始终开启，无按角色粒度。

**Fragility**:
- 10 个 agent 共 10 种不同的标志名 — 每个都是不同的字符串
- 全有或全无：无按角色的权限粒度
- Claude 的权限警告对话框检测依赖精确文本
- 无退出选项 — 每个 agent 都以完全绕过权限运行

**API mapping**: `POST /authorize` — 按调用的授权，带基于角色的规则

---

## 19. 非交互模式

**What**: GT 以非交互模式运行 agent 执行特定任务。

**Code**:
- `internal/config/agents.go` — `NonInteractiveConfig`（约第 92 行）：
  `ExecSubcommand`（如 "exec"）、`PromptFlag`（如 "-p"）、
  `OutputFormatFlag`（如 "--output-format json"）
- `internal/config/agents.go` — `PromptMode`（约第 98 行）："arg" 或 "none"

**Flow**: GT→Agent。GT 构造带标志的 CLI 调用。

**Fragility**:
- Exec 子命令和标志名因 agent 而异
- 输出格式解析依赖 agent 的输出结构
- 并非所有 agent 支持非交互执行

**API mapping**: `POST /prompt` 带结构化 I/O 替代 CLI 标志组合

---

## 20. 会话恢复/分支

**What**: GT 恢复先前的会话或分支会话以进行对话回顾。

**Code**:
- `internal/config/agents.go` — 每个预设的 `ResumeFlag`、`ContinueFlag`、`ResumeStyle`
  （"flag" vs "subcommand"）；`BuildResumeCommand()`（约第 534 行）
- `internal/cmd/seance.go` — `runSeance()`（约第 85 行）：为前任回顾
  启动 `claude --fork-session --resume <id>`
- `internal/session/startup.go` — `FormatStartupBeacon()`（约第 69 行）：
  `[GAS TOWN] recipient <- sender • timestamp • topic` 格式

**Flow**: GT→Agent。GT 构造带会话 ID 的恢复命令。

**Fragility**:
- 恢复语义因 agent 而异（标志 vs 子命令）
- `--fork-session` 是 Claude Code 专有的
- 会话 ID 存储在环境变量和文件中 — 多个真相来源
- Beacon 格式由 LLM 解析 — 格式变更影响理解

**API mapping**: `POST /context` 带有 `mode: "resume"` 和会话历史

---

## 21. 配置目录隔离

**What**: GT 为每个账户隔离 agent 配置，以支持凭证轮换。

**Code**:
- `internal/config/agents.go` — 每个预设的 `ConfigDirEnv`（如 "CLAUDE_CONFIG_DIR"）、
  `ConfigDir`（如 ".claude"）
- `internal/config/env.go` — `CLAUDE_CONFIG_DIR` 在 `AgentEnv()` 中设置（约第 148 行）
- `internal/quota/keychain.go` — `KeychainServiceName()`（约第 35 行）：
  配置目录的 SHA-256 哈希，用于按账户的 keychain 隔离
- 账户目录模式：`~/.claude-accounts/<handle>/`

**Flow**: GT→Agent。GT 设置配置目录环境变量；agent 用于所有设置。

**Fragility**:
- 配置目录布局是 Claude Code 内部实现
- 账户之间的符号链接切换很脆弱
- keychain 服务名的 SHA-256 键值依赖 Claude Code 约定

**API mapping**: `POST /identity` 带有 `credentials` — 运行时自行管理其配置

---

## 22. 主题/显示（tmux 状态栏）

**What**: GT 应用角色特定的 tmux 状态栏主题。

**Code**:
- `internal/cmd/theme.go` — `runTheme()`：应用角色/rig 特定的 tmux 状态行格式
- 在 `internal/session/lifecycle.go` 的 `StartSession()` 步骤中应用

**Flow**: GT→tmux。仅显示，不影响 agent 行为。

**Fragility**:
- 纯粹装饰性 — 但主题字符串用于空闲检测（⏵⏵）
- 主题依赖 tmux 作为终端复用器

**API mapping**: 不属于 agent API — 显示关注点留在 GT 中

---

## 23. Agent 输出捕获（tmux capture-pane）

**What**: GT 为多种目的读取 agent 终端输出。

**Code**:
- `internal/tmux/tmux.go` — `CapturePaneTrimmed()`、`CapturePaneLines()`：
  从 agent 终端捕获 N 行
- 使用方：空闲检测（5 行）、速率限制扫描（30 行）、对话框检测、
  就绪轮询、nudge 验证
- `internal/telemetry/recorder.go` — `RecordPaneRead()`（约第 266 行）：
  每次调用 capture-pane 的 OTel 事件

**Flow**: Agent→GT（推断的）。GT 读取终端；agent 不知情。

**Fragility**:
- 终端内容是非结构化文本 — 解析始终是正则/启发式
- capture-pane 仅获取可见终端缓冲区 — 回滚有限
- 多窗格会话需要先调用 FindAgentPane()

**API mapping**: 已消除 — `POST /lifecycle` 和 `POST /telemetry` 提供
结构化数据；无需抓取终端

---

## 24. 完成/退出信号

**What**: Agent 通过 GT 命令和意图文件发出工作完成信号。

**Code**:
- `internal/cmd/done.go` — `runDone()`（约第 81 行）：持久的 polecat 模型，
  转为 IDLE 并保留沙箱；退出常量：`ExitCompleted`、`ExitEscalated`、
  `ExitDeferred`（约第 65 行）
- `internal/cmd/signal_stop.go` — `runSignalStop()`（约第 47 行）：Stop hook
  处理器，检查未读邮件和被 sling 的工作，返回 JSON
  `{"decision":"block"|"approve","reason":"..."}`
- `internal/witness/handlers.go` — `HandlePolecatDone()`（约第 110 行）：
  处理 POLECAT_DONE 消息

**Flow**: Agent→GT。Agent 调用 `gt done`；GT 处理退出类型。

**Fragility**:
- 完成检测依赖 agent 调用 `gt done`（一个 GT CLI 命令）
- Stop hook 必须解析 Claude Code 期望的 JSON 格式
- 4 种退出类型但无结构化错误报告
- 停止状态追踪（在 /tmp 中）用于防止无限阻塞循环

**API mapping**: `POST /lifecycle` 带有 `event: "stopping"` + 退出元数据

---

## 25. 环境变量注入

**What**: GT 通过 tmux 向 agent 会话注入 30+ 个环境变量。

**Code**:
- `internal/config/env.go` — `AgentEnv()`（约第 65 行）：生成完整环境映射
  （GT_*、BD_*、GIT_*、CLAUDE_*、OTEL_*、凭证透传）
- 三种传播机制：
  1. `tmux.SetEnvironment()` — 通过 `set-environment` 的会话级
  2. `config.PrependEnv()` — 命令前的内联 `export K=V &&`
  3. `config.EnvForExecCommand()` — 子进程的 `cmd.Env` 追加
- 安全防护：`NODE_OPTIONS=""`（清除 VSCode 调试器）、`CLAUDECODE=""`
  （防止嵌套会话检测）
- 凭证透传：40+ 云 API 变量（Anthropic、AWS、Google、proxy、mTLS）

**Flow**: GT→Agent。GT 设置环境变量；agent 继承。

**Fragility**:
- 三种传播机制可能分歧
- 环境变量对会话中的任何进程可见（安全隐患）
- 凭证透传列表必须手动维护
- tmux SetEnvironment 仅影响新的 shell 调用，不影响运行中的进程

**API mapping**: `POST /identity` 带有 `env` 映射 — 单次结构化投递

---

## 26. 遥测（OTel 集成）

**What**: GT 为所有 agent 操作发出 OpenTelemetry 指标和日志。

**Code**:
- `internal/telemetry/telemetry.go` — `Init()`（约第 104 行）：OTel provider 设置，
  VictoriaMetrics/VictoriaLogs 端点，30s 导出间隔
- `internal/telemetry/recorder.go` — 18 种事件类型：
  `RecordSessionStart()`、`RecordSessionStop()`、`RecordPromptSend()`、
  `RecordPaneRead()`、`RecordPrime()`、`RecordAgentStateChange()`、
  `RecordPolecatSpawn()`、`RecordPolecatRemove()`、`RecordSling()`、
  `RecordMail()`、`RecordNudge()`、`RecordDone()`、`RecordDaemonRestart()`、
  `RecordFormulaInstantiate()`、`RecordConvoyCreate()`、`RecordPaneOutput()`、
  `RecordBDCall()`、`RecordPrimeContext()`
- 17 个 OTel Int64Counter 指标（gastown.session.starts.total 等）
- `internal/telemetry/subprocess.go` — `SetProcessOTELAttrs()`：
  将 OTEL_RESOURCE_ATTRIBUTES 传播到子进程

**Flow**: GT→指标后端。Agent 操作由 GT 追踪，非 agent。

**Fragility**:
- OTel 导出依赖 VictoriaMetrics/Logs 的可用性
- 没有关联 ID 贯穿所有事件（PR #2068 提议 run.id）
- Agent 对记录什么和如何记录没有发言权

**API mapping**: `POST /telemetry` — agent 用 run_id 推送自己的事件

---

## 27. 事件日志（.events.jsonl）

**What**: GT 将所有重要事件记录到 JSONL 文件中。

**Code**:
- `internal/events/events.go` — `Log()`（约第 85 行）、`LogFeed()`（约第 98 行）、
  `LogAudit()`（约第 103 行）：带 flock 追加到 `.events.jsonl`
- 事件类型（约第 36-77 行）：sling、handoff、done、hook、unhook、spawn、kill、
  boot、halt、session_start、session_end、session_death、mass_death、
  patrol_*、merge_*、scheduler_*
- `internal/tui/feed/events.go` — `GtEventsSource`（约第 216 行）：
  tail .events.jsonl 供 TUI feed 显示

**Flow**: GT→文件。事件来自 GT 操作，非 agent 报告。

**Fragility**:
- 所有事件写入单个 JSONL 文件 — 无轮转或大小管理
- flock 序列化在高并发下可能争用
- 没有关联 ID 将事件链接到特定 agent 运行

**API mapping**: `POST /telemetry` 事件取代 GT 端日志用于 agent 报告的数据

---

## 28. 僵尸检测与恢复

**What**: GT 检测并恢复僵尸会话（tmux 存活，agent 已死）。

**Code**:
- `internal/doctor/zombie_check.go` — `ZombieSessionCheck.Run()`（约第 33 行）：
  过滤已知 GT 会话，排除 crew，调用 `IsAgentAlive()`；
  `ZombieSessionCheck.Fix()`（约第 113 行）：杀前重新验证（TOCTOU 防护），
  永远不杀 crew 会话
- `internal/daemon/wisp_reaper.go` — 过期 wisp 清理的 wisp reaper
- `internal/witness/handlers.go` — witness 巡逻，采取重启优先策略（非销毁优先）
- `internal/dog/health.go` — `HealthChecker.Check()`（约第 46 行）：
  使用 CheckSessionHealth() 的 dog 特定健康检查
- `internal/witness/spawn_count.go` — spawn 风暴断路器：
  `ShouldBlockRespawn()`（约第 74 行），超过阈值后升级到 mayor

**Flow**: GT→GT。内部监控，agent 是被动主体。

**Fragility**:
- 僵尸检测依赖进程树遍历（依赖平台）
- 宽限期和阈值是经验值（僵尸杀死宽限、挂起阈值）
- 检测与操作之间的 TOCTOU 间隙
- 断路器状态在 JSON 文件中

**API mapping**: `GET /health` — agent 直接报告状态；僵尸检测
变得微不足道（无响应 = 已死）

---

## 跨切面主题

### 关联性缺口
没有单一 ID 连接：OTel 事件 ↔ 对话记录 ↔ 成本条目 ↔
会话事件 ↔ bead。Factory Worker API 中的 `run_id` 解决了这个问题。

### Claude Code 耦合
28 个触点中有 17 个依赖 Claude Code 内部实现：
- 提示符前缀（`❯`）、状态栏（`⏵⏵`）、JSONL 格式、配置目录布局、
  keychain 服务命名、会话索引、hook JSON 格式、权限对话框文本、
  绕过标志名、恢复标志语义、`sessions-index.json`、记录
  消息结构、用量字段嵌套。

### Agent 对等性缺口
非 Claude agent 缺失：
- Hooks（无自动 priming、guard 或邮件注入）
- 对话日志访问（无 JSONL 记录）
- 成本追踪（无可解析的记录）
- 恢复/分支（不同或无机制）
- 权限对话框处理（不同的 UI）

Factory Worker API 消除了这一点 — 一个 API，所有 agent。

### 推送 vs 抓取
当前：GT 抓取 6+ 个来源（tmux 窗格、JSONL 文件、进程树、心跳
文件、keychain、配置目录）。
拟议：Agent 推送生命周期事件、遥测和健康状态 — GT 不再抓取。

---

## 摘要：触点 → API 端点

| # | 触点 | 当前机制 | API 端点 |
|---|------|----------|----------|
| 1 | 提示词投递 | tmux send-keys 8 步 | `POST /prompt` |
| 2 | 投递模式 | immediate/queue/wait-idle | `POST /prompt` priority |
| 3 | 空闲检测 | 提示符前缀 + 状态栏 | `POST /lifecycle` |
| 4 | 速率限制检测 | 窗格内容正则 | `POST /lifecycle` |
| 5 | 账户轮换 | macOS keychain 交换 | `POST /identity` |
| 6 | 会话生命周期 | 13 步 tmux 创建 | `POST /lifecycle` |
| 7 | 启动准入 | 容量门控 | 内部（非面向 agent） |
| 8 | Agent 身份 | 30+ 环境变量 | `POST /identity` |
| 9 | 上下文注入 | 10 段文本输出 | `POST /context` |
| 10 | Hooks | settings.json 安装 | 多个端点 |
| 11 | Guard 脚本 | PreToolUse 退出码 2 | `POST /authorize` |
| 12 | JSONL 抓取 | 文件系统记录读取 | `POST /telemetry` |
| 13 | 成本追踪 | 硬编码定价表 | `POST /telemetry` |
| 14 | 进程存活 | pgrep 树遍历 | `GET /health` |
| 15 | 健康检查 | 3 级 tmux 检查 | `GET /health` |
| 16 | 心跳文件 | JSON 文件写入/轮询 | `GET /health` |
| 17 | 工作目录检测 | 5 种方法（tmux、env、path） | `POST /identity` |
| 18 | 权限绕过 | 10 种供应商特定标志 | `POST /authorize` |
| 19 | 非交互模式 | CLI 标志组合 | `POST /prompt` |
| 20 | 会话恢复/分支 | --resume/--fork 标志 | `POST /context` |
| 21 | 配置目录隔离 | CLAUDE_CONFIG_DIR env | `POST /identity` |
| 22 | 主题/显示 | tmux 状态栏 | 非面向 agent |
| 23 | 输出捕获 | tmux capture-pane | 已消除 |
| 24 | 完成/退出信号 | gt done CLI 调用 | `POST /lifecycle` |
| 25 | 环境变量注入 | 3 种传播机制 | `POST /identity` |
| 26 | OTel 遥测 | GT 端记录 | `POST /telemetry` |
| 27 | 事件日志 | .events.jsonl 追加 | `POST /telemetry` |
| 28 | 僵尸检测 | 进程树 + 阈值 | `GET /health` |

**28 个触点 → 7 个 API 端点。** 每个 hack 都被结构化通信所取代。