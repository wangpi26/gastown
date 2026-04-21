+++
name = "tool-updater"
description = "当有可用更新时，通过 Homebrew 升级 beads (bd) 和 dolt"
version = 1

[gate]
type = "cooldown"
duration = "168h"

[tracking]
labels = ["plugin:tool-updater", "category:maintenance"]
digest = true

[execution]
timeout = "10m"
notify_on_failure = true
severity = "medium"
+++

# Tool Updater

检查并应用 `beads`（bd）和 `dolt` 的 Homebrew 更新。

gt 由 `rebuild-gt` 插件单独重建（它从源码构建，而非 Homebrew）。

## 运行

```bash
cd /Users/jeremy/gt/plugins/tool-updater && bash run.sh
```