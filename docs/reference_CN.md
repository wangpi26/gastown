# Gas Town 参考

Gas Town 内部的技术参考。请先阅读 README。

> 目录结构详情参见 [architecture.md](design/architecture.md)。

## Beads 路由

Gas Town 根据问题 ID 前缀路由 Beads 命令。你不需要考虑使用哪个数据库 — 只需使用问题 ID。

```bash
bd show gp-xyz    # 路由到 greenplace Rig 的 Beads
bd show hq-abc    # 路由到 Town 级 Beads
bd show wyv-123   # 路由到 wyvern Rig 的 Beads
```

**工作原理**：路由定义在 `~/gt/.beads/routes.jsonl` 中。每个 Rig 的前缀映射到其 Beads 位置（该 Rig 中 Mayor 的克隆）。

| 前缀 | 路由到 | 用途 |
|------|--------|------|
| `hq-*` | `~/gt/.beads/` | Mayor 邮件、跨 Rig 协调 |
| `gp-*` | `~/gt/greenplace/mayor/rig/.beads/` | Greenplace 项目问题 |
| `wyv-*` | `~/gt/wyvern/mayor/rig/.beads/` | Wyvern 项目问题 |

调试路由：`BD_DEBUG_ROUTING=1 bd show <id>`

## 配置

### Rig 配置（`config.json`）

```json
{
  "type": "rig",
  "name": "myproject",
  "git_url": "https://github.com/...",
  "default_branch": "main",
  "beads": { "prefix": "mp" }
}
```

**Rig 配置字段：**

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `default_branch` | `string` | `"main"` | Rig 的默认分支。在 `gt rig add` 期间从远程自动检测。作为 Refinery 的合并目标，以及当没有集成分支活跃时作为 Polecat 的基础分支。 |

### 设置（`settings/config.json`）

```json
{
  "theme": {
    "disabled": false,
    "name": "forest",
    "custom": {
      "bg": "#111111",
      "fg": "#eeeeee"
    },
    "role_themes": {
      "witness": "rust",
      "refinery": "plum",
      "crew": "none"
    }
  },
  "merge_queue": {
    "enabled": true,
    "run_tests": true,
    "setup_command": "",
    "typecheck_command": "",
    "lint_command": "",
    "test_command": "",
    "build_command": "",
    "on_conflict": "assign_back",
    "delete_merged_branches": true,
    "retry_flaky_tests": 1,
    "poll_interval": "30s",
    "max_concurrent": 1,
    "integration_branch_polecat_enabled": true,
    "integration_branch_refinery_enabled": true,
    "integration_branch_template": "integration/{title}",
    "integration_branch_auto_land": false
  }
}
```

**主题字段：**

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `disabled` | `bool` | `false` | 禁用 Rig 的 tmux 状态/窗口主题 |
| `name` | `string` | 按 Rig 名称自动分配 | 使用命名的内置调色板主题 |
| `custom.bg` | `string` | 未设置 | 自定义 tmux 背景色 |
| `custom.fg` | `string` | 未设置 | 自定义 tmux 前景色 |
| `role_themes` | `map[string]string` | 未设置 | `witness`、`refinery`、`crew`、`polecat` 的每角色覆盖；使用 `"none"` 禁用某角色的主题 |

主题解析：
- 无 `theme` 配置：按 Rig 名称自动分配内置调色板主题
- `disabled: true`：跳过 `status-style` 和 `window-style`
- `name`：使用该内置主题
- `custom`：使用精确的 `{bg, fg}` 颜色
- `role_themes`：覆盖 Rig 内角色特定的会话

Town 级角色默认值位于 `mayor/config.json`：

```json
{
  "theme": {
    "disabled": false,
    "name": "forest",
    "custom": {
      "bg": "#111111",
      "fg": "#eeeeee"
    },
    "role_defaults": {
      "mayor": "forest",
      "deacon": "plum",
      "witness": "rust",
      "crew": "none"
    }
  }
}
```

`role_defaults` 支持 `mayor`、`deacon`、`witness`、`refinery`、`crew` 和 `polecat`。

