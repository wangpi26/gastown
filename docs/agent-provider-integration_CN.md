# 代理提供商集成指南

> 如何将你的代理 CLI 集成到 Gas Town（以及即将推出的 Gas City）。

本指南面向构建编码代理 CLI 的团队，希望其代理能参与 Gas Town 的多代理编排。它解释了现有的扩展点、四个集成深度层级以及前瞻性的 Gas City 提供商合约。

## Gas Town 是什么

Gas Town 是一个多代理工作空间管理器，通过 tmux 会话编排编码代理（Claude、Gemini、Codex、Cursor、AMP、OpenCode、Copilot 等）。它提供：

- **身份和角色管理** — 每个代理获得一个角色（Polecat、Crew、Witness、Refinery）及相应的上下文和权限
- **工作分配** — Bead（问题跟踪）、邮件和基于 Hook 的调度
- **会话生命周期** — 启动、恢复、交接和上下文循环
- **合并队列** — 自动测试和合并代理工作
- **代理间通信** — Nudge、邮件和共享状态

核心设计原则是**松耦合**：Gas Town 通过 tmux 和环境变量编排代理。它不导入代理库、不链接代理代码、也不要求代理导入 Gas Town 代码。集成是配置，而非编译。

## 集成层级

| 层级 | 工作量 | 你获得什么 | 你提供什么 |
|------|--------|-----------|-----------|
| **0：零集成** | 无 | 基本 tmux 编排 | 一个能在终端运行的 CLI |
| **1：预设** | JSON 配置文件 | 完整生命周期、恢复、进程检测 | `agents.json` 中的预设条目 |
| **2：Hook** | 设置文件或插件 | 上下文注入、工具守卫、邮件投递 | Hook 安装函数 |
| **3：深度** | 代码 + 脚本 | 非交互模式、会话 Fork、包装器 | 原生 API 集成 |

大多数代理团队应首先面向**层级 1**（15 分钟工作量），然后如果其 CLI 支持 Hook/插件系统，则考虑**层级 2**。

---

## 层级 0：零集成

**任何能在终端运行的 CLI 都可在 Gas Town 中零改动工作。**

Gas Town 在 tmux 会话中启动代理，并通过 `send-keys` 通信。如果你的代理有 REPL 或接受文本输入，Gas Town 可以：

- 在 tmux 面板中启动它
- 通过按键注入发送工作指令
- 通过 `pane_current_command` 检测活跃性
- 通过 `capture-pane` 读取输出

这是"tmux 垫片层" — 可用但对时序敏感且无投递确认。你可以免费获得基本编排。

**层级 0 缺失的功能：**
- 无会话恢复（每次都是全新会话）
- 无自动上下文注入（代理不知道自己的 Gas Town 角色）
- 基于延迟的就绪检测（Gas Town 猜测你何时就绪）
- 无进程名检测（Gas Town 无法区分你的代理和 `bash`）

---

## 层级 1：预设注册

**仅需 JSON 配置。无需修改 Gas Town 或你的代理代码。**

预设告诉 Gas Town 启动、检测、恢复和与你的代理通信所需的一切。你通过创建 JSON 文件来注册 — 无需 Go 代码、无需 PR、无需构建步骤。

### 配置放置位置

有三个级别，按顺序检查：

| 级别 | 路径 | 范围 |
|------|------|------|
| Town | `~/gt/settings/agents.json` | Town 内所有 Rig |
| Rig | `~/gt/<rig>/settings/agents.json` | 仅单个 Rig |
| 内置 | 编译进 `gt` 二进制文件 | 随 Gas Town 发布 |

对于外部代理团队，**Town 级别**是正确选择。用户将你的配置放入 `~/gt/settings/agents.json`，每个 Rig 都能使用。

### 注册表 Schema

该文件是一个 `AgentRegistry` JSON 对象：

```json
{
  "version": 1,
  "agents": {
    "kiro": {
      ...preset fields...
    }
  }
}
```

`version` 字段必须为 `1`（当前 Schema 版本）。`agents` 映射的键是 Gas Town 配置中使用的代理名称（例如，Rig 设置中的 `"agent": "kiro"`）。

### AgentPresetInfo 字段参考

