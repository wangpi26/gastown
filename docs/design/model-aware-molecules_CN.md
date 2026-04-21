# Model-Aware Molecule 约束

> 为 molecule 步骤添加模型特定约束及订阅感知路由的计划。

**状态**：进行中
**负责**：设计
**相关**：[molecules.md](../concepts/molecules.md) | [agent-provider-interface.md](agent-provider-interface.md)

---

## 与 Consensus 的关系

Consensus 和 model-aware molecules 是**互补的层级**，共享相同的会话感知基础设施，但服务于不同目的：

| | Consensus | Molecules |
|---|---|---|
| **模式** | 扇出 | DAG 路由 |
| **形状** | 同一提示词 → N 个 agent → 比较 | N 个步骤 → 每步最佳模型 |
| **会话基础设施** | `GT_AGENT` + `AgentPresetInfo` 就绪 | 相同 — 复用，非重建 |
| **路由目标** | 多样性（多视角） | 最优性（每步正确模型） |

Consensus v2 建立的 provider 解析管道 — `GT_AGENT` env 查找 → `AgentPresetInfo` → 就绪检测（提示词轮询或延迟回退）— 正是 molecule 路由器调度所需的会话感知基础设施。参见第 5.3 节（两阶段路由）。

---

## 1. 引言 / 概述

Molecule 目前支持基于依赖的 DAG 执行，但缺乏指定**哪个 AI 模型**执行每个步骤的能力。随着多个 AI 提供方（Anthropic、OpenAI、DeepSeek、Google 等）和访问类型（API key 和 Claude Code 等订阅）的出现，我们需要：

1. **按步骤的模型约束** — 为每个步骤指定所需模型或能力
2. **订阅支持** — 支持 Claude Code 和其他基于订阅的访问（对成本优化至关重要）
3. **自动定价数据** — 从 OpenRouter 获取实时定价；回退到缓存数据
4. **Meta-model 路由** — 轻量级启发式根据成本、质量和配额选择模型
5. **本地使用追踪** — 将调用记录到 `~/.gt/usage.jsonl`（OTel 叠加/可选）

---

## 2. 设计目标

| 目标 | 描述 |
|------|------|
| **Molecule 级约束** | 为 molecule 步骤添加模型/能力约束 |
| **订阅支持** | 同时支持 API key 和订阅访问 |
| **实时定价** | 从 OpenRouter 获取定价，带 24 小时本地缓存 |
| **静态基准** | 内置 MMLU/SWE 分数；通过 `~/.gt/models.toml` 覆盖 |
| **Meta-Model 路由** | 仅启发式评分：无 LLM 调用 |
| **本地使用追踪** | 始终写入 `~/.gt/usage.jsonl`；OTel 为叠加 |
| **DAG 兼容** | 与现有 molecule DAG 结构兼容 |
| **向后兼容** | 现有 formula 无需修改即可工作 |

---

## 3. 质量门控

本计划中的所有实现故事必须通过以下质量门控：

- `go test ./...`
- `golangci-lint run`
- 订阅访问检测的手动验证

---

## 4. 用户故事

### US-001：基于订阅的访问配置

**描述**：作为 Gas Town 操作员，我希望能配置 Claude Code 订阅，使其因成本原因自动优先于 API key。

**验收标准**：
- [ ] 环境变量 `CLAUDE_CODE_SUBSCRIPTION=active` 启用订阅检测
- [ ] 订阅元数据（计划类型、账户）从环境变量读取
- [ ] 当两者都可用时，订阅访问优先于 API key
- [ ] `bd ready --json` 包含订阅配额信息

### US-002：模型能力数据库

**描述**：作为开发者，我希望有内置的模型能力数据库（MMLU、SWE、成本），供路由系统使用而无需手动配置。

**验收标准**：
- [x] `internal/models/database.go` 包含带基准分数的静态模型条目
- [x] 定价从 OpenRouter（`https://openrouter.ai/api/v1/models`）获取，带 24 小时缓存
- [x] 缓存存储于 `~/.gt/models_pricing_cache.json`；获取失败时优雅降级（使用零定价）
- [x] `~/.gt/models.toml` 可覆盖或扩展任何字段，包括价格、基准、新模型
- [x] `GetModel(db, id)` 返回模型元数据或 nil
- [x] `LoadDatabase(gtDir)` = 静态 + OpenRouter 定价 + 用户覆盖

