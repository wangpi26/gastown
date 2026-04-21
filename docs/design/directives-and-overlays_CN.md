# 角色 Directive 和 Formula Overlay

> 操作员可自定义 agent 行为，无需修改 Go 二进制文件。

> **参考示例：** [`docs/contrib-harnesses/`](../contrib-harnesses/)
> 包含贡献者可以直接放入自己 rig 的 directive 和 overlay 的复制-适配版本。
> 例如，`polecat-pr-flow/` 是一个以 GitHub PR 审查而非规范 Refinery 合并队列为
> 工作门的 rig。

## 问题

MEOW 栈将 formula 和角色模板嵌入二进制文件 — 这是有意集中化以保证一致性，
但没有留下覆盖路径。操作员无法在 rig 或 town 级别自定义 agent 行为。

**具体故障：** 多个 crew 成员在 PR 审查任务期间自主地发布了 `gh pr review`
评论到 GitHub。Formula 写着"发布到 GitHub"，操作员没有办法说"实际上，
在这个 rig 中，改为回报结果。"

## 设计：两个层级

### 第一层：角色 Directive

在 prime 时注入的按角色行为边界。操作员编写的 Markdown，修改给定角色的
agent 行为，无论它们运行哪个 formula。

**文件布局：**

```
~/gt/directives/<role>.md              # Town 级（所有 rig）
~/gt/<rig>/directives/<role>.md        # Rig 级（因排在最后而优先）
```

**注入点：** 在角色模板之后，上下文文件和 handoff 内容之前。
Directive 带有权威标记："Rig Policy — 在冲突时覆盖 formula 指令。"

**优先级：** Town 和 rig directive **连接**。如果两者都存在，
合并输出为 `<town content>\n<rig content>`。Rig directive 拥有
最后发言权，因此在冲突指令上有效地覆盖 town directive。

**实现：**
- 加载器：`internal/config/directives.go` → `LoadRoleDirective(role, townRoot, rigName) string`
- 集成：`internal/cmd/prime_output.go` → `outputRoleDirectives(ctx RoleContext)`
- 在 `outputPrimeContext()` 之后的 `gt prime` 管道中调用

### 第二层：Formula Overlay

在 rig 或 town 作用域下的按 formula、按步骤覆盖。类似 CSS 的步骤修改，
在 prime 时解析后、渲染前应用。

**文件布局：**

```
~/gt/formula-overlays/<formula>.toml        # Town 级
~/gt/<rig>/formula-overlays/<formula>.toml  # Rig 级（完全优先）
```

**优先级：** Rig 级 overlay **完全替代** town 级 overlay（不合并）。
如果 rig overlay 存在，town overlay 被完全忽略。这防止冲突的步骤修改
不可预测地合并。

**实现：**
- 加载器：`internal/formula/overlay.go` → `LoadFormulaOverlay(formulaName, townRoot, rigName) (*FormulaOverlay, error)`
- 应用器：`internal/formula/overlay.go` → `ApplyOverlays(f *Formula, overlay *FormulaOverlay) []string`
- 集成：`internal/cmd/prime_molecule.go` → `applyFormulaOverlays()` 在 `showFormulaStepsFull()` 中调用

## TOML 格式（Overlay）

Formula overlay 使用带有 `[[step-overrides]]` 数组的 TOML：

```toml
[[step-overrides]]
step_id = "submit-review"
mode = "replace"
description = """
将你的审查发现回报到对话中，而不是发布到 GitHub。
格式为带有等级和发现的结构化摘要。"""

[[step-overrides]]
step_id = "build"
mode = "append"
description = """
同时运行集成测试: npm run test:integration"""

[[step-overrides]]
step_id = "deprecated-step"
mode = "skip"
```

### 覆盖模式

| 模式 | 效果 | 是否需要 `description` |
|------|------|----------------------|
| `replace` | 完全替换步骤描述 | 是 |
| `append` | 在现有步骤描述后追加文本（换行分隔） | 是 |
| `skip` | 从 formula 中移除步骤 | 否 |

### Skip 模式的依赖处理

当步骤被跳过时，依赖它的步骤继承其 `needs`（依赖项）。
这保持了 formula DAG 的完整性。例如，如果步骤 B 依赖步骤 A，
而步骤 A 被跳过，那么步骤 B 继承步骤 A 所依赖的一切。

### 验证规则

- 每个覆盖都必须有 `step_id`
- `mode` 必须是以下之一：`replace`、`append`、`skip`
- 格式错误的 TOML 在加载时返回错误
- 不匹配任何 formula 步骤的步骤 ID 会产生警告（过时覆盖）

## Directive 格式（Markdown）

角色 directive 是普通 Markdown 文件。没有特殊语法 — 内容在
agent 的 prime 输出中带上权威标题逐字注入。

```markdown
## PR 审查策略

不要通过 `gh pr review` 直接向 GitHub 发布审查评论。
相反，将发现作为结构化摘要回报到对话中。

## 代码风格

提交前始终运行 `npm run lint --fix`。
遵循代码库中的现有模式。
```

## CLI 命令

> **注意：** CLI 命令正在 gt-3kg.5 中添加。以下界面
> 反映了计划中的设计。

### Directive 命令

```bash
gt directive show <role> [--rig <rig>]    # 显示带来源的活动 directive
gt directive edit <role> [--rig <rig>]    # 在编辑器中打开（如需要则创建文件）
gt directive list                         # 列出所有 directive 文件
```

### Overlay 命令

```bash
gt formula overlay show <formula> [--rig <rig>]   # 显示带来源的活动 overlay
gt formula overlay edit <formula> [--rig <rig>]   # 在编辑器中打开（如需要则创建文件）
gt formula overlay list                           # 列出所有 overlay 文件
```

