# Contributor Harness

**Role Directive** 和 **Formula Overlay** 的参考示例，贡献者和运维者可以将其放入自己的 Gas Town 环境中以自定义 Agent 行为——无需修改框架。

这些示例默认不生效。它们是起点，你将其复制到自己的 `~/gt/<rig>/directives/` 或 `~/gt/<rig>/formula-overlays/` 目录，并根据自己的 Rig 需求进行调整。

参见 [`docs/design/directives-and-overlays.md`](../design/directives-and-overlays.md) 了解扩展面的设计（Directive 和 Overlay 如何加载、注入点、优先级、验证）。规范的 prime-time 参考是 `~/gt/docs/PRIMING.md` 的"Role Directives and Formula Overlays"章节。

## 可用的 Harness

| Harness | 功能 |
|---------|------|
| [`polecat-pr-flow/`](polecat-pr-flow/) | 让 Polecat 在运行 `gt done` 之前为其分支打开 GitHub PR。适用于使用 PR 审核流程而非规范 Refinery 合并队列流程的 Rig。 |

## 范围

每个 Harness 故意小巧而专注——足以展示 Directive 或 Overlay 的形态，而不覆盖每个边缘情况。你需要阅读它、复制它、调整它。

Harness **不会**：

- 修改 Go 源码或 Formula 源真文件
- 改变任何未选择加入的人的默认 Agent 行为
- 替代 `gt doctor` 验证——安装后运行 `gt doctor` 确认 Overlay 健康

## 贡献新 Harness

如果你构建了其他运维者可能想要的 Directive 或 Overlay，开一个 PR 在此处添加新目录，包含：

- `README.md` — 功能、安装方法、如何验证它已生效
- Directive（`<role>.md`）和/或 Overlay（`<formula>.toml`）文件
- 保持简洁。Harness 是一个可工作的示例，不是一个完整产品。