### US-003：Meta-Model 路由逻辑

**描述**：作为系统，我希望有轻量级路由算法，基于任务需求和成本约束选择模型，无需调用另一个 LLM。

**验收标准**：
- [x] `internal/models/router.go` 实现 `SelectModel()`，仅使用启发式
- [x] 路由考虑：provider、access_type、min_mmlu、min_swe、requires、max_cost
- [x] 订阅访问可用时优先（成本 = $0）
- [x] 决策包含：选定模型、原因、成本、MMLU/SWE 分数
- [x] 当没有模型满足约束时返回错误

### US-004：Molecule 步骤约束语法

**描述**：作为 formula 作者，我希望使用简单的 TOML 语法在 molecule 步骤中指定模型约束。

**验收标准**：
- [x] 步骤支持 `model = "claude-sonnet-4-5"` 用于精确模型
- [x] 步骤支持 `provider = "anthropic"` 用于某提供方的任何模型
- [x] 步骤支持 `model = "auto"` 用于启发式路由
- [x] 步骤支持 `min_mmlu = 85` 和 `min_swe = 70` 用于质量阈值
- [x] 步骤支持 `requires = ["vision", "code_execution"]` 用于能力约束
- [x] 步骤支持 `access_type = "subscription"` 用于要求订阅访问
- [x] 步骤支持 `max_cost = 0.01` 用于成本约束（USD 每 1K token，合计）
- [x] 解析器验证所有新字段；拒绝未知能力和无效范围
- [x] `model` 和 `provider` 不能同时设置（解析器错误）

### US-005：使用追踪

**描述**：作为系统，我希望本地追踪模型使用，使操作员无需依赖 OTel 即可监控成本。

**验收标准**：
- [x] `internal/models/usage.go` 记录使用到 `~/.gt/usage.jsonl`（始终）
- [x] 每条记录：时间戳、模型 ID、provider、access_type、tokens in/out、cost、success、latency、reason
- [x] `LoadUsage(gtDir, since)` 读取和过滤条目
- [x] `MonthlyStats(entries, year, month)` 按模型聚合
- [x] `TotalCost(entries)` 汇总 USD 成本
- [x] OTel 集成为叠加 — 如果设置了 `GT_OTEL_LOGS_URL`，调用方单独发出 OTel 事件

### US-006：增强的 `gt prime` 模型信息

**描述**：作为操作员，我希望 `gt prime` 能显示每个步骤有哪些模型可用以及将使用哪个模型。

**验收标准**：
- [ ] 每个步骤显示：约束类型、推荐模型、访问类型、预估成本
- [ ] 当主要模型不可用时列出回退模型
- [ ] `gt step <step-id>` 使用模型路由执行特定步骤
- [ ] 视觉指示：`✓ subscription` vs `$0.003/K api_key`

### US-007：带模型分配的批量 DAG 执行

**描述**：作为操作员，我希望执行整个 molecule 并自动按步骤分配模型。

**验收标准**：
- [ ] `gt mol execute --auto-route <mol-id>` 读取约束并按步骤路由
- [ ] 并行步骤在可用时同时执行
- [ ] 路由失败时显示哪个约束无法满足

### US-008：使用报告 CLI

**描述**：作为操作员，我希望 `gt usage` 能显示全面的使用统计。

**验收标准**：
- [ ] `gt usage` 显示月度摘要：总成本、调用次数、订阅使用
- [ ] 表格：provider、模型、token、成本、成功率
- [ ] `gt usage --month 2025-02` 过滤到特定月份
- [ ] 历史数据从 `~/.gt/usage.jsonl` 加载

---

## 5. 技术设计

### 5.1 访问类型

```go
// internal/models/database.go

// ModelEntry 上的 SubscriptionEligible bool 表示该模型可通过
// 订阅访问（例如 Claude Code 用于 Anthropic 模型）。
// 调用方从环境变量检测订阅可用性，并作为
// StepConstraints.SubscriptionActive 传入。
```

注意：Claude Code 是一种**访问方式**，不是模型。不要创建虚假的 `"claude-code"` 模型条目。正确的建模是在 Anthropic 模型条目上设置 `SubscriptionEligible: true`，在订阅活跃时路由决策上设置 `AccessType: "subscription"`。

