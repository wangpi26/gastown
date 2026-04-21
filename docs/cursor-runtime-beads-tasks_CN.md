# Cursor 运行时计划 — Beads 任务（交接）

这些问题跟踪 **Cursor 运行时对等性**、**用户侧文档清晰度**（预设 `cursor` vs CLI `cursor-agent` / `agent`）以及 **`.cursor/` 入门引导**。问题 ID 因数据库而异。

**创建问题（幂等 — 跳过已存在的 `cursor-runtime`+`plan` 问题）：**

```bash
./scripts/cursor-runtime-bd-tasks.sh
```

**完整任务范围**：参见 Cursor 对等计划（你编辑器计划文件夹中的 `cursor_runtime_parity_df5a36d7.plan.md`）中的 **§10a** 和 **§4b**。

**T5（文档 + CLI）** 明确涵盖：

- `gt config` / `internal/cmd/config.go` 帮助 — 列出**所有**内置预设，不仅是 claude/gemini/codex。
- **README** 前提条件 — 可选的 **Cursor Agent CLI** 安装；明确**预设 `cursor`** 与二进制文件的区别。
- **docs/INSTALLING.md**、**docs/reference.md** — 与 README 相同的内置列表；关于 **`cursor`** → `cursor-agent` 的简短说明。

**贡献指南**：[`CONTRIBUTING.md`](../CONTRIBUTING.md)。不要在仓库根目录添加 `.beads/issues.jsonl`（CI 会报错）。持久化 Beads 数据库变更时使用 `bd vc commit`。

**迁移**：如果你使用旧版脚本创建了任务，请在 `bd` 中**重命名 T5** 以匹配计划 §10a 中的表格，或关闭重复项。