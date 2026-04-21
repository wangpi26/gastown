# Formula 解析架构

> **状态：部分实现** — 基本 formula 解析可用。层级强制、Mol Mall 集成和 HOP 联邦计划中。

> Formula 存在哪里、如何被发现，以及如何扩展到 Mol Mall

## 问题

Formula 目前存在于多个位置，没有明确的优先级：
- `internal/formula/formulas/`（事实来源，嵌入二进制文件）
- `.beads/formulas/`（在运行时由 `gt install` 提供）
- Crew 目录有自己的 `.beads/formulas/`（分歧的副本）

当 agent 运行 `bd cook mol-polecat-work` 时，获得的是哪个版本？

## 设计目标

1. **可预测的解析** — 明确的优先级规则
2. **本地自定义** — 无需 fork 即可覆盖系统默认值
3. **项目特定 formula** — 为协作者提交的工作流
4. **Mol Mall 就绪** — 架构支持远程 formula 安装
5. **联邦就绪** — Formula 可通过 HOP（Highway Operations Protocol）跨 town 共享

## 三层级解析

```
┌─────────────────────────────────────────────────────────────────┐
│                     FORMULA 解析顺序                              │
│                    （最具体的胜出）                                │
└─────────────────────────────────────────────────────────────────┘

层级 1: PROJECT（rig 级）
  位置: <project>/.beads/formulas/
  来源: 提交到项目仓库
  用例: 项目特定工作流（deploy、test、release）
  示例:  ~/gt/gastown/.beads/formulas/mol-gastown-release.formula.toml

层级 2: TOWN（用户级）
  位置: ~/gt/.beads/formulas/
  来源: Mol Mall 安装、用户自定义
  用例: 跨项目工作流、个人偏好
  示例:  ~/gt/.beads/formulas/mol-polecat-work.formula.toml（自定义）

层级 3: SYSTEM（嵌入）
  位置: 编译到 gt 二进制文件
  来源: 构建时的 internal/formula/formulas/
  用例: 默认值、受信模式、回退
  示例:  mol-polecat-work.formula.toml（出厂默认）
```

### 解析算法

```go
func ResolveFormula(name string, cwd string) (Formula, Tier, error) {
    // 层级 1: 项目级（从 cwd 向上查找 .beads/formulas/）
    if projectDir := findProjectRoot(cwd); projectDir != "" {
        path := filepath.Join(projectDir, ".beads", "formulas", name+".formula.toml")
        if f, err := loadFormula(path); err == nil {
            return f, TierProject, nil
        }
    }

    // 层级 2: Town 级
    townDir := getTownRoot() // ~/gt 或 $GT_HOME
    path := filepath.Join(townDir, ".beads", "formulas", name+".formula.toml")
    if f, err := loadFormula(path); err == nil {
        return f, TierTown, nil
    }

    // 层级 3: 嵌入（系统）
    if f, err := loadEmbeddedFormula(name); err == nil {
        return f, TierSystem, nil
    }

    return nil, 0, ErrFormulaNotFound
}
```

### 为什么是这个顺序

**Project 优先**因为：
- 项目维护者最了解他们的工作流
- 协作者通过 git 获得一致的行为
- CI/CD 使用与开发者相同的 formula

**Town 居中**因为：
- 用户自定义覆盖系统默认值
- Mol Mall 安装不需要项目变更
- 跨项目的一致性

**System 是回退**因为：
- 始终可用（编译进去的）
- 出厂重置目标
- "受信"版本

## Formula 身份

### 当前格式

```toml
formula = "mol-polecat-work"
version = 4
description = "..."
```

### 扩展格式（Mol Mall 就绪）

```toml
[formula]
name = "mol-polecat-work"
version = "4.0.0"                          # Semver
author = "steve@gastown.io"                # 作者身份
license = "MIT"
repository = "https://github.com/steveyegge/gastown"

[formula.registry]
uri = "hop://molmall.gastown.io/formulas/mol-polecat-work@4.0.0"
checksum = "sha256:abc123..."              # 完整性验证
signed_by = "steve@gastown.io"             # 可选签名

[formula.capabilities]
# 此 formula 运用哪些能力？用于 agent 路由。
primary = ["go", "testing", "code-review"]
secondary = ["git", "ci-cd"]
```