### 5.2 模型能力数据库

```go
// internal/models/database.go

type ModelEntry struct {
    ID            string   // "claude-sonnet-4-5"
    Provider      string   // "anthropic"
    Name          string   // "Claude Sonnet 4.5"
    OpenRouterID  string   // "anthropic/claude-sonnet-4-5"（用于定价获取）

    // 基准分数（静态，可通过 ~/.gt/models.toml 覆盖）
    MMLUScore     float64
    SWEScore      float64

    // 能力
    Vision        bool
    CodeExecution bool
    ContextWindow int

    // USD 每 1K token 定价（从 OpenRouter 获取，缓存 24 小时）
    CostPer1KIn   float64
    CostPer1KOut  float64

    SubscriptionEligible bool
    GoodFor              []string
}

// LoadDatabase 合并：静态基准 → OpenRouter 定价 → ~/.gt/models.toml 覆盖
func LoadDatabase(gtDir string) []ModelEntry
```

**外部定价来源**：OpenRouter（`https://openrouter.ai/api/v1/models`）
- 无需 API key
- 返回数百个模型的按 token 定价
- 响应缓存到 `~/.gt/models_pricing_cache.json`，24 小时有效
- 获取超时：5 秒；失败非致命（使用零定价作为回退）

**基准数据**：在 `staticDB` 中静态内置（来自已发布的评估）。
通过 `~/.gt/models.toml` 覆盖或扩展：

```toml
# 覆盖内置模型的基准
[models.claude-sonnet-4-5]
mmlu = 84.5
swe = 52.0

# 添加静态数据库中没有的新模型
[models.my-local-model]
provider = "custom"
mmlu = 70.0
cost_per_1k_in = 0.0
cost_per_1k_out = 0.0
good_for = ["coding"]
```

### 5.3 两阶段路由

路由按两个顺序阶段进行。任何阶段都不进行 LLM 调用。

#### 阶段 1 — 模型选择（`SelectModel`）

根据步骤约束和评分启发式从能力数据库中选择最优模型。

```go
// internal/models/router.go

type StepConstraints struct {
    Model      string   // 精确 ID 或 "auto"
    Provider   string
    AccessType string   // "subscription" | "api_key"
    MinMMLU    float64
    MinSWE     float64
    Requires   []string
    MaxCost    float64  // USD 每 1K token（合计）
    // 由调用方从 env/config 填充：
    SubscriptionActive bool
}

type RoutingDecision struct {
    // 模型选择（阶段 1）
    ModelID      string
    Provider     string
    AccessType   string   // "subscription" | "api_key"
    Reason       string
    CostPer1KIn  float64
    CostPer1KOut float64
    MMLUScore    float64
    SWEScore     float64

    // 会话解析（阶段 2）— 未找到活动会话时为 nil
    SessionID    string   // tmux 会话名，例如 "gt-gastown-polecat-Toast"
    AgentPreset  string   // 解析的 GT_AGENT 值，例如 "claude"、"gemini"
}

func SelectModel(constraints StepConstraints, db []ModelEntry) (*RoutingDecision, error)
```

评分：

| 因素 | 权重 |
|------|------|
| 订阅活跃 + 模型符合条件 | +40 分 |
| MMLU 分数（标准化 0–100） | 最多 30 分 |
| SWE 分数（标准化 0–100） | 最多 20 分 |
| 成本节省（$0.10/1K 上限的倒数） | 最多 10 分 |

#### 阶段 2 — 会话解析（`ResolveSession`）

选定模型后，查找运行该模型的**活跃、空闲 tmux 会话**。这复用了
provider 解析管道中已有的 `GT_AGENT` + `AgentPresetInfo` 基础设施 —
与 Consensus v2 使用的逻辑相同。

```go
// internal/models/router.go（计划中）

// ResolveSession 扫描运行中的 tmux 会话并返回第一个
// 空闲且运行选定模型的会话。未找到匹配会话时返回 nil。
//
// 解析：
//  1. 列出活跃的 tmux 会话
//  2. 读取每个会话的 GT_AGENT 环境变量
//  3. 查找该 agent 名称的 AgentPresetInfo
//  4. 检查就绪状态：提示词轮询（ReadyPromptPrefix 例如 "❯ "）或延迟回退（ReadyDelayMs）
//  5. 返回匹配 ModelID 且空闲的第一个会话
func ResolveSession(decision *RoutingDecision, tmux Tmux) *RoutingDecision
```