**合并队列字段：**

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | `bool` | `true` | 合并队列是否激活 |
| `run_tests` | `bool` | `true` | 合并前运行测试 |
| `setup_command` | `string` | `""` | 设置/安装命令（如 `pnpm install`） |
| `typecheck_command` | `string` | `""` | 类型检查命令（如 `tsc --noEmit`） |
| `lint_command` | `string` | `""` | Lint 命令（如 `eslint .`） |
| `test_command` | `string` | `""` | 要运行的测试命令。空 = 跳过 |
| `build_command` | `string` | `""` | 构建命令（如 `go build ./...`） |
| `on_conflict` | `string` | `"assign_back"` | 冲突策略：`assign_back` 或 `auto_rebase` |
| `delete_merged_branches` | `bool` | `true` | 合并后删除源分支 |
| `retry_flaky_tests` | `int` | `1` | 不稳定测试重试次数 |
| `poll_interval` | `string` | `"30s"` | Refinery 轮询新 MR 的频率 |
| `max_concurrent` | `int` | `1` | 最大并发合并数 |
| `integration_branch_polecat_enabled` | `*bool` | `true` | Polecat 从集成分支自动获取工作树 |
| `integration_branch_refinery_enabled` | `*bool` | `true` | `gt done` / `gt mq submit` 自动目标为集成分支 |
| `integration_branch_template` | `string` | `"integration/{title}"` | 分支名模板（`{title}`、`{epic}`、`{prefix}`、`{user}`） |
| `integration_branch_auto_land` | `*bool` | `false` | Refinery 巡逻在所有子项关闭时自动着陆 |

集成分支详情参见 [Integration Branches](concepts/integration-branches.md)。

### 运行时（`.runtime/` - gitignored）

进程状态、PID、临时数据。

### Rig 级配置

Rig 通过分层配置支持：
1. **Wisp 层**（`.beads-wisp/config/`）- 临时、本地覆盖
2. **Rig 身份 Bead 标签** - 持久 Rig 设置
3. **Town 默认值**（`~/gt/settings/config.json`）
4. **系统默认值** - 编译时的回退

#### Polecat 分支命名

配置 Polecat 的自定义分支名模板：

```bash
# 通过 Wisp 设置（临时 - 用于测试）
echo '{"polecat_branch_template": "adam/{year}/{month}/{description}"}' > \
  ~/gt/.beads-wisp/config/myrig.json

# 或通过 Rig 身份 Bead 标签设置（持久）
bd update gt-rig-myrig --labels="polecat_branch_template:adam/{year}/{month}/{description}"
```

**模板变量：**

| 变量 | 描述 | 示例 |
|------|------|------|
| `{user}` | 来自 `git config user.name` | `adam` |
| `{year}` | 当前年份（YY 格式） | `26` |
| `{month}` | 当前月份（MM 格式） | `01` |
| `{name}` | Polecat 名称 | `alpha` |
| `{issue}` | 不含前缀的问题 ID | `123`（来自 `gt-123`） |
| `{description}` | 清理后的问题标题 | `fix-auth-bug` |
| `{timestamp}` | 唯一时间戳 | `1ks7f9a` |

**默认行为（向后兼容）：**

当 `polecat_branch_template` 为空或未设置时：
- 有问题：`polecat/{name}/{issue}@{timestamp}`
- 无问题：`polecat/{name}-{timestamp}`

**配置示例：**

```bash
# GitHub 企业格式
"adam/{year}/{month}/{description}"

# 简单功能分支
"feature/{issue}"

# 包含 Polecat 名称以便识别
"work/{name}/{issue}"
```

## Formula 格式

```toml
formula = "name"
type = "workflow"           # workflow | expansion | aspect
version = 1
description = "..."

[vars.feature]
description = "..."
required = true

[[steps]]
id = "step-id"
title = "{{feature}}"
description = "..."
needs = ["other-step"]      # 依赖
```

**组合：**

```toml
extends = ["base-formula"]

[compose]
aspects = ["cross-cutting"]

[[compose.expand]]
target = "step-id"
with = "macro-formula"
```

## Molecule 生命周期

