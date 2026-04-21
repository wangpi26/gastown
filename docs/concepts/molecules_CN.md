# Molecule

Molecule 是 Gas Town 中协调多步工作的工作流模板。

## Molecule 生命周期

```
Formula (source TOML) ─── "Ice-9"
    │
    ▼ bd cook
Protomolecule (frozen template) ─── Solid
    │
    ├─▶ bd mol pour ──▶ Mol (persistent) ─── Liquid
    │
    └─▶ bd mol wisp --root-only ──▶ Root Wisp (ephemeral) ─── Vapor
```

**仅根 Wisp**（默认）：Formula 步骤不会物化为数据库行。只创建一个根 Wisp。Agent 在 prime 时从嵌入的 Formula 中内联读取步骤。这防止了 Wisp 积累（约 6,000+ 行/天 降至约 400 行/天）。

**浇筑 Wisp**（`pour = true`）：步骤会物化为带检查点恢复的子 Wisp。如果会话终止，已完成的步骤保持关闭状态，工作从上一个检查点恢复。对代价高昂、低频的工作流使用 pour——在这种场景下丢失进度是代价惨重的（例如发布工作流）。

## 核心概念

| 术语 | 说明 |
|------|------|
| **Formula** | 定义工作流步骤的源 TOML 模板 |
| **Protomolecule** | 冻结的模板，可以实例化 |
| **Molecule** | 活跃的工作流实例（仅根 Wisp） |
| **Wisp** | 用于巡逻和 Polecat 工作的临时 Molecule（不同步） |
| **仅根** | 只创建根 Wisp；步骤从嵌入的 Formula 中读取 |
| **Pour** | Formula 标志（`pour = true`）；步骤物化为带检查点恢复的子 Wisp |

## Agent 如何看待步骤

Agent 不会使用 `bd mol current` 或 `bd close <step-id>` 来处理 Formula 工作流。相反，Formula 步骤在 Agent 运行 `gt prime` 时内联渲染：

```
**Formula Checklist** (10 steps from mol-polecat-work):

### Step 1: Load context and verify assignment
Initialize your session and understand your assignment...

### Step 2: Set up working branch
Ensure you're on a clean feature branch...
```

Agent 按清单完成工作，完成时运行 `gt done`（Polecat）或 `gt patrol report`（巡逻 Agent）。

## Molecule 命令

### Beads 操作（bd）

```bash
# Formulas
bd formula list              # 可用的 Formula
bd formula show <name>       # Formula 详情
bd cook <formula>            # Formula → Proto

# Molecules（数据操作）
bd mol list                  # 可用的 Proto
bd mol show <id>             # Proto 详情
bd mol wisp <proto>          # 创建 Wisp（默认仅根）
bd mol bond <proto> <parent> # 附加到已有的 Mol
```

### Agent 操作（gt）

```bash
# Hook 管理
gt hook                    # 我的 Hook 上有什么？
gt prime                   # 显示内联 Formula 清单
gt mol attach <bead> <mol>   # 将 Molecule 固定到 Bead
gt mol detach <bead>         # 从 Bead 取消固定 Molecule

# 巡逻生命周期
gt patrol new              # 创建巡逻 Wisp 并挂载
gt patrol report --summary "..."  # 关闭当前巡逻，开始下一轮
```

## Polecat 工作流

Polecat 通过其 Hook 接收工作——一个附加到 issue 的根 Wisp。它们在运行 `gt prime` 时看到内联的 Formula 清单，并按顺序完成每个步骤。

### Polecat 工作流摘要

```
1. 生成并挂载工作到 Hook
2. gt prime               # 显示内联 Formula 清单
3. 按顺序完成每个步骤
4. 持久化发现：bd update <issue> --notes "..."
5. gt done                # 提交、清除沙盒、退出
```

### Molecule 类型

| 类型 | 存储 | 用例 |
|------|------|------|
| **仅根 Wisp**（`pour = false`） | `.beads/`（临时） | Polecat 工作、巡逻 — 高频、低代价步骤 |
| **浇筑 Wisp**（`pour = true`） | `.beads/`（子 Wisp） | 发布、长工作流 — 低频、高代价步骤 |

**经验法则**：如果崩溃后丢失进度会让你后悔，就设置 `pour = true`。高频 + 低代价步骤 = 内联（默认）。低频 + 高代价步骤 = pour。

## 巡逻工作流

巡逻 Agent（Deacon、Witness、Refinery）循环执行巡逻 Formula：

```
1. gt patrol new          # 创建仅根巡逻 Wisp
2. gt prime               # 显示内联巡逻清单
3. 按顺序完成每个步骤
4. gt patrol report --summary "..."  # 关闭 + 开始下一轮
```

`gt patrol report` 原子性地关闭当前巡逻根并生成下一个循环的根。

## 最佳实践

1. **尽早持久化发现** — 在会话终止前执行 `bd update <issue> --notes "..."`
2. **完成时运行 `gt done`** — Polecat 必须（推送、提交到 MQ、清除）
3. **使用 `gt patrol report`** — 巡逻 Agent 用于循环（替代 squash+new 模式）
4. **提交发现的工作** — 使用 `bd create` 记录发现的 bug，不要自行修复