来自 `internal/config/agents.go` 中 `AgentPresetInfo` 结构体的每个字段：

| 字段 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `name` | string | 是 | 预设标识符（如 `"kiro"`） |
| `command` | string | 是 | CLI 二进制名称或路径（如 `"kiro"`） |
| `args` | string[] | 是 | 自主模式默认参数（如 `["--yolo"]`） |
| `env` | map[string]string | 否 | 额外环境变量（与 GT_* 变量合并） |
| `process_names` | string[] | 否 | 用于 tmux 活跃检测的进程名 |
| `session_id_env` | string | 否 | 代理设置的用于会话 ID 跟踪的环境变量 |
| `resume_flag` | string | 否 | 用于恢复会话的标志或子命令 |
| `resume_style` | string | 否 | `"flag"`（如 `--resume <id>`）或 `"subcommand"`（如 `resume <id>`） |
| `supports_hooks` | bool | 否 | 代理是否有 Hook/插件系统 |
| `supports_fork_session` | bool | 否 | 是否支持 `--fork-session` |
| `non_interactive` | object | 否 | 无头执行设置（见下文） |
| `prompt_mode` | string | 否 | `"arg"`（提示作为 CLI 参数）或 `"none"`（无提示支持）。默认：`"arg"` |
| `config_dir_env` | string | 否 | 代理配置目录的环境变量 |
| `config_dir` | string | 否 | 顶级配置目录名（如 `".kiro"`） |
| `hooks_provider` | string | 否 | Hook 框架标识符（用于层级 2） |
| `hooks_dir` | string | 否 | Hook/设置文件的目录 |
| `hooks_settings_file` | string | 否 | 设置/插件文件名 |
| `hooks_informational` | bool | 否 | `true` 表示 Hook 仅为指令（不可执行） |
| `ready_prompt_prefix` | string | 否 | 就绪检测的提示字符串（如 `"❯ "`） |
| `ready_delay_ms` | int | 否 | 就绪检测的回退延迟（毫秒） |
| `instructions_file` | string | 否 | 指令文件名（默认：`"AGENTS.md"`） |
| `emits_permission_warning` | bool | 否 | 代理是否显示启动权限警告 |

**NonInteractiveConfig**（用于 `non_interactive` 字段）：

| 字段 | 类型 | 描述 |
|------|------|------|
| `subcommand` | string | 非交互执行的子命令（如 `"exec"`） |
| `prompt_flag` | string | 传递提示的标志（如 `"-p"`） |
| `output_flag` | string | 结构化输出的标志（如 `"--json"`） |

### 示例：Kiro 预设

```json
{
  "version": 1,
  "agents": {
    "kiro": {
      "name": "kiro",
      "command": "kiro",
      "args": ["--autonomous"],
      "process_names": ["kiro", "node"],
      "session_id_env": "KIRO_SESSION_ID",
      "resume_flag": "--resume",
      "resume_style": "flag",
      "prompt_mode": "arg",
      "ready_prompt_prefix": "> ",
      "ready_delay_ms": 5000,
      "instructions_file": "AGENTS.md",
      "non_interactive": {
        "prompt_flag": "-p",
        "output_flag": "--json"
      }
    }
  }
}
```

### 内置预设：GitHub Copilot CLI

`copilot` 作为内置预设发布 — 无需 JSON 文件。它使用 `--yolo` 标志进行自主模式，使用标志风格的会话恢复。Copilot CLI 通过 `.github/hooks/gastown.json` 支持完整的可执行生命周期 Hook：

```json
{
  "name": "copilot",
  "command": "copilot",
  "args": ["--yolo"],
  "process_names": ["copilot"],
  "resume_flag": "--resume",
  "resume_style": "flag",
  "ready_delay_ms": 5000,
  "hooks_provider": "copilot",
  "hooks_dir": ".github/hooks",
  "hooks_settings_file": "gastown.json",
  "instructions_file": "AGENTS.md"
}
```

Gas Town 在代理的工作目录中配置 `.github/hooks/gastown.json`，包含标准生命周期 Hook（`sessionStart`、`userPromptSubmitted`、`preToolUse`、`sessionEnd`）。这与 Claude Code 的 Hook 事件相同，只是使用 Copilot 的 JSON 格式。

