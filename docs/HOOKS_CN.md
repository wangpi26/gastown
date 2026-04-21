# Gas Town Hook 管理

Gas Town 工作空间的集中式 Hook 管理。

## 概述

Gas Town 为所有支持的代理管理上下文注入。具体机制因代理而异：

| 代理 | Hook 机制 | 管理的文件 |
|------|-----------|-----------|
| Claude Code、Gemini | `settings.json` 生命周期 Hook | `<role>/.claude/settings.json` |
| OpenCode | JS 插件 | `workDir/.opencode/gastown.js` |
| GitHub Copilot | JSON 生命周期 Hook | `workDir/.github/hooks/gastown.json` |
| Codex、其他 | 启动提示回退 | *（无文件 — 仅提示）* |

> **GitHub Copilot 说明**：Copilot CLI 通过 `.github/hooks/gastown.json` 支持完整的可执行生命周期 Hook（`sessionStart`、`userPromptSubmitted`、`preToolUse`、`sessionEnd`）。这与 Claude Code 的生命周期覆盖范围相同，只是以 Copilot 的 JSON 格式而非 Claude 的 `settings.json` 格式提供。下方的 `gt hooks` 命令仅适用于 Claude Code（和 Gemini）。

Gas Town 管理 Gastown 托管父目录中的 `.claude/settings.json` 文件，并通过 `--settings` 标志传递给 Claude Code。这保持了客户仓库的整洁，同时提供针对角色的 Hook 配置。Hook 系统提供单一事实来源，包含基础配置和每个角色/每个 Rig 的覆盖。

## 架构

```
~/.gt/hooks-base.json              ← 共享基础配置（所有代理）
~/.gt/hooks-overrides/
  ├── crew.json                    ← 所有 Crew 工作者的覆盖
  ├── witness.json                 ← 所有 Witness 的覆盖
  ├── gastown__crew.json           ← 专门针对 gastown Crew 的覆盖
  └── ...
```

**合并策略**：`base → role → rig+role`（更具体的优先）

对于 `gastown/crew` 这样的目标：
1. 从基础配置开始
2. 应用 `crew` 覆盖（如存在）
3. 应用 `gastown/crew` 覆盖（如存在）

## 生成的目标

每个 Rig 在共享父目录中（而非每个工作树）生成设置：

| 目标 | 路径 | 覆盖键 |
|------|------|--------|
| Crew（共享） | `<rig>/crew/.claude/settings.json` | `<rig>/crew` |
| Witness | `<rig>/witness/.claude/settings.json` | `<rig>/witness` |
| Refinery | `<rig>/refinery/.claude/settings.json` | `<rig>/refinery` |
| Polecat（共享） | `<rig>/polecats/.claude/settings.json` | `<rig>/polecats` |

Town 级目标：
- `mayor/.claude/settings.json`（键：`mayor`）
- `deacon/.claude/settings.json`（键：`deacon`）

设置通过 `--settings <path>` 传递给 Claude Code，作为单独的优先级层加载，与项目设置进行加法合并。

## 命令

### `gt hooks sync`

从基础配置 + 覆盖重新生成所有 `.claude/settings.json` 文件。保留非 Hook 字段（editorMode、enabledPlugins 等）。

```bash
gt hooks sync             # 写入所有设置文件
gt hooks sync --dry-run   # 预览变更但不写入
```

### `gt hooks diff`

显示 `sync` 会产生的变更，但不实际写入。

```bash
gt hooks diff             # 显示差异
gt hooks diff --no-color  # 纯文本输出
```

### `gt hooks base`

在 `$EDITOR` 中编辑共享基础配置。

```bash
gt hooks base             # 在编辑器中打开
gt hooks base --show      # 打印当前基础配置
```

### `gt hooks override <target>`

编辑特定角色或 Rig+角色的覆盖。

```bash
gt hooks override crew              # 编辑 Crew 覆盖
gt hooks override gastown/witness   # 编辑 gastown Witness 覆盖
gt hooks override crew --show       # 打印当前覆盖
```

### `gt hooks list`

显示所有托管的 settings.local.json 位置及其同步状态。

```bash
gt hooks list             # 显示所有目标
gt hooks list --json      # 机器可读输出
```

### `gt hooks scan`

扫描工作空间中的现有 Hook（读取当前设置文件）。

```bash
gt hooks scan             # 列出所有 Hook
gt hooks scan --verbose   # 显示 Hook 命令
gt hooks scan --json      # JSON 输出
```

### `gt hooks init`

从现有 settings.local.json 文件引导基础配置。分析所有当前设置，提取公共 Hook 作为基础，并为每个目标的差异创建覆盖。

```bash
gt hooks init             # 引导基础配置和覆盖
gt hooks init --dry-run   # 预览将要创建的内容
```

