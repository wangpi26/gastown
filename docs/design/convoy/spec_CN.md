# Convoy Manager 规格

> Daemon 驻留的事件驱动完成和搁浅 convoy 恢复。

**状态**：实现完成（所有故事 DONE）
**所有者**：Daemon 子系统
**相关**：[convoy-lifecycle.md](convoy-lifecycle.md) | [convoy-manager.md](../daemon/convoy-manager.md)

---

## 1. 问题陈述

Convoy 分组工作但不驱动工作。完成依赖于基于轮询的单个 Deacon 巡逻周期运行 `gt convoy check`。当 Deacon 宕机或缓慢时，convoy 停滞。工作完成但循环永不落定：

```
创建 -> 跟踪 -> 执行 -> Issue 关闭 -> ??? -> Convoy 关闭
```

该缺口需要三个能力：
1. **事件驱动完成** — 对 issue 关闭做出反应，而非轮询。
2. **搁浅恢复** — 捕获事件驱动路径遗漏的 convoy（崩溃、重启、过期状态）。
3. **冗余观察** — 多个 agent 检测完成，因此单一故障不阻塞循环。

---

## 2. 架构

### 2.1 ConvoyManager（daemon 驻留）

`gt daemon` 内的两个 goroutine：

| Goroutine | 触发 | 功能 |
|-----------|---------|--------------|
| **事件轮询** | `GetAllEventsSince` 每 5s，所有 rig 存储 + hq | 检测 `EventClosed` / `EventStatusChanged(closed)`，调用 `CheckConvoysForIssue` |
| **搁浅扫描** | `gt convoy stranded --json` 每 30s | 通过 `gt sling` 分派第一个就绪 issue，通过 `gt convoy check` 自动关闭空 convoy |

两个 goroutine 都可 context 取消，通过 `sync.WaitGroup` 协调关闭。

事件轮询为所有已知 rig 打开 beads 存储（通过 `routes.jsonl`）以及 town 级别 hq 存储。已停驻/已停靠的 rig 在轮询期间被跳过。Convoy 查找始终使用 hq 存储，因为 convoy 是 `hq-*` 前缀的。每个存储有独立的事件 ID 高水位标记。

### 2.2 共享观察者（`convoy.CheckConvoysForIssue`）

由 daemon 事件轮询调用的共享函数：

| 观察者 | 何时 | 入口点 |
|----------|------|-------------|
| **Daemon 事件轮询** | 任何 rig 存储或 hq 中检测到关闭事件 | `convoy.CheckConvoysForIssue`（传入 hq 存储） |

共享函数：
1. 查找跟踪已关闭 issue 的 convoy（在 hq 存储上通过 SDK `GetDependentsWithMetadata`，按 `tracks` 类型过滤）
2. 跳过已关闭的 convoy
3. 对开放 convoy 运行 `gt convoy check <id>`
4. 如果 check 后 convoy 仍开放，通过 `gt sling` 分配下一个就绪 issue
5. 幂等 — 对同一事件多次调用安全

### 2.3 关键设计决策

| 决策 | 理由 |
|----------|-----------|
| SDK 轮询（非 CLI 流式） | 避免子进程生命周期管理，更简单的重启语义 |
| 高水位标记（atomic int64） | 单调递增，无重复事件处理 |
| 每次扫描每个 convoy 分派一个 issue | 防止批量溢出；下一个 issue 在下一个关闭事件时分派 |
| 搁浅扫描作为安全网 | 捕获事件驱动路径遗漏的 convoy（崩溃恢复） |
| Nil 存储仅禁用事件轮询 | 搁浅扫描在没有 beads SDK 的情况下仍可用（降级模式） |
| 解析二进制路径（PATCH-006） | ConvoyManager 在启动时解析 `gt`/`bd` 以避免 PATH 问题 |

---

## 3. 故事

### 图例

| 状态 | 含义 |
|--------|---------|
| DONE | 已实现、测试、集成 |
| DONE-PARTIAL | 已实现但有已知缺口 |
| TODO | 尚未实现 |

### 质量门控（适用于所有实现故事）

每个实现故事必须通过以下命令：
- `go test ./...`
- `golangci-lint run`

---

