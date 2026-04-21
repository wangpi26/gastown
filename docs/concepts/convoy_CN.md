# Convoy

Convoy 是跨 Rig 追踪批量工作的一级单元。

## 快速上手

```bash
# 创建一个追踪若干 issue 的 convoy
gt convoy create "Feature X" gt-abc gt-def --notify overseer

# 查看进度
gt convoy status hq-cv-abc

# 列出活跃的 convoy（仪表盘）
gt convoy list

# 查看所有 convoy，包括已着陆的
gt convoy list --all
```

## 概念

**Convoy** 是一个持久化的追踪单元，用于监控跨多个 Rig 的相关 issue。当你启动一项工作——哪怕是单个 issue——convoy 都会追踪它，让你能看到它何时着陆、包含了什么内容。

```
                 🚚 Convoy (hq-cv-abc)
                         │
            ┌────────────┼────────────┐
            │            │            │
            ▼            ▼            ▼
       ┌─────────┐  ┌─────────┐  ┌─────────┐
       │ gt-xyz  │  │ gt-def  │  │ bd-abc  │
       │ gastown │  │ gastown │  │  beads  │
       └────┬────┘  └────┬────┘  └────┬────┘
            │            │            │
            ▼            ▼            ▼
       ┌─────────┐  ┌─────────┐  ┌─────────┐
       │  nux    │  │ furiosa │  │  amber  │
       │(polecat)│  │(polecat)│  │(polecat)│
       └─────────┘  └─────────┘  └─────────┘
                         │
                    "the swarm"
                    (ephemeral)
```

## Convoy 与 Swarm 的区别

| 概念 | 是否持久化 | ID | 说明 |
|------|-----------|-----|------|
| **Convoy** | 是 | hq-cv-* | 追踪单元。你创建、追踪、接收通知的对象。 |
| **Swarm** | 否 | 无 | 临时性的。"当前正在处理该 convoy issue 的工作者们。" |
| **Stranded Convoy** | 是 | hq-cv-* | 有就绪工作但没有 Polecat 分配的 Convoy。需要关注。 |

当你"启动一个 swarm"时，实际上是在做：
1. 创建一个 convoy（追踪单元）
2. 将 Polecat 分配给被追踪的 issue
3. "Swarm"就是那些 Polecat 工作时的临时集合

当 issue 关闭时，convoy 着陆并通知你。Swarm 随之消散。

## Convoy 生命周期

```
OPEN ──(所有 issue 关闭)──► LANDED/CLOSED
  ↑                              │
  └──(添加更多 issue)───────────┘
       (自动重新打开)
```

| 状态 | 说明 |
|------|------|
| `open` | 活跃追踪中，工作进行中 |
| `closed` | 所有被追踪的 issue 已关闭，通知已发送 |

向已关闭的 convoy 添加 issue 会自动重新打开它。

## 命令

### 创建 Convoy

```bash
# 追踪跨 Rig 的多个 issue
gt convoy create "Deploy v2.0" gt-abc bd-xyz --notify gastown/joe

# 追踪单个 issue（仍会创建 convoy 以便仪表盘可见）
gt convoy create "Fix auth bug" gt-auth-fix

# 使用默认通知（来自配置）
gt convoy create "Feature X" gt-a gt-b gt-c
```

### 添加 Issue

```bash
# 向已有 convoy 添加 issue
gt convoy add hq-cv-abc gt-new-issue
gt convoy add hq-cv-abc gt-issue1 gt-issue2 gt-issue3

# 向已关闭的 convoy 添加需要先重新打开
bd update hq-cv-abc --status=open
gt convoy add hq-cv-abc gt-followup-fix
```

### 查看状态

```bash
# 显示 issue 和活跃的工作者（swarm）
gt convoy status hq-abc

# 所有活跃的 convoy（仪表盘）
gt convoy status
```

