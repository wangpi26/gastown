## PR 流程 Polecat 策略

> **Rig 策略 — 与 Formula 指令冲突时以此为准。**

此 Rig 使用 **Polecat → GitHub PR → 人工审核** 流程。推送分支后，你必须在运行 `gt done` 之前确保有一个 GitHub PR 已打开。

这覆盖了嵌入在 `mol-polecat-work` 中的规范 Refinery 合并队列假设。`gt done` 仍是完成信号——但在此 Rig 中，可见的 PR 是审核的门控产物。

### 实现后必需的步骤

1. 显式推送分支（不要依赖 `gt done` 来推送）：
   ```bash
   git push -u origin HEAD
   ```
2. 检查该分支是否已有 PR：
   ```bash
   gh pr view "$(git branch --show-current)" >/dev/null 2>&1
   ```
3. 如果没有 PR，创建一个指向基础分支的 PR：
   ```bash
   gh pr create --fill --base main
   ```
4. 然后再运行 `gt done`。

### 禁止事项

- 没有 PR 就运行 `gt done` — 审核循环会断掉。
- 合并自己的 PR。维护者或合并队列负责合并。
- 直接推送到 `main`。

### 如果 `gh` 命令失败

认证、速率限制、缺少 PR 模板或未知基础分支——不要为了给自己解阻而跳过 PR 创建。升级给你的 Witness：

```bash
gt mail send <rig>/witness -s "HELP: gh pr create failed" -m "Branch: $(git branch --show-current)
Error: <paste>
Tried: <what you attempted>"
```