> 完整生命周期图和详细命令参考参见 [concepts/molecules.md](concepts/molecules.md)。

**摘要**：Formula (TOML) --`bd cook`--> Protomolecule --`bd mol pour`--> Molecule（持久）或 Wisp（临时） --`bd squash`--> 摘要。

| 操作 | bd（数据） | gt（代理） |
|------|-----------|-----------|
| Cook/pour/wisp | `bd cook`、`bd mol pour/wisp` | — |
| Squash/burn | `bd mol squash/burn <id>` | `gt mol squash/burn`（附加） |
| 导航 | `bd mol current`、`bd mol show` | `gt hook`、`gt mol current` |
| 附加 | — | `gt mol attach/detach` |

## 代理生命周期

### Polecat 关闭

```
1. 按公式清单工作（由 gt prime 内联显示）
2. 通过 gt done 提交到合并队列
3. gt done 核弹清理沙箱并退出
4. Witness 移除工作树 + 分支
```

### 会话循环

```
1. 代理注意到上下文快满了
2. gt handoff（向自己发送邮件）
3. 管理器终止会话
4. 管理器启动新会话
5. 新会话读取交接邮件
```

## 环境变量

Gas Town 通过 `config.AgentEnv()` 为每个代理会话设置环境变量。这些在代理生成时设置在 tmux 会话环境中。

### 核心变量（所有代理）

| 变量 | 用途 | 示例 |
|------|------|------|
| `GT_ROLE` | 代理角色类型 | `mayor`、`witness`、`polecat`、`crew` |
| `GT_ROOT` | Town 根目录 | `/home/user/gt` |
| `BD_ACTOR` | 代理身份（归属用） | `gastown/polecats/toast` |
| `GIT_AUTHOR_NAME` | 提交归属（与 BD_ACTOR 相同） | `gastown/polecats/toast` |
| `BEADS_DIR` | Beads 数据库位置 | `/home/user/gt/gastown/.beads` |

### Rig 级变量

| 变量 | 用途 | 角色 |
|------|------|------|
| `GT_RIG` | Rig 名称 | witness, refinery, polecat, crew |
| `GT_POLECAT` | Polecat 工作者名称 | 仅 polecat |
| `GT_CREW` | Crew 工作者名称 | 仅 crew |
| `BEADS_AGENT_NAME` | 用于 Beads 操作的代理名称 | polecat, crew |

### 其他变量

| 变量 | 用途 |
|------|------|
| `GIT_AUTHOR_EMAIL` | 工作空间所有者邮箱（来自 git config） |
| `GT_TOWN_ROOT` | 覆盖 Town 根目录检测（手动使用） |
| `CLAUDE_RUNTIME_CONFIG_DIR` | 自定义 Claude 设置目录 |

### 按角色的环境变量

| 角色 | 关键变量 |
|------|---------|
| **Mayor** | `GT_ROLE=mayor`、`BD_ACTOR=mayor` |
| **Deacon** | `GT_ROLE=deacon`、`BD_ACTOR=deacon` |
| **Boot** | `GT_ROLE=deacon/boot`、`BD_ACTOR=deacon-boot` |
| **Witness** | `GT_ROLE=witness`、`GT_RIG=<rig>`、`BD_ACTOR=<rig>/witness` |
| **Refinery** | `GT_ROLE=refinery`、`GT_RIG=<rig>`、`BD_ACTOR=<rig>/refinery` |
| **Polecat** | `GT_ROLE=polecat`、`GT_RIG=<rig>`、`GT_POLECAT=<name>`、`BD_ACTOR=<rig>/polecats/<name>` |
| **Crew** | `GT_ROLE=crew`、`GT_RIG=<rig>`、`GT_CREW=<name>`、`BD_ACTOR=<rig>/crew/<name>` |

### Doctor 检查

`gt doctor` 命令验证正在运行的 tmux 会话是否具有正确的环境变量。不匹配报告为警告：

```
⚠ env-vars: Found 3 env var mismatch(es) across 1 session(s)
    hq-mayor: missing GT_ROOT (expected "/home/user/gt")
```

