# Dog 执行模型：命令式 vs Formula 调度

## 状态：活跃设计文档
创建日期：2026-02-27

## 问题陈述

Gas Town 的 dog（daemon 巡逻例程）使用两种执行模型：

1. **命令式 Go**（定时器触发 → Go 代码运行）：Doctor、Reaper、JSONL 备份、Dolt 备份
2. **仅 Formula**（定时器触发 → molecule 被倒入 → ... 什么也没发生）：Compactor（曾是存根）、~~Janitor~~（已移除）

仅 formula 的 dog 是坏的，因为没有 agent 从定时器上下文解释它们的 molecule。
Molecule 系统需要一个空闲的 dog 来执行 formula，但定时器不管 dog 是否可用都会触发。

在 Beads Flows 工作之后，Compactor 已升级为命令式 Go。
Janitor dog 已被完全移除 — 测试基础设施从专用的 3308 端口 Dolt 测试服务器
迁移到 testcontainers-go（Docker），从源头消除了孤立测试数据库问题。

本文档记录了未来的目标执行模型。

## 当前状态（Testcontainers 迁移后）

| Dog | 模型 | 可用？ | 备注 |
|-----|------|--------|------|
| Doctor | 命令式 Go（466 行） | 是 | 7 项健康检查、GC、僵尸杀死 |
| Reaper | 命令式 Go（658 行） | 是 | Close、purge、auto-close、mail purge |
| JSONL Backup | 命令式 Go（619 行） | 是 | Export、scrub、filter、spike detect、push |
| Dolt Backup | 命令式 Go | 是 | 文件系统备份同步 |
| Compactor | 命令式 Go（新） | 是 | 当 commit > 阈值时 flatten + GC |

## 目标模型

### 可靠性关键的 dog 保持命令式 Go

必须按计划运行、无人值守、不依赖 agent 的 dog：

- **Doctor**：健康检查是基础。即使所有 agent 都死了也必须运行。
- **Reaper**：数据卫生不能依赖 agent 可用性。
- **Compactor**：压缩必须按其 24 小时计划确定性地运行。
- **JSONL Backup**：备份完整性不能留给 agent 调度。
- **Dolt Backup**：与 JSONL 相同。

**原则**：如果 dog 的失败会导致 Clown Show，它必须是命令式 Go。

### 增强/机会主义的 dog 迁移到插件调度

失败只是不方便而非灾难性的 dog：

- 未来：装饰性清理、指标收集、日志轮转。

### 插件调度模型

对于插件调度的 dog：

1. 从 daemon `Run()` 循环中移除专用定时器
2. 创建 `plugins/<dog>/plugin.md`，带冷却门控
3. `handleDogs()` 在冷却到期时调度到空闲 dog
4. Dog agent 解释插件 formula 并执行

**关键约束**：`handleDogs()` 调度路径已经存在且可工作。
问题在于基于定时器的 dog 绕过了它。插件 dog 正确地使用它。

## 迁移路径

### 未来的 dog 默认使用插件
- 新的 dog 应该从插件开始，除非是可靠性关键的
- 现有的命令式 dog 保持为 Go（工作正常、经过测试、可靠）

## 决策：不要迁移正常工作的命令式 dog

Doctor、Reaper、Compactor 和备份 dog 作为命令式 Go 可靠工作。
将它们迁移到 formula+agent 会：

1. 添加对 agent 可用性的依赖
2. 引入延迟（agent 启动、formula 解释）
3. 在关键路径上冒回归风险
4. 没有任何收益 — 它们已经工作正常

**唯一应该使用 formula 调度的 dog 是那些 agent 智能
能增加价值的**，或者 dog 的任务本质上非关键的。