仅在不存在基础配置时有效。使用 `gt hooks base` 编辑已有的基础配置。

### `gt hooks registry` / `gt hooks install`

浏览并从注册表安装 Hook。

```bash
gt hooks registry                  # 列出可用的 Hook
gt hooks install <hook-id>         # 安装一个 Hook 到基础配置
```

## 当前注册表 Hook

注册表（`~/gt/hooks/registry.toml`）定义了 7 个 Hook，其中 5 个默认启用：

| Hook | 事件 | 默认启用 | 角色 |
|------|------|----------|------|
| pr-workflow-guard | PreToolUse | 是 | crew, polecat |
| session-prime | SessionStart | 是 | all |
| pre-compact-prime | PreCompact | 是 | all |
| mail-check | UserPromptSubmit | 是 | all |
| costs-record | Stop | 是 | crew, polecat, witness, refinery |
| clone-guard | PreToolUse | 否 | crew, polecat |
| dangerous-command-guard | PreToolUse | 是 | crew, polecat |

settings.json 文件中存在一些尚未纳入注册表的 Hook：

- **bd init guard**（gastown/crew, beads/crew）- 阻止在 `.beads/` 内执行 `bd init*`
- **mol patrol guards**（gastown 角色）- 阻止持久的巡逻 Molecule
- **tmux clear-history**（gastown 根目录）- 会话启动时清除终端历史
- **SessionStart .beads/ validation**（gastown/crew, beads/crew）- 验证 CWD

## 设计决策：注册表作为目录还是事实来源

> **决策：注册表是目录，不是事实来源。**
>
> 注册表（`registry.toml`）列出可用的 Hook。基础/覆盖系统（`~/.gt/hooks-base.json` + `~/.gt/hooks-overrides/`）定义哪些 Hook 处于活跃状态。`gt hooks install` 从注册表复制到基础/覆盖配置中。
>
> 这种分离提供了：
> - 每台机器的自定义（不同机器的 PATH 差异）
> - 每个角色的覆盖，不会污染共享注册表
> - "存在哪些 Hook" 和 "哪些 Hook 在哪里激活" 之间的清晰区分
>
> 注册表是菜单。基础/覆盖是点单。

## 已知不足

1. **注册表未覆盖所有活跃 Hook** — settings.json 文件中有多个 Hook 不在 `registry.toml` 中（bd-init-guard、mol-patrol-guard、tmux-clear、cwd-validation）。这些应该被添加以便 `gt hooks install` 能管理它们。

2. **除了 pr-workflow 外没有其他 `gt tap` 命令** — Tap 框架只有一个守卫实现了。`gt tap guard dangerous-command` 在注册表中被引用但尚不存在。优先顺序：dangerous-command、bd-init、mol-patrol，然后是 audit git-push。

3. **没有 `gt tap disable/enable` 便捷命令** — 每个工作树的启用/禁用可以通过覆盖机制实现（`gt hooks override` 配合空 Hook 列表），但尚无便捷封装。

4. **私有 Hook（settings.local.json）** — Claude Code 支持 `settings.local.json` 用于个人覆盖。Gas Town 尚未管理这些。优先级较低，因为 Gas Town 主要由代理操作。

5. **Hook 排序** — 目前无需操作。合并链（base -> override）产生确定性顺序，且每个匹配器的合并确保每个事件类型只有一条记录。

## 集成

### `gt rig add`

创建新 Rig 时，会自动为新 Rig 的所有目标（Crew、Witness、Refinery、Polecat）同步 Hook。

### `gt doctor`

`hooks-sync` 检查验证所有 settings.local.json 文件是否与 `gt hooks sync` 生成的结果一致。使用 `gt doctor --fix` 自动修复不同步的目标。

## 每个匹配器的合并语义

当覆盖具有与基础条目相同的匹配器时，覆盖**完全替换**基础条目。不同的匹配器则追加。带有空 Hook 列表的覆盖条目将**移除**该匹配器。

基础配置示例：
```json
{
  "SessionStart": [
    { "matcher": "", "hooks": [{ "type": "command", "command": "gt prime" }] }
  ]
}
```

Witness 的覆盖：
```json
{
  "SessionStart": [
    { "matcher": "", "hooks": [{ "type": "command", "command": "gt prime --witness" }] }
  ]
}
```

结果：Witness 获得 `gt prime --witness` 而不是 `gt prime`（相同匹配器 = 替换）。

## 默认基础配置

当不存在基础配置时，系统使用合理的默认值：

- **SessionStart**：PATH 设置 + `gt prime --hook`
- **PreCompact**：PATH 设置 + `gt prime --hook`
- **UserPromptSubmit**：PATH 设置 + `gt mail check --inject`
- **Stop**：PATH 设置 + `gt costs record`