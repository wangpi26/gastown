+++
name = "rebuild-gt"
description = "从 gastown 源码重新构建过期的 gt 二进制文件"
version = 2

[gate]
type = "cooldown"
duration = "1h"

[tracking]
labels = ["plugin:rebuild-gt", "rig:gastown", "category:maintenance"]
digest = true

[execution]
timeout = "5m"
notify_on_failure = true
severity = "medium"
+++

# 重新构建 gt 二进制文件

检查 gt 二进制文件是否过期（构建自比 HEAD 更旧的提交）并重新构建。

**安全约束**：此插件只能向前重建（二进制文件是 HEAD 的祖先）
且只能在 main 分支上操作。重建到更旧或分叉的提交曾导致
崩溃循环——每个新会话的启动钩子失败，Witness 重新拉起它，
循环每隔 1-2 分钟重复一次。

## 门控检查

Deacon 在派发前会评估此条件。如果门控关闭，跳过。

## 检测

检查二进制文件是否过期：

```bash
gt stale --json
```

解析 JSON 输出并检查以下字段：
- 如果 `"stale": false` → 记录成功 wisp 并提前退出（二进制文件是最新的）
- 如果 `"safe_to_rebuild": false` → **不要重建**。记录跳过 wisp 并退出。
  这意味着仓库不在 main 分支上，或 HEAD 不是二进制文件
  提交的后代（将是一次降级）。
- 如果 `"safe_to_rebuild": true` → 继续构建

如果 `safe_to_rebuild` 为 false，记录跳过 wisp：
```bash
bd create --wisp-type patrol \
  --labels type:plugin-run,plugin:rebuild-gt,rig:gastown,result:skipped \
  --description "Skipped: not safe to rebuild (forward=$FORWARD, main=$ON_MAIN)" \
  "Plugin: rebuild-gt [skipped]"
```

## 预检

构建前，验证源码仓库是否干净且在 main 分支上：

```bash
cd ~/gt/gastown/mayor/rig
git status --porcelain  # 必须干净
git branch --show-current  # 必须是 "main"
```

如果任一检查失败，跳过重建并记录 wisp。

## 执行

从源码重建（mayor/rig 目录是规范源码位置）：

```bash
cd ~/gt/gastown/mayor/rig && make build && make safe-install
```

**重要**：使用 `make safe-install`（而非 `make install`），以避免
在有活跃会话时重启守护进程。safe-install 替换二进制文件但不会
重启守护进程——会话将在下一个周期中加载新的二进制文件。

## 记录结果

成功时：
```bash
bd create --wisp-type patrol \
  --labels type:plugin-run,plugin:rebuild-gt,rig:gastown,result:success \
  --description "Rebuilt gt: $OLD → $NEW ($N commits)" \
  "Plugin: rebuild-gt [success]"
```

失败时：
```bash
bd create --wisp-type patrol \
  --labels type:plugin-run,plugin:rebuild-gt,rig:gastown,result:failure \
  --description "Build failed: $ERROR" \
  "Plugin: rebuild-gt [failure]"

gt escalate --severity=medium \
  --subject="Plugin FAILED: rebuild-gt" \
  --body="$ERROR" \
  --source="plugin:rebuild-gt"
```