通过重启会话修复：`gt shutdown && gt up`

## 代理工作目录和设置

每个代理在特定的工作目录中运行，并有自己的 Claude 设置。理解此层级结构对于正确配置至关重要。

### 按角色的工作目录

| 角色 | 工作目录 | 备注 |
|------|----------|------|
| **Mayor** | `~/gt/mayor/` | Town 级协调器，与 Rig 隔离 |
| **Deacon** | `~/gt/deacon/` | 后台监督守护进程 |
| **Witness** | `~/gt/<rig>/witness/` | 无 git 克隆，仅监控 Polecat |
| **Refinery** | `~/gt/<rig>/refinery/rig/` | main 分支上的工作树 |
| **Crew** | `~/gt/<rig>/crew/<name>/rig/` | 持久的人工工作空间克隆 |
| **Polecat** | `~/gt/<rig>/polecats/<name>/rig/` | Polecat 工作树（临时沙箱） |

注意：每个 Rig 的 `<rig>/mayor/rig/` 目录不是工作目录 — 它是持有该 Rig 权威 `.beads/` 数据库的 git 克隆。

### 设置文件位置

设置安装在 Gastown 管理的父目录中，通过 `--settings` 标志传递给 Claude Code。这保持了客户仓库的整洁：

```
~/gt/
├── mayor/.claude/settings.json              # Mayor 设置（cwd = 设置目录）
├── deacon/.claude/settings.json             # Deacon 设置（cwd = 设置目录）
└── <rig>/
    ├── crew/.claude/settings.json           # 所有 Crew 成员共享
    ├── polecats/.claude/settings.json       # 所有 Polecat 共享
    ├── witness/.claude/settings.json        # Witness 设置
    └── refinery/.claude/settings.json       # Refinery 设置
```

`--settings` 标志将这些作为单独的优先级层加载，与客户仓库中的任何项目级设置加法合并。

### CLAUDE.md

仅 `~/gt/CLAUDE.md` 存在于磁盘 — 一个最小身份锚点，防止代理在上下文压缩或新会话后失去 Gas Town 身份。

完整的角色上下文（每个角色约 300-500 行）由 `gt prime` 通过 SessionStart Hook 临时注入。不创建每个目录的 CLAUDE.md 或 AGENTS.md 文件。

**为什么没有每个目录的文件？**
- Claude Code 从 CWD 向上遍历 CLAUDE.md — `~/gt/` 下的所有代理都会找到 Town 根文件
- AGENTS.md（用于 Codex）从 git 根向下遍历 — 父目录不可见，因此每个目录的 AGENTS.md 从不起作用
- 真正的上下文来自 `gt prime`，使磁盘上的引导指针变得多余

### 客户仓库文件（CLAUDE.md 和 .claude/）

Gas Town 不再使用 git sparse checkout 隐藏客户仓库文件。客户仓库可以有自己的 `.claude/` 目录和 `CLAUDE.md` — 这些在所有工作树（Crew、Polecat、Refinery、mayor/rig）中都被保留。

Gas Town 的上下文来自 Town 根的 `CLAUDE.md` 身份锚点（所有代理通过 Claude Code 的向上目录遍历拾取）、通过 SessionStart Hook 的 `gt prime`，以及客户仓库自己的 `CLAUDE.md`。这些安全共存，因为：

- **`--settings` 标志提供 Gas Town 设置**，作为与客户项目设置加法合并的单独层，两者整洁共存
- **`gt prime` 临时注入角色上下文**，通过 SessionStart Hook，与客户的 `CLAUDE.md` 加法叠加 — 两者都会被加载
- Gas Town 设置位于父目录中（不在客户仓库中），因此客户的 `.claude/` 文件完全保留

**Doctor 检查**：`gt doctor` 在旧版 sparse checkout 仍然配置时会发出警告。运行 `gt doctor --fix` 移除。工作树中被跟踪的 `settings.json` 文件被识别为客户项目配置，不会被标记为过期。

### 设置继承

Claude Code 的设置从多个来源分层：