> **就绪检测说明**：Copilot CLI 不发出可检测的提示前缀，因此 Gas Town 使用 5 秒延迟代替基于提示的检测。会话就绪所需时间比 Claude 略长。

> **企业要求**：Copilot CLI 必须在两个级别启用才能使用：
> 1. Enterprise → Settings → AI controls → Copilot → **"Copilot in the CLI" = Enabled**
> 2. Org → Settings → Copilot → Policies → **"Copilot in the CLI" = Enabled**
>
> 用户还需要分配 Copilot 席位。参见 [GitHub Copilot in the CLI](https://docs.github.com/en/copilot/using-github-copilot/using-github-copilot-in-the-command-line)。

激活方式：
```bash
gt config default-agent copilot        # 设为全镇默认
gt start --agent copilot               # 或按命令传递
```

### 激活自定义预设

一旦 JSON 文件存在，配置 Rig（或整个 Town）使用它：

```json
// 在 ~/gt/<rig>/settings/config.json
{
  "type": "rig-settings",
  "version": 1,
  "agent": "kiro"
}
```

或设为全镇默认：

```json
// 在 ~/gt/settings/config.json
{
  "type": "town-settings",
  "version": 1,
  "default_agent": "kiro"
}
```

你也可以按角色分配代理以优化成本：

```json
{
  "type": "town-settings",
  "version": 1,
  "default_agent": "claude",
  "role_agents": {
    "witness": "kiro",
    "polecat": "kiro"
  }
}
```

### 解析顺序

当 Gas Town 启动代理会话时，通过以下链解析配置：

1. 角色特定覆盖（Rig 设置中的 `role_agents[role]`）
2. 角色特定覆盖（Town 设置中的 `role_agents[role]`）
3. Rig 的 `agent` 字段
4. Town 的 `default_agent` 字段
5. 内置回退：`"claude"`

每一步中，代理名称在以下位置查找：
1. Rig 的自定义代理（`rig settings/agents.json`）
2. Town 的自定义代理（`town settings/agents.json`）
3. 内置预设（编译进 `gt`）

这意味着你的 JSON 预设会被自动发现 — 无需代码修改。

---

## 层级 2：Hook 集成

Hook 让 Gas Town 在会话启动时向你的代理注入上下文、守卫工具调用和投递邮件。根据你的代理支持什么，有三种模式。

### 模式 A：兼容 Claude 的 settings.json

如果你的代理支持带有生命周期 Hook 的 `settings.json`（如 Claude Code 或 Gemini CLI），Gas Town 可以自动安装 Hook。

**Hook 功能：**

| Hook | 事件 | 命令 |
|------|------|------|
| `SessionStart` | 代理会话开始 | `gt prime --hook && gt mail check --inject` |
| `PreCompact` | 上下文压缩前 | `gt prime --hook` |
| `UserPromptSubmit` | 用户发送消息 | `gt mail check --inject` |
| `PreToolUse` | 工具执行前 | `gt tap guard pr-workflow`（守卫 PR 创建） |
| `Stop` | 会话结束 | `gt costs record` |

参考模板：`internal/claude/config/settings-autonomous.json`

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "gt prime --hook && gt mail check --inject"
          }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "gt mail check --inject"
          }
        ]
      }
    ]
  }
}
```

**集成方式**：注册一个 `HookInstallerFunc` 将此设置文件写入正确位置。函数签名（来自 `internal/config/agents.go`）：

```go
type HookInstallerFunc func(settingsDir, workDir, role, hooksDir, hooksFile string) error
```

参数：
- `settingsDir` — Gas Town 管理的父目录（供使用 `--settings` 标志的代理使用）
- `workDir` — 代理的工作目录（客户仓库克隆）
- `role` — Gas Town 角色（`"polecat"`、`"crew"`、`"witness"`、`"refinery"`）
- `hooksDir` — 来自预设的 `hooks_dir` 字段
- `hooksFile` — 来自预设的 `hooks_settings_file` 字段

注册在 `internal/runtime/runtime.go` 中通过 `init()` 完成：

```go
config.RegisterHookInstaller("kiro", func(settingsDir, workDir, role, hooksDir, hooksFile string) error {
    // 将设置文件写入适当位置
    return kiro.EnsureSettingsForRoleAt(settingsDir, role, hooksDir, hooksFile)
})
```

### 模式 B：插件/脚本 Hook

如果你的代理使用插件系统（如 OpenCode 的 JS 插件），Gas Town 可以安装插件文件代替 settings.json。

参考：`internal/opencode/plugin/gastown.js`

```javascript
export const GasTown = async ({ $, directory }) => {
  const role = (process.env.GT_ROLE || "").toLowerCase();
  const autonomousRoles = new Set(["polecat", "witness", "refinery", "deacon"]);

  const run = async (cmd) => {
    try {
      await $`/bin/sh -lc ${cmd}`.cwd(directory);
    } catch (err) {
      console.error(`[gastown] ${cmd} failed`, err?.message || err);
    }
  };

  const injectContext = async () => {
    await run("gt prime");
    if (autonomousRoles.has(role)) {
      await run("gt mail check --inject");
    }
  };

  return {
    event: async ({ event }) => {
      if (event?.type === "session.created") {
        await injectContext();
      }
      if (event?.type === "session.compacted") {
        await injectContext();
      }
    },
  };
};
```

关键命令相同（`gt prime`、`gt mail check --inject`）。投递机制适配代理的插件 API。

### 模式 C：信息型 Hook（指令文件）

如果你的代理不支持可执行 Hook 但读取指令/上下文文件，Gas Town 可以安装包含启动指令的 Markdown 文件。

参考：`internal/hooks/templates/copilot/copilot-instructions.md`

```markdown
# Gas Town Agent Context

