+++
name = "dolt-backup"
description = "带变更检测的智能 Dolt 数据库备份"
version = 2

[gate]
type = "cooldown"
duration = "15m"

[tracking]
labels = ["plugin:dolt-backup", "category:data-safety"]
digest = true

[execution]
timeout = "5m"
notify_on_failure = true
severity = "high"
+++

# Dolt Backup

通过 `dolt backup sync` 将生产 Dolt 数据库同步到文件系统备份。
通过 `run.sh` 执行——无需 AI 介入。

## 功能说明

1. 对每个生产数据库（hq, beads, gt）：将 HEAD 哈希与上次备份对比
2. 跳过未变更的数据库
3. 对有变更的数据库执行 `dolt backup sync`
4. 仅在实际备份操作失败时才上报（FAILED > 0）

## 用法

```bash
./run.sh                          # 正常执行
./run.sh --dry-run                # 仅报告不同步
./run.sh --databases hq,beads    # 仅处理指定数据库
```