`edit` 命令在目录和文件不存在时创建它们（遵循 `gt hooks override` 先例）。
`show` 命令显示带来源标注（town vs rig）的已解析内容。

## gt doctor 集成

`overlay-health` doctor 检查验证 formula overlay：

```bash
gt doctor                    # 运行所有检查，包括 overlay 健康
```

**检查内容：**
- 扫描所有 town 级和 rig 级 overlay TOML 文件
- 解析每个 overlay 并加载对应的嵌入 formula
- 验证每个 `step_id` 存在于当前 formula 版本中
- 报告过时的步骤 ID（formula 已更新，overlay 未更新）

**结果：**
- **正常：** "N overlay(s) healthy" 或 "no overlay files found"
- **警告：** 发现过时步骤 ID（可自动修复）
- **错误：** 格式错误的 TOML（需要手动修复）

**自动修复：**

```bash
gt doctor --fix              # 移除过时的 step-override 条目
```

修复会移除引用不存在步骤 ID 的步骤覆盖。如果文件中的所有覆盖都过时，
则删除整个文件。格式错误的 TOML 不会被修改。

**实现：** `internal/doctor/overlay_health_check.go`

## 实例演示：PR 审查覆盖

这是推动此功能的动机用例。

### 问题

`mol-polecat-work` formula 有一个名为 `submit-review` 的步骤，告诉
polecats 使用 `gh pr review --comment` 将审查结果发布到 GitHub。
在 gastown rig 中，操作员希望 polecats 改为在对话中回报发现。

### 解决方案

**步骤 1：创建 rig 级 formula overlay。**

```bash
mkdir -p ~/gt/gastown/formula-overlays
```

创建 `~/gt/gastown/formula-overlays/mol-polecat-work.toml`：

```toml
[[step-overrides]]
step_id = "submit-review"
mode = "replace"
description = """
将你的审查发现回报到对话中。格式为：

## 审查: <文件或组件>
**等级:** A-F
**发现:**
- 严重: ...
- 主要: ...
- 次要: ...

不要通过 gh pr review 向 GitHub 发布评论。"""
```

**步骤 2：用 gt doctor 验证。**

```bash
gt doctor
# ✓ overlay-health: 1 overlay(s) healthy
```

**步骤 3：用 gt prime 测试。**

```bash
gt prime --explain
# 显示: "Formula overlay: applying 1 override(s) for mol-polecat-work (rig=gastown)"
```

现在 gastown rig 中运行 `mol-polecat-work` 的任何 polecat 都将看到
替换步骤，而不是原始的"发布到 GitHub"指令。

### 如果 Formula 变更了怎么办？

如果未来的 `gt` 版本将 `submit-review` 重命名为 `post-results`，
overlay 的 `step_id` 将变得过时。下次 `gt doctor` 运行时：

```
⚠ overlay-health: stale step IDs in gastown/formula-overlays/mol-polecat-work.toml:
  - step_id "submit-review" not found in formula mol-polecat-work
```

运行 `gt doctor --fix` 移除过时的覆盖。操作员然后
创建一个针对 `post-results` 的新覆盖。

## 设计理据

### 为什么是两个层级，而不是一个？

Directive 和 overlay 在不同粒度上解决不同的问题：

| 方面 | Directive | Overlay |
|------|-----------|---------|
| 作用域 | 整个角色行为 | 单个 formula 步骤 |
| 粒度 | 宽泛策略 | 精确修改 |
| 格式 | Markdown（散文） | TOML（结构化） |
| 优先级 | 连接（叠加） | 替代（排他） |
| 示例 | "永远不要发布到 GitHub" | "在步骤 X 中，改为做 Y" |

一个角色 directive 说"永远不要发布到 GitHub"适用于任何 formula、
任何步骤。一个针对 `mol-polecat-work` 中 `submit-review` 的 overlay
仅适用于该特定 formula 中的该特定步骤。

两者都需要：directive 提供宽泛护栏，overlay 提供精确修复。

### 为什么不直接修改 Formula？

Formula 嵌入在 Go 二进制文件中。修改它们需要重新构建
和重新部署。Directive 和 overlay 是外部配置文件，在下次
`gt prime` 时立即生效。

### 架构协调

- **契合 gt prime 管道：** 角色模板 → directive → 上下文 → handoff → formula
- **遵循 hooks 覆盖先例：** `~/.gt/hooks-overrides/<target>.json`
- **扩展属性层：** Rig > town > system 优先级
- **符合 ZFC：** Go 传输内容，agent 解释指令
- **仅涉及 gt：** `bd` 不渲染 formula，因此 overlay 仅限 gt

### 需要管理的不和谐

- **冲突指令：** Directive 说"不要 X"，formula 说"做 X" →
  通过注入时的明确权威框架缓解（"Rig Policy — 在冲突时覆盖 formula 指令"）
- **不稳定的步骤 ID：** Formula 步骤不是稳定的 API；步骤 ID 可能
  跨版本变化 → `gt doctor` 对过时 overlay 发出警告
- **可发现性：** `gt prime --explain` 显示带来源标注的
  活动 directive/overlay

## 文件参考

| 文件 | 用途 |
|------|------|
| `internal/config/directives.go` | Directive 加载器（`LoadRoleDirective`） |
| `internal/config/directives_test.go` | Directive 测试 |
| `internal/formula/overlay.go` | Overlay 加载器和应用器 |
| `internal/formula/overlay_test.go` | Overlay 测试 |
| `internal/cmd/prime_output.go` | `outputRoleDirectives()` 集成 |
| `internal/cmd/prime_molecule.go` | `applyFormulaOverlays()` 集成 |
| `internal/doctor/overlay_health_check.go` | Doctor 检查和自动修复 |