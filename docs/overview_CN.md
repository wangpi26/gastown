# 理解 Gas Town

本文档提供 Gas Town 架构的概念概述，重点介绍角色分类法以及不同代理之间的交互方式。

## Gas Town 为什么存在

随着 AI 代理成为工程工作流的核心，团队面临新的挑战：

- **问责性**：谁做了什么？哪个代理引入了这个 Bug？
- **质量**：哪些代理可靠？哪些需要调优？
- **效率**：如何将工作路由到合适的代理？
- **规模**：如何跨仓库和团队协调代理？

Gas Town 是一个编排层，将 AI 代理工作视为结构化数据。每个操作都有归属。每个代理都有工作记录。每项工作都有来源。完整的设计理念参见 [Why These Features](why-these-features.md)，术语参见 [Glossary](glossary.md)。

## 角色分类法

Gas Town 有多种代理类型，每种具有不同的职责和生命周期。

### 基础设施角色

这些角色管理 Gas Town 系统本身：

| 角色 | 描述 | 生命周期 |
|------|------|----------|
| **Mayor** | 全局协调器，位于 mayor/ | 单例，持久 |
| **Deacon** | 后台监督守护进程（[看门狗链](design/watchdog-chain.md)） | 单例，持久 |
| **Witness** | 每个 Rig 的 Polecat 生命周期管理器 | 每个 Rig 一个，持久 |
| **Refinery** | 每个 Rig 的合并队列处理器 | 每个 Rig 一个，持久 |

### 工作者角色

这些角色执行实际的项目工作：

| 角色 | 描述 | 生命周期 |
|------|------|----------|
| **Polecat** | 具有持久身份和临时会话的工作者 | Witness 管理（[详情](concepts/polecat-lifecycle.md)） |
| **Crew** | 拥有自己克隆的持久工作者 | 长寿命，用户管理 |
| **Dog** | Deacon 的基础设施任务助手 | 持久身份，Deacon 管理 |

## Convoy：跟踪工作

**Convoy** 是你在 Gas Town 中跟踪批量工作的方式。当你启动工作时 — 即使是单个问题 — 创建一个 Convoy 来跟踪它。

```bash
# 创建一个跟踪若干问题的 Convoy
gt convoy create "Feature X" gt-abc gt-def --notify overseer

# 检查进度
gt convoy status hq-cv-abc

# 活跃 Convoy 仪表板
gt convoy list
```

**为什么 Convoy 重要：**
- "进行中"工作的统一视图
- 跨 Rig 跟踪（Convoy 在 hq-* 中，问题在 gt-*、bd-* 中）
- 工作完成时自动通知
- 已完成工作的历史记录（`gt convoy list --all`）

"蜂群"是当前被分配给 Convoy 问题的工作者集合。当问题关闭时，Convoy 就着陆了。详情参见 [Convoys](concepts/convoy.md)。

## Crew vs Polecat

两者都做项目工作，但有关键区别：

| 方面 | Crew | Polecat |
|------|------|---------|
| **生命周期** | 持久（用户控制） | 临时（Witness 控制） |
| **监控** | 无 | Witness 监视、Nudge、回收 |
| **工作分配** | 人工指导或自分配 | 通过 `gt sling` 派发 |
| **Git 状态** | 直接推送到 main | 在分支上工作，Refinery 合并 |
| **清理** | 手动 | 完成后自动 |
| **身份** | `<rig>/crew/<name>` | `<rig>/polecats/<name>` |

**何时使用 Crew**：
- 探索性工作
- 长期项目
- 需要人工判断的工作
- 你希望直接控制的任务

**何时使用 Polecat**：
- 离散的、明确定义的任务
- 批量工作（通过 Convoy 跟踪）
- 可并行化的工作
- 受益于监督的工作

## Dog vs Crew

**Dog 不是工作者**。这是一个常见的误解。

