# Integration Branch

> 在共享分支上组织 Epic 工作，作为一个单元着陆到 main。

Integration Branch 为 Gas Town 流水线中 Epic 范围的工作提供端到端支持。当你为 Epic 创建 Integration Branch 时，它成为每个阶段的自动目标：Polecat 从 Integration Branch 生成 worktree（这样它们启动时就包含了同级的工作），Refinery 将完成的 MR 合并到 Integration Branch 而非 main，当所有 Epic 子项关闭后，Refinery 可以将 Integration Branch 着陆回其基础分支（默认为 main，或创建时通过 `--base-branch` 指定的分支），作为一个单独的 merge commit。

着陆可以按命令执行，也可以通过巡逻自动执行。结果是整个 Epic 从第一次 Sling 到最终着陆，作为一个连贯单元流经系统，无需任何手动指定分支目标。

## 工作流

1. **创建 Epic 及其子项。** 将工作组织为带有子任务（或子 Epic）的 Epic。在子项之间设置依赖关系，以定义哪些可以并行运行，哪些必须等待。

2. **创建 Integration Branch。** 这是所有子项工作累积的共享分支。
   ```bash
   gt mq integration create gt-auth-epic
   ```

3. **创建 Convoy 来追踪工作。** Convoy 为整个 Epic 的进度提供单一仪表盘。
   ```bash
   gt convoy create "Auth overhaul" gt-auth-tokens gt-auth-sessions gt-auth-middleware
   ```

4. **Sling 第一波工作。** 找出没有阻塞者的子项并 Sling 到 Rig。使用 `--no-convoy` 因为追踪 Convoy 已存在。
   ```bash
   gt sling gt-auth-tokens gastown --no-convoy
   gt sling gt-auth-sessions gastown --no-convoy
   ```

5. **Polecat 处理工作。** 每个 Polecat 从 Integration Branch 生成 worktree，因此启动时已包含已着陆的同级工作。Polecat 完成后，提交 merge request。

6. **Refinery 合并到 Integration Branch。** Refinery 将每个 MR 合并到 Integration Branch 而非 main，并将子任务标记为完成。

7. **通过 Convoy 追踪进度。** 每次 Refinery 完成一个任务，Convoy 状态都会更新。
   ```bash
   gt convoy status hq-cv-abc
   ```

8. **Sling 下一波工作。** 当一波完成且其依赖的子项解除阻塞时，Sling 下一批。那些 Polecat 将从 Integration Branch 启动——它现在包含了前一波的所有工作。
   ```bash
   gt sling gt-auth-middleware gastown --no-convoy
   ```

9. **完成时着陆。** 当 Epic 下所有子项关闭后，Integration Branch 准备着陆。如果启用了 `integration_branch_auto_land`，Refinery 会在巡逻时自动执行。否则，手动着陆：
   ```bash
   gt mq integration land gt-auth-epic
   ```
   这将 Integration Branch 合并回其基础分支（默认 main），作为一个 merge commit，删除分支，并关闭 Epic。

## 概念

### 问题

没有 Integration Branch，Epic 工作零散着陆：

```
Child A ──► MR ──► main     （周二着陆）
Child B ──► MR ──► main     （周三着陆，破坏了 A 的工作）
Child C ──► MR ──► main     （周四着陆，依赖 A+B 一起）
```

每个子项独立合并。如果 Child C 依赖 A 和 B 的连贯性，你就依赖于合并顺序，并期望着陆之间不出问题。

### 解决方案

Integration Branch 将 Epic 工作批量组织在共享分支上，然后原子着陆：

```
                           Epic: gt-auth-epic
                                  │
                    ┌─────────────┼─────────────┐
                    │             │             │
               Child A       Child B       Child C
                    │             │             │
                    ▼             ▼             ▼
               ┌────────┐  ┌────────┐  ┌────────┐
               │  MR A  │  │  MR B  │  │  MR C  │
               └───┬────┘  └───┬────┘  └───┬────┘
                   │           │           │
                   └───────────┼───────────┘
                               ▼
                 integration/gt-auth-epic
                    (shared branch)
                               │
                               ▼ gt mq integration land
                          base branch
                    (main or --base-branch)
                     (single merge commit)
```