### S-01：事件驱动 convoy 完成检测 [DONE]

**描述**：当 issue 关闭时，daemon 通过 SDK 轮询检测关闭事件并触发 convoy 完成检查。

**实现**：`ConvoyManager.runEventPoll` + `pollEvents` 在 `convoy_manager.go`

**验收标准**：
- [x] 以 5 秒间隔轮询 `GetAllEventsSince`
- [x] 检测 `EventClosed` 事件
- [x] 检测 `new_value == "closed"` 的 `EventStatusChanged`
- [x] 跳过非关闭事件（不触发关闭路径）
- [x] 跳过 `issue_id` 为空的事件
- [x] 对每个检测到的关闭调用 `convoy.CheckConvoysForIssue`
- [x] 高水位标记单调递增（无重复处理）
- [x] `GetAllEventsSince` 出错时记录日志并在下一间隔重试
- [x] Nil 存储禁用事件轮询（立即返回）
- [x] Context 取消干净退出

**测试**：
- [x] `TestEventPoll_DetectsCloseEvents` — 真实 beads 存储，创建+关闭 issue，验证日志
- [x] `TestEventPoll_SkipsNonCloseEvents` — 仅创建，无关闭检测

**纠正备注**："零副作用"负面断言已通过 `TestEventPoll_SkipsNonCloseEvents_NegativeAssertion` 添加（验证无子进程调用、无关闭检测、无非关闭事件的 convoy 活动）。原在 S-11 中跟踪；现已解决。

---

### S-02：周期性搁浅 convoy 扫描 [DONE]

**描述**：每 30 秒，扫描搁浅 convoy（未分配工作或为空）。分派就绪工作或自动关闭空 convoy。

**实现**：`ConvoyManager.runStrandedScan` + `scan` + `findStranded` + `feedFirstReady` + `closeEmptyConvoy` 在 `convoy_manager.go`

**验收标准**：
- [x] 启动时立即运行，然后每 `scanInterval` 运行
- [x] 调用 `gt convoy stranded --json` 并解析输出
- [x] 对 `ready_count > 0` 的 convoy：通过 `gt sling <id> <rig> --no-boot` 分派第一个就绪 issue
- [x] 对 `ready_count == 0` 的 convoy：运行 `gt convoy check <id>` 自动关闭
- [x] 通过 `beads.ExtractPrefix` + `beads.GetRigNameForPrefix` 解析 issue 前缀到 rig 名称
- [x] 跳过前缀未知的 issue（记录日志）
- [x] 跳过 rig 未知的 issue（记录日志）
- [x] 分派失败后继续处理下一个 convoy
- [x] Context 取消可在迭代中途退出
- [x] 扫描间隔为 0 或负数时默认为 30s

**测试**：
- [x] `TestScanStranded_FeedsReadyIssues` — mock gt，验证 sling 日志文件
- [x] `TestScanStranded_ClosesEmptyConvoys` — mock gt，验证 check 日志文件
- [x] `TestScanStranded_NoStrandedConvoys` — 空列表：断言 sling 日志不存在、check 日志不存在、日志中无 convoy 活动
- [x] `TestScanStranded_DispatchFailure` — 第一次 sling 失败，扫描继续
- [x] `TestConvoyManager_ScanInterval_Configurable` — 0 → 默认，自定义保留
- [x] `TestStrandedConvoyInfo_JSONParsing` — JSON 往返

---

### S-03：共享 convoy 观察者函数 [DONE]

**描述**：用于检查 convoy 完成和分配下一个就绪 issue 的共享函数，可从任何观察者调用。

**实现**：`CheckConvoysForIssue` + `feedNextReadyIssue` 在 `convoy/operations.go`

**验收标准**：
- [x] 通过 `GetDependentsWithMetadata` 按 `tracks` 类型过滤查找跟踪 convoy
- [x] 过滤掉 `blocks` 依赖
- [x] 跳过已关闭的 convoy
- [x] 对开放 convoy 运行 `gt convoy check <id>`
- [x] Check 后仍开放：通过 `gt sling` 分配下一个就绪 issue
- [x] Ready = open 状态 + 无负责人
- [x] 一次分配一个 issue（第一个匹配）
- [x] 通过 `extractIssueID` 处理 `external:prefix:id` 包装格式
- [x] 通过 `GetIssuesByIDs` 刷新 issue 状态以实现跨 rig 准确性
- [x] 如果最新状态不可用则回退到依赖元数据
- [x] Nil 存储立即返回
- [x] Nil logger 替换为 no-op（不 panic）
- [x] 幂等（对同一 issue 多次调用安全）
- [x] 返回检查过的 convoy ID 列表

