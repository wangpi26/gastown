# Gastown/Beads 清理命令参考

Gastown/beads 生态系统中所有清理相关命令的完整目录，按范围和严重程度分类组织。

---

## 进程清理

| 命令 | 功能说明 |
|------|----------|
| `gt cleanup` | 终止未绑定到活跃 tmux 会话的孤立 Claude 进程 |
| `gt orphans procs list` | 列出孤立的 Claude 进程（PPID=1） |
| `gt orphans procs kill` | 终止孤立的 Claude 进程（`--aggressive` 用于 tmux 验证模式） |
| `gt deacon cleanup-orphans` | 终止孤立的 Claude 子代理进程（无控制 TTY） |
| `gt deacon zombie-scan` | 查找/终止不在活跃 tmux 会话中的僵尸 Claude 进程 |

## Polecat（代理沙箱）清理

| 命令 | 功能说明 |
|------|----------|
| `gt polecat remove <rig>/<polecat>` | 移除 polecat 工作树/目录（若会话正在运行则失败） |
| `gt polecat nuke <rig>/<polecat>` | 核弹级操作：终止会话、删除工作树、删除分支、关闭 Bead |
| `gt polecat nuke <rig> --all` | 核弹级清理某 rig 下的所有 Polecat |
| `gt polecat gc <rig>` | GC 过期的 polecat 分支（孤立的、旧时间戳的） |
| `gt polecat stale <rig>` | 检测过期的 Polecat；`--cleanup` 自动核弹清理 |
| `gt polecat check-recovery` | 核弹清理前的安全检查（SAFE_TO_NUKE vs NEEDS_RECOVERY） |
| `gt polecat identity remove <rig> <name>` | 移除一个 polecat 身份 |
| `gt done` | Polecat 自清理：推送分支、提交 MR（默认行为）、自毁工作树、终止自身会话。对于 `--status ESCALATED\|DEFERRED` 或 `no_merge` 路径跳过 MR |

## Git 产物清理

| 命令 | 功能说明 |
|------|----------|
| `gt prune-branches` | 移除过期的本地 polecat 跟踪分支（`git fetch --prune` + 安全删除） |
| `gt orphans` | 查找从未合并的孤立提交（仅检测） |
| `gt orphans kill` | 清理孤立提交（`git gc --prune=now`）+ 终止孤立进程 |

## Rig 级清理

| 命令 | 功能说明 |
|------|----------|
| `gt rig reset` | 重置交接内容、过期邮件、孤立的 in_progress 问题 |
| `gt rig reset --handoff` | 仅清除交接内容 |
| `gt rig reset --mail` | 仅清除过期邮件 |
| `gt rig reset --stale` | 重置孤立的 in_progress 问题 |
| `gt rig remove <name>` | 从注册表注销 Rig，清理 Beads 路由 |
| `gt rig shutdown <rig>` | 停止所有代理：Polecat、Refinery、Witness |
| `gt rig stop <rig>...` | 停止一个或多个 Rig |
| `gt rig restart <rig>...` | 停止后重启（停止阶段会执行清理） |

## 全镇关停

| 命令 | 功能说明 |
|------|----------|
| `gt down` | 停止所有基础设施（Refinery、Witness、Mayor、Boot、Deacon、Daemon、Dolt） |
| `gt down --polecats` | 同时停止所有 Polecat 会话 |
| `gt down --all` | 完整关停，含孤立进程清理和验证 |
| `gt down --nuke` | 终止整个 tmux 服务器（破坏性操作 - 同时终止非 GT 会话） |
| `gt shutdown` | "收工"模式 - 停止代理并移除 Polecat 工作树/分支。标志控制激进程度（`--graceful`、`--force`、`--nuclear`、`--polecats-only` 等） |

## Crew 工作空间清理

| 命令 | 功能说明 |
|------|----------|
| `gt crew stop [name]` | 停止 Crew 的 tmux 会话 |
| `gt crew restart [name]` | 终止并重新启动 Crew（"全新状态"，无交接邮件） |
| `gt crew remove <name>` | 移除工作空间，关闭代理 Bead |
| `gt crew remove <name> --purge` | 彻底清除：删除代理 Bead、取消分配 Beads、清除邮件 |
| `gt crew pristine [name]` | 将工作空间与远程同步（`git pull`） |

## 临时数据/事件清理

| 命令 | 功能说明 |
|------|----------|
| `gt compact` | 基于 TTL 的压缩：提升/删除超过 TTL 的 Wisp |
| `gt krc prune` | 从 KRC 事件存储中清理过期事件 |
| `gt krc config reset` | 将 KRC TTL 配置重置为默认值 |
| `gt krc decay` | 显示取证价值衰减报告（清理指导） |

