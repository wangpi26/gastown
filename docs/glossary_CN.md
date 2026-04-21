# Gas Town 术语表

Gas Town 是一个代理式开发环境，使用 `gt` 和 `bd`（Beads）二进制文件，在 git 管理的目录中配合 tmux 协调管理多个 Claude Code 实例。

## 核心原则

### MEOW（Molecular Expression of Work）
将大型目标分解为给代理的详细指令。由 Beads、Epic、Formula 和 Molecule 支持。MEOW 确保工作被分解为可跟踪的、原子化的单元，代理可以自主执行。

### GUPP（Gas Town Universal Propulsion Principle）
"如果你的 Hook 上有工作，你必须执行它。"这一原则确保代理自主处理可用工作而无需等待外部输入。GUPP 是自主运行的脉搏。

### NDI（Nondeterministic Idempotence）
通过编排可能不可靠的流程来确保有用结果的总体目标。持久化的 Bead 和监督代理（Witness、Deacon）保证工作流最终完成，即使单个操作可能失败或产生不同结果。

## 环境

### Town
管理总部（如 `~/gt/`）。Town 协调所有 Rig 中的工作者，并容纳 Town 级代理如 Mayor 和 Deacon。

### Rig
Gas Town 管理下的项目专用 Git 仓库。每个 Rig 有自己的 Polecat、Refinery、Witness 和 Crew 成员。Rig 是实际开发工作发生的地方。

## Town 级角色

### Mayor
幕僚长代理，负责发起 Convoy、协调工作分配和向用户通知重要事件。Mayor 在 Town 级运行，拥有跨所有 Rig 的可见性。

### Deacon
运行持续巡逻周期的守护信标。Deacon 确保工作者活跃、监控系统健康，并在代理无响应时触发恢复。可将 Deacon 视为系统的看门狗。

### Dogs
Deacon 的维护代理团队，处理后台任务如清理、健康检查和系统维护。

### Boot（Dog）
一个特殊的 Dog，每 5 分钟检查一次 Deacon，确保看门狗本身仍在运作。这创建了一条问责链。

## Rig 级角色

### Polecat
具有持久身份但临时会话的工作者代理。每个 Polecat 有一个永久的代理 Bead、CV 链和工作历史，在多次任务中累积。会话和沙箱是临时的 — 为特定任务生成、完成后清理 — 但身份持久存在。它们在隔离的 git worktree 中工作以避免冲突。

### Refinery
管理 Rig 的合并队列。Refinery 智能地合并来自 Polecat 的变更，处理冲突并确保代码质量，然后变更才能到达主分支。

### Witness
监督 Rig 内 Polecat 和 Refinery 的巡逻代理。Witness 监控进度、检测卡住的代理，并可以触发恢复操作。

### Crew
用于持久协作的长寿命命名代理。与临时的 Polecat 不同，Crew 成员在会话间保持上下文，适合持续的工作关系。

## 工作单元

### Bead
存储在 Dolt 中的 Git 支持的原子工作单元。Bead 是 Gas Town 中工作跟踪的基本单元。它们可以表示问题、任务、Epic 或任何可跟踪的工作项。

### Formula
基于 TOML 的工作流源模板。Formula 定义了常见操作的可复用模式，如巡逻周期、代码审查或部署。

### Protomolecule
用于实例化 Molecule 的模板类。Protomolecule 定义了工作流的结构和步骤，而不与特定工作项绑定。

### Molecule
持久的链式 Bead 工作流。Molecule 表示多步骤流程，其中每个步骤作为 Bead 被跟踪。它们在代理重启后依然存活，确保复杂工作流完成。

### Wisp
运行后销毁的临时 Bead。Wisp 是用于不需要永久跟踪的瞬态操作的轻量级工作项。

### Hook
每个代理的特殊固定 Bead。Hook 是代理的主要工作队列 — 当你的 Hook 上出现工作时，GUPP 规定你必须执行它。

## 工作流命令

### Convoy
包裹相关 Bead 的主要工作单。Convoy 将相关任务组合在一起，可以分配给多个工作者。使用 `gt convoy create` 创建。

### Slinging
通过 `gt sling` 向代理分配工作。当你将工作 Sling 到 Polecat 或 Crew 成员时，你是在将工作放到他们的 Hook 上等待执行。

### Nudging
代理间通过 `gt nudge` 进行实时消息传递。Nudge 允许即时通信，无需经过邮件系统。

### Handoff
通过 `/handoff` 刷新代理会话。当上下文满了或代理需要重新开始时，Handoff 将工作状态转移到新会话。

### Seance
通过 `gt seance` 与之前的会话通信。允许代理查询前任的上下文和早期工作的决策。

### Patrol
维持系统脉搏的临时循环。巡逻代理（Deacon、Witness）持续循环健康检查并根据需要触发操作。

---

*本术语表由 [Clay Shirky](https://github.com/cshirky) 在 [Issue #80](https://github.com/steveyegge/gastown/issues/80) 中贡献。*