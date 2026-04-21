# 持久化 Polecat 池

**Issue:** gt-lpop
**状态:** 设计
**作者:** Mayor

## 问题

Polecat 生命周期中有三个概念被混为一谈：

| 概念 | 生命周期 | 当前行为 |
|------|----------|----------|
| **身份** | 长期存活（名称、CV、账本） | 在 nuke 时被销毁 |
| **沙箱** | 每次分配（worktree、分支） | 在 nuke 时被销毁 |
| **会话** | 临时（Claude 上下文窗口） | = polecat 生命周期 |

后果：
- 推送前 nuke polecat 会导致工作丢失
- 219 个因 worktree 被销毁而产生的过期远程分支
- 分派缓慢（每次分配约 5 秒的 worktree 创建时间）
- 丢失能力记录（CV、完成历史）
- 空闲的 polecat 被视为浪费而被 nuke

## 设计

### 生命周期分离

```
IDENTITY（持久化）
  名称: "furiosa"
  Agent bead: gt-gastown-polecat-furiosa
  CV: 工作历史、语言、完成率
  生命周期: 创建一次，永不销毁（除非明确退役）

SANDBOX（每次分配，可复用）
  Worktree: polecats/furiosa/gastown/
  Branch: polecat/furiosa/<issue>@<timestamp>
  生命周期: 在分配之间同步到 main，不销毁

SESSION（临时）
  Tmux: gt-gastown-furiosa
  Claude 上下文: 在 compaction/handoff 时轮换
  生命周期: 独立于身份和沙箱
```

### 池状态

```
         ┌──────────┐
    ┌───►│  IDLE    │◄──── 同步沙箱到 main
    │    └────┬─────┘      清除 hook
    │         │ gt sling
    │         ▼
    │    ┌──────────┐
    │    │ WORKING  │◄──── 会话活跃，hook 已设置
    │    └────┬─────┘
    │         │ 工作完成
    │         ▼
    │    ┌──────────┐
    └────┤  DONE    │──── 推送分支，提交 MR
         └──────────┘
```

正常路径中没有 `nuke`。Polecat 循环：IDLE → WORKING → DONE → IDLE。

### 池管理

**池大小：** 每个 rig 固定。在 `rig.config.json` 中配置：
```json
{
  "polecat_pool_size": 4,
  "polecat_names": ["furiosa", "nux", "toast", "slit"]
}
```

**初始化：** `gt rig add` 或 `gt polecat pool init <rig>` 创建 N 个具有身份和 worktree 的 polecat。它们以 IDLE 状态启动。

**分派：** `gt sling <bead> <rig>` 找到一个 IDLE 的 polecat（已经通过 `FindIdlePolecat()` 实现），分配工作，启动会话。不需要创建 worktree。

**完成：** 当 polecat 完成工作时：
1. 推送分支到 origin
2. 提交 MR（如果有代码变更）
3. 清除 hook_bead
4. 同步 worktree：`git checkout main && git pull`
5. 将状态设为 IDLE
6. 会话保持活跃或轮换——无关紧要，身份持久存在

### 沙箱同步（DONE → IDLE 转换）

当工作完成且 MR 已合并（或无代码变更）：

```bash
# 在 polecat 的 worktree 中
git checkout main
git pull origin main
git branch -D polecat/furiosa/<old-issue>@<timestamp>
# Worktree 现在是干净的，在 main 上，准备好接受下一次分配
```

当新的工作被 sling 时：
```bash
# 从当前 main 创建新分支
git checkout -b polecat/furiosa/<new-issue>@<timestamp>
# 开始工作
```

不需要 worktree 的添加/删除。只是在现有 worktree 上的分支操作。

### Refinery 集成

Refinery 无需更改。Refinery 仍然：
1. 看到来自 polecat 分支的 MR
2. 审查并合并到 main
3. 删除远程 polecat 分支（新增：添加此步骤）

Polecat 不关心——它已经在 DONE → IDLE 期间本地切换到了 main。

### Witness 集成

Witness 巡逻行为（已发布）：
- 看到空闲 polecat → 健康状态，跳过
- **卡住检测：** Polecat 在 WORKING 状态太久 → 升级（不 nuke）
- **死会话检测：** 会话死亡但状态=WORKING → 重启会话（不 nuke polecat）