## Dolt 数据库清理

| 命令 | 功能说明 |
|------|----------|
| `gt dolt cleanup` | 从 `.dolt-data/` 中移除孤立数据库 |
| `gt dolt stop` | 停止 Dolt SQL 服务器 |
| `gt dolt rollback [backup-dir]` | 从备份恢复 `.beads`，重置元数据 |

## Bead/Hook 清理

| 命令 | 功能说明 |
|------|----------|
| `gt close <bead-id>` | 关闭 Bead（生命周期终止） |
| `gt unsling` / `gt unhook` | 从代理的 Hook 中移除工作，将 Bead 状态重置为 "open" |
| `gt hook clear` | unsling 的别名 |

## Dog（基础设施工作者）清理

| 命令 | 功能说明 |
|------|----------|
| `gt dog remove <name>` | 移除工作树和 Dog 目录 |
| `gt dog remove --all` | 移除所有 Dog |
| `gt dog clear <name>` | 将卡住的 Dog 重置为空闲状态 |
| `gt dog done [name]` | 标记 Dog 为完成，清除工作字段 |

## Convoy 清理

| 命令 | 功能说明 |
|------|----------|
| `gt convoy close <id>` | 关闭一个 Convoy Bead |
| `gt convoy land <id>` | 关闭 Convoy，清理 Polecat 工作树，发送完成通知 |

## 邮件清理

| 命令 | 功能说明 |
|------|----------|
| `gt mail delete <msg-id>` | 删除特定消息 |
| `gt mail archive <msg-id>` | 归档消息（`--stale` 用于过期消息） |
| `gt mail clear [target]` | 删除某收件箱的所有消息（全镇静默） |

## 其他状态清理

| 命令 | 功能说明 |
|------|----------|
| `gt namepool reset` | 释放所有已占用的 Polecat 名称 |
| `gt checkpoint clear` | 移除检查点文件 |
| `gt issue clear` | 清除 tmux 状态行上的问题 |
| `gt doctor --fix` | 自动修复：孤立会话、Wisp GC、过期重定向、工作树有效性 |

## 系统级清理

| 命令 | 功能说明 |
|------|----------|
| `gt disable --clean` | 禁用 Gastown + 移除 Shell 集成 |
| `gt shell remove` | 从 RC 文件中移除 Shell 集成 |
| `gt config agent remove <name>` | 移除自定义代理定义 |
| `gt uninstall` | 完整移除：Shell 集成、包装脚本、状态/配置/缓存目录 |
| `make clean` | 移除编译的 `gt` 二进制文件 |

## 脚本

| 命令 | 功能说明 |
|------|----------|
| `scripts/migration-test/reset-vm.sh` | 将虚拟机恢复到初始 v0.5.0 状态（测试环境） |

## 内部机制（自动/副作用）

| 函数 | 所在位置 | 功能说明 |
|------|----------|----------|
| `cleanupOrphanedProcesses()` | `polecat.go` | 在 nuke/stale 清理后自动运行 |
| `selfNukePolecat()` | `done.go` | 在 `gt done` 期间自毁工作树 |
| `selfKillSession()` | `done.go` | 自终止 tmux 会话 |
| `rollbackSlingArtifacts()` | `sling.go` | 清理部分 sling 失败的产物 |
| `cleanStaleHookedBeads()` | `unsling.go` | 修复卡在 "hooked" 状态的 Bead |
| `gt signal stop` | `signal_stop.go` | 在轮次边界清除停止状态临时文件 |
| `make install` | `Makefile` | 移除过期的 `~/go/bin/gt` 和 `~/bin/gt` 二进制文件 |

---

## 清理层级（从低到高严重程度）

| 层级 | 范围 | 关键命令 |
|------|------|----------|
| **L0** | 临时数据 | `gt compact`、`gt krc prune`（基于 TTL 的生命周期） |
| **L1** | 进程 | `gt cleanup`、`gt orphans procs kill`、`gt deacon cleanup-orphans` |
| **L2** | Git 产物 | `gt prune-branches`、`gt polecat gc`、`gt orphans kill` |
| **L3** | 代理/会话 | `gt polecat nuke`、`gt done`、`gt shutdown`、`gt down` |
| **L4** | 工作空间 | `gt rig reset`、`gt doctor --fix`、`gt dolt cleanup` |
| **L5** | 系统 | `gt uninstall`、`gt disable --clean` |

**总计：清理生态系统包含约 62 个命令/函数。**