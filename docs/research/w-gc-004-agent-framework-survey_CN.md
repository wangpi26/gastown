# 调研：Agent 编排框架 vs Gas City

**Wanted：** w-gc-004 — 调研现有 Agent 编排框架
**完成者：** tmchow
**日期：** 2026-03-08

## 执行摘要

本调研考察了七个主要的多 Agent 编排框架，并将其角色/任务模型与 Gas City 的声明式方法进行比较。这些框架涵盖了从极简（OpenAI Swarm）到企业级（Microsoft Agent Framework）的范围，每个框架在如何定义、协调和装备 Agent 工具方面都有不同的理念。

Gas Town 的关键差异点是它的**进程模型架构** — 持久的、命名的角色（Mayor、Witness、Polecat 等），具有固定职责，通过 Hook 和 Bead 通信，由 Git 和 Dolt 支持崩溃存活状态。这更接近操作系统的进程模型，而非大多数框架使用的 LLM 对话即控制平面模式。

Gas City是计划中在 Gas Town 之上的声明式层 — 一种角色格式和 Formula 引擎，可以使 Gas Town 的模式可移植和用户可定义。它目前还不作为独立产品存在（这正是 w-gc-001 和 w-gc-003 旨在构建的）。

---

## Gas Town / Gas City：参考架构

在比较外部框架之前，详细了解 Gas Town 的架构很重要，因为目标是识别 Gas City 声明式角色格式可以借鉴的内容。