1. 当前工作目录中的 `.claude/settings.json`（客户项目）
2. 父目录中的 `.claude/settings.json`（向上遍历）
3. `~/.claude/settings.json`（用户全局设置）
4. `--settings <path>` 标志（作为单独的加法层加载）

Gas Town 使用 `--settings` 标志从 Gastown 管理的父目录注入角色特定的设置。这与客户项目设置加法合并而非覆盖。

### 设置模板

Gas Town 根据角色类型使用两种设置模板：

| 类型 | 角色 | 关键区别 |
|------|------|----------|
| **交互式** | Mayor、Crew | 邮件在 `UserPromptSubmit` Hook 上注入 |
| **自主式** | Polecat、Witness、Refinery、Deacon | 邮件在 `SessionStart` Hook 上注入 |

自主代理可能在无用户输入的情况下启动，因此需要在会话开始时检查邮件。交互式代理等待用户提示。

### 故障排除

| 问题 | 解决方案 |
|------|---------|
| 代理使用了错误的设置 | 检查 `gt doctor`，验证角色父目录中的 `.claude/settings.json` |
| 找不到设置 | 运行 `gt install` 重新创建设置，或 `gt doctor --fix` |
| 源仓库设置泄漏 | 运行 `gt doctor --fix` 移除旧版 sparse checkout |
| Mayor 设置影响 Polecat | Mayor 应在 `mayor/` 中运行，而非 Town 根目录 |

## CLI 参考

### Town 管理

```bash
gt install [path]            # 创建 Town
gt install --git             # 带 git init
gt doctor                    # 健康检查
gt doctor --fix              # 自动修复
```

### 配置

```bash
# 代理管理
gt config agent list [--json]     # 列出所有代理（内置 + 自定义）
gt config agent get <name>        # 显示代理配置
gt config agent set <name> <cmd>  # 创建或更新自定义代理
gt config agent remove <name>     # 移除自定义代理（内置受保护）

# 默认代理
gt config default-agent [name]    # 获取或设置全镇默认代理
```

**内置代理**：`claude`、`gemini`、`codex`、`cursor`、`auggie`、`amp`、`opencode`、`copilot`

> **GitHub Copilot 说明**：`copilot` 预设使用 `.github/hooks/gastown.json` 中的可执行生命周期 Hook（`sessionStart`、`userPromptSubmitted`、`preToolUse`、`sessionEnd`）— 与 Claude Code 相同的生命周期事件，使用 Copilot 的 JSON 格式。Copilot 使用 5 秒就绪延迟代替基于提示的检测。需要 Copilot 席位和组织级 CLI 策略已启用。

**自定义代理**：通过 CLI 或 JSON 按 Town 定义：
```bash
gt config agent set claude-glm "claude-glm --model glm-4"
gt config agent set claude "claude-opus"  # 覆盖内置
gt config default-agent claude-glm       # 设置默认
```

**高级代理配置**（`settings/agents.json`）：
```json
{
  "version": 1,
  "agents": {
    "opencode": {
      "command": "opencode",
      "args": [],
      "resume_flag": "--session",
      "resume_style": "flag",
      "non_interactive": {
        "subcommand": "run",
        "output_flag": "--format json"
      }
    }
  }
}
```

**Rig 级代理**（`<rig>/settings/config.json`）：
```json
{
  "type": "rig-settings",
  "version": 1,
  "agent": "opencode",
  "agents": {
    "opencode": {
      "command": "opencode",
      "args": ["--session"]
    }
  }
}
```

**支持 ACP 的自定义代理**（`settings/config.json`）：
```json
{
  "type": "town-settings",
  "version": 1,
  "default_agent": "opencode-acp-debug",
  "agents": {
    "opencode-acp-debug": {
      "command": "opencode",
      "acp": {
        "command": "acp",
        "args": ["--debug", "--print-logs"]
      }
    }
  }
}
```

`acp` 字段配置代理通信协议支持：
- `command`：ACP 子命令（如 `"acp"` 用于 `opencode acp`）
- `args`：传递给 ACP 子命令的额外参数