**测试**：
- [x] `TestGetTrackingConvoys_FiltersByTracksType` — 真实存储，blocks 被过滤
- [x] `TestIsConvoyClosed_ReturnsCorrectStatus` — 真实存储，open vs closed
- [x] `TestExtractIssueID` — 所有包装变体
- [x] `TestFeedNextReadyIssue_SkipsNonOpenIssues` — 过滤逻辑
- [x] `TestFeedNextReadyIssue_FindsReadyIssue` — 第一个匹配
- [x] `TestCheckConvoysForIssue_NilStore` — 返回 nil
- [x] `TestCheckConvoysForIssue_NilLogger` — 不 panic
- [x] `TestCheckConvoysForIssueWithAutoStore_NoStore` — 不存在路径，nil

---

### S-04：Witness 集成 [REMOVED]

**描述**：Witness convoy 观察者被移除。Daemon 的多 rig 事件轮询（监控所有 rig 数据库 + hq）为任何 rig 的关闭事件提供事件驱动覆盖。搁浅扫描（30s）提供备份。Witness 的核心工作是 polecat 生命周期管理 — convoy 跟踪与之正交。

**历史**：最初在 `handlers.go` 中有 6 个 `CheckConvoysForIssueWithAutoStore` 调用点（1 个 post-merge，5 个僵尸清理路径）。全部是纯副作用通知 hook。当 daemon 获得多 rig 事件轮询时被移除。

---

### S-05：Refinery 集成 [REMOVED]

**描述**：Refinery convoy 观察者被移除。Daemon 事件轮询（5s）和 witness 观察者提供足够覆盖。Refinery 观察者在整个功能生命周期中静默损坏（S-17：错误根路径），无可见影响，确认其他两个观察者足够。由于 beads 不可用会禁用整个 town（不只是 convoy 检查），第三个观察者的"降级模式"理由不成立。

**历史**：最初在合并后调用 `CheckConvoysForIssueWithAutoStore`。S-17 发现它传递了 rig path 而非 town root。S-18 修复了它。随后因不必要冗余被移除。

---

### S-06：Daemon 生命周期集成 [DONE]

**描述**：ConvoyManager 与 daemon 干净地启动和停止。

**实现**：集成在 `daemon.go` 的 `Run()` 和 `shutdown()` 方法中。

**验收标准**：
- [x] 在 daemon 启动时打开 beads 存储（不可用时为 nil）
- [x] 传递已解析的 `gtPath`/`bdPath` 给 ConvoyManager
- [x] 传递 `logger.Printf` 用于 daemon 日志集成
- [x] 在 feed curator 之后启动
- [x] 在 beads 存储关闭前停止（正确关闭顺序）
- [x] 在有界时间内完成停止（无挂起）

**测试**：
- [x] `TestDaemon_StartsManagerAndScanner` — 用 mock 二进制文件启动 + 停止
- [x] `TestDaemon_StopsManagerAndScanner` — 停止在 5s 内完成

---

### S-07：MR bead 中的 Convoy 字段 [DONE]

**描述**：Merge-request bead 携带 convoy 跟踪字段，用于优先级评分和饥饿预防。

**实现**：`beads/fields.go` 中 `MRFields` 结构体的 `ConvoyID` 和 `ConvoyCreatedAt`

**验收标准**：
- [x] `convoy_id` 字段已解析和格式化
- [x] `convoy_created_at` 字段已解析和格式化
- [x] 支持下划线、连字符和驼峰式键变体
- [x] Refinery 用于合并队列优先级评分

---

### S-08：ConvoyManager 生命周期安全 [DONE]

**描述**：Start/Stop 在边缘条件下安全。