You are running inside Gas Town, a multi-agent workspace manager.

## Startup Protocol

On session start or after compaction, run:
\`\`\`
gt prime
\`\`\`
This loads your full role context, mail, and pending work.
```

在预设中设置 `hooks_informational: true`。Gas Town 将通过 tmux nudge 作为回退发送 `gt prime`（因为 Hook 不会自动运行）。

> **注意**：GitHub Copilot CLI 之前使用模式 C，但现在支持完整的可执行生命周期 Hook（相当于模式 B，使用自己的 JSON 格式）。参见上方的内置 Copilot 预设部分了解当前配置。

### Gas Town 如何选择回退策略

启动回退矩阵（来自 `internal/runtime/runtime.go`）：

| 有 Hook | 有提示 | 上下文来源 | 工作指令 |
|---------|--------|-----------|---------|
| 是 | 是 | Hook 运行 `gt prime` | 在 CLI 提示参数中 |
| 是 | 否 | Hook 运行 `gt prime` | 通过 Nudge 发送 |
| 否 | 是 | 提示中包含 "Run `gt prime`" | 延迟 Nudge |
| 否 | 否 | 通过 Nudge 发送 "Run `gt prime`" | 延迟 Nudge |

有 Hook 的代理获得最可靠的体验。没有 Hook 时，Gas Town 回退到基于 tmux 的投递，配合时序启发式。

---

## 层级 3：深度集成

这些是启用高级编排功能的可选能力。

### 非交互模式

用于 Gas Town 的公式系统（自动化工作流）和 Dog（基础设施助手）进行无头执行。通过 `non_interactive` 预设字段配置：

```json
{
  "non_interactive": {
    "subcommand": "exec",
    "prompt_flag": "-p",
    "output_flag": "--json"
  }
}
```

Gas Town 构建命令为：`kiro exec -p "prompt" --json`

### 会话 Fork

如果你的代理支持 Fork 过去的会话（创建只读副本以供检查），设置 `supports_fork_session: true`。用于 `gt seance` 命令与过去的代理会话对话。

### 包装脚本

对于完全不支持 Hook 的代理，包装脚本可以在启动代理之前注入 Gas Town 上下文。

参考：`internal/wrappers/scripts/gt-codex`

```bash
#!/bin/bash
set -e

if command -v gt &>/dev/null; then
    gt prime 2>/dev/null || true
fi

exec codex "$@"
```

包装脚本在 `exec` 真正的代理二进制之前运行 `gt prime`。用户将其作为 `gt-codex` 安装到 PATH 中。

### 实验性 Codex Hook（通过自定义 Profile）