所有子 MR 先合并到 Integration Branch。子项可以相互构建。一切就绪后，一条命令全部着陆。

### 有无 Integration Branch 对比

| 方面 | 无 Integration Branch | 有 Integration Branch |
|------|---------------------|----------------------|
| MR 目标 | main | integration/{epic} |
| 着陆时机 | 每个 MR 独立着陆 | 所有 MR 一起着陆 |
| 跨子项依赖 | 有风险——依赖合并顺序 | 安全——子项共享分支 |
| 回滚 | 还原个别提交 | 还原一个 merge commit |
| main 上的 CI | 每个 MR 运行一次 | 对组合工作运行一次 |

## 生命周期

### 1. 创建 Epic

```bash
bd create --type=epic --title="Auth overhaul"
# → gt-auth-epic
```

在 Epic 下正常创建子 issue。

### 2. 创建 Integration Branch

```bash
gt mq integration create gt-auth-epic
# → Created integration/gt-auth-epic from origin/main
# → Stored branch name in epic metadata
```

这会推送一个新分支到 origin 并在 Epic 上记录其名称。

### 3. Sling 工作

正常将子项分配给 Polecat：

```bash
gt sling gt-auth-tokens gastown
gt sling gt-auth-sessions gastown
```

当 issue 是拥有 Integration Branch 的 Epic 的子项时，Polecat 会自动检测 Integration Branch。无需手动指定目标。

### 4. MR 合并到 Integration Branch

当 Polecat 运行 `gt done` 或 `gt mq submit` 时，自动检测生效：

```
gt done
  → 检测到父 Epic gt-auth-epic
  → 找到 integration/gt-auth-epic 分支
  → 提交 MR 目标为 integration/gt-auth-epic（而非 main）
```

Refinery 处理这些 MR 并将它们合并到 Integration Branch。

### 5. 完成时着陆

当所有子项关闭且所有 MR 合并后：

```bash
gt mq integration land gt-auth-epic
# → Verified all MRs merged
# → Merged integration/gt-auth-epic → base branch (--no-ff)
# → Tests passed
# → Pushed to origin
# → Deleted integration/gt-auth-epic
# → Closed epic gt-auth-epic
```

## 自动检测

Integration Branch 无需手动指定目标即可工作。三个系统自动检测它们：

| 系统 | 作用 | 配置门控 |
|------|------|---------|
| `gt done` / `gt mq submit` | 将 MR 目标指向 Integration Branch 而非 main | `integration_branch_refinery_enabled` |
| Polecat 生成 | 从 Integration Branch 源出 worktree | `integration_branch_polecat_enabled` |
| Refinery 巡逻 | 检查 Integration Branch 是否准备着陆 | `integration_branch_auto_land` |

### 检测算法

当 `gt done` 或 `gt mq submit` 运行时：

| 步骤 | 动作 | 结果 |
|------|------|------|
| 1 | 加载配置，检查 `integration_branch_refinery_enabled` | 如果为 false，跳过检测 |
| 2 | 从分支名获取当前 issue ID | 例如 `gt-auth-tokens` |
| 3 | 遍历父链（最多 10 层） | 找到祖先 Epic |
| 4 | 对每个 Epic：从元数据读取 `integration_branch:` | 获取存储的分支名 |
| 5 | 回退：从模板生成名称 | 例如 `integration/{title}` |
| 6 | 检查分支是否存在（本地，然后远程） | 验证其真实性 |
| 7 | 如果找到，将 MR 目标指向该分支 | 而非 main |

`gt mq submit` 上的 `--epic` 标志绕过自动检测，使用配置的模板解析目标分支（默认为 `integration/{epic}`）。

## 分支命名

### 模板变量

| 变量 | 说明 | 示例 |
|------|------|------|
| `{epic}` | 完整 Epic ID | `gt-auth-epic` |
| `{prefix}` | Epic 前缀（第一个连字符之前） | `gt` |
| `{user}` | 来自 `git config user.name` | `klauern` |

### 优先级

| 优先级 | 来源 | 示例 |
|--------|------|------|
| 1（最高） | create 时的 `--branch` 标志 | `--branch "feat/{epic}"` |
| 2 | 配置中的 `integration_branch_template` | `"{user}/{epic}"` |
| 3（最低） | 默认 | `"integration/{title}"` |

