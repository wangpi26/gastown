# 测试分析：Convoy Stage 与 Launch

`gt convoy stage` 和 `gt convoy launch` 的测试计划（PRD：`stage-launch/prd.md`）。

---

## 关键不变式

| # | 不变式 | 类别 | 影响范围 | 当前已测试？ |
|---|-----------|----------|-------------|-------------------|
| I-1 | 阻塞 dep 中的循环必须阻止 convoy 创建 | data | **高** | 否 |
| I-2 | 无有效 rig 的 bead 必须阻止分派 | data | **高** | 否 |
| I-3 | 不可 sling 类型必须排除在分派之外 | data | **中** | 部分（`IsSlingableType` 在 operations_test.go） |
| I-4 | 波次计算必须是 DAG 的有效拓扑排序 | data | **中** | 否 |
| I-5 | Launch 不得分派非波次 1 bead | 安全 | **高** | 否 |
| I-6 | Stage 创建跟踪所有解析后代的 convoy | data | **中** | 否 |
| I-7 | Epic 在首个子任务开始时推进到 in_progress | data | **低** | 否 |
| I-8 | Epic 在所有子任务完成时关闭 | data | **中** | 否 |
| I-9 | 阻塞者关闭后 daemon feeder 仅分派未阻塞 issue | data | **高** | 部分（`isIssueBlocked` 在 operations_test.go） |
| I-10 | 已停驻 rig 阻止该 rig 的分派 | 安全 | **中** | 否 |

---

## 缺陷类别

| 类别 | 描述 | PRD 用户故事 |
|----------|-------------|----------------|
| **C-1：DAG 验证** | 循环检测、rig 解析、类型过滤 | US-003、US-004、US-005 |
| **C-2：波次计算** | Kahn 算法、拓扑排序、波次分配 | US-006 |
| **C-3：分派安全** | Launch 边界、波次门控、幂等性 | US-007、US-008 |
| **C-4：Epic 状态管理** | 状态转换、子项进展跟踪 | US-009 |
| **C-5：Convoy 生命周期** | Staged/launched/closed 转换 | US-012 |
| **C-6：辅助功能** | 交互模式、集成分支、输出格式 | US-010、US-011、US-017 |

---

## 测试计划

### 层 1：单元测试（纯函数，无 IO）

这些测试无外部依赖，运行快速，验证核心算法逻辑。

| ID | 测试 | 证明内容 | 不变式 | 复杂性 |
|----|------|---------------|-----------|------------|
| U-01 | `TestKahn_CycleDetection` | Kahn 算法正确检测循环 | I-1 | 中 |
| U-02 | `TestKahn_LinearChain` | 线性 A→B→C DAG 生成波次 [A],[B],[C] | I-4 | 低 |
| U-03 | `TestKahn_DiamondDAG` | A→B,A→C,B→D,C→D 生成波次 [A],[B,C],[D] | I-4 | 中 |
| U-04 | `TestKahn_ParallelRoots` | 无依赖的 bead 全部进入波次 1 | I-4 | 低 |
| U-05 | `TestKahn_ParentChildIgnored` | parent-child dep 不增加入度 | I-4 | 低 |
| U-06 | `TestKahn_NonSlingableFiltered` | Epic 类型不进入波次计算 | I-3, I-4 | 低 |
| U-07 | `TestKahn_SingleBead` | 单 bead → 波次 1 | I-4 | 低 |
| U-08 | `TestKahn_EmptyDAG` | 空 bead 列表 → 无波次 | I-4 | 低 |
| U-09 | `TestResolveRig_PrefixExists` | 前缀映射到 rig | I-2 | 低 |
| U-10 | `TestResolveRig_PrefixMissing` | 缺失前缀返回错误 | I-2 | 低 |
| U-11 | `TestIsSlingableType_AllTypes` | 过滤 epic/annotation/MR/convoy | I-3 | 低 |
| U-12 | `TestIsSlingableType_TaskSubtypes` | task、bug、feature 可 sling | I-3 | 低 |

### 层 2：集成测试（bead 存储 + git，无网络）

这些测试使用真实 bead 存储（内存 Dolt）和真实 git worktree，但不分派 polecat 或创建 tmux 会话。