Gas Town 还支持实验性的 Codex Hook 路径，供定义了带显式 Hook 设置的自定义 Codex 代理 Profile 的用户使用。

仅在以下两个条件同时满足时使用：
- 你的自定义代理 Profile 设置了 `prompt_mode: "arg"` 加上 `hooks.provider: "codex"`、`hooks.dir: ".codex"` 和 `hooks.settings_file: "hooks.json"`
- Codex 通过 `[features].codex_hooks = true` 启用了上游 Hook 功能

这通过现有的提供商安装器路径安装 `.codex/hooks.json`，并有意保持实现小巧：
- `SessionStart` 运行 `gt prime --hook`
- 自主模式 `SessionStart` 还运行 `gt mail check --inject`
- `Stop` 运行 `gt costs record`

自定义 Profile 示例：

```json
{
  "agents": {
    "codex-worker-hooks": {
      "command": "codex",
      "args": ["--dangerously-bypass-approvals-and-sandbox"],
      "prompt_mode": "arg",
      "hooks": {
        "provider": "codex",
        "dir": ".codex",
        "settings_file": "hooks.json"
      }
    }
  }
}
```

此路径不尝试更广泛的 Hook 对等，如工具守卫、提示提交 Hook 或预压缩行为。

默认的内置 `codex` 预设不变。它保持无 Hook 回退路径，上方的 `gt-codex` 包装器指导仍适用于该默认路径，除非你明确选择自定义支持 Hook 的 Codex Profile。

### 斜杠命令

Gas Town 将斜杠命令（如 `/commit`、`/handoff`）配置到代理配置目录中。如果你的代理从配置目录读取命令，在预设中设置 `config_dir`，Gas Town 将在那里配置命令。

---

## 能力矩阵

当前代理能力一览：

| 代理 | Hook | 恢复 | 非交互 | Fork | 提示模式 | 进程名 |
|------|------|------|--------|------|----------|--------|
| Claude | 是（settings.json） | `--resume`（标志） | 原生 | 是 | arg | node, claude |
| Gemini | 是 | `--resume`（标志） | `-p` | 否 | arg | gemini |
| Codex | 否 | `resume`（子命令） | `exec` 子命令 | 否 | none | codex |
| Cursor | 是（`.cursor/hooks.json`） | `--resume`（标志） | `-p` / `--print` + `--output-format` | 否 | arg | cursor-agent, agent |
| Auggie | 否 | `--resume`（标志） | 否 | 否 | arg | auggie |
| AMP | 否 | `threads continue`（子命令） | 否 | 否 | arg | amp |
| OpenCode | 是（插件 JS） | 否 | `run` 子命令 | 否 | none | opencode, node, bun |

---

## Gas City 提供商合约（前瞻性）

Gas Town 正在被 Gas City 接替，后者将隐式提供商接口正式化为显式合约。该合约源自 Gas Town 当前通过 tmux 垫片实现的功能 — 使原本基于启发式的方式变为原生。

### 接口

```
interface AgentProvider {
    // --- 层级 1：必需 ---

    // 生命周期
    Start(workDir string, env map[string]string) -> Process
    IsReady() -> bool
    IsAlive() -> bool

    // 通信
    SendMessage(text string) -> error
    GetStatus() -> AgentStatus

    // 身份
    Name() -> string
    Version() -> string

    // --- 层级 2：推荐 ---

    // 上下文注入
    InjectContext(context string) -> error
    OnSessionStart(callback) -> void

    // 会话管理
    Resume(sessionID string) -> Process
    SessionID() -> string

    // 工具守卫
    OnToolCall(callback) -> void

    // --- 层级 3：高级 ---

    // 会话 Fork
    ForkSession(sessionID string) -> Process

    // 非交互执行
    Exec(prompt string) -> Result

    // 成本跟踪
    GetUsage() -> UsageReport
}
```

### 什么保持不变

- JSON 预设注册（`agents.json`）
- 基于环境的身份（`GT_ROLE`、`GT_RIG`、`BD_ACTOR`）
- Hook 模式（`gt prime` 用于上下文，`gt mail check --inject` 用于邮件）
- Tmux 作为通用回退

### Gas City 中的变化

