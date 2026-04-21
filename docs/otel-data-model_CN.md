# Gastown OTel 数据模型

所有 Gastown 遥测事件均为 OTel 日志记录，通过 OTLP（`GT_OTEL_LOGS_URL`）导出。每条记录携带一个 `run.id` 属性 — 一个在每次代理生成时创建的 UUID — 因此来自单个代理会话的所有记录都可以被检索和关联。

---

## 1. 身份层级

### 1.1 Instance

最外层分组。在代理生成时从机器主机名和 Town 根目录的基本名称派生。

| 属性 | 类型 | 描述 |
|------|------|------|
| `instance` | string | `hostname:basename(town_root)` — 如 `"laptop:gt"` |
| `town_root` | string | Town 根目录的绝对路径 — 如 `"/Users/pa/gt"` |

### 1.2 Run

每次代理生成产生一个 `run.id` UUID。该会话的所有 OTel 记录携带相同的 `run.id`。

| 属性 | 类型 | 来源 |
|------|------|------|
| `run.id` | string (UUID v4) | 生成时创建；通过 `GT_RUN` 传播 |
| `instance` | string | `hostname:basename(town_root)` |
| `town_root` | string | Town 根目录绝对路径 |
| `agent_type` | string | `"claudecode"`、`"opencode"`、`"copilot"` 等 |
| `role` | string | `polecat` · `witness` · `mayor` · `refinery` · `crew` · `deacon` · `dog` · `boot` |
| `agent_name` | string | 角色内的具体名称（如 `"wyvern-Toast"`）；单例角色等于角色名 |
| `session_id` | string | tmux 面板名称 |
| `rig` | string | Rig 名称；Town 级代理为空 |

---

## 2. 事件

### `agent.instantiate`

每次代理生成时发出一次。锚定该 Run 的所有后续事件。

| 属性 | 类型 | 描述 |
|------|------|------|
| `run.id` | string | Run UUID |
| `instance` | string | `hostname:basename(town_root)` |
| `town_root` | string | Town 根目录绝对路径 |
| `agent_type` | string | `"claudecode"` · `"opencode"` · `"copilot"` · … |
| `role` | string | Gastown 角色 |
| `agent_name` | string | 代理名称 |
| `session_id` | string | tmux 面板名称 |
| `rig` | string | Rig 名称（空 = Town 级） |
| `issue_id` | string | 分配给此代理的工作项 Bead ID |
| `git_branch` | string | 生成时工作目录的 git 分支 |
| `git_commit` | string | 生成时工作目录的 HEAD SHA |

---

### `session.start` / `session.stop`

tmux 会话生命周期事件。

| 属性 | 类型 | 描述 |
|------|------|------|
| `run.id` | string | Run UUID |
| `session_id` | string | tmux 面板名称 |
| `role` | string | Gastown 角色 |
| `status` | string | `"ok"` · `"error"` |

---

### `prime`

每次 `gt prime` 调用时发出。渲染的公式作为 `prime.context` 单独发出（相同属性加上 `formula`）。

| 属性 | 类型 | 描述 |
|------|------|------|
| `run.id` | string | Run UUID |
| `role` | string | Gastown 角色 |
| `hook_mode` | bool | 当从 Hook 调用时为 true |
| `formula` | string | 完整渲染的公式（仅 `prime.context`） |
| `status` | string | `"ok"` · `"error"` |

---

### `prompt.send`

每次 `gt sendkeys` 向代理的 tmux 面板派发。

| 属性 | 类型 | 描述 |
|------|------|------|
| `run.id` | string | Run UUID |
| `session` | string | tmux 面板名称 |
| `keys` | string | 提示文本（需选择启用：`GT_LOG_PROMPT_KEYS=true`；截断至 256 字节） |
| `keys_len` | int | 提示长度（字节） |
| `debounce_ms` | int | 应用的防抖延迟 |
| `status` | string | `"ok"` · `"error"` |

---

### `agent.event`

代理会话日志中每个内容块的记录。仅在 `GT_LOG_AGENT_OUTPUT=true` 时发出。