| 方面 | Dog | Crew |
|------|-----|------|
| **归属** | Deacon | 人类 |
| **用途** | 基础设施任务 | 项目工作 |
| **范围** | 窄、专注的工具 | 通用 |
| **生命周期** | 很短（单次任务） | 长寿命 |
| **示例** | Boot（分诊 Deacon 健康） | Joe（修复 Bug、添加功能） |

Dog 是 Deacon 处理系统级任务的助手：
- **Boot**：在守护进程 tick 上分诊 Deacon 健康
- 未来的 Dog 可能处理：日志轮转、健康检查等

如果你需要在另一个 Rig 中工作，使用 **worktree**，而不是 Dog。

## 跨 Rig 工作模式

当 Crew 成员需要在另一个 Rig 上工作时：

### 方案 1：Worktree（首选）

在目标 Rig 中创建一个 worktree：

```bash
# gastown/crew/joe 需要修复一个 beads bug
gt worktree beads
# 创建 ~/gt/beads/crew/gastown-joe/
# 身份保留：BD_ACTOR = gastown/crew/joe
```

目录结构：
```
~/gt/beads/crew/gastown-joe/     # gastown 的 joe 在 beads 上工作
~/gt/gastown/crew/beads-wolf/    # beads 的 wolf 在 gastown 上工作
```

### 方案 2：派发到本地工作者

对于应该由目标 Rig 拥有的工作：

```bash
# 在目标 Rig 中创建问题
bd create --prefix beads "Fix authentication bug"

# 创建 Convoy 并 Sling 到目标 Rig
gt convoy create "Auth fix" bd-xyz
gt sling bd-xyz beads
```

### 何时使用哪种方式

| 场景 | 方法 |
|------|------|
| 需要快速修复某物 | Worktree |
| 工作应出现在你的 CV 中 | Worktree |
| 工作应由目标 Rig 团队完成 | 派发 |
| 基础设施/系统任务 | 让 Deacon 处理 |

## 目录结构

Town 根目录（`~/gt/`）包含基础设施目录（`mayor/`、`deacon/`）和按项目划分的 Rig。每个 Rig 持有一个裸仓库（`.repo.git/`）、一个权威的 Beads 数据库（`mayor/rig/.beads/`）和代理目录（`witness/`、`refinery/`、`crew/`、`polecats/`）。

> 完整目录树参见 [architecture.md](design/architecture.md)。

## 身份和归属

所有工作归属于执行它的行动者：

```
Git 提交：      Author: gastown/crew/joe <owner@example.com>
Beads 问题：     created_by: gastown/crew/joe
事件：           actor: gastown/crew/joe
```

跨 Rig 工作时身份也保持不变：
- `gastown/crew/joe` 在 `~/gt/beads/crew/gastown-joe/` 中工作
- 提交仍归属于 `gastown/crew/joe`
- 工作出现在 joe 的 CV 中，而非 beads Rig 的工作者

## 推进原则

所有 Gas Town 代理遵循同一核心原则：

> **如果你发现 Hook 上有工作，你就执行它。**

无论角色如何都适用。Hook 就是你的任务。立即执行，无需等待确认。Gas Town 是一台蒸汽机 — 代理是活塞。

## 模型评估和 A/B 测试

Gas Town 的归属系统通过跟踪每个代理的完成时间、质量信号和修订次数来实现客观的模型比较。在类似任务上部署不同模型，使用 `bd stats` 比较结果。

关于工作历史和基于能力的路由详情，参见 [Why These Features](why-these-features.md)。

## 常见错误

1. **用 Dog 做用户工作**：Dog 是 Deacon 的基础设施。使用 Crew 或 Polecat。
2. **混淆 Crew 和 Polecat**：Crew 是持久的、人工管理的。Polecat 是临时的、Witness 管理的。
3. **在错误的目录中工作**：Gas Town 使用 cwd 进行身份检测。留在你的主目录中。
4. **工作已 Hook 后等待确认**：Hook 就是你的任务。立即执行。
5. **该派发时创建 Worktree**：如果工作应由目标 Rig 拥有，改为派发。