### 模板变量

| 变量 | 说明 | 示例 |
|------|------|------|
| `{title}` | 经过清理的 Epic 标题（小写、连字符、最多 60 字符） | `add-user-authentication` |
| `{epic}` | 完整 Epic ID | `RA-123` |
| `{prefix}` | Epic 前缀，第一个连字符之前 | `RA` |
| `{user}` | Git user.name | `klauern` |

### 示例

```bash
# 默认模板（使用 Epic 标题）
gt mq integration create gt-auth-epic
# → integration/add-user-authentication  （来自 Epic 标题）

# 配置中的自定义模板: "{user}/{prefix}/{epic}"
gt mq integration create RA-123
# → klauern/RA/RA-123

# 使用 --branch 标志覆盖
gt mq integration create RA-123 --branch "feature/{epic}"
# → feature/RA-123
```

创建的实际分支名存储在 Epic 的元数据中，所以无论使用哪个模板，自动检测总能找到正确的分支。

如果两个 Epic 生成了相同的分支名（标题相同），会自动追加来自 Epic ID 的数字后缀（例如 `integration/add-auth-456`）。

## 命令

### `gt mq integration create <epic-id>`

为 Epic 创建 Integration Branch。

```bash
gt mq integration create <epic-id> [flags]
```

**标志：**

| 标志 | 说明 | 默认值 |
|------|------|--------|
| `--branch` | 覆盖分支名模板 | 配置的模板或 `integration/{title}` |
| `--base-branch` | 从此分支创建而非 Rig 的默认分支（也设置 `land` 合并回的目标） | `origin/<default_branch>` |

**执行步骤：**

1. 验证 Epic 存在
2. 从模板生成分支名（展开变量）
3. 验证分支名（git 安全字符）
4. 从基础分支创建本地分支
5. 推送到 origin
6. 在 Epic 元数据中存储分支名和基础分支

**错误情况：**

- Epic 未找到
- 分支已存在
- 生成的分支名中有无效字符

### `gt mq integration status <epic-id>`

显示 Epic 的 Integration Branch 状态。

```bash
gt mq integration status <epic-id> [flags]
```

**标志：**

| 标志 | 说明 |
|------|------|
| `--json` | 以 JSON 输出 |

**输出包含：**

- 分支名和创建日期
- 领先于 main 的提交数
- 已合并的 MR（已关闭、目标为 Integration Branch）
- 待处理的 MR（开放、目标为 Integration Branch）
- 子 issue 进度（已关闭 / 总数）
- 是否准备着陆
- 自动着陆配置

**准备着陆的条件**（全部必须为真）：

1. Integration Branch 有领先于 main 的提交
2. Epic 有子项
3. 所有子项都已关闭
4. 没有待处理的 MR（所有提交的工作都已合并）

### `gt mq integration land <epic-id>`

将 Epic 的 Integration Branch 合并回其基础分支。

```bash
gt mq integration land <epic-id> [flags]
```

**标志：**

| 标志 | 说明 | 默认值 |
|------|------|--------|
| `--force` | 即使仍有 MR 开放也着陆 | `false` |
| `--skip-tests` | 合并后跳过测试运行 | `false` |
| `--dry-run` | 仅预览，不做变更 | `false` |

**执行步骤：**

1. 验证 Epic 存在且有 Integration Branch
2. 从 Epic 元数据读取基础分支（如未存储则默认为 Rig 的 `default_branch`）
3. 检查目标为 Integration Branch 的所有 MR 已合并
4. 获取最新引用并检查幂等性（如果已合并，跳到清理）
5. 获取文件锁（防止并发着陆竞争）
6. 创建临时 worktree（避免干扰运行中的 Agent）
7. 使用 `--no-ff` 将 Integration Branch 合并到基础分支
8. 运行测试（除非 `--skip-tests`）
9. 验证合并带来了变更（防止空合并）
10. 推送到 origin
11. 删除 Integration Branch（本地和远程）
12. 关闭 Epic

**幂等重试：** 如果 land 在推送后但在清理（分支删除/Epic 关闭）前崩溃，重新运行相同命令是安全的。幂等性检查会检测到 Integration Branch 已经是目标分支的祖先，直接跳到清理。

**错误情况：**