**仓库：** [github.com/steveyegge/gastown](https://github.com/steveyegge/gastown)（Go，`gt` 二进制）
**配套：** [github.com/steveyegge/beads](https://github.com/steveyegge/beads)（Go，`bd` 二进制）
**当前版本：** v0.6.0

### 角色层级（四层）

Gas Town 跨分层层级定义运营角色。角色以 **Go 模板文件** 实现，位于 `internal/templates/roles/*.md.tmpl`，通过 `gt prime` 命令注入到 Claude Code 会话中。`GT_ROLE` 环境变量决定渲染哪个角色模板。角色检测也通过检查当前工作目录路径工作（例如 `<rig>/witness/rig/` 触发 Witness 角色）。

**基础设施层：**
- **Boot**：处理启动时的初始上下文注入 — 有自己的模板（`boot.md.tmpl`）
- **Deacon**：位于 `~/gt/deacon/` 的中央健康监督者 — "daemon beacon"运行连续的巡逻循环。监控系统健康，确保工作者活跃，触发恢复。早期因 bug 臭名昭著（Yegge 警告说它在 v0.4.0 修复前"巡逻时屠杀所有其他工作者"）
- **Dogs**：Deacon 的辅助 Agent，负责基础设施任务（临时性，不用于用户工作）

**全局协调层：**
- **Mayor**：位于 `~/gt/mayor/` 的 Town 级协调者 — 人类的主要 AI 管家。启动 Convoy，分配工作，跨所有 Rig 协调。Mayor 可以且应当在最快路径是编辑代码时编辑代码。"Gas Town 是蒸汽机，Mayor 是主驱动轴。"

**每个 Rig 的管理层：**
- **Witness**：每个 Rig 的巡逻 Agent — 监督 Polecat 和 Refinery，监控进度，检测卡住的 Agent，触发恢复
- **Refinery**：每个 Rig 的合并队列处理器 — 处理质量控制、合并冲突解决、分支清理

**工作者层：**
- **Polecat**：临时工作者，为单个任务生成后终止。每个都有自己的 **git worktree**（轻量级，共享 bare repo）以实现完全隔离。
- **Crew**：持久辅助 Agent，用于扩展工作 — 长期存在、用户管理，拥有自己的完整 **clone**（而非 worktree）。适合持续的工作关系。

### Agent 身份（三个持久要素）

每个 Agent 拥有：
1. **Role Bead**：定义角色的规则和 priming 指令
2. **Agent Bead**：跨会话重启存活的持久身份 — 构成 CV/信誉账本的基础
3. **Hook**：Bead 支持的队列，工作附加其上

这种身份与会话的分离是关键差异点 — 会话是临时的，但 Agent 的身份和工作状态在 Dolt 中持久。每一次完成都被记录，每一次交接被日志，每个关闭的 Bead 都成为永久能力账本的一部分。

### Priming 如何工作

当会话启动时，`gt prime` 执行多步骤上下文注入：
1. 检查 Agent Hook 上是否有被 Sling 的工作
2. 检测自主模式并调整行为
3. 如果在 Molecule 步骤上工作，输出 Molecule 上下文
4. 输出上一次会话检查点用于崩溃恢复
5. 运行 `bd prime` 输出 beads 工作流上下文
6. 运行 `gt mail check --inject` 注入待处理邮件

### GUPP：Gas Town 通用推进原则

调度公理：**"如果你的 Hook 上有工作，你必须执行它。"**

这确保了：
- Agent 在启动时检查 Hook 并自动恢复工作
- 工作通过 Git 支持的状态跨会话崩溃存活
- 不需要中央调度器 — 拉取式执行模型
- 无需确认，无需提问，无需等待 — 立即执行

### Hook 系统

每个工作者都有一个专用的 **Hook** — 一个 Bead 被固定，工作附加其上。流程：

1. 工作通过 `gt sling <bead-id> <rig>` 分配
2. 工作（一个 Molecule）落在目标 Agent 的 Hook 上
3. GUPP 激活：Agent 检测工作并立即执行
4. 完成后，Hook 被清除，下一个 Molecule 跳到前面

Hook 周围的通信原语：
- **Mail**：用于 Agent 间协调的异步持久化消息
- **Nudge**：通过 tmux 直接注入会话（`gt nudge`）
- **Peek**：不中断的状态检查（`gt peek`）

### Molecule/Formula 栈（工作流原语）

从模板到执行的分层工作流抽象：

| 层级 | 名称 | CLI 命令 | 说明 |
|---|---|---|---|
| 源 | **Formula** | — | `internal/formula/formulas/` 中的 TOML 源文件 — 定义循环、Gate、组合 |
| 编译 | **Protomolecule** | `bd cook` | 编译后的、git 冻结的工作流模板，可以部署 |
| 活跃 | **Molecule (Mol)** | `bd mol pour` | 运行中的工作流实例，在 beads 中追踪，崩溃存活 |
| 临时 | **Wisp** | — | 仅在巡逻循环内存中存在的轻量级工作流 |

示例：`internal/formula/formulas/release.formula.toml` 定义了"标准发布流程"，步骤为：bump-version → run-tests → build → create-tag → publish。

### Gate（异步协调）

阻塞条件，使工作暂停而不阻塞其他任务：
- `gh:run` — 等待 GitHub Actions 完成
- `gh:pr` — 等待 pull request 事件
- `timer` — 等待持续时间经过
- `human` — 等待人工批准
- `mail` — 等待来自另一个 Agent 的消息

### Beads（工作追踪）

Beads 是存储在 **Dolt 数据库**（版本控制的 SQL，具有 Git 语义）中的原子工作项。截至 Beads v0.51.0，Dolt 是唯一后端 — 旧的 SQLite + JSONL 管道已被移除。Dolt 的**单元格级合并**意味着来自多个 Agent 的并发更新可以在列级别自动解决，而非行级别 — 对多 Agent 操作至关重要。

Bead ID 使用格式 `prefix-XXXXX`（例如 `gt-abc12`、`hq-x7k2m`），基于哈希的 ID 防止跨 Agent 和分支的合并冲突。Beads 经历状态转换：`open` → `working` → `done`/`parked`。"Bead"和"issue"可互换使用。

**Bead 类型：**
- **Issue Bead**：带有 ID、描述、状态、受托人、依赖和阻塞者的工作项
- **Agent Bead**：追踪 Agent 状态和 Hook 的身份 Bead — 信誉账本的基础
- **Hook Bead**：作为 Agent 工作队列的特殊固定 Bead
- **Convoy Bead**：将工作项包装为可追踪交付单元的集合

**更高级别的聚合：**
- **Epic**：将 Bead 组织为树的层级集合（例如 `bd-a3f8e9.1`、`bd-a3f8e9.2`）
- **Convoy**：追踪组合目标（如发布）的交付单元
- **Patrol**：用于队列清理和健康检查的循环工作流

### 升级层级

- **Tier 1**（Deacon）：基础设施故障和守护进程问题
- **Tier 2**（Mayor）：跨 Rig 协调和资源冲突
- **Tier 3**（Overseer/人类）：设计决策和人类判断

### 工作区结构

```
~/gt/
├── deacon/          # 基础设施 Agent
├── mayor/           # Town 协调者
├── <rig-name>/      # 每个项目的目录
│   ├── witness/     # Polecat 监督者
│   ├── refinery/    # 合并处理器
│   ├── crew/        # 持久助手
│   ├── polecats/    # 临时工作者
│   └── rig/         # Git 仓库
└── routes.jsonl     # 前缀路由配置
```

### 当前状态（v0.6.0，2026 年 3 月）

Gas Town 处于早期阶段（2026 年 1 月发布）但快速演进 — 1500+ GitHub issue，450+ 贡献者。v0.6.0 添加了 convoy 所有权、基于检查点的崩溃恢复、数据驱动预设注册表的 Agent 工厂、Gemini 和 Copilot CLI 集成、非破坏性 Nudge 投递和子模块支持。社区生态在增长：Kubernetes operator（gastown-operator）、Web GUI（gastown-gui）和 Rust 版 beads。

**Wasteland** 联邦层刚刚上线（PR #1552，2026 年 3 月）— 通过 Dolt 和 DoltHub 将数千个 Gas Town 连接成信任评分的劳动力市场。这正是我们用来追踪此任务的系统。

**Gas City** — 声明式角色格式和 Formula 引擎层 — 是计划中的下一步。目前，角色以 Go 模板定义；Gas City 将使它们可移植、用户可定义和可组合。本调研直接为该设计提供参考。

---

## 外部框架摘要

### 1. AutoGen (Microsoft) → Microsoft Agent Framework

**最新版本：** v0.4.7（正被 Microsoft Agent Framework 取代，RC 2026 年 2 月）
**仓库：** [github.com/microsoft/autogen](https://github.com/microsoft/autogen)

**角色模型：**
- Agent 通过 `AssistantAgent`、`UserProxyAgent`、`CodeExecutorAgent` 等定义
- 每个 Agent 获得系统消息（角色提示）、模型客户端和工具列表
- AutoGen 0.4 使用事件驱动的、基于参与者的架构，具有分层 API：Core（消息原语）和 AgentChat（高级抽象）

**任务模型：**
- 任务是隐式的 — 你向团队传递消息字符串，团队协调 Agent 来解决
- 没有专门的 `Task` 类。对话就是任务
- 终止条件（`MaxMessageTermination`、`TextMentionTermination`）定义任务何时"完成"

**协调：**
- Agent 分组为团队：`RoundRobinGroupChat`、`SelectorGroupChat`、`Swarm`
- `SelectorGroupChat` 使用 LLM 每轮选择下一个发言者
- `Swarm` 模式使用显式的 `HandoffMessage` 进行 Agent 间路由

**最简示例：**
```python
from autogen_agentchat.agents import AssistantAgent
from autogen_agentchat.teams import RoundRobinGroupChat
from autogen_agentchat.conditions import MaxMessageTermination

analyst = AssistantAgent("analyst", model_client=client,
    system_message="You analyze code for bugs and security issues.")
fixer = AssistantAgent("fixer", model_client=client,
    system_message="You fix bugs identified by the analyst.")

team = RoundRobinGroupChat([analyst, fixer],
    termination_condition=MaxMessageTermination(6))
result = await team.run(task="Review and fix auth.py")
```

**工具：** Python 函数包装为 `FunctionTool`，在 Agent 构建时分配。

**核心优势：** 灵活的编排模式，强大的异步支持，AutoGen Studio UI。

**核心局限：** 对话即控制流可能不透明；0.2→0.4 的转换导致了碎片化。现在正在合并到 Microsoft Agent Framework — AutoGen 今后只接收 bug 修复。

**状态：** AutoGen 和 Semantic Kernel 正在合并为 **Microsoft Agent Framework**（GA 目标 2026 Q1）。新框架在 Semantic Kernel 的插件模型之上添加了基于图的工作流 API，将 AutoGen 的多 Agent 模式与 SK 的企业基础结合。

### 2. CrewAI

**最新版本：** v1.10.1（2026 年 3 月 4 日）
**仓库：** [github.com/crewAIInc/crewAI](https://github.com/crewAIInc/crewAI)

**角色模型：**
- **role / goal / backstory** 三元组 — Agent 以职位头衔、目标和叙事背景定义
- 故意直观：镜像你向人类专家交代工作的方式

**任务模型：**
- 显式的 `Task` 类，有 `description`、`expected_output` 和分配的 `agent`
- 任务通过 `context` 参数链式传递 — 一个任务的输出馈入另一个
- 通过 `output_pydantic` 或 `output_json` 支持结构化输出
- `human_input=True` 在继续前暂停以供人类审核

**协调：**
- 流程类型：**Sequential**（有序管道）、**Hierarchical**（管理者 Agent 委派）
- **Flows**（v1.x 新增）：使用 `@start()` 和 `@listen()` 装饰器的事件驱动架构，用于精细控制。补充自主 Crews 模式

**核心优势：** 最低学习曲线。role/goal/backstory 隐喻立即可理解。现在完全独立于 LangChain（从零重写）。

**核心局限：** 与基于图的框架相比控制流有限。Sequential/Hierarchical 是主要模式 — 没有任意分支或循环。层级模式下 token 消耗大。

### 3. LangGraph (LangChain)

**最新版本：** v1.0（稳定，2.0 前无破坏性变更）
**仓库：** [github.com/langchain-ai/langgraph](https://github.com/langchain-ai/langgraph)

**角色模型：**
- Agent/节点是 `StateGraph` 中的 **Python 函数**。每个节点接收状态、执行工作、返回状态更新
- 预构建的 `create_react_agent()` 用于标准工具调用循环
- 子图可以作为节点嵌入，用于层级/多 Agent 设计

**任务模型：**
- 没有显式的任务抽象。图就是任务 — 你用初始状态调用它，它流经节点
- 状态是带有注释 reducer 函数的 `TypedDict`
- 任务隐含在图的拓扑和终止条件中

**协调：**
- **有向图**，具有显式边（普通、条件、循环）
- 条件边：路由函数检查状态并返回下一个节点名
- **循环是一等的** — 原生支持迭代 Agent 循环（ReAct、反思）

**核心优势：** 显式拓扑控制 — 你看到并控制节点如何连接。一流的流式支持、人在回路（`interrupt_before`/`interrupt_after`）和检查点（持久化、时间旅行、故障恢复）。所有框架中最灵活。

**核心局限：** 简单用例需要更多样板。与 LangChain 消息 Schema 紧耦合。所有节点共享状态类型可能有限制。

### 4. OpenAI Agents SDK（Swarm 的继任者）

**最新版本：** 生产就绪 SDK（2026 年取代 Swarm）
**仓库：** [github.com/openai/openai-agents-python](https://openai.github.io/openai-agents-python/)

**角色模型：**
- Agent 以 `name`、`instructions`（系统提示）和 `tools`/`handoffs` 定义
- 最初是 Swarm（约 300 LOC，教学性）→ 演化为带生产功能的 Agents SDK

**任务模型：**
- 没有显式的任务对象。你调用 `Runner.run(agent, "your request")`，Agent 处理
- 向其他 Agent 的 Handoff 表示为 LLM 的工具（例如 `transfer_to_refund_agent`）
- Runner 处理执行 Agent、handoff 和工具调用

**协调：**
- **Handoff** 是核心原语 — 列在 Agent 的 `handoffs` 参数中
- 没有中央编排器、规划器或 DAG。只是 Agent 之间互相移交
- Agents SDK 添加了：guardrails、人在回路、带仪表板查看器的追踪、会话管理

**核心优势：** 极致的简洁和透明。最小抽象。Agents SDK 在保持 handoff 模型的同时添加了生产级追踪和 guardrails。

**核心局限：** 没有 handoff 能表达的复杂路由、分支或循环。适合线性或树形工作流。

### 5. Claude Agent SDK (Anthropic)

**最新版本：** 从 Claude Code SDK 更名（2026 年 3 月）
**仓库：** [github.com/anthropics/claude-agent-sdk-python](https://github.com/anthropics/claude-agent-sdk-python)

**角色模型：**
- Agent 以模型、instructions（系统提示）和 tools 定义
- 强调 **单 Agent + 工具** 模式 — 一个强大 Agent 配合丰富工具访问
- 多 Agent 通过层级委派：Agent 将子 Agent 作为工具调用

**任务模型：**
- 单 Agent 循环：你给 Agent 一个 prompt，它推理并调用工具直到完成
- 子 Agent 在父 Agent 循环中作为工具被调用
- 没有单独的任务抽象 — prompt 就是任务

**协调：**
- Agent 循环：Claude 迭代地推理 → 调用工具 → 处理结果
- SDK 管理对话轮次循环、工具执行和结果注入
- 子 Agent 委派用于多 Agent 场景

**核心优势：** 与 Claude 原生能力（工具使用、扩展思维）深度集成。基于 MCP 的工具系统可组合且基于标准。内置安全 guardrails 和人在回路。

**核心局限：** 较不适合 swarm 风格的多 Agent 模式。单 Agent + 委派模型强大但不同于对等 Agent 协调。

### 6. Google ADK (Agent Development Kit)

**最新版本：** 2026 年 2 月 26 日更新。Python + TypeScript 支持。
**仓库：** [github.com/google/adk-python](https://github.com/google/adk-python)

**角色模型：**
- Agent 以 `instructions`、`tools` 和 `sub_agents` 定义 — 支持**树状层级**
- 代码优先理念："Agent 开发应该像软件开发"

**任务模型：**
- 你向根 Agent 发送消息；它按需委派给子 Agent
- 内置组合模式处理编排：`SequentialAgent`、`LoopAgent`、`ParallelAgent`
- 会话状态跨交互持久

**协调：**
- Agent-as-tool 模式：父 Agent 委派给子 Agent
- 内置模式：`SequentialAgent`（管道）、`LoopAgent`（迭代）、`ParallelAgent`
- 内置会话管理和状态持久化

**核心优势：** 紧密的 Google Cloud / Gemini 集成。基于 Web 的开发者 UI 用于测试/调试。增长的第三方生态。尽管优化 Gemini 但模型无关。

**核心局限：** 比竞争对手更年轻。生态仍在成熟。最紧密的集成路径会将你锁定到 Google Cloud。

### 7. Microsoft Agent Framework (Semantic Kernel + AutoGen)

**最新版本：** 发布候选，2026 年 2 月 19 日（GA 目标 2026 Q1 末）
**仓库：** [learn.microsoft.com/en-us/agent-framework](https://learn.microsoft.com/en-us/agent-framework/overview/)

**角色模型：**
- 合并了 Semantic Kernel 的 **Kernel + Plugin** 模型与 AutoGen 的 **多 Agent 模式**
- Agent 获得 kernel（plugins + services）、instructions，参与群聊
- Plugin 是 `@kernel_function` 装饰方法的集合 — 高度可组合

**任务模型：**
- 基于图的工作流 API 定义多步骤、多 Agent 任务流
- `AgentGroupChat` 配合选择/终止策略用于对话式任务
- Process framework 用于结构化的非对话式工作流

**协调：**
- 新的**基于图的工作流 API**用于复杂的多步骤、多 Agent 工作流
- 内置编排模式：sequential、parallel、Magentic（web + code + file agents）
- `AgentGroupChat` 配合选择策略（round-robin、LLM-selected）和终止策略

**核心优势：** 企业级。多语言（Python、C#、Java）。成熟的插件系统。合并创造了最全面的 Microsoft 代理 AI 产品。

**核心局限：** 最重的框架。需要从 AutoGen 或 SK 迁移。复杂性对简单用例可能是杀鸡用牛刀。

---

## 对比矩阵

| 维度 | Gas Town | CrewAI | LangGraph | AutoGen/MAF | OpenAI Agents SDK | Claude Agent SDK | Google ADK |
|---|---|---|---|---|---|---|---|
| **角色定义** | 命名运营角色（Mayor、Witness、Polecat），带 Role Bead + Agent Bead | role/goal/backstory 三元组 | 图节点（函数） | Agent 类 + 系统提示 | instructions + handoffs | instructions + tools (MCP) | instructions + tools + sub_agents |
| **任务定义** | Beads（带 ID、状态、受托人的原子工作项）+ Molecules（多步工作流） | 带有 description + expected_output 的 `Task` 类 | 图拓扑就是任务（以初始状态调用） | 隐式（给团队的消息） | 隐式（给 Runner 的消息） | 隐式（给 Agent 循环的 prompt） | 隐式（给根 Agent 的消息） |
| **角色持久性** | 持久（身份通过 Agent Bead + GUPP 跨重启存活） | 每次 kickoff | 基于检查点 | 每次会话 | 每次会话 | 每次会话 | 会话管理 |
| **协调** | Hook + GUPP（拉取式）+ Molecules（工作流图） | Sequential / Hierarchical / Flows | 显式图边 | GroupChat + strategies / Graph workflows | Agent handoffs | Agent 循环 + 委派 | 层级 + Sequential/Loop/Parallel |
| **通信** | Hook（Bead 支持的队列）+ mail（异步）+ nudge/peek（直接） | 任务上下文链 | Typed state + reducers | 异步消息 / GroupChat | 对话 + handoff 返回 | 对话轮次 | 消息传递 |
| **工作流定义** | Formulas (TOML) → Protomolecules → Molecules + Gates | 带有 expected_output 的 Tasks | 图边（代码） | GroupChat 配置 | Routines（instructions） | Agent 循环 | SequentialAgent / LoopAgent |
| **工具模型** | 每个角色的 Claude Code 原生工具 | Agent/任务级工具列表 | bind_tools() + ToolNode | Agent 上的 FunctionTool | Python 函数 | MCP 服务器（进程内） | 函数 + 合作伙伴集成 |
| **并行性** | 20-30 并行 Polecat（OS 进程，Git worktree） | async_execution 标志，kickoff_for_each | 图中的并行分支 | 异步消息，分布式运行时 | 单线程 handoffs | 单 Agent 循环 | ParallelAgent 模式 |
| **状态管理** | Beads（Dolt 数据库，单元格级合并）+ Bead 状态机（open→working→done） | Crew memory（短/长/实体） | 检查点化的 typed state | 消息历史 | 对话上下文 | 对话上下文 | 会话状态 + 持久化 |
| **人在回路** | Tier 3 升级 + `human` gate + Mayor 监督 | 任务上的 human_input 标志 | 节点上的 interrupt_before/after | UserProxyAgent | Agents SDK guardrails | 内置安全模式 | 内置支持 |
| **成熟度** | 早期（2026 年 1 月，450+ 贡献者） | 稳定（v1.10.1） | 稳定（v1.0） | 过渡中（→ MAF RC） | 生产就绪 | 活跃开发 | 增长中（2026 年 2 月更新） |

---

## Gas Town 模型相对于框架的差距

### 1. 没有声明式角色 Schema（尚未）
Gas Town 拥有命名的运营角色，带有包含 priming 指令的 Role Bead，但角色目前定义为 **Go 模板文件**（`internal/templates/roles/*.md.tmpl`），编译到 `gt` 二进制中。这意味着添加或修改角色需要更改 Go 源代码并重新编译。CrewAI 的 role/goal/backstory 三元组和 Google ADK 的 instructions/tools/sub_agents 格式都是面向用户的、有文档的、易于在运行时扩展的。**这正是 w-gc-001 旨在解决的** — 将角色定义提取为结构化的、可解析的格式（YAML/TOML），用户无需触碰 Go 代码即可自定义。

### 2. Formulas 仅限 TOML，尚未成为完整 DSL
Gas Town 的 Formula → Protomolecule → Molecule 管道是强大的工作流抽象，但 Formulas 目前是具有临时结构的 TOML 文件。对比 LangGraph 的 typed Python 图定义或 Microsoft Agent Framework 的基于图的工作流 API — 两者都提供 IDE 支持、类型检查和程序化组合。Formula 引擎（w-gc-003）可以从更结构化的 DSL 或 Schema 中受益。

### 3. 没有标准化工具 Schema
Claude Agent SDK（MCP）、Microsoft Agent Framework（plugins/kernel functions）和 Google ADK（合作伙伴集成）等框架有明确定义的工具接口。Gas Town Agent 使用 Claude Code 的原生工具，但没有 Gas Town 特定的工具抽象、注册表或按角色的工具范围限定。

### 4. 可观测性与追踪
OpenAI Agents SDK 和 Microsoft Agent Framework 都包含内置追踪（OpenTelemetry、仪表板查看器）。LangGraph 有 LangSmith 集成。Gas Town 有 Deacon/Witness 巡逻、peek/nudge 和 Bead 状态转换 — 这些功能上可用但有机生长。没有结构化的追踪格式、可查询的 span 数据或时间线可视化。

### 5. 动态路由基于 Gate 而非图
Gas Town 有 Gate（gh:run、gh:pr、timer、human、mail）用于异步协调，这是好的。但对比 LangGraph 的条件边，函数检查状态并路由到不同节点 — Gas Town 的 Molecules 目前不支持基于 Agent 输出的任意条件分支。GUPP 原则（"立即执行"）优化吞吐量而非路由灵活性。

### 6. 演进中的跨框架可移植性
Gas Town 最初仅支持 Claude Code，但 v0.6.0 添加了 Gemini 和 Copilot CLI 集成。然而，角色模板系统（`gt prime` 通过 CLAUDE.md 约定注入 Go 模板）仍然深度绑定到 Claude Code 的 priming 模型。调研的每个其他框架在其核心抽象中都是模型无关的。如果 Gas City 旨在成为可移植协议，角色格式应该抽象 LLM 运行时，带有 Claude Code、Gemini 等的适配层。

---

## Gas City 可借鉴的模式

### 1. CrewAI 的 Role/Goal/Backstory 三元组 → Gas City 角色格式
该三元组简单、直观且经过规模验证（v1.10，30k+ stars）。Gas Town 已经有包含 priming 指令的 Role Bead — 下一步是将其形式化为 Schema。Gas City 角色定义可以将 CrewAI 的角色模式与 Gas Town 的运营特性结合：
```yaml
role: Witness
goal: 监控 Polecat 健康并检测卡住的工作者
backstory: 你监督此 Rig 中所有活跃的 Polecat...
layer: rig-management          # Gas Town 层级
tools: [patrol, health-check, escalate]
hooks: [polecat-completion, merge-ready]
gates: [human, timer]
constraints: [read-only-unless-escalating]
escalation_tier: 2
```
这把 Gas Town 的已有模式映射到可移植的、用户可定义的格式。

### 2. LangGraph 的检查点 + 时间旅行 → Bead 版本化
Gas Town 已经有 Git 支持的 Beads，在理念上类似 LangGraph 的检查点。LangGraph v1.0 使**时间旅行调试**成为一等功能（从任何检查点回放，检查任何节点的状态）。Gas Town 可以显式地暴露这一点："显示 Bead mp-001 在提交 X 时的状态"或"从上一个 Gate 回放此 Polecat 的 Molecule。"Git 历史已经在那了 — 只需要查询层。

### 3. OpenAI Agents SDK 的类型化 Handoff → 形式化 SLING → HOOK → GUPP
Gas Town 的 sling/hook/GUPP 流程在功能上类似于 Agents SDK 的 handoff 模式，但通过基于文件的 Hook 而非进程内返回实现。Agents SDK 为 handoff 添加了**追踪和 span 数据**，使流程可观察。Gas Town 可以添加结构化的 handoff 事件：谁 Sling 了什么给谁，GUPP 何时激活，命中了什么 Gate。

### 4. Google ADK 的 Agent 层级 → Gas City 的 Town/Rig/Worker 模型
Google ADK 的树状 Agent 层级（带 `sub_agents` 的 Agent）紧密镜像 Gas Town 的 Town → Rig → Worker 结构。ADK 的内置模式直接类比：
- `SequentialAgent` → Refinery 合并队列（顺序处理）
- `LoopAgent` → Deacon 巡逻循环（循环工作流）
- `ParallelAgent` → Polecat swarm（并行执行）

这些可以为 Gas City 的 Formula 引擎（w-gc-003）如何声明式地组合 Agent 行为提供参考。

### 5. Microsoft Agent Framework 的插件模型 → Gas City 工具范围限定
Kernel + Plugin 模式（函数分组为命名插件，选择性对 Agent 可用）可以为基于角色的工具范围限定提供参考。Gas Town 目前给所有 Agent Claude Code 的完整工具集。有了插件：
```yaml
role: Refinery
plugins: [git-merge, conflict-resolution, test-runner]
# 不获得: file-create, web-search, 等。
```
这映射到最小权限原则 — Agent 只获得其角色需要的工具。

### 6. CrewAI Flows → 类型化 Hook 事件
CrewAI 新的 Flows 功能（事件驱动、精细控制）在理念上与 Gas Town 的 Hook + mail 系统一致。关键新增是**事件类型化** — Flows 定义显式的事件类型和带 Schema 的处理器。Gas Town 的 Hook 目前接受 Molecules（工作流实例），但 Hook 触发机制可以被形式化：
```toml
[hook.events]
polecat_complete = { schema = "bead_id, branch, test_result" }
merge_conflict = { schema = "bead_id, conflicting_files" }
escalation = { schema = "tier, source_agent, reason" }
```

### 7. LangGraph 的条件边 → Formula 分支
LangGraph 的条件路由（函数检查状态并返回下一节点）可以增强 Gas Town 的 Molecule 系统。目前，Molecules 是线性的，Gate 用于异步等待。添加条件分支：
```toml
[molecule.steps]
run_tests = { next_on_pass = "merge", next_on_fail = "notify_witness" }
```
这将使 Formulas 更具表现力，而不放弃 GUPP 的拉取式执行模型。

### 8. Gas Town 的独特模式（其他框架应该借鉴的）
值得注意的是 Gas Town 做了其他框架无法匹配的事：
- **GUPP** — 拉取式、崩溃存活的执行。没有其他框架有类似的"工作持久并自动恢复"原语。
- **Git-worktree 隔离** — 每个 Polecat 在自己的 worktree 中。大多数框架共享状态；Gas Town 给每个工作者自己的文件系统。
- **身份与会话的分离** — Agent Bead 跨会话崩溃存活。其他框架在会话结束时丢失 Agent 状态。每次完成都成为永久能力账本的一部分。
- **Dolt 单元格级合并** — 来自多个 Agent 的并发更新在列级别解决，而非行级别。这就是为什么 20-30 个并行 Agent 可以写入同一个 Beads 数据库而没有持续冲突。没有其他框架有这个。
- **Formula → Protomolecule → Molecule 管道** — 编译的、版本化的工作流定义（`bd cook` → `bd mol pour`）。最接近的类比是 LangGraph 的编译图，但 Gas Town 的是 Git 冻结且崩溃存活的。
- **升级层级** — 从 Deacon（Tier 1）→ Mayor（Tier 2）→ Human（Tier 3）的结构化升级比大多数框架的二元人在回路更细粒度。
- **Wasteland** — 没有其他 Agent 框架有将多个用户的 Agent 工作区联合成信任评分劳动力市场的概念。

---

## 建议

1. **将 Gas City 角色格式设计为带 Gas Town 运营字段的 YAML**（直接馈入 w-gc-001）。借鉴 CrewAI 的 role/goal/backstory 用于角色，但添加 Gas Town 特定字段：layer、hooks、gates、escalation_tier、tools/plugins。Role Bead 应从此 YAML 生成。

2. **用 Schema 形式化 Hook 事件** — Gas Town 的 Hook 强大但无类型。添加事件 Schema 使被 Sling 的工作、完成、升级和 Gate 触发都有定义的结构。这使系统可调试和可组合。

3. **添加结构化可观测性** — 结构化的 handoff 事件日志（谁 Sling 了什么给谁、GUPP 激活、Gate 命中、升级触发）可以让 Gas Town 接近 OpenAI Agents SDK 和 LangGraph Platform 提供的功能。不需要 OpenTelemetry — 即使每个 Rig 一个可查询的 JSONL 事件日志也很有价值。

4. **考虑 MCP 用于工具层** — Claude Agent SDK 基于 MCP 的工具符合标准，能让 Gas City 定义跨 LLM 后端工作的角色专用工具集。这解决了 Claude Code 耦合差距。

5. **向 Formulas 添加条件分支** — 为 Formula 引擎（w-gc-003）借鉴 LangGraph 的条件边模式。Molecules 应该支持"如果测试通过则合并；如果测试失败则通知 Witness"而无需新 Molecule。

6. **不要收敛到对话即控制平面** — Gas Town 的进程模型方法（持久命名角色、并行 OS 进程、Git-worktree 隔离、基于 Hook 的通信、GUPP）是真正差异化的。其他每个框架都通过 LLM 对话或进程内函数调用路由。Gas Town 通过 Git 状态和基于文件的 Hook 路由。这是特性而非局限 — 这正是 20-30 并行 Agent 和崩溃恢复的基础。目标应该是形式化这些模式，而非用其他人正在做的来替代。

---

## 结论

本调研揭示了多 Agent 领域中一个根本性的架构分裂。调研的每个外部框架 — AutoGen、CrewAI、LangGraph、OpenAI Agents SDK、Claude Agent SDK、Google ADK、Microsoft Agent Framework — 都使用**对话作为控制平面**。Agent 通过 LLM 推理互相发送消息来协调。无论是 CrewAI 的任务链、LangGraph 的状态传递图，还是 AutoGen 的 GroupChat，LLM 始终在路由和协调决策的循环中。

Gas Town 是唯一使用**进程模型**的框架。Agent 通过 Dolt 中的外部状态协调，Hook、Bead 和 GUPP 提供调度和持久化原语。LLM 做*工作*（写代码、审核、合并），但不做*路由*。路由是确定性的：Sling 将工作放到 Hook 上，GUPP 激活，Agent 执行。这就是为什么 Gas Town 可以扩展到 20-30 个并行 Agent，而基于对话的框架在 3-5 个以上就困难 — 对话中每增加一个 Agent 都会倍增 token 成本和路由复杂性，而 Gas Town 中每增加一个 Polecat 只是另一个拥有自己 worktree 的独立进程。

**对 Gas City 的战略含义是明确的：不要收敛。** Gas City 的声明式角色格式应该形式化 Gas Town 的进程模型模式 — 带 Hook、Gate、升级层和 GUPP 语义的运营角色 — 而非采用其他框架的角色/图/对话范式。Gas City 应该借鉴的是*人体工程学*（CrewAI 的直观角色定义、LangGraph 的类型化状态、Microsoft 的插件模型），而非*架构*。

具体来说：
- **从 CrewAI**：role/goal/backstory Schema 模式 — 简单、直观、经过验证 — 作为 Gas City 角色定义的用户面层。但扩展 Gas Town 运营字段（layer、hooks、gates、escalation_tier）。
- **从 LangGraph**：Formulas 的条件分支，以及使时间旅行调试成为基于已有 Git/Dolt 历史的一等功能的理念。
- **从 Microsoft Agent Framework**：用于角色范围工具分配的插件模型，实现按角色的最小权限工具访问。
- **从整个领域**：结构化可观测性。Gas Town 的有机监控（Deacon 巡逻、Witness Nudge）能用，但可查询的事件日志会使系统在大规模时可调试。

Gas Town 架构的独特性既是其最大风险也是最大优势。风险是生态隔离 — 每个其他框架的工具、教程和社区知识都假设对话即控制平面。优势是 Gas Town 解决了一个没有其他人在解决的问题：可靠地协调许多并行的、易崩溃的、上下文有限的 AI 会话，在真实代码的生产规模上工作。

---

## 来源

### 框架
- [AutoGen GitHub](https://github.com/microsoft/autogen)
- [AutoGen v0.4 公告](https://www.microsoft.com/en-us/research/blog/autogen-v0-4-reimagining-the-foundation-of-agentic-ai-for-scale-extensibility-and-robustness/)
- [Microsoft Agent Framework 概览](https://learn.microsoft.com/en-us/agent-framework/overview/)
- [Microsoft Agent Framework RC 公告](https://devblogs.microsoft.com/semantic-kernel/migrate-your-semantic-kernel-and-autogen-projects-to-microsoft-agent-framework-release-candidate/)
- [Semantic Kernel + AutoGen 合并](https://visualstudiomagazine.com/articles/2025/10/01/semantic-kernel-autogen--open-source-microsoft-agent-framework.aspx)
- [CrewAI 文档 / 变更日志](https://docs.crewai.com/en/changelog)
- [CrewAI PyPI](https://pypi.org/project/crewai/)
- [LangGraph v1.0 公告](https://blog.langchain.com/langchain-langgraph-1dot0/)
- [LangGraph GitHub](https://github.com/langchain-ai/langgraph)
- [OpenAI Swarm GitHub](https://github.com/openai/swarm)
- [OpenAI Agents SDK 概览](https://lexogrine.com/blog/openai-swarm-multi-agent-framework-2026)
- [Claude Agent SDK 文档](https://platform.claude.com/docs/en/agent-sdk/overview)
- [Claude Agent SDK GitHub](https://github.com/anthropics/claude-agent-sdk-python)
- [Google ADK 文档](https://google.github.io/adk-docs/)
- [Google ADK GitHub](https://github.com/google/adk-python)
- [Google ADK 集成生态](https://developers.googleblog.com/supercharge-your-ai-agents-adk-integrations-ecosystem/)

### Gas Town / Gas City
- [Gas Town GitHub](https://github.com/steveyegge/gastown) — 角色模板源码位于 `internal/templates/roles/*.md.tmpl`
- [Beads GitHub](https://github.com/steveyegge/beads) — `bd` 二进制，Dolt 支持的工作追踪
- [Gas Town 文档](https://docs.gastownhall.ai/)
- [Gas Town 词汇表](https://github.com/steveyegge/gastown/blob/main/docs/glossary.md)
- [Welcome to Gas Town (Yegge)](https://steve-yegge.medium.com/welcome-to-gas-town-4f25ee16dd04)
- [Welcome to the Wasteland (Yegge, 2026年3月)](https://steve-yegge.medium.com/welcome-to-the-wasteland-a-thousand-gas-towns-a5eb9bc8dc1f)
- [Gas Town 紧急用户手册 (Yegge)](https://steve-yegge.medium.com/gas-town-emergency-user-manual-cf0e4556d74b)
- [A Day in Gas Town (DoltHub)](https://www.dolthub.com/blog/2026-01-15-a-day-in-gas-town/)
- [Gas Town 的 Agent 模式 (Maggie Appleton)](https://maggieappleton.com/gastown)
- [GasTown 和两种多 Agent](https://paddo.dev/blog/gastown-two-kinds-of-multi-agent/)
- [Gas Town 架构深入 (DeepWiki)](https://deepwiki.com/numman-ali/n-skills/4.1.1-gas-town:-architecture-and-core-concepts)
- [Gas Town 阅读笔记 (Torq)](https://reading.torqsoftware.com/notes/software/ai-ml/agentic-coding/2026-01-15-gas-town-multi-agent-orchestration-framework/)
- [SE Daily 采访 Yegge](https://softwareengineeringdaily.com/2026/02/12/gas-town-beads-and-the-rise-of-agentic-development-with-steve-yegge/)
- [Wasteland CLI PR #1552](https://github.com/steveyegge/gastown/pull/1552)

### 比较
- [AutoGen vs LangGraph vs CrewAI 2026](https://dev.to/synsun/autogen-vs-langgraph-vs-crewai-which-agent-framework-actually-holds-up-in-2026-3fl8)
- [2026 年 AI Agent 大对决](https://dev.to/topuzas/the-great-ai-agent-showdown-of-2026-openai-autogen-crewai-or-langgraph-1ea8)
- [LangGraph 2026 更新](https://www.agentframeworkhub.com/blog/langgraph-news-updates-2026)