- 提供商可以原生实现 `IsReady()`，而不依赖提示前缀扫描或延迟启发式
- `SendMessage()` 替代支持它的提供商的 tmux `send-keys`
- `GetStatus()` 替代 tmux `capture-pane` 屏幕抓取
- `InjectContext()` 提供 Hook 当前功能的标准 API

**底线**：如果你今天在层级 1 集成（JSON 预设），你已经完成了 Gas City 合约的 90%。JSON 字段直接映射到提供商接口能力。

---

## 设计原则

### 发现，而非跟踪

代理活跃性从 tmux 状态推导，而非在数据库中跟踪。进程名和就绪提示是被观察的，而非自报告的。

### ZFC：零框架认知

代理决定如何处理指令。Gas Town 提供传输通道（tmux、Hook、Nudge），但不为代理做决策。接口是关于通信通道的，而非控制流。

### 优雅降级

每个能力都有回退方案：
- 没有 Hook？ → 通过 tmux 的启动回退命令
- 没有提示模式？ → Nudge 投递
- 没有恢复？ → 带交接邮件的全新会话
- 没有进程 API？ → Tmux pane_current_command

系统在零原生 API 支持下也能工作（只是不太可靠）。

---

## 常见错误

这些是我们在集成尝试中见过的问题模式。

### 硬编码到 GT 内部

将你的代理作为 Go 常量添加到 `agents.go`、在 `types.go` 中添加 switch case，或修改 `runtime.go` 会创建紧耦合。你的代理将成为 Gas Town 的构建时依赖。相反，使用运行时加载的 JSON 注册表（`settings/agents.json`）。

### 修改默认解析函数

`types.go` 中的 `default*()` 函数从预设注册表解析值。在这里添加代理特定的 case 意味着每次 Gas Town 发布都必须包含你的代理默认值。预设结构体已有这些值的字段 — 在 JSON 预设中设置它们。

### Fork Hook 模板

复制并修改 Claude 的 `settings-autonomous.json` 会造成维护负担。Hook 命令（`gt prime`、`gt mail check`）是与代理无关的。适配到你的代理 Hook 格式，但不要更改底层命令。

### 耦合 Gas Town 的内部模块结构

导入 Gas Town Go 包、引用内部文件路径或依赖内部数据结构意味着你的集成会在 Gas Town 重构时中断。公共接口是：
- `gt` CLI 命令（`gt prime`、`gt mail`、`gt hook` 等）
- 环境变量（`GT_ROLE`、`GT_RIG`、`GT_ROOT`、`BD_ACTOR`）
- JSON 配置文件（`settings/agents.json`）

### 跳过预设直接使用 RuntimeConfig 技巧

Rig `settings/config.json` 中的 `RuntimeConfig` 是向后兼容路径。现代方法是预设注册。RuntimeConfig 可用但缺少仅通过 `AgentPresetInfo` 可用的功能，如会话恢复、进程检测和非交互模式。

---

## 分步指南：今天集成你的代理

### 第 1 步：创建预设文件（5 分钟）

创建 `~/gt/settings/agents.json`（或添加到已有文件）：

```json
{
  "version": 1,
  "agents": {
    "your-agent": {
      "name": "your-agent",
      "command": "your-agent-cli",
      "args": ["--autonomous", "--no-confirm"],
      "process_names": ["your-agent-cli"],
      "prompt_mode": "arg",
      "ready_delay_ms": 5000,
      "instructions_file": "AGENTS.md"
    }
  }
}
```

### 第 2 步：测试基本启动（5 分钟）

```bash
# 为 Rig 设置你的代理为默认
gt config set agent your-agent --rig <rigname>

# 或用一次性覆盖测试
gt crew start jack --agent your-agent
```

验证：
- 代理在 tmux 面板中启动
- `gt prime` 内容被投递（通过 Hook、提示或 Nudge）
- 代理可以接收 Nudge（`gt nudge <rig>/crew/jack "hello"`）

### 第 3 步：添加会话恢复（如果支持）

添加到你的预设：

```json
{
  "session_id_env": "YOUR_AGENT_SESSION_ID",
  "resume_flag": "--resume",
  "resume_style": "flag"
}
```

测试：启动一个会话，记下会话 ID，终止 tmux 面板，验证重启时代理能恢复上下文。