自定义代理从其基础命令的预设继承 ACP 支持。例如，使用 `"command": "opencode"` 的自定义代理自动从 opencode 预设继承 ACP 支持。你可以通过显式指定 `acp` 字段来覆盖或扩展 ACP 参数。

**代理解析顺序**：Rig 级 → Town 级 → 内置预设。

对于 OpenCode 自主模式，在你的 Shell 配置文件中设置环境变量：
```bash
export OPENCODE_PERMISSION='{"*":"allow"}'
```

### Rig 管理

```bash
gt rig add <name> <url>
gt rig list
gt rig remove <name>
```

### Convoy 管理（主仪表板）

```bash
gt convoy list                          # 活跃 Convoy 仪表板
gt convoy status [convoy-id]            # 显示进度（🚚 hq-cv-*）
gt convoy create "name" [issues...]     # 创建跟踪问题的 Convoy
gt convoy create "name" gt-a bd-b --notify mayor/  # 带通知
gt convoy list --all                    # 包含已着陆的 Convoy
gt convoy list --status=closed          # 仅已着陆的 Convoy
```

注意："蜂群"是临时的（Convoy 问题上的工作者）。参见 [Convoys](concepts/convoy.md)。

### 工作分配

```bash
# 标准工作流：先 Convoy，再 Sling
gt convoy create "Feature X" gt-abc gt-def
gt sling gt-abc <rig>                    # 分配给 Polecat
gt sling gt-abc <rig> --agent codex      # 为此 sling/spawn 覆盖运行时
gt sling <proto> --on gt-def <rig>       # 带工作流模板

# 快速 Sling（自动创建 Convoy）
gt sling <bead> <rig>                    # 自动创建 Convoy 以便仪表板可见
```

代理覆盖：

- `gt start --agent <alias>` 为此启动覆盖 Mayor/Deacon 运行时。
- `gt mayor start|attach|restart --agent <alias>` 和 `gt deacon start|attach|restart --agent <alias>` 同理。
- `gt start crew <name> --agent <alias>` 和 `gt crew at <name> --agent <alias>` 覆盖 Crew 工作者的运行时。

### 通信

```bash
gt mail inbox
gt mail read <id>
gt mail send <addr> -s "Subject" -m "Body"
gt mail send --human -s "..."    # 发送给监督者
```

### 升级

```bash
gt escalate "topic"              # 默认：MEDIUM 严重级别
gt escalate -s CRITICAL "msg"    # 紧急，需立即关注
gt escalate -s HIGH "msg"        # 重要阻碍
gt escalate -s MEDIUM "msg" -m "Details..."
```

完整协议参见 [escalation.md](design/escalation.md)。

### 会话

```bash
gt handoff                   # 请求循环（上下文感知）
gt handoff --shutdown        # 终止（Polecat）
gt session stop <rig>/<agent>
gt peek <agent>              # 检查健康
gt nudge <agent> "message"   # 向代理发送消息
gt seance                    # 列出可发现的前驱会话
gt seance --talk <id>        # 与前驱对话（完整上下文）
gt seance --talk <id> -p "Where is X?"  # 一次性提问
```

**会话发现**：每个会话有一个启动 Nudge，在 Claude 的 `/resume` 选择器中可搜索：

```
[GAS TOWN] recipient <- sender • timestamp • topic[:mol-id]
```

示例：`[GAS TOWN] gastown/crew/gus <- human • 2025-12-30T15:42 • restart`

**重要**：始终使用 `gt nudge` 向 Claude 会话发送消息。绝不使用原始 `tmux send-keys` — 它无法正确处理 Claude 的输入。`gt nudge` 使用字面模式 + 防抖 + 单独的 Enter 以确保可靠投递。

### 紧急情况

```bash
gt stop --all                # 终止所有会话
gt stop --rig <name>         # 终止 Rig 会话
```

### 健康检查

```bash
gt deacon health-check <agent>   # 发送健康检查 ping，跟踪响应
gt deacon health-state           # 显示所有代理的健康检查状态
```

### 合并队列（MQ）

