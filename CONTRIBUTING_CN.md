# 参与 Gas Town 贡献

感谢你对贡献的关注！Gas Town 是实验性软件，我们欢迎任何有助于探索这些想法的贡献。

## 快速上手

1. Fork 本仓库
2. 克隆你的 Fork
3. 安装前置依赖（参见 README.md）
4. 构建和测试：`go build -o gt ./cmd/gt && go test ./...`

## 开发工作流

受信任的贡献者使用直接合入 main 的工作流。外部贡献者：

1. 从 `main` 创建功能分支
2. 进行修改
3. 确保测试通过：`go test ./...`
4. 提交 Pull Request

### PR 分支命名

**绝不要从你 Fork 的 `main` 分支创建 PR。** 始终为每个 PR 创建专用分支：

```bash
# 正确 - 每个 PR 使用专用分支
git checkout -b fix/deacon-startup upstream/main
git checkout -b feat/auto-seance upstream/main

# 错误 - 从 main 发起 PR 会积累无关的提交
git checkout main  # 不要从这里发起 PR！
```

为什么这很重要：
- 从 `main` 发起的 PR 会包含你 Fork 中推送的所有提交
- 多个贡献者推送到同一个 Fork 的 `main` 会造成混乱
- 审查者无法辨别哪些提交属于哪个 PR
- 你无法同时开启多个 PR

分支命名约定：
- `fix/*` - Bug 修复
- `feat/*` - 新功能
- `refactor/*` - 代码重构
- `docs/*` - 仅文档修改

## 代码风格

- 遵循标准 Go 规范（`gofmt`、`go vet`）
- 保持函数聚焦且精简
- 为非显而易见的逻辑添加注释
- 为新功能编写测试

## 设计哲学

Gas Town 遵循两个核心原则，它们塑造着每一项贡献。理解这些原则将为你（和审查者）节省时间。

### 零框架认知（ZFC）

**Go 提供传输。Agent 提供认知。**

Gas Town 的 Go 代码处理底层管道：tmux 会话、消息投递、hooks、
nudge、文件传输和可观测性原语（如 `bd show --json`）。
所有推理、判断和决策都通过 molecule formula 和角色模板在 AI Agent 中完成。

这意味着：
- **Go 中不硬编码阈值。** 不要写 `if age > 5*time.Minute`
  来判断 Agent 是否卡住。将 age 暴露为数据，让 Agent 自己决定。
- **Go 中不写启发式逻辑。** 不要写模式匹配 Agent 行为的检测逻辑。
  给 Agent 提供观察工具，让它们自己推理。
- **Formula 优于子命令。** 如果功能是"检测 X 然后执行 Y"，这
  很可能是一个 molecule 步骤，而不是新的 `gt` 子命令。

**判断标准：** 在添加 Go 代码之前，问自己——"我添加的是传输还是
认知？" 如果答案是认知，那应该是一个 molecule 步骤或
formula 指令。

完整论述请参见
[Zero Framework Cognition](https://steve-yegge.medium.com/zero-framework-cognition-a-way-to-build-resilient-ai-applications-56b090ed3e69)。

### 苦涩教训对齐

Gas Town 押注于模型变得越来越智能，而非手工打造的启发式逻辑变得越来越精巧。如果 AI Agent 能观察数据并据此推理，我们就暴露数据（传输）而非编码推理（认知）。今天笨拙的启发式就是明天的技术债——但干净的可观测性原语会历久弥新。

**示例：**

| 好的做法（传输） | 不好的做法（Go 中的认知） |
|---|---|
| `gt nudge <session> "message"` | Go 代码决定*何时* nudge |
| `bd show --json` 暴露步骤状态 | Go 代码决定步骤状态*意味着什么* |
| `tmux has-session` 检查存活状态 | Go 代码硬编码"N 分钟后视为卡住" |

## 贡献内容

好的入门贡献：
- 有清晰重现步骤的 Bug 修复
- 文档改进
- 未测试代码路径的测试覆盖
- 小而聚焦的功能

对于较大的变更，请先创建 Issue 讨论方案。

## 提交消息

- 使用现在时态（"Add feature" 而非 "Added feature"）
- 首行保持在 72 个字符以内
- 适用时引用 Issue：`Fix timeout bug (gt-xxx)`

## 测试

提交前运行完整测试套件：

```bash
go test ./...
```

针对特定包：

```bash
go test ./internal/wisp/...
go test ./cmd/gt/...
```

### 集成测试守卫

集成测试（标记为 `//go:build integration`）需要外部资源，
在某些环境中可能不可用。使用 `internal/testutil` 中的辅助函数，
在缺少前置条件时优雅地跳过：

| 辅助函数 | 使用场景 |
|--------|-------------|
| `testutil.RequireDoltContainer(t)` | 测试需要运行中的 Dolt SQL 服务器（启动 Docker 容器） |
| `testutil.StartIsolatedDoltContainer(t)` | 测试需要独立的 Dolt 实例（每个测试一个容器） |
| `testutil.RequireTownEnv(t)` | 测试需要活跃的 Gas Town 工作空间（检查 `workspace.FindFromCwd` + `rigs.json`）；返回根路径 |

**`requireDoltServer`**（位于 `internal/cmd`）是 `testutil.RequireDoltContainer` 的本地封装，
被 `cmd` 包的集成测试使用。

**何时使用哪种守卫：**

- 连接 Dolt 的测试（创建数据库、运行 SQL） →
  `RequireDoltContainer` 或 `StartIsolatedDoltContainer`
- 需要真实 Gas Town 目录树的测试（通过 shell 调用 `gt`/`bd` 并
  依赖工作空间检测） → `RequireTownEnv`
- 通过 `t.TempDir()` 创建自己的临时 town 的测试 → 不需要守卫
  （它们是自包含的）

对于有大量依赖 Dolt 的测试的包，建议在 `TestMain` 函数中添加
`testutil.EnsureDoltContainerForTestMain()`，这样包中所有测试
共享一个容器。

## 发布

发布从格式为 `vX.Y.Z` 的标签切出。完整工作流参见 [RELEASING.md](RELEASING.md)。
需要了解的一个安全防护：

- `make check-version-tag` 验证 `internal/cmd/version.go` 中的
  `Version` 常量是否匹配 HEAD 处的标签。发布工作流在
  GoReleaser 之前运行此检查，不匹配时发布失败。防止
  [#3459](https://github.com/steveyegge/gastown/issues/3459) 再次发生。
  在版本号更新后本地运行此命令，可以在推送标签前捕获偏差。

## 有问题？

创建 Issue 提出关于贡献的问题。我们很乐意帮助！