| ID | 测试 | 证明内容 | 不变式 | 复杂性 |
|----|------|---------------|-----------|------------|
| E-01 | `TestStage_CreatesConvoy_WithAllDescendants` | Stage 创建跟踪所有后代的 convoy | I-6 | 高 |
| E-02 | `TestStage_CycleDependency_FailsWithError` | 循环阻止创建，错误消息识别循环 | I-1 | 高 |
| E-03 | `TestStage_InvalidRig_WarnsButCreates` | 无效 rig → 警告，仍创建 convoy | I-2 | 中 |
| E-04 | `TestStage_AllInvalidRigs_FailsWithError` | 所有 rig 缺失 → 错误，无 convoy | I-2 | 中 |
| E-05 | `TestStage_NonSlingableTypes_Filtered` | Epic 不被跟踪或分派 | I-3 | 中 |
| E-06 | `TestStage_AlreadyTracked_Warns` | 已跟踪 bead → 警告 | US-018 | 中 |
| E-07 | `TestStage_DryRun_NoConvoyCreated` | `--dry-run` 不创建 convoy | US-011 | 低 |
| E-08 | `TestLaunch_DispatchesOnlyWave1` | 仅波次 1 bead 被分派 | I-5 | 高 |
| E-09 | `TestLaunch_Wave2NotDispatched` | 波次 2 bead 不被分派 | I-5 | 中 |
| E-10 | `TestLaunch_Idempotent` | 重复 launch 不重新分派 | US-007 | 中 |
| E-11 | `TestLaunch_WaveOverride` | `--wave 3` 从波次 3 开始分派 | US-016 | 中 |
| E-12 | `TestFeeder_DispatchesAfterBlockerCloses` | 阻塞者关闭 → 依赖者分派 | I-9 | 高 |
| E-13 | `TestFeeder_SkipsBlockedIssues` | 仍有阻塞者 → 不分派 | I-9 | 中 |
| E-14 | `TestFeeder_SkipsNonSlingableTypes` | Epic 类型不被分派 | I-3 | 低 |
| E-15 | `TestEpicStatus_OpenToInProgress` | 首个子任务开始 → epic in_progress | I-7 | 中 |
| E-16 | `TestEpicStatus_InProgressToClosed` | 所有子项关闭 → epic 关闭 | I-8 | 中 |
| E-17 | `TestParkedRig_SkipsDispatch` | 已停驻 rig → 不分派，警告 | I-10 | 中 |
| E-18 | `TestParkedRig_AllParked_Fails` | 所有 rig 停驻 → 错误 | I-10 | 中 |
| E-19 | `TestConvoyStatus_ShowsWaveInfo` | 状态显示波次信息 | US-012 | 低 |

### 层 3：端到端测试（完整管道）

这些测试运行完整 stage → launch → feeder → complete 循环。

| ID | 测试 | 证明内容 | 复杂性 |
|----|------|---------------|------------|
| F-01 | `TestE2E_SimpleEpic_StageLaunchComplete` | 3 任务 epic：stage → launch → 全部关闭 → convoy 关闭 | 高 |
| F-02 | `TestE2E_DiamondDAG_WavesRespected` | Diamond DAG：波次正确，依赖被尊重 | 高 |
| F-03 | `TestE2E_Cycle_RejectedAtStage` | 循环：stage 报错，不创建 convoy | 中 |
| F-04 | `TestE2E_ParkedRig_SkippedThenResumed` | 停驻 rig：跳过，解除停驻，重新 stage → 分派 | 高 |

---

## 测试基础设施需求

### Bead 存储夹具

集成测试需要一个包含已知结构 DAG 的填充 bead 存储：

```
gt-epic-root (epic)
├── gt-sub-epic-a (epic)
│   ├── gt-task-a1 (task) [无依赖]
│   ├── gt-task-a2 (task) [被 gt-task-a1 阻塞]
│   └── gt-task-a3 (task) [被 gt-task-a1 阻塞]
├── gt-sub-epic-b (epic)
│   ├── gt-task-b1 (task) [无依赖]
│   └── gt-task-b2 (task) [被 gt-task-b1 阻塞]
└── gt-task-standalone (task) [无依赖]
```

此夹具支持：
- 循环检测（添加循环 dep 破坏它）
- 波次计算（3 波次：a1+b1+standalone、a2+a3+b2、完成）
- 类型过滤（epic 被排除）
- Rig 解析（所有 `gt-*` → gastown）

### 用于分派测试的 Mock tmux

分派测试不应依赖真实 tmux。需要：
- 捕获 `gt sling` 调用的 mock tmux 服务器
- 验证仅 sling 正确波次的 bead
- 验证不 sling 非波次 1 bead

---

## 风险领域

### 高风险：循环检测（I-1）

循环检测阻止 convoy 创建。如果它失败，用户可能创建有无限依赖循环的 convoy，使分派永远卡住。

**缓解**：Kahn 算法数学上保证检测循环。测试只需验证实现正确使用算法。

### 高风险：波次门控（I-5）

Launch 必须仅分派波次 1 bead。如果波次门控失败，被阻塞的任务可能在阻塞者完成前运行，导致：
- 在未准备好的代码上工作的 polecat
- 浪费的 token 预算
- 合并冲突