**验收标准**：
- [x] `Stop()` 幂等（双重调用不 deadlock）
- [x] 在 `Start()` 之前调用 `Stop()` 立即返回
- [x] `Start()` 防止双重调用（`convoy_manager.go:50-51,80-83` 中的 `atomic.Bool` 和 `CompareAndSwap`）

**测试**：
- [x] `TestManagerLifecycle_StartStop` — 基本启动 + 停止
- [x] `TestConvoyManager_DoubleStop_Idempotent` — 双重停止
- [x] `TestStart_DoubleCall_Guarded` — 第二次 Start() 是无操作，记录警告

---

### S-09：子进程 context 取消 [DONE]

**描述**：ConvoyManager 和观察者中的所有子进程调用传播 context 取消，因此 daemon 关闭不会被挂起的子进程阻塞。

**实现**：所有 `exec.Command` 调用替换为 `exec.CommandContext`。通过 `setProcessGroup` + `syscall.Kill(-pid, SIGKILL)` 杀死进程组，防止孤儿子进程。

**验收标准**：
- [x] ConvoyManager 中的所有 `exec.Command` 调用使用 `exec.CommandContext(m.ctx, ...)`（`convoy_manager.go:200,241,257`）
- [x] operations.go 中的所有 `exec.Command` 调用接受并使用 context 参数
- [x] Daemon 关闭在有界时间内完成，即使 `gt` 子进程挂起（`convoy_manager_integration_test.go:154-206`）
- [x] 被杀死的子进程不留下孤儿子进程（`convoy_manager.go`、`operations.go`）

---

### S-10：operations.go 中解析的二进制路径 [DONE]

**描述**：观察者子进程调用使用解析的二进制路径而非裸 `"gt"`，以避免依赖 PATH 的行为漂移。

**实现**：`CheckConvoysForIssue` 通过 `exec.LookPath("gt")` 解析，回退到裸 `"gt"`。将 `gtPath` 参数传递给 `operations.go` 中的 `runConvoyCheck` 和 `dispatchIssue`。

**验收标准**：
- [x] `runConvoyCheck` 和 `dispatchIssue` 接受 `gtPath` 参数
- [x] `CheckConvoysForIssue` 传递解析的路径
- [x] 所有调用者已更新：daemon（解析的 `m.gtPath`）
- [x] 解析失败时回退到裸 `"gt"`

---

### S-11：测试缺口 — 优先级 1（高影响范围不变式） [DONE]

**描述**：填补测试计划分析中识别的核心不变式测试缺口。

**新增测试**：

| 测试 | 证明内容 |
|------|---------------|
| `TestFeedFirstReady_MultipleReadyIssues_DispatchesOnlyFirst` | 3 个就绪 issue → sling 日志仅包含第一个 issue ID |
| `TestFeedFirstReady_UnknownPrefix_Skips` | Issue 前缀不在 routes.jsonl 中 → 不调用 sling，记录错误 |
| `TestFeedFirstReady_UnknownRig_Skips` | 前缀已解析但 rig 查找失败 → 不调用 sling |
| `TestFeedFirstReady_EmptyReadyIssues_NoOp` | `ReadyIssues=[]` 但 `ReadyCount>0` → 不崩溃，不分派 |
| `TestEventPoll_SkipsNonCloseEvents_NegativeAssertion` | 断言零副作用（无子进程调用，无 convoy 活动） |

**验收标准**：
- [x] 5 个测试全部通过
- [x] 每个测试有显式断言（无无断言的"不 panic"测试）

---

### S-12：测试缺口 — 优先级 2（错误路径） [DONE]

**描述**：覆盖之前无测试的错误路径。

**新增测试**：

| 测试 | 证明内容 |
|------|---------------|
| `TestFindStranded_GtFailure_ReturnsError` | `gt convoy stranded` 非零退出 → 返回错误 |
| `TestFindStranded_InvalidJSON_ReturnsError` | `gt` 返回非 JSON stdout → 返回解析错误 |
| `TestScan_FindStrandedError_LogsAndContinues` | `scan()` 在 `findStranded` 失败时不 panic |
| `TestPollEvents_GetAllEventsSinceError` | `GetAllEventsSince` 返回错误 → 记录日志，下一间隔重试 |