### 第 4 步：添加 Hook（如果你的代理支持）

从上方 Hook 集成部分选择模式 A、B 或 C。

如果你的代理支持兼容 Claude 的 `settings.json` Hook：
1. 在预设中设置 `hooks_provider`、`hooks_dir` 和 `hooks_settings_file`
2. 在你的代理 Go 包中注册 `HookInstallerFunc`
3. 在 `internal/runtime/runtime.go` 的 `init()` 中注册

如果你的代理读取自定义指令文件：
1. 在预设中设置 `hooks_informational: true`
2. 设置 `hooks_dir` 和 `hooks_settings_file` 指向你的指令文件
3. 注册一个写入 Gas Town 指令的 Hook 安装器

### 第 5 步：添加非交互模式（如果支持）

添加到你的预设：

```json
{
  "non_interactive": {
    "subcommand": "run",
    "prompt_flag": "-p",
    "output_flag": "--json"
  }
}
```

这使你的代理可用于公式执行和 Dog 任务。

---

## 常见问题

### 我需要向 Gas Town 提交 PR 吗？

**层级 0-1 不需要**。JSON 预设是用户管理的配置。用户将文件放入他们的 Town 设置即可使用。

**层级 2（Hook 安装器注册）需要**，如果你想让它内置。但用户也可以手动安装 Hook 或通过包装脚本而无需任何 PR。

### 如果我的代理不支持自主模式怎么办？

Gas Town 需要自主模式（无确认提示）进行无人值守操作。如果你的代理没有 `--yolo` 或 `--dangerously-skip-permissions` 等效选项，Gas Town 无法将其用于 Polecat 或自动化角色。但仍可用于 Crew（人工监督）会话。

### Gas Town 设置哪些环境变量？

| 变量 | 示例 | 用途 |
|------|------|------|
| `GT_ROLE` | `gastown/crew/jack` | 代理在系统中的角色 |
| `GT_RIG` | `gastown` | 代理所属的 Rig |
| `GT_ROOT` | `/Users/me/gt` | Town 根目录 |
| `BD_ACTOR` | `gastown/crew/jack` | 问题跟踪的 Beads 身份 |
| `GIT_AUTHOR_NAME` | `gastown/crew/jack` | Git 提交身份 |
| `GT_AGENT` | `kiro` | 当前活跃的代理预设 |
| `GT_SESSION_ID_ENV` | `KIRO_SESSION_ID` | 保存会话 ID 的环境变量 |

### `gt prime` 是什么？

`gt prime` 是上下文注入命令。它将代理的角色文档、邮件、Hook 工作和系统指令以 Markdown 格式输出到 stdout。代理读取此输出以了解自己的身份和当前任务。这是对代理而言最重要的 Gas Town 命令。

### 我可以覆盖内置预设吗？

可以。`settings/agents.json` 中的用户定义代理优先于同名的内置预设。如果需要，你可以覆盖 `"claude"`。

### `AgentPresetInfo` 和 `RuntimeConfig` 有什么区别？

`AgentPresetInfo` 是静态预设 — 你在 JSON 中配置的内容。它描述了代理的能力和默认值。

`RuntimeConfig` 是完全解析的运行时配置，由预设与用户覆盖合并并填充默认值后产生。这是 Gas Town 实际用于构建启动命令的内容。

`RuntimeConfigFromPreset()` 将前者转换为后者。
`normalizeRuntimeConfig()` 从预设的 `default*()` 函数填充默认值。

### 进程检测如何工作？

Gas Town 将 `tmux display-message -p '#{pane_current_command}'` 与预设的 `process_names` 列表匹配。如果你的代理作为 Node.js 进程运行，你可能需要 `["node", "your-agent"]`，因为 tmux 可能报告其中任一名称。

### 就绪检测如何工作？

两种策略：

1. **提示前缀** — Gas Town 扫描 tmux 面板中的 `ready_prompt_prefix`（如 `"❯ "`）。可靠但需要已知的提示格式。
2. **延迟** — Gas Town 等待 `ready_delay_ms` 毫秒。当代理有无法扫描已知提示的 TUI 时使用。

在预设中设置一个或两个。有提示前缀时优先使用。