**就绪检测**直接取自 `AgentPresetInfo` — 无新机制：

| Agent 类型 | 检测方法 | 来源 |
|---|---|---|
| Claude | 提示符前缀轮询（`❯ `） | `AgentPresetInfo.ReadyPromptPrefix` |
| OpenCode、Codex | 延迟回退 | `AgentPresetInfo.ReadyDelayMs` |
| 自定义 agent | 延迟回退 | 相同 |

**调度结果**：
- 找到活跃空闲会话 → 直接向该会话调度步骤
- 未找到匹配会话 → 使用选定模型生成新会话（`AgentPresetInfo.Command + Args`）

这意味着 molecule 步骤按**模型能力**定位活跃会话，而非仅按名称。
指定 `min_mmlu = 85` 的步骤将路由到恰好在运行合格模型的任何空闲会话，
formula 作者无需知道会话名称。

### 5.4 Molecule 步骤约束

```toml
# 所有约束字段均为可选且向后兼容。
# 没有约束的现有步骤接受任何可用 agent。

[[steps]]
id = "analyze-requirements"
title = "分析需求"
needs = ["load-context"]
# 选项 A: 精确模型
model = "claude-sonnet-4-5"

[[steps]]
id = "code-generation"
title = "代码生成"
needs = ["analyze-requirements"]
# 选项 B: 带质量和成本约束的启发式路由
model = "auto"
min_mmlu = 85
min_swe = 50
max_cost = 0.01

[[steps]]
id = "quick-scan"
title = "快速扫描"
# 选项 C: provider + 能力过滤
provider = "openai"
requires = ["code_execution"]

[[steps]]
id = "security-audit"
title = "安全审计"
# 选项 D: 优先订阅（零成本）
access_type = "subscription"
```

`model` 和 `provider` 互斥（同时设置时解析器报错）。

### 5.5 使用追踪

```go
// internal/models/usage.go

type UsageEntry struct {
    Timestamp  time.Time `json:"timestamp"`
    ModelID    string    `json:"model_id"`
    Provider   string    `json:"provider"`
    AccessType string    `json:"access_type"`
    TaskType   string    `json:"task_type"`
    TokensIn   int       `json:"tokens_in"`
    TokensOut  int       `json:"tokens_out"`
    CostUSD    float64   `json:"cost_usd"`
    Success    bool      `json:"success"`
    LatencyMs  int       `json:"latency_ms"`
    Reason     string    `json:"reason,omitempty"`
}

func RecordUsage(gtDir string, entry UsageEntry) error     // 追加到 usage.jsonl
func LoadUsage(gtDir string, since time.Time) ([]UsageEntry, error)
func MonthlyStats(entries []UsageEntry, year int, month time.Month) map[string]*ModelStats
func EstimateCost(model *ModelEntry, tokensIn, tokensOut int) float64
```

**OTel 集成**：想要 OTel 可观察性的调用方单独发出 `agent.usage` OTel 日志事件
（参见 `docs/otel-data-model.md`）。`usage.jsonl` 始终写入，不依赖于 OTel 是否配置。

---

## 6. 环境变量

```bash
# 订阅检测
export CLAUDE_CODE_SUBSCRIPTION=active    # 启用订阅偏好
export CLAUDE_CODE_ACCOUNT=user@co.com   # 信息性
export CLAUDE_CODE_PLAN=pro              # 信息性

# API Key 访问（现有）
export ANTHROPIC_API_KEY=sk-ant-xxx
export OPENAI_API_KEY=sk-openai-xxx
export GOOGLE_API_KEY=xxx
export DEEPSEEK_API_KEY=xxx

# 模型默认值（新增）
export GT_DEFAULT_MODEL=claude-sonnet-4-5  # 无约束步骤的回退
export GT_PREFERRED_PROVIDER=anthropic

# 阈值（新增）
export GT_MIN_MMLU=80
export GT_MIN_SWE=50
export GT_MAX_COST=0.005

# 使用追踪
export GT_TRACK_USAGE=true   # 默认 true；设为 false 禁用
```