**验收标准**：
- [x] 4 个测试全部通过
- [x] 日志断言中验证错误消息

---

### S-13：测试缺口 — 优先级 3（生命周期边缘情况） [DONE]

**描述**：覆盖测试计划中识别的生命周期边缘情况。

**新增测试**：

| 测试 | 证明内容 |
|------|---------------|
| `TestScan_ContextCancelled_MidIteration` | 大型搁浅列表 + 中途取消 → 干净退出 |
| `TestScanStranded_MixedReadyAndEmpty` | 异构搁浅列表正确路由 ready→sling 和 empty→check |
| `TestStart_DoubleCall_Guarded` — 第二次 `Start()` 是无操作，记录警告 |

**验收标准**：
- [x] 3 个测试全部通过

---

### S-14：测试基础设施改进 [DONE]

**描述**：改进测试工具质量，减少重复。

**项目**：

| 项目 | 影响 |
|------|--------|
| 提取 `mockGtForScanTest(t, opts)` 辅助函数 | 被 5+ 扫描测试使用（`convoy_manager_test.go:57-117`） |
| 为所有 mock 脚本添加副作用日志器 | 所有 mock 脚本写入调用日志，用于正面/负面断言 |
| 修复 `DispatchFailure` 测试日志器以捕获 `fmt.Sprintf(format, args...)` | 断言验证带正确 ID 的渲染消息 |
| 将 `TestScanStranded_NoStrandedConvoys` 转为负面测试 | 断言 sling/check 日志不存在 |

**验收标准**：
- [x] 共享 mock 构建器存在且被 >= 3 个扫描测试使用（5 个测试使用）
- [x] 所有 mock 脚本写入调用日志文件（负面测试可断言为空）
- [x] convoy_manager_test.go 中无无断言测试

---

### S-15：文档更新 [DONE]

**描述**：更新过期文档以反映当前实现。

**项目**：

| 文档 | 问题 |
|----------|-------|
| `docs/design/daemon/convoy-manager.md` | Mermaid 图显示 `bd activity --follow` 但实现使用 SDK `GetAllEventsSince` 轮询 |
| `docs/design/daemon/convoy-manager.md` | 文本说"流错误时以 5s 退避重启" — 无流，无退避；是轮询-重试循环 |
| `docs/design/convoy/testing.md` | 行"流失败触发退避 + 重试循环"已过期（无流） |
| `docs/design/convoy/testing.md` | `TestDoubleStop_Idempotent` 列为缺口但现已存在 |
| `docs/design/convoy/convoy-lifecycle.md` | 观察者表列出 Deacon 为主要第三观察者；实现使用 Refinery |
| `docs/design/convoy/convoy-lifecycle.md` | "无手动关闭"声明已过期；`gt convoy close --force` 存在 |
| `docs/design/convoy/convoy-lifecycle.md` | 到 convoy 概念文档的相对链接已断（`../concepts/...`） |
| `docs/design/convoy/spec.md` | 文件映射测试计数与当前套件偏离 |

**验收标准**：
- [x] Mermaid 图显示 SDK 轮询架构
- [x] 文本准确描述轮询-重试语义
- [x] testing.md 反映当前测试清单
- [x] 生命周期观察者和手动关闭部分匹配实现
- [x] 生命周期文档中的断链已修复
- [x] Spec 文件映射计数和命令列表匹配当前源码

**完成备注**：在此审查过程中完成；refinery root-path 语义的剩余歧义在 S-17 中单独跟踪。

---

### S-16：DONE 故事的纠正跟进 [DONE]

**描述**：为标记为 DONE 的故事中发现的不准确添加显式纠正任务，不改变实现状态本身。

**理由**：DONE 故事在附近重构后仍可能包含过期的支撑叙述或清单细节。纠正被显式跟踪，以避免静默编辑历史交付声明。

**范围**：
- S-01：澄清非关闭事件"零副作用"目前在添加负面子进程断言前是部分的（见 S-11）
- S-04：用 `handlers.go` 中的符号/节锚点替换脆弱的行号调用点引用
- S-05：验证/澄清 refinery `townRoot` vs rig-path 参数假设，用于 `CheckConvoysForIssueWithAutoStore`