输出示例：
```
🚚 hq-cv-abc: Deploy v2.0

  Status:    ●
  Progress:  2/4 completed
  Created:   2025-12-30T10:15:00-08:00

  Tracked Issues:
    ✓ gt-xyz: Update API endpoint [task]
    ✓ bd-abc: Fix validation [bug]
    ○ bd-ghi: Update docs [task]
    ○ gt-jkl: Deploy to prod [task]
```

### 列出 Convoy（仪表盘）

```bash
# 活跃的 convoy（默认）— 主要关注视图
gt convoy list

# 所有 convoy，包括已着陆的
gt convoy list --all

# 仅已着陆的 convoy
gt convoy list --status=closed

# JSON 输出
gt convoy list --json
```

输出示例：
```
Convoys

  🚚 hq-cv-w3nm6: Feature X ●
  🚚 hq-cv-abc12: Bug fixes ●

Use 'gt convoy status <id>' for detailed view.
```

## 通知

当 convoy 着陆（所有被追踪的 issue 已关闭）时，订阅者会收到通知：

```bash
# 指定订阅者
gt convoy create "Feature X" gt-abc --notify gastown/joe

# 多个订阅者
gt convoy create "Feature X" gt-abc --notify mayor/ --notify --human
```

通知内容：
```
🚚 Convoy Landed: Deploy v2.0 (hq-cv-abc)

Issues (3):
  ✓ gt-xyz: Update API endpoint
  ✓ gt-def: Add validation
  ✓ bd-abc: Update docs

Duration: 2h 15m
```

## 从 Epic 创建

自动发现已有 Epic 的子 issue。适用于当规划/分解工具已经将工作组织为带有子 Bead 的 Epic 时。

```bash
# 从 Epic 自动发现子 issue
gt convoy create --from-epic gt-epic-abc

# 覆盖 convoy 名称（默认使用 epic 标题）
gt convoy create --from-epic gt-epic-abc "Custom convoy name"

# 结合其他标志
gt convoy create --from-epic gt-epic-abc --owned --merge=direct
```

**工作原理：**
1. 验证给定的 Bead 是一个 Epic
2. BFS 遍历父子层级结构，找到可 Sling 的后代
3. 创建一个标准 convoy（`hq-cv-*`），追踪所有可 Sling 的子项（task、bug、feature、chore）

不可 Sling 的类型（子 Epic、decision）会被递归遍历但不会直接追踪。只有叶子工作项才会出现在 convoy 中。

## Sling 时自动创建 Convoy

当你 Sling 一个没有已有 convoy 的单个 issue 时：

```bash
gt sling bd-xyz beads/amber
```

这会自动创建一个 convoy，使所有工作都出现在仪表盘上：
1. 创建 convoy："Work: bd-xyz"
2. 追踪该 issue
3. 分配 Polecat

即使是"一个人的 swarm"也能获得 convoy 可见性。

## 跨 Rig 追踪

Convoy 存在于 town 级别的 Bead 中（`hq-cv-*` 前缀），可以追踪来自任何 Rig 的 issue：

```bash
# 追踪来自多个 Rig 的 issue
gt convoy create "Full-stack feature" \
  gt-frontend-abc \
  gt-backend-def \
  bd-docs-xyz
```

`tracks` 关系的特点：
- **非阻塞**：不影响 issue 的工作流
- **可累加**：随时可以添加 issue
- **跨 Rig**：convoy 在 hq-* 中，issue 在 gt-*、bd-* 等

## Convoy 与 Rig 状态的对比

| 视图 | 范围 | 显示内容 |
|------|------|---------|
| `gt convoy status [id]` | 跨 Rig | Convoy 追踪的 issue + 工作者 |
| `gt rig status <rig>` | 单个 Rig | Rig 中所有工作者 + 其 Convoy 成员 |

用 convoy 查看"这批工作的状态是什么？"
用 Rig 状态查看"这个 Rig 里的每个人在做什么？"

## 另见

- [Propulsion Principle](propulsion-principle.md) - 工作者执行模型
- [Mail Protocol](../design/mail-protocol.md) - 通知投递