注意：`CLAUDE_CODE_QUOTA` **不是**真正的环境变量 — Claude Code 不以编程方式暴露 token 配额。
如果需要配额追踪，从 `~/.gt/usage.jsonl` 中 `access_type="subscription"` 的条目推导。

---

## 7. 配置文件

### `~/.gt/models.toml` — 模型数据库覆盖

```toml
# 覆盖内置基准分数
[models.claude-sonnet-4-5]
mmlu = 84.5

# 添加新模型
[models.deepseek-v3-local]
provider = "deepseek"
mmlu = 88.0
swe = 48.0
cost_per_1k_in = 0.00014
cost_per_1k_out = 0.00028
context_window = 131072
good_for = ["coding", "reasoning"]
```

### `~/.gt/models_pricing_cache.json` — OpenRouter 定价缓存（自动管理）

由 `LoadDatabase` 写入；24 小时后刷新。不要手动编辑。

---

## 8. CLI 集成

### `gt prime` 输出中的步骤约束（计划中）

```
### 步骤 2: 分析需求
  约束: model=auto, min_mmlu=85
  推荐: claude-opus-4-5 (subscription, $0.00)
  回退: claude-sonnet-4-5 (api_key, $0.003/1K)

### 步骤 3: 代码生成
  约束: provider=openai, requires=[code_execution]
  推荐: gpt-4o ($0.0025/1K in)
```

### 新命令（计划中）

```bash
gt step <step-id>                       # 使用模型路由执行步骤
gt mol execute --auto-route <mol-id>    # 带路由的批量 DAG 执行
gt usage                                # 月度成本摘要
gt usage --month 2025-02                # 过滤到特定月份
gt model route --task coding --mmlu 85  # 调试：测试路由逻辑
```

---

## 9. Formula 示例

### 示例 1：订阅优先工作流

```toml
formula = "mol-subscription-aware"
version = 1

[[steps]]
id = "code-review"
title = "代码审查"
access_type = "subscription"
model = "auto"
description = "审查代码变更"

[[steps]]
id = "implement-fixes"
title = "实施修复"
needs = ["code-review"]
model = "auto"
description = "实施修复"
```

### 示例 2：多模型代码审查

```toml
formula = "mol-multi-model-review"
version = 1

[[steps]]
id = "claude-review"
title = "使用 Claude 审查"
model = "claude-sonnet-4-5"
description = "审查代码变更"

[[steps]]
id = "gpt-review"
title = "使用 GPT-4o 审查"
model = "gpt-4o"
parallel = true
description = "审查相同代码"

[[steps]]
id = "synthesize"
title = "综合发现"
needs = ["claude-review", "gpt-review"]
min_mmlu = 85
description = "合并两份审查"
```

### 示例 3：成本优化工作流

```toml
formula = "mol-cost-optimized"
version = 1

[[steps]]
id = "quick-scan"
title = "快速扫描"
model = "auto"
max_cost = 0.001
description = "使用最廉价合格模型进行快速概览"

[[steps]]
id = "deep-work"
title = "深度工作"
needs = ["quick-scan"]
model = "auto"
min_mmlu = 85
max_cost = 0.01
description = "在预算内使用质量模型进行深入工作"
```

---

## 10. 实现阶段

### 阶段 1：模型数据库 + 步骤约束（已完成）

- [x] 创建 `internal/models/database.go` — 静态基准 + OpenRouter 定价 + TOML 覆盖
- [x] 创建 `internal/models/router.go` — `SelectModel()` 启发式评分
- [x] 创建 `internal/models/usage.go` — 本地 JSONL 追踪；`MonthlyStats`、`EstimateCost`
- [x] 向 `internal/formula/types.go` Step 结构体添加路由字段
- [x] 在 `internal/formula/parser.go` 中验证新字段

### 阶段 2：订阅发现（P0）

- [ ] 检测 `CLAUDE_CODE_SUBSCRIPTION` 并将 `SubscriptionActive` 传入 `StepConstraints`
- [ ] 检测 API key 环境变量（现有模式）以确定可用提供方
- [ ] 发现逻辑的单元测试

### 阶段 3：会话感知调度（P0）

使用已有 `GT_AGENT` + `AgentPresetInfo` 基础设施实现 `ResolveSession()`：