**验收标准**：
- [x] 纠正备注被添加到受影响的 DONE 故事，不降级状态
- [x] S-04 调用点引用不再依赖固定行号
- [x] S-05 包含关于 root-path 假设和验证状态的显式备注

**状态备注**：所有纠正备注已更新。S-01 负面断言测试现在存在（已解决）。S-04 调用点已使用语义描述。S-05 备注已更新以反映 S-17 验证发现（错误路径，S-18 中修复）。

---

### S-17：Refinery 观察者 root-path 验证 [DONE]

**描述**：验证 refinery 将 `e.rig.Path` 传入 `CheckConvoysForIssueWithAutoStore` 对 convoy 可见性是否正确。

**上下文**：
- 观察者辅助函数在 `<townRoot>/.beads/dolt` 下打开 beads 存储
- Refinery 当前传递 rig path，而非显式 town root

**发现**：

当前行为**不正确**。`e.rig.Path` 是 rig 级别路径（`<townRoot>/<rigName>`），在 `rig/manager.go` 中设置为 `filepath.Join(m.townRoot, name)`。`OpenStoreForTown` 构造 `<path>/.beads/dolt`，因此 refinery 打开 `<townRoot>/<rigName>/.beads/dolt` 而非 `<townRoot>/.beads/dolt`。

Rig 级别的 `.beads/` 目录通常包含重定向文件（指向 `mayor/rig/.beads`）或 rig 范围的元数据 — 不是持有 convoy 数据的 town 级别 Dolt 数据库。因此 `beadsdk.Open` 要么失败（无 `dolt/` 目录），要么打开不包含 convoy 跟踪依赖的 rig 范围存储。两种情况下 `CheckConvoysForIssueWithAutoStore` 都静默返回 nil，实际上**禁用了 refinery 观察者的 convoy 检查**。

其他观察者正确处理：
- **Witness**：在调用前通过 `workspace.Find(workDir)` 解析 town root
- **Daemon**：直接传递 `d.config.TownRoot`

**需要修复**：在传递给 `CheckConvoysForIssueWithAutoStore` 前，通过 `workspace.Find` 从 `e.rig.Path` 解析 town root，匹配 witness 模式。见 S-18 实现。

**验收标准**：
- [x] 行为预期已记录（town root vs rig root）
- [x] 如果当前行为正确，添加代码注释/spec 备注解释原因
- [x] 如果不正确，创建实现跟进故事并交叉引用 → S-18

---

### S-18：修复 refinery convoy 观察者 town-root 路径 [DONE]

**描述**：修复 refinery 的 `CheckConvoysForIssueWithAutoStore` 调用以传递 town root 而非 rig path，使 convoy 检查实际打开正确的 beads 存储。

**上下文**：由 S-17 验证识别。Refinery 传递了 `e.rig.Path`（`<townRoot>/<rigName>`），但函数期望 town root。这静默禁用了 refinery 的 convoy 观察。

**实现**：`engineer.go` 现在在调用 `CheckConvoysForIssueWithAutoStore` 前通过 `workspace.Find(e.rig.Path)` 解析 town root，匹配 witness 模式。

**验收标准**：
- [x] Refinery 在调用 `CheckConvoysForIssueWithAutoStore` 前通过 `workspace.Find(e.rig.Path)` 解析 town root
- [x] 模式匹配 witness 实现（如果找不到 town root 则优雅回退）
- [x] `workspace` 包导入已添加到 `engineer.go`
- [x] 修复后移除 `engineer.go` 中的 BUG(S-17) 注释

---

## 4. 关键不变式