**缓解**：波次 1 分派是 launch 中的简单过滤。Feeder 的 `isIssueBlocked` 检查提供第二安全网。两层必须都失败才会出现问题。

### 中风险：Epic 状态管理（I-7、I-8）

Epic 状态管理是用户便利，非安全关键。如果它失败：
- Epic 保留 `open` 而非进展到 `in_progress`
- 不正确关闭的 epic 可能过早触发 convoy 关闭

**缓解**：Epic 状态变更是非破坏性操作。过度关闭的 convoy 可以重新开放。不正确保留 `open` 的 epic 只是可见性问题。

---

## 缺口分析

### 已覆盖（由现有代码）

| 不变式 | 位置 |
|-----------|----------|
| 可 sling 类型过滤 | `internal/convoy/operations.go:IsSlingableType`（在 `operations_test.go` 中测试） |
| Issue 阻塞检查 | `internal/convoy/operations.go:isIssueBlocked`（在 `operations_test.go` 中测试） |
| Convoy 完成检测 | `internal/daemon/convoy_manager.go`（在 `convoy_manager_test.go` 中测试） |

### 缺口（新增代码）

| 不变式 | 需要什么 |
|-----------|-----------------|
| I-1 | `TestKahn_CycleDetection` + `TestStage_CycleDependency_FailsWithError` |
| I-2 | `TestResolveRig_*` + `TestStage_InvalidRig_*` |
| I-4 | `TestKahn_*`（8 个变体） |
| I-5 | `TestLaunch_DispatchesOnlyWave1` + `TestLaunch_Wave2NotDispatched` |
| I-6 | `TestStage_CreatesConvoy_WithAllDescendants` |
| I-7 | `TestEpicStatus_OpenToInProgress` |
| I-8 | `TestEpicStatus_InProgressToClosed` |
| I-10 | `TestParkedRig_SkipsDispatch` + `TestParkedRig_AllParked_Fails` |

---

## 与 ConvoyManager 测试的重叠

ConvoyManager spec（`spec.md`）有自己的测试计划。以下是我们如何避免重复：

| 测试领域 | ConvoyManager Spec | Stage-Launch Spec |
|-----------|--------------------|--------------------|
| 事件驱动分派 | 覆盖（S-01 测试） | E-12、E-13 测试与波次的交互 |
| 搁浅扫描 | 覆盖（S-02 测试） | 此处不适用 |
| `IsSlingableType` | 覆盖（operations_test.go） | U-11、U-12 测试波次计算中的过滤 |
| `isIssueBlocked` | 覆盖（operations_test.go） | E-13 验证波次门控交互 |
| Convoy 完成检测 | 覆盖（S-03 测试） | F-01 端到端验证 |
| 子进程取消 | 覆盖（S-09 测试） | 此处不适用 |

**经验法则**：ConvoyManager spec 测试机械观察者。Stage-launch spec 测试波次感知分派以及 stage/launch 如何与观察者交互。

---

## 实现顺序

1. **层 1 单元测试** — 可立即实现（纯算法，无依赖）
2. **层 2 集成测试夹具** — 构建夹具，然后单个测试
3. **层 3 端到端** — 最后，在层 2 稳定后

### 建议的首次 PR

1. Kahn 算法实现 + 单元测试（U-01 到 U-12）
2. Stage 命令实现 + 集成测试（E-01 到 E-07）
3. Launch 命令实现 + 集成测试（E-08 到 E-11）
4. Feeder 增强 + 集成测试（E-12 到 E-14）
5. Epic 状态管理 + 测试（E-15 到 E-16）
6. 辅助功能 + 测试（E-17 到 E-19）

每个 PR 可独立审查和合并。

---

## 流失败触发退避 + 重试循环

> **此部分已过期。** 流式架构（`bd activity --follow`）已替换为 SDK 轮询（`GetAllEventsSince`）。没有流，没有退避 + 重试循环。daemon 在固定间隔（5s 事件轮询，30s 搁浅扫描）重试，无论成功或失败。

---

## 重复缺口条目

> **此部分已过期。** 以下缺口最初在 ConvoyManager 审计中识别，现已解决：
>
> - `TestDoubleStop_Idempotent` — 在 S-08 中实现
> - 负面断言测试 — 在 S-11 中实现

---

## 测试命令速查表

```bash
# 运行所有 convoy 相关测试
go test ./internal/convoy/... ./internal/daemon/... -v -count=1

# 仅运行 stage-launch 单元测试
go test ./internal/convoy/... -v -run "TestKahn|TestResolveRig|TestIsSlingable"

# 仅运行 ConvoyManager 测试
go test ./internal/daemon/... -v -run "TestConvoyManager|TestScan|TestEventPoll"

# 运行集成测试（需要真实 bead 存储）
go test ./internal/daemon/... -v -run "TestConvoyManager" -tags=integration
```