### 版本解析

当存在多个版本时：

```bash
bd cook mol-polecat-work          # 按层级顺序解析
bd cook mol-polecat-work@4        # 特定主版本
bd cook mol-polecat-work@4.0.0    # 精确版本
bd cook mol-polecat-work@latest   # 显式最新
```

## Crew 目录问题

### 当前状态

Crew 目录（`gastown/crew/max/`）是 rigged repo 的 git worktree。它们有：
- 自己的 `.beads/formulas/`（来自 worktree）
- 这些可能与 `mayor/rig/.beads/formulas/` 分歧

### 修复方案

Crew 不应该有自己的 formula 副本。选项：

**选项 A：符号链接/重定向**
```bash
# crew/max/.beads/formulas -> ../../mayor/rig/.beads/formulas
```
所有 crew 共享 rig 的 formula。

**选项 B：按需提供**
Crew 目录没有 `.beads/formulas/`。解析降级到：
1. Town 级（~/gt/.beads/formulas/）
2. 系统（嵌入的）

**选项 C：Gitignore 排除**
通过 `.gitignore` 从 crew worktree 中排除 `.beads/formulas/`。

**建议：选项 B** — Crew 不需要项目级 formula。他们在项目上工作，
不定义项目的工作流。

## 命令

### 现有

```bash
bd formula list              # 可用 formula（应显示层级）
bd formula show <name>       # Formula 详情
bd cook <formula>            # Formula → Proto
```

### 增强

```bash
# 列出带层级信息
bd formula list
  mol-polecat-work          v4    [project]
  mol-polecat-code-review   v1    [town]
  mol-witness-patrol        v2    [system]

# 显示解析路径
bd formula show mol-polecat-work --resolve
  解析中: mol-polecat-work
  ✓ 找到于: ~/gt/gastown/.beads/formulas/mol-polecat-work.formula.toml
  层级: project
  版本: 4

  检查的解析路径:
  1. [project] ~/gt/gastown/.beads/formulas/ ← 已找到
  2. [town]    ~/gt/.beads/formulas/
  3. [system]  <嵌入>

# 覆盖层级用于测试
bd cook mol-polecat-work --tier=system    # 强制嵌入版本
bd cook mol-polecat-work --tier=town      # 强制 town 版本
```

### 未来（Mol Mall）

```bash
# 从 Mol Mall 安装
gt formula install mol-code-review-strict
gt formula install mol-code-review-strict@2.0.0
gt formula install hop://acme.corp/formulas/mol-deploy

# 管理已安装的 formula
gt formula list --installed              # Town 级有什么
gt formula upgrade mol-polecat-work      # 更新到最新
gt formula pin mol-polecat-work@4.0.0    # 锁定版本
gt formula uninstall mol-code-review-strict
```

## 迁移路径

### 阶段 1：解析顺序（当前）

1. 在 `bd cook` 中实现三层级解析
2. 添加 `--resolve` 标志显示解析路径
3. 更新 `bd formula list` 显示层级
4. 修复 crew 目录（选项 B）

### 阶段 2：Town 级 Formula

1. 确立 `~/gt/.beads/formulas/` 为 town formula 位置
2. 添加 `gt formula` 命令管理 town formula
3. 支持手动安装（复制文件，在 `.installed.json` 中追踪）

### 阶段 3：Mol Mall 集成

1. 定义注册表 API（见 mol-mall-design.md）
2. 从远程实现 `gt formula install`
3. 添加版本锁定和升级流程
4. 添加完整性验证（校验和、可选签名）

### 阶段 4：联邦（HOP）

1. 在 formula schema 中添加能力标签
2. 追踪 formula 执行用于 agent 问责
3. 启用联邦（通过 Highway Operations Protocol 跨 town 共享 formula）
4. 作者归属和验证记录

## 相关文档

- [Mol Mall 设计](mol-mall-design.md) - 注册表架构
- [Molecules](../concepts/molecules.md) - Formula → Proto → Mol 生命周期