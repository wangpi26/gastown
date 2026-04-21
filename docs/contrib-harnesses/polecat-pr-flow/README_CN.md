# polecat-pr-flow

一个参考 Harness，适用于通过 **GitHub PR** 而非规范 Refinery **合并队列** 流程来门控 Polecat 工作的 Rig。

它指导 Polecat 在最终构建/pre-verify 通过后，推送分支并打开（或确认）GitHub PR，然后再运行 `gt done`。

## 包含内容

| 文件 | 用途 |
|------|------|
| `polecat.md` | PR 流程 Rig 中 Polecat 的 Role Directive。广泛的护栏——适用于 Polecat 运行的任何 Formula。 |
| `mol-polecat-work.toml` | Formula Overlay，在 `mol-polecat-work` 的 `submit-and-exit` 步骤中追加 PR 创建步骤。精准——仅影响此 Formula。 |

两层都有意为之：Directive 设置 Rig 级别的期望（"打开 PR，不要自己合并"），Overlay 将具体命令接入 Polecat 在 `gt prime` 时实际看到的工作流。

## 安装

```bash
# Role Directive（Rig 范围）
mkdir -p ~/gt/<rig>/directives
cp polecat.md ~/gt/<rig>/directives/polecat.md

# Formula Overlay（Rig 范围）
mkdir -p ~/gt/<rig>/formula-overlays
cp mol-polecat-work.toml ~/gt/<rig>/formula-overlays/mol-polecat-work.toml
```

将 `<rig>` 替换为你的 Rig 名称（例如 `gastown`、`longeye`）。对于 Town 级别安装，去掉 `<rig>/` 部分——但这几乎从来不是你想要的，因为不同的 Rig 合理地使用不同的流程。

## 验证是否生效

```bash
# 根据当前 Formula 验证 Overlay 步骤 ID
gt doctor
# Expect: overlay-health: N overlay(s) healthy

# 检查应用 Overlay 后渲染的 Formula
gt formula overlay show mol-polecat-work --rig <rig>

# 查看 prime 时将注入的 Directive 文本
gt directive show polecat --rig <rig>

# 端到端：查看 Polecat 会看到什么
gt prime --explain
# Expect: "Formula overlay: applying 1 override(s) for mol-polecat-work (rig=<rig>)"
```

## 这个做了什么/不做什么

**做了：**

- 告诉 Polecat 在 `gt done` 之前推送并打开 PR
- 设置 Rig 级别策略，PR 是审核产物
- 将 `gh pr create` 失败作为升级上报给 Witness，而非静默跳过

**不做了：**

- 修改 `gt done` 行为（无 Go 变更）
- 通过框架级验证强制 PR 创建（Agent 仍可能不按规矩来）
- 合并 PR（那是维护者/合并队列的事）
- 如果你的 Rig 还使用其他自定义，替代 Directive 或 Overlay——将这些内容合并到你已有的文件中，而非覆盖它们

## 何时 Fork 此 Harness

如果你的 Rig 需要额外的 PR 流程约束（必需审核者、特定标签、CODEOWNERS 执行、`gt done` 前的 CI 检查），复制此 Harness 并调整。它是一个起点模板，不是一个即插即用的产品。