| 属性 | 类型 | 描述 |
|------|------|------|
| `run.id` | string | Run UUID |
| `session` | string | tmux 面板名称 |
| `native_session_id` | string | 代理原生会话 UUID（Claude Code：JSONL 文件名 UUID） |
| `agent_type` | string | 适配器名称 |
| `event_type` | string | `"text"` · `"tool_use"` · `"tool_result"` · `"thinking"` |
| `role` | string | `"assistant"` · `"user"` |
| `content` | string | 内容截断至 512 字节（设置 `GT_LOG_AGENT_CONTENT_LIMIT=0` 禁用） |

对于 `tool_use`：`content = "<tool_name>: <truncated_json_input>"`
对于 `tool_result`：`content = <truncated tool output>`

---

### `agent.usage`

每个助手轮次一条记录（而非每个内容块，以避免重复计数）。仅在 `GT_LOG_AGENT_OUTPUT=true` 时发出。

| 属性 | 类型 | 描述 |
|------|------|------|
| `run.id` | string | Run UUID |
| `session` | string | tmux 面板名称 |
| `native_session_id` | string | 代理原生会话 UUID |
| `input_tokens` | int | 来自 API 使用字段的 `input_tokens` |
| `output_tokens` | int | 来自 API 使用字段的 `output_tokens` |
| `cache_read_tokens` | int | `cache_read_input_tokens` |
| `cache_creation_tokens` | int | `cache_creation_input_tokens` |

---

### `bd.call`

每次 `bd` CLI 调用，无论是 Go 守护进程还是代理在 Shell 中执行。

| 属性 | 类型 | 描述 |
|------|------|------|
| `run.id` | string | Run UUID |
| `subcommand` | string | bd 子命令（`"ready"`、`"update"`、`"create"` 等） |
| `args` | string | 完整参数列表 |
| `duration_ms` | float | 挂钟耗时（毫秒） |
| `stdout` | string | 完整 stdout（需选择启用：`GT_LOG_BD_OUTPUT=true`） |
| `stderr` | string | 完整 stderr（需选择启用：`GT_LOG_BD_OUTPUT=true`） |
| `status` | string | `"ok"` · `"error"` |

---

### `mail`

Gastown 邮件系统的所有操作。

| 属性 | 类型 | 描述 |
|------|------|------|
| `run.id` | string | Run UUID |
| `operation` | string | `"send"` · `"read"` · `"archive"` · `"list"` · `"delete"` 等 |
| `msg.id` | string | 消息标识符 |
| `msg.from` | string | 发件人地址 |
| `msg.to` | string | 收件人，逗号分隔 |
| `msg.subject` | string | 主题 |
| `msg.body` | string | 消息正文（需选择启用：`GT_LOG_MAIL_BODY=true`；截断至 256 字节） |
| `msg.thread_id` | string | 线程 ID |
| `msg.priority` | string | `"high"` · `"normal"` · `"low"` |
| `msg.type` | string | 消息类型（`"work"`、`"notify"`、`"queue"` 等） |
| `status` | string | `"ok"` · `"error"` |

对于有消息可用的操作（发送、读取），使用 `RecordMailMessage(ctx, operation, MailMessageInfo{…}, err)`。对于无内容的操作（列表、按 ID 归档），使用 `RecordMail(ctx, operation, err)`。

---

### `agent.state_change`

代理转换到新状态时发出（idle → working 等）。

| 属性 | 类型 | 描述 |
|------|------|------|
| `run.id` | string | Run UUID |
| `agent_id` | string | 代理标识符 |
| `new_state` | string | 新状态（`"idle"`、`"working"`、`"done"` 等） |
| `hook_bead` | string | 代理当前处理的 Bead ID；无则为空 |
| `status` | string | `"ok"` · `"error"` |

---

### `mol.cook` / `mol.wisp` / `mol.squash` / `mol.burn`

公式工作流各阶段发出的 Molecule 生命周期事件。

**`mol.cook`** — 公式编译为 Proto（创建 Wisp 的前提）：

| 属性 | 类型 | 描述 |
|------|------|------|
| `run.id` | string | Run UUID |
| `formula_name` | string | 公式名称（如 `"mol-polecat-work"`） |
| `status` | string | `"ok"` · `"error"` |

**`mol.wisp`** — Proto 实例化为活跃的 Wisp（临时 Molecule 实例）：