- Epic 没有 Integration Branch
- 存在待处理的 MR（使用 `--force` 覆盖）
- 测试失败
- 空合并（没有变更可着陆）

## 配置

### 默认分支

Rig 的 `default_branch`（在 `config.json` 中设置，在 `gt rig add` 时自动检测）控制着没有 Integration Branch 活跃时工作合并到哪里。它也是创建 Integration Branch 时的默认基础分支。如果你的项目使用 `develop` 或 `master` 而非 `main`，在 Rig 配置中设置一次，整个流水线都会遵循：

```json
{
  "type": "rig",
  "name": "myproject",
  "default_branch": "develop"
}
```

### Integration Branch 设置

所有 Integration Branch 字段位于 Rig 设置（`settings/config.json`）的 `merge_queue` 下：

```json
{
  "merge_queue": {
    "enabled": true,
    "integration_branch_polecat_enabled": true,
    "integration_branch_refinery_enabled": true,
    "integration_branch_template": "integration/{title}",
    "integration_branch_auto_land": false
  }
}
```

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `integration_branch_polecat_enabled` | `*bool` | `true` | Polecat 自动从 Integration Branch 源出 worktree |
| `integration_branch_refinery_enabled` | `*bool` | `true` | `gt mq submit` 和 `gt done` 自动检测 Integration Branch 作为 MR 目标 |
| `integration_branch_template` | `string` | `"integration/{title}"` | 分支名模板（支持 `{title}`、`{epic}`、`{prefix}`、`{user}`） |
| `integration_branch_auto_land` | `*bool` | `false` | Refinery 巡逻在所有子项关闭时自动着陆 |

**注意：** `*bool` 字段使用指针语义——`null`/省略意味着"使用默认值"（polecat/refinery enabled 为 true，auto-land 为 false）。显式设为 `false` 以禁用。

## 自动着陆

当 `integration_branch_auto_land` 为 `true` 时，Refinery 巡逻会自动着陆准备就绪的 Integration Branch。

### 工作原理

在每个巡逻周期中，Refinery：

1. 列出所有开放的 Epic：`bd list --type=epic --status=open`
2. 检查每个 Epic 的 Integration Branch：`gt mq integration status <epic-id>`
3. 如果 `ready_to_land: true`：执行 `gt mq integration land <epic-id>`
4. 如果未就绪：跳过（Epic 工作未完成）

### 自动着陆条件

两个配置门控都必须为 true：

- `integration_branch_refinery_enabled: true`（Integration 功能已开启）
- `integration_branch_auto_land: true`（自动着陆已开启）

任一为 false，巡逻步骤就会提前退出。

### 何时启用

| 场景 | 建议 |
|------|------|
| 可信的 CI，不需要人工审核 | 启用自动着陆 |
| 着陆前需要人工审批 | 保持禁用（默认），手动着陆 |
| 两者混合 | 保持禁用，使用 `gt mq integration land` 手动控制 |

## 安全防护

Integration Branch 着陆受三层防护保护：

### 第一层：Formula 和角色指令

Refinery Formula 和角色模板明确禁止通过原始 git 命令着陆 Integration Branch。只有 `gt mq integration land` 是授权的。

### 第二层：Pre-Push Hook

`.githooks/pre-push` Hook 检测对默认分支的推送是否引入了 Integration Branch 内容。它使用基于祖先关系的检测：如果任何 `origin/integration/*` 分支尖端从推送的提交中变为新可达的，则推送会被阻止，除非设置了 `GT_INTEGRATION_LAND=1`。

默认分支通过 `refs/remotes/origin/HEAD` 动态检测（回退：`main`），因此无论 Rig 的分支命名如何都能工作。

这能捕获所有合并风格：`--no-ff`、`--ff-only`、默认合并和 rebase。只有 cherry-pick（产生新的 SHA）无法被检测。

**范围**：此检查匹配 `integration/` 前缀下的分支（默认模板）。产生 `integration/` 之外分支的自定义模板不受此 Hook 覆盖——第一层（Formula 语言）是这些情况下的防护。

**要求**：必须配置 `core.hooksPath` 才能使 Hook 生效。新 Rig 会自动获得。已有 Rig：运行 `gt doctor --fix`。

### 第三层：授权代码路径