```bash
gt mq list [rig]             # 显示合并队列
gt mq next [rig]             # 显示最高优先级的合并请求
gt mq submit                 # 将当前分支提交到合并队列
gt mq status <id>            # 显示合并请求详细状态
gt mq retry <id>             # 重试失败的合并请求
gt mq reject <id>            # 拒绝合并请求
```

#### 集成分支命令

```bash
gt mq integration create <epic-id>              # 创建集成分支
gt mq integration create <epic-id> --branch "feat/{title}"  # 自定义模板
gt mq integration create <epic-id> --base-branch develop   # 非 main 基础
gt mq integration status <epic-id>              # 显示分支状态
gt mq integration status <epic-id> --json       # JSON 输出
gt mq integration land <epic-id>                # 合并到基础分支（默认：main）
gt mq integration land <epic-id> --dry-run      # 仅预览
gt mq integration land <epic-id> --force        # 强制着陆（含开放 MR）
gt mq integration land <epic-id> --skip-tests   # 跳过测试
```

完整工作流参见 [Integration Branches](concepts/integration-branches.md)。

## Beads 命令（bd）

```bash
bd ready                     # 无阻碍的工作
bd list --status=open
bd list --status=in_progress
bd show <id>
bd create --title="..." --type=task
bd update <id> --status=in_progress
bd close <id>
bd dep add <child> <parent>  # 子项依赖父项
```

## 巡逻代理

Deacon、Witness 和 Refinery 使用 Wisp 运行持续巡逻循环：

| 代理 | 巡逻 Molecule | 职责 |
|------|---------------|------|
| **Deacon** | `mol-deacon-patrol` | 代理生命周期、插件执行、健康检查 |
| **Witness** | `mol-witness-patrol` | 监控 Polecat、Nudge 卡住的工作者 |
| **Refinery** | `mol-refinery-patrol` | 处理合并队列、审查 MR、检查集成分支 |

```
1. gt patrol new               # 创建仅根的 Wisp
2. gt prime                    # 内联显示巡逻清单
3. 按每个步骤工作
4. gt patrol report --summary "..."  # 关闭 + 开始下一个周期
```

## 插件 Molecule

插件是带有特定标签的 Molecule：

```json
{
  "id": "mol-security-scan",
  "labels": ["template", "plugin", "witness", "tier:haiku"]
}
```

巡逻 Molecule 动态绑定插件：

```bash
bd mol bond mol-security-scan $PATROL_ID --var scope="$SCOPE"
```

## Formula 调用模式

**关键**：不同的公式类型需要不同的调用方式。

### 工作流公式（顺序步骤，单个 Polecat）

示例：`shiny`、`shiny-enterprise`、`mol-polecat-work`

```bash
gt sling <formula> --on <bead-id> <target>
gt sling shiny-enterprise --on gt-abc123 gastown
```

### Convoy 公式（并行段，多个 Polecat）

示例：`code-review`

**不要对 Convoy 公式使用 `gt sling`！** 它会失败并提示 "convoy type not supported"。

```bash
# 正确调用 - 使用 gt formula run：
gt formula run code-review --pr=123
gt formula run code-review --files="src/*.go"

# 干运行预览：
gt formula run code-review --pr=123 --dry-run
```

### 识别公式类型

```bash
gt formula show <name>   # 显示 "Type: convoy" 或 "Type: workflow"
bd formula list          # 按类型列出公式
```

### 为什么这很重要

- `gt sling` 尝试 cook+pour 公式，这对 convoy 类型会失败
- `gt formula run` 直接处理 Convoy 派发，生成并行的 Polecat
- Convoy 公式创建多个 Polecat（每段一个）+ 综合步骤

## 常见问题

| 问题 | 解决方案 |
|------|---------|
| 代理在错误目录 | 检查 cwd，`gt doctor` |
| Beads 前缀不匹配 | 检查 `bd show` vs Rig 配置 |
| 工作树冲突 | 检查工作树状态，`gt doctor` |
| 工作者卡住 | `gt nudge`，然后 `gt peek` |
| Git 状态脏 | 提交或丢弃，然后 `gt handoff` |

> 架构详情（裸仓库模式、Beads 作为控制面板、非确定性幂等性）参见 [architecture.md](design/architecture.md)。