| 属性 | 类型 | 描述 |
|------|------|------|
| `run.id` | string | Run UUID |
| `formula_name` | string | 公式名称 |
| `wisp_root_id` | string | 创建的 Wisp 的根 Bead ID |
| `bead_id` | string | 绑定到 Wisp 的基础 Bead；独立公式 Sling 时为空 |
| `status` | string | `"ok"` · `"error"` |

**`mol.squash`** — Molecule 执行完成并折叠为摘要：

| 属性 | 类型 | 描述 |
|------|------|------|
| `run.id` | string | Run UUID |
| `mol_id` | string | Molecule 根 Bead ID |
| `done_steps` | int | 完成的步骤数 |
| `total_steps` | int | Molecule 中的总步骤数 |
| `digest_created` | bool | 设置了 `--no-digest` 标志时为 false |
| `status` | string | `"ok"` · `"error"` |

**`mol.burn`** — Molecule 被销毁且未创建摘要：

| 属性 | 类型 | 描述 |
|------|------|------|
| `run.id` | string | Run UUID |
| `mol_id` | string | Molecule 根 Bead ID |
| `children_closed` | int | 关闭的后代步骤 Bead 数量 |
| `status` | string | `"ok"` · `"error"` |

---

### `bead.create`

Molecule 实例化期间（`bd mol pour` / `InstantiateMolecule`）每个子 Bead 创建时发出。允许跟踪给定 Molecule 的完整父 → 子 Bead 图。

| 属性 | 类型 | 描述 |
|------|------|------|
| `run.id` | string | Run UUID |
| `bead_id` | string | 新创建的子 Bead ID |
| `parent_id` | string | 父（Wisp 根/基础）Bead ID |
| `mol_source` | string | 驱动实例化的 Molecule Proto Bead ID |

---

### 其他事件

所有事件都携带 `run.id`。

| 事件体 | 关键属性 |
|--------|---------|
| `sling` | `bead`、`target`、`status` |
| `nudge` | `target`、`status` |
| `done` | `exit_type`（`COMPLETED` · `ESCALATED` · `DEFERRED`）、`status` |
| `polecat.spawn` | `name`、`status` |
| `polecat.remove` | `name`、`status` |
| `formula.instantiate` | `formula_name`、`bead_id`、`status`（顶级公式-绑定-Bead 结果） |
| `convoy.create` | `bead_id`、`status` |
| `daemon.restart` | `agent_type` |
| `pane.output` | `session`、`content`（需选择启用：`GT_LOG_PANE_OUTPUT=true`） |

---

## 3. 建议的索引属性

```
run.id, instance, town_root, session_id, rig, role, agent_type,
event_type, msg.thread_id, msg.from, msg.to
```

---

## 4. 环境变量

| 变量 | 设置者 | 描述 |
|------|--------|------|
| `GT_RUN` | tmux 会话环境 + 子进程 | Run UUID；所有事件的相关键 |
| `GT_OTEL_LOGS_URL` | 守护进程启动 | OTLP 日志端点 URL |
| `GT_OTEL_METRICS_URL` | 守护进程启动 | OTLP 指标端点 URL |
| `GT_LOG_AGENT_OUTPUT` | 运维人员 | 选择启用：流式传输 Claude JSONL 会话事件（内容默认截断至 512 字节） |
| `GT_LOG_AGENT_CONTENT_LIMIT` | 运维人员 | 覆盖 `agent.event` 中的内容截断；设 `0` 禁用（仅限专家） |
| `GT_LOG_BD_OUTPUT` | 运维人员 | 选择启用：在 `bd.call` 记录中包含 bd stdout/stderr |
| `GT_LOG_PANE_OUTPUT` | 运维人员 | 选择启用：流式传输原始 tmux 面板输出 |
| `GT_LOG_MAIL_BODY` | 运维人员 | 选择启用：在 `mail` 记录中包含邮件正文（截断至 256 字节） |
| `GT_LOG_PROMPT_KEYS` | 运维人员 | 选择启用：在 `prompt.send` 记录中包含提示文本（截断至 256 字节） |
| `GT_LOG_PRIME_CONTEXT` | 运维人员 | 选择启用：在 `prime.context` 记录中记录完整渲染的公式 |

`GT_RUN` 也作为 `bd` 子进程的 `OTEL_RESOURCE_ATTRIBUTES` 中的 `gt.run_id` 暴露，将其自身的遥测与父 Run 关联。