| # | 不变式 | 类别 | 影响范围 | 故事 | 已测试？ |
|---|-----------|----------|-------------|-------|---------|
| I-1 | Issue 关闭触发 `CheckConvoysForIssue` | 数据 | 高 | S-01 | 是 |
| I-2 | 非关闭事件产生零副作用 | 安全 | 低 | S-01 | 是（`TestEventPoll_SkipsNonCloseEvents_NegativeAssertion`） |
| I-3 | 高水位标记单调递增 | 数据 | 高 | S-01 | 隐式 |
| I-4 | Convoy check 幂等 | 数据 | 低 | S-03 | 是 |
| I-5 | 有就绪工作的搁浅 convoy 被分派 | 活跃性 | 高 | S-02 | 是 |
| I-6 | 空搁浅 convoy 被自动关闭 | 数据 | 中 | S-02 | 是 |
| I-7 | 分派失败后扫描继续 | 活跃性 | 中 | S-02 | 是 |
| I-8 | Context 取消停止两个 goroutine | 活跃性 | 高 | S-06 | 是 |
| I-9 | 每次扫描每个 convoy 分配一个 issue | 安全 | 中 | S-02 | 隐式 |
| I-10 | 未知前缀/rig 跳过 issue（不崩溃） | 安全 | 中 | S-02 | 是（`TestFeedFirstReady_UnknownPrefix_Skips`、`_UnknownRig_Skips`） |
| I-11 | `Stop()` 幂等 | 安全 | 低 | S-08 | 是 |
| I-12 | 关闭时子进程取消 | 活跃性 | 高 | S-09 | 是（`TestConvoyManager_ShutdownKillsHangingSubprocess`） |

---

## 5. 故障模式

### 事件轮询

| 故障 | 可能性 | 恢复 | 已测试？ |
|---------|------------|----------|---------|
| `GetAllEventsSince` 错误 | 低 | 下一 5s 间隔重试 | 是（`TestPollEvents_GetAllEventsSinceError`） |
| Beads 存储为 nil | 中 | 事件轮询禁用，搁浅扫描继续 | 是 |
| `issue_id` 为空的关闭事件 | 低 | 跳过 | 否 |
| `CheckConvoysForIssue` panic | 低 | Daemon 进程崩溃 → 重启 | 否 |

### 搁浅扫描

| 故障 | 可能性 | 恢复 | 已测试？ |
|---------|------------|----------|---------|
| `gt convoy stranded` 错误 | 低 | 记录日志，跳过周期 | 是（`TestFindStranded_GtFailure_ReturnsError`） |
| `gt` 返回无效 JSON | 低 | 记录日志，跳过周期 | 是（`TestFindStranded_InvalidJSON_ReturnsError`） |
| `gt sling` 分派失败 | 中 | 记录日志，继续到下一个 convoy | 是 |
| `gt convoy check` 失败 | 低 | 记录日志，继续到下一个 convoy | 否 |
| Issue 前缀未知 | 低 | 记录日志，跳过 issue | 是（`TestFeedFirstReady_UnknownPrefix_Skips`） |
| 前缀对应 rig 未知 | 低 | 记录日志，跳过 issue | 是（`TestFeedFirstReady_UnknownRig_Skips`） |
| `gt` 子进程挂起 | 低 | Context 取消杀死进程组 | 是（`TestConvoyManager_ShutdownKillsHangingSubprocess`） |

### 生命周期

| 故障 | 可能性 | 恢复 | 已测试？ |
|---------|------------|----------|---------|
| `Start()` 前调用 `Stop()` | 低 | `wg.Wait()` 立即返回 | 否 |
| 双重 `Stop()` | 低 | 幂等 | 是 |
| 双重 `Start()` | 低 | 已防护（`atomic.Bool`，无操作） | 是（`TestStart_DoubleCall_Guarded`） |
| 子进程阻塞关闭 | 低 | Context 取消杀死进程组 | 是（`TestConvoyManager_ShutdownKillsHangingSubprocess`） |

---

## 6. 文件映射

### 核心实现

| 文件 | 内容 |
|------|----------|
| `internal/daemon/convoy_manager.go` | ConvoyManager：事件轮询 + 搁浅扫描 goroutine |
| `internal/convoy/operations.go` | 共享 `CheckConvoysForIssue`、`feedNextReadyIssue`、`getTrackingConvoys`、`IsSlingableType`、`isIssueBlocked` |
| `internal/beads/routes.go` | `ExtractPrefix`、`GetRigNameForPrefix`（前缀 → rig 解析） |
| `internal/beads/fields.go` | `MRFields.ConvoyID`、`MRFields.ConvoyCreatedAt`（MR bead 中的 convoy 跟踪） |

### 集成点