### Nuke 变成什么

`gt polecat nuke` 仅保留用于异常情况：
- Polecat worktree 不可恢复地损坏
- 需要回收磁盘空间
- 退役一个 rig

它应该是罕见的手动操作，而非正常工作流的一部分。

### 分支污染解决方案

有了持久化 polecat，分支有明确的归属：
- 活跃分支：polecat 正在其上 WORKING
- 已合并分支：Refinery 在合并后删除
- 废弃分支：polecat 在 DONE → IDLE 时同步到 main，旧分支在本地删除

219 个过期分支来自被 nuke 的 polecat 从未清理。有了持久化 polecat，分支生命周期由 polecat 自身管理。

### 一次性清理

对于现有的 219 个过期分支：
```bash
# 删除所有不属于活跃 polecat 的远程 polecat 分支
git branch -r | grep 'origin/polecat/' | grep -v 'furiosa/gt-ziiu' | grep -v 'nux/gt-uj16' \
  | sed 's/origin\///' | xargs -I{} git push origin --delete {}
```

## 实现阶段

### 阶段 1: 止血 — 已发布
- Witness 不再 nuke 空闲的 polecat
- `gt polecat done` 转换到 IDLE 而非触发 nuke
- Refinery 在合并后删除远程分支

### 阶段 2: 池初始化 — 已推迟
- `gt polecat pool init <rig>` 创建 N 个持久化 polecat
- 池大小在 rig.config.json 中配置
- Worktree 创建一次，在分配之间复用

**状态：** Polecat 通过 `gt sling` 的 `FindIdlePolecat()` 和 `AllocateAndAdd()` 按需分配。预分配没有必要，因为空闲 polecat 会自动复用。池大小强制是未来的优化，而非阻碍。

### 阶段 3: 沙箱同步 — 已发布
- DONE → IDLE 转换将 worktree 同步到 main（`done.go`）
- IDLE → WORKING 通过 `ReuseIdlePolecat()` 创建新分支（不需要 worktree add）
- `gt sling` 通过 `FindIdlePolecat()` 偏好空闲 polecat
- 仅分支复用消除了约 5 秒的 worktree 创建开销

### 阶段 4: 会话独立性 — 已发布
- 会话轮换不影响 polecat 状态
- 死会话由 witness 重启（重启优先策略，不自动 nuke）
- Handoff 跨会话边界保留 polecat 身份
- `gt handoff` 适用于所有角色（Mayor、Crew、Witness、Refinery、Polecat）

### 阶段 5: 一次性清理 — 部分发布
- 合并后 Polecat 分支清理：已发布（已合入 main；PR #2436/#2437 已关闭）
- 合并后 Refinery 通知 mayor：尚未发布
- 池对账（`ReconcilePool`）：尚未实现

### 实现状态总结

| 组件 | 状态 | 关键文件 |
|------|------|----------|
| `gt done`（推送、MR、空闲、沙箱同步） | 已发布 | `internal/cmd/done.go` |
| `gt sling`（空闲复用、仅分支修复） | 已发布 | `internal/cmd/sling.go`、`polecat_spawn.go` |
| `gt handoff`（会话轮换、所有角色） | 已发布 | `internal/cmd/handoff.go` |
| Witness 巡逻（僵尸、过期、孤儿检测） | 已发布 | `internal/witness/handlers.go`、`internal/polecat/manager.go` |
| 清理管道（POLECAT_DONE → MERGE_READY → MERGED） | 已发布 | `internal/witness/handlers.go`、`internal/refinery/engineer.go` |
| 空闲 polecat 异端修复（跳过健康空闲） | 已发布 | `internal/witness/handlers.go` |
| 重启优先策略（不自动 nuke） | 已发布 | `internal/polecat/manager.go` |
| 合并后 Polecat 分支始终删除 | 已发布 | `internal/refinery/engineer.go` |
| 合并后 Refinery 通知 mayor | 未发布 | — |
| 池大小强制 | 已推迟 | — |
| `ReconcilePool()` | 已推迟 | — |
| `gt polecat pool init` 命令 | 已推迟 | — |