`gt mq integration land` 命令使用 `PushWithEnv()` 设置 `GT_INTEGRATION_LAND=1`，允许通过 Hook 推送。来自任何 Agent 或用户的原始 `git push` 不会设置此变量，将被阻止。手动设置环境变量是可能的，但不是受支持的工作流——该变量是基于策略的信任边界，而非基于能力的安全机制。

### 为什么需要三层？

| 层 | 类型 | 强度 | 局限 |
|---|------|------|------|
| Formula/Role | 软 | 覆盖所有分支模式 | AI Agent 可以忽略指令 |
| Pre-push Hook | 硬 | 在 git 边界阻止所有合并风格 | 仅匹配 `integration/*` 前缀；环境变量是基于策略的 |
| 代码路径 | 硬 | Land 命令设置绕过环境变量 | 需要 Hook 处于活跃状态 |

各层互补。Formula 覆盖自定义模板；Hook 为默认模板提供硬性执行（通过祖先检测捕获合并、快进和 rebase）；代码路径确保 CLI 命令能绕过 Hook。

## 构建流水线配置

Integration Branch 与不同的项目工具链配合使用。Rig 的构建流水线命令会自动注入到 polecat-work、refinery-patrol 和 sync-workspace Formula 中，使 Agent 知道如何为每个项目验证工作。

### 五命令流水线

命令按此顺序运行（任何一个为空 = 跳过）：

1. **setup** — 安装依赖（例如 `pnpm install`）
2. **typecheck** — 静态类型检查（例如 `tsc --noEmit`）
3. **lint** — 代码风格和质量（例如 `eslint .`）
4. **test** — 运行测试套件（例如 `go test ./...`）
5. **build** — 编译/打包（例如 `go build ./...`）

### 示例配置

**Go 项目**（默认所有命令为空——按 Rig 配置）：
```json
{
  "merge_queue": {
    "test_command": "go test ./...",
    "lint_command": "golangci-lint run ./...",
    "build_command": "go build ./..."
  }
}
```

**TypeScript 项目：**
```json
{
  "merge_queue": {
    "setup_command": "pnpm install",
    "typecheck_command": "tsc --noEmit",
    "lint_command": "eslint .",
    "test_command": "pnpm test:unit",
    "build_command": "pnpm build"
  }
}
```

### 命令如何流入 Formula

命令从 `<rig>/settings/config.json` 自动注入到 Formula 变量中：

- **Refinery 巡逻**：`buildRefineryPatrolVars()` 在 `gt prime` 期间读取 Rig 配置
- **Polecat 工作/同步**：`loadRigCommandVars()` 在 `gt sling` 期间读取 Rig 配置

用户提供的 `--var` 标志在 `gt sling` 上会覆盖 Rig 配置值。

### 空 = 跳过

任何留空（或未配置）的命令都会被 Formula 静默跳过。这意味着 Go Rig 不需要 `setup_command` 或 `typecheck_command`，TypeScript Rig 可以添加全部五个而不影响 Go Rig。

在 Integration Branch 上工作的 Polecat 自动继承 Rig 的构建流水线——无需按分支配置。

## 反模式

### 在工作开始后创建 Integration Branch

**错误做法：** 先 Sling 子项，然后稍后创建 Integration Branch。

在 Integration Branch 存在之前 Sling 的子项会以 main 为目标。它们的 MR 不会流入 Integration Branch。先创建 Integration Branch，再 Sling 任何子项工作。

### 手动指定 Integration Branch

**错误做法：** 在 `gt mq submit` 上使用 `--branch integration/gt-epic`。

自动检测会处理这个。如果你发现自己在手动指定，请检查：
- Integration Branch 是否真的存在
- `integration_branch_refinery_enabled` 不是 `false`
- 该 issue 是 Epic 的子项（或后代）

### 着陆不完整的 Epic

**错误做法：** 使用 `--force` 在子项仍然开放时着陆。

这违背了目的。Integration Branch 的存在是为了让工作一起着陆。如果需要提前着陆，先关闭或移除未完成的子项。

## 另见

- [Polecat Lifecycle](polecat-lifecycle.md) — Polecat 如何向合并队列提交
- [Reference](../reference.md) — 包含 MQ 命令的完整 CLI 参考