| 文件 | 如何使用 convoy |
|------|-------------------|
| `internal/daemon/daemon.go` | 打开多 rig beads 存储，在 `Run()` 中创建 ConvoyManager，在 `shutdown()` 中停止 |
| `internal/witness/handlers.go` | Convoy 观察者已移除（S-04 REMOVED） |
| `internal/refinery/engineer.go` | Convoy 观察者已移除（S-05 REMOVED） |
| `internal/cmd/convoy.go` | CLI：`gt convoy create/status/list/add/check/stranded/close/land` |
| `internal/cmd/sling_convoy.go` | `gt sling` 期间的自动 convoy 创建 |
| `internal/cmd/formula.go` | `executeConvoyFormula` 用于 convoy 类型 formula |

### 测试

| 文件 | 测试内容 |
|------|--------------|
| `internal/daemon/convoy_manager_test.go` | ConvoyManager 单元测试（22 个测试） |
| `internal/daemon/convoy_manager_integration_test.go` | ConvoyManager 集成测试（2 个测试，`//go:build integration`） |
| `internal/convoy/store_test.go` | 观察者存储辅助（3 个测试） |
| `internal/convoy/operations_test.go` | 操作函数边缘情况 + 安全守卫测试 |
| `internal/daemon/daemon_test.go` | Daemon 级别 manager 生命周期（2 个 convoy 测试） |

### 设计文档

| 文件 | 内容 |
|------|----------|
| `docs/design/convoy/convoy-lifecycle.md` | 问题陈述、设计原则、流程图 |
| `docs/design/convoy/spec.md` | 本文档（包含测试工具记分卡和剩余缺口） |
| `docs/design/daemon/convoy-manager.md` | ConvoyManager 架构图（SDK 轮询 + 搁浅扫描） |

---

## 7. 审查发现 → 故事映射

| 发现 | 故事 |
|---------|-------|
| 基于流的 convoy-manager 文档已过期 | S-15 |
| 测试文档有过期的流/退避和重复缺口条目 | S-15 |
| 生命周期观察者/手动关闭声明已过期 | S-15 |
| Spec 文件映射命令/测试计数偏离 | S-15 |
| DONE 故事需要显式纠正处理 | S-16 |
| Refinery 观察者 root-path 歧义残留 | S-17（已验证） |
| Refinery root-path 修复需要 | S-18 |

---

## 8. 非目标（本规格）

这些在 convoy-lifecycle.md 中记录为未来工作，但**不在**本规格范围内：

- Convoy 负责人/请求者字段和定向通知（生命周期文档中 P2）
- Convoy 超时/SLA（`due_at` 字段，逾期显示）（生命周期文档中 P3）
- Convoy 重新开放命令（通过添加隐式实现，显式命令推迟）
- ConvoyManager 的测试时钟注入（P3 — 有用但不阻塞）

---

## 测试工具与剩余缺口

### 工具记分卡

| 维度 | 评分（1-5） | 关键缺口 |
|-----------|-------------|---------|
| 夹具与设置 | 4 | `mockGtForScanTest` 共享构建器覆盖扫描测试；processLine 路径有独立设置 |
| 隔离 | 4 | 临时目录 + `t.Setenv(PATH)` 可靠；Windows 正确跳过；无共享状态 |
| 可观察性 | 4 | 所有 mock 脚本发出调用日志；负面测试断言日志文件不存在/为空 |
| 速度 | 4 | 所有 convoy-manager 测试快速运行；当前套件中无长时间间隔等待 |
| 确定性 | 4 | 无真实时间依赖；ticker 测试使用长间隔以避免竞争 |

### 测试时钟注入（P3）

**问题**：ConvoyManager 使用 `time.Ticker`，默认 30s。测试"按间隔运行"需要等待或注入时钟。

**提议**：向 ConvoyManager 添加 `clock` 字段（接口，带 `NewTicker(d)` 方法），默认为真实时间。测试注入带即时 tick 的假时钟。

**复合价值**：所有周期性 daemon 组件受益。

**状态**：未实现。测试使用长间隔（10min）以防止测试期间 ticker 触发。

### 剩余测试缺口

- 添加 `TestProcessLine_EmptyIssueID`（`issue_id` 为空的关闭事件）
- 扩展多 rig 事件轮询的集成测试覆盖