- [ ] 扫描活跃 tmux 会话；读取每个会话的 `GT_AGENT` 环境变量（已在 `sling_helpers.go` 中完成）
- [ ] 按 agent 名称查找 `AgentPresetInfo` 以获取 `ReadyPromptPrefix` / `ReadyDelayMs`
- [ ] 实现空闲检查：对有 `ReadyPromptPrefix` 的 agent 进行提示词轮询，否则使用延迟回退
- [ ] 返回 agent 匹配 `RoutingDecision.ModelID` 的第一个空闲会话；设置 `SessionID` + `AgentPreset`
- [ ] 如无匹配：使用选定模型的 `AgentPresetInfo.Command + Args` 生成新会话
- [ ] 会话匹配和就绪检测的单元测试

### 阶段 4：CLI 集成（P1）

- [ ] 更新 `gt prime` 以显示模型约束、路由推荐和每步的活跃会话
- [ ] 实现 `gt step` 用于带两阶段路由的单步执行
- [ ] 实现 `gt mol execute --auto-route` 用于批量 DAG 执行
- [ ] 实现 `gt usage` 和 `gt usage --month`

### 阶段 5：调度时的使用记录（P1）

- [ ] 将 `RecordUsage` 钩入 agent 调度路径
- [ ] 可用时从 `agent.usage` OTel 事件推导 `TokensIn`/`TokensOut`，或估算
- [ ] OTel：路由决策做出时可选发出 `model.route` 事件

---

## 11. 技术考量

### 订阅 vs API Key 优先级

当同一提供方同时有订阅和 API key 可用时：

1. **订阅优先** — 成本已付；零增量成本
2. **API key 为回退** — 订阅不活跃时使用
3. 订阅配额不被 Claude Code 以编程方式暴露；从 usage.jsonl 追踪

### 轻量级路由

`SelectModel()` 是纯启发式 — 无 LLM 调用：

| 因素 | 权重 | 备注 |
|------|------|------|
| 订阅活跃 + 模型符合条件 | +40 分 | 免费 = 始终优先 |
| MMLU 分数 | 最多 30 分 | 通用知识质量 |
| SWE 分数 | 最多 20 分 | 代码特定质量 |
| 成本节省 | 最多 10 分 | $0.10/1K 上限的倒数 |
| 配额可用性 | 硬过滤 | 评分前应用 |

### 向后兼容

没有路由字段的步骤接受任何空闲 agent — 行为不变：

```toml
[[steps]]
id = "simple-step"
title = "简单步骤"
needs = ["previous-step"]
# 无模型约束 → 任何空闲 agent
```

---

## 12. 成功指标

- Formula 步骤可以指定按步骤的模型约束
- 订阅访问被检测并自动优先于 API key
- 模型定价从 OpenRouter 获取并本地缓存（无需 API key）
- 使用被本地追踪到 `~/.gt/usage.jsonl`，无论 OTel 是否配置
- 现有 formula 无需修改继续工作

---

## 13. 开放问题

| 问题 | 讨论 |
|------|------|
| **调度机制** | 已解决：`ResolveSession()` 直接定位活跃 tmux 会话。路由决策（`SessionID`、`AgentPreset`）是调度目标 — 无需单独的 env 注入。步骤描述通过已有的 `tmux send-keys` / nudge 路径发送。 |
| **模型 ID ↔ GT_AGENT 映射** | `GT_AGENT` 值是 agent 预设名（`"claude"`、`"gemini"`），而非模型 ID（`"claude-sonnet-4-5"`）。需要映射：`AgentPresetInfo` 可以携带 `DefaultModelID` 字段，或会话在生成时设置额外的 `GT_MODEL` 环境变量用于精确匹配。 |
| **同模型的多个会话** | 如果两个 Claude 会话都空闲且都符合条件，哪个获得步骤？当前提议：第一个空闲会话胜出（FIFO）。替代：轮询或基于负载。 |
| **基于成本的自动切换** | 系统是否应在预算接近耗尽时中途切换到更便宜的模型？ |
| **模型性能学习** | 历史成功率（来自 usage.jsonl）是否应影响路由权重？ |
| **多订阅支持** | 是否支持同时使用多个 Claude Code 团队订阅？ |