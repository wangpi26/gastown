# Gas Town 新手指南

本指南面向 Gas Town 新用户，按工作流程阶段从易到难组织，帮助你快速上手 `gt` 命令体系。

---

## 目录

1. [入门：环境与基础概念](#1-入门环境与基础概念)
2. [创建工作：Bead 与 Formula](#2-创建工作bead-与-formula)
3. [分配任务：Sling 与 Convoy](#3-分配任务sling-与-convoy)
4. [查看进度：Status、Convoy、Trail](#4-查看进度statusconvoytrail)
5. [消息通信：Mail、Nudge、Escalate](#5-消息通信mailnudgeescalate)
6. [诊断排错：Doctor、Vitals、Orphans](#6-诊断排错doctorvitalsorphans)
7. [高级操作：Mountain、MQ、Formula Run](#7-高级操作mountainmqformula-run)

---

## 1. 入门：环境与基础概念

### 1.1 核心概念速览

| 术语 | 含义 |
|------|------|
| **Town** | 整个工作空间根目录（`~/gt/`），管理所有 Rig |
| **Rig** | 项目级容器，一个 Rig 对应一个 Git 仓库，包含 Witness、Refinery、Polecat 等 |
| **Bead** | 最小工作单元，存储在 Dolt 数据库中，类似 issue/task |
| **Polecat** | 短暂 worker：有持久身份但会话临时，由 Witness 管理 |
| **Crew** | 长驻 worker：持久化的命名工作区，由用户管理 |
| **Witness** | 每 Rig 一个，监控 Polecat 健康状态 |
| **Refinery** | 每 Rig 一个，处理合并队列 |
| **Mayor** | 全局协调者，负责跨 Rig 工作分发 |
| **Deacon** | 守护进程，持续巡逻系统健康 |
| **Convoy** | 工作批次追踪单元，跟踪一组相关 Bead |
| **Sling** | 将工作分配给 agent 的核心命令 |
| **Hook** | 每个 agent 的工作挂钩，hook 上有工作就必须执行（GUPP 原则） |
| **Formula** | 可复用的工作流模板（TOML 格式） |
| **Molecule** | Formula 实例化后的可执行工作链 |

### 1.2 初始化与安装

```bash
# 创建新的 Gas Town 工作空间（首次安装）
gt install ~/my-workspace

# 将当前目录初始化为一个 Rig
gt init

# 初始化 Git 仓库（用于 Gas Town HQ）
gt git-init
```

### 1.3 启动与停止

```bash
# 启动 Gas Town（启动 Deacon + Mayor）
gt start

# 启动所有 Gas Town 服务（Deacon + Mayor + 所有 Rig 的 Witness 和 Refinery）
gt up

# 启动所有 Rig 的 Witness 和 Refinery
gt start --all

# 启动某个 Crew worker
gt start gastown/crew/dave

# 优雅关闭
gt shutdown

# 停止所有 Gas Town 服务
gt down

# 紧急停车（冻结所有 agent，上下文保留）
gt estop
gt estop --rig gastown            # 只冻结某个 Rig
gt thaw                           # 恢复运行
gt thaw --rig gastown             # 恢复某个 Rig
```

### 1.4 查看系统信息

```bash
# 查看 Gas Town 版本和更新内容
gt info

# 查看版本号
gt version

# 检查二进制是否过时
gt stale
```

### 1.3 查看全局状态

```bash
# 工作空间总览
gt status

# 详细输出
gt status --verbose

# 持续刷新
gt status --watch

# 快速模式（跳过 mail 查询）
gt status --fast
```

### 1.4 身份与上下文

```bash
# 查看当前身份
gt whoami

# 加载当前角色的完整上下文（compaction 后恢复用）
gt prime

# 加载 hook 上的工作上下文
gt prime --hook
```

### 1.5 目录结构概览

```
~/gt/                           ← Town 根目录
├── mayor/                      ← Mayor（全局协调）
├── deacon/                     ← Deacon（守护进程）
└── gastown/                    ← Rig（项目容器）
    ├── .beads/                 ← Beads 数据库（Dolt）
    ├── refinery/               ← Refinery（合并队列处理器）
    ├── witness/                ← Witness（Polecat 健康监控）
    ├── crew/
    │   └── dave/               ← Crew worker 工作区
    └── polecats/
        └── chrome/             ← Polecat 工作区（git worktree）
```

---

## 2. 创建工作：Bead 与 Formula

### 2.1 用 bd 创建 Bead

`bd` 是 Beads 的命令行工具，用于管理工作项。

```bash
# 创建一个 task
bd create --title="Fix login bug" --type=task --priority=2

# 创建一个 bug
bd create --title="Crash on empty input" --type=bug --priority=1

# 带描述创建
bd create --title="Add dark mode" --type=task --priority=3 --description="Support system-level dark theme"

# 快速捕获（只输出 ID）
bd q "Fix typo in README"

# 创建并添加标签
bd create --title="Refactor auth" --type=task -l refactor -l security

# 在另一个 Rig 创建
bd create --rig beads "bd CLI bug"
```

### 2.2 查看 Bead

```bash
# 查看 Bead 详情（自动路由到正确的 Rig）
bd show gt-abc

# 通过 gt 查看 Bead（等价于 bd show）
gt show gt-abc

# 显示 Bead 内容（适合长文本）
gt cat gt-abc

# 列出所有 Bead
bd list

# 按状态筛选
bd list --status=open
bd list --status=in_progress

# 搜索
bd search "login bug"

# 查看可立即开始的工作（无阻塞依赖）
bd ready
```

### 2.3 更新与关闭 Bead

```bash
# 认领工作
bd update gt-abc --status=in_progress

# 添加备注（关键：session 死亡后仍可保留）
bd update gt-abc --notes "Found the root cause: null pointer in auth.go:42"

# 添加设计分析
bd update gt-abc --design "Plan: refactor auth module to use dependency injection"

# 关闭 Bead
bd close gt-abc

# 通过 gt 关闭 Bead（支持批量关闭）
gt close gt-abc gt-def

# 关闭但不做代码修改
bd close gt-abc --reason="no-changes: already fixed upstream"
```

### 2.4 记忆管理

Polecat 和 Crew 可以跨 session 保留记忆，用于持久化常用上下文。

```bash
# 存储一条记忆
gt remember "This project uses bun, not npm"

# 列出所有记忆
gt memories

# 搜索记忆
gt memories --search "bun"

# 删除一条记忆
gt forget <memory-id>
```

### 2.5 Bead 依赖管理

```bash
# 添加依赖（gt-def 需要 gt-abc 先完成）
bd dep add gt-def gt-abc

# 查看阻塞关系
bd blocked

# 查看依赖图
bd graph
```

### 2.5 Formula：可复用工作流

Formula 是 TOML 格式的工作流模板，可以实例化为 Molecule 执行。

```bash
# 列出可用 Formula
gt formula list

# 查看 Formula 详情（确认类型后再选择调用方式）
gt formula show mol-polecat-work

# 创建新 Formula
gt formula create my-workflow

# 运行 Formula
gt formula run mol-review --pr=123
```

> **注意：** Formula 分为 **workflow Formula** 和 **convoy Formula** 两种类型。Workflow Formula 用 `gt sling <formula> <target>` 调用；convoy Formula 用 `gt formula run <formula>` 调用。对 convoy Formula 使用 `gt sling` 会报错。用 `gt formula show <name>` 查看类型后再选择调用方式。

### 2.6 Molecule 工作链

Molecule 是 Formula 实例化后的执行链，包含多个有序步骤。

```bash
# 查看当前 hook 上的 Molecule 状态
gt mol status

# 查看当前应执行的步骤
gt mol current

# 查看执行进度
gt mol progress

# 完成当前步骤
gt mol step done

# 可视化依赖 DAG
gt mol dag
```

---

## 3. 分配任务：Sling 与 Convoy

### 3.1 gt sling：核心分配命令

`gt sling` 是 Gas Town 中最核心的工作分配命令——将 Bead 挂到 agent 的 hook 上并立即开始。

```bash
# 将 Bead 分配给某个 Rig（自动 spawn Polecat）
gt sling gt-abc gastown

# 分配给特定 Polecat
gt sling gt-abc gastown/chrome

# 分配给 Crew worker
gt sling gt-abc gastown --crew mel

# 分配给 Mayor
gt sling gt-abc mayor

# 分配给自己
gt sling gt-abc

# 附带自然语言指令
gt sling gt-abc gastown --args "patch release"
gt sling gt-abc --args "focus on security"

# 附带上下文消息
gt sling gt-abc gastown -s "Auth refactor" -m "Focus on SQL injection"

# 多行消息用 stdin
gt sling gt-abc gastown --stdin <<'EOF'
Focus on:
1. SQL injection in query builders
2. XSS in template rendering
EOF
```

#### Sling Formula

```bash
# Sling 一个 Formula（自动 cook + wisp + attach + nudge）
gt sling mol-release mayor/

# 在已有 Bead 上应用 Formula
gt sling mol-review --on gt-abc

# 传入 Formula 变量
gt sling towers-of-hanoi --var disks=3
```

#### 批量 Sling

```bash
# 批量分配（每个 Bead 独立 spawn Polecat）
gt sling gt-abc gt-def gt-ghi gastown

# 限制并发数（避免 Dolt 过载）
gt sling gt-abc gt-def gastown --max-concurrent 3
```

#### 合并策略

```bash
# 默认：走合并队列（推荐，经过 Refinery 质量验证）
gt sling gt-abc gastown --merge=mr

# 保留在 feature branch（不做合并，适合需要审查的工作）
gt sling gt-abc gastown --merge=local
```

> **警告：** `--merge=direct` 会绕过所有质量门禁直接推到 main。Polecat **禁止**使用此选项——所有工作必须通过 Refinery 合并队列。此选项仅限人工操作者在特殊情况下使用。

### 3.2 gt assign：快速分配给 Crew

`gt assign` 是 `bd create` + `gt hook` 的快捷方式，专用于 Crew worker。

```bash
# 创建 Bead 并挂到 Crew worker 的 hook
gt assign monet "Fix the auth token refresh bug"

# 带描述
gt assign monet "Review error handling" -d "The retry logic looks wrong"

# 指定类型和优先级
gt assign monet "Fix auth bug" --type bug --priority 1

# 分配后唤醒
gt assign monet "Fix auth bug" --nudge

# 指定 Rig
gt assign monet "Fix auth bug" --rig beads
```

### 3.3 gt hook：管理工作挂钩

```bash
# 查看自己 hook 上的工作
gt hook

# 将 Bead 挂到自己 hook
gt hook gt-abc

# 将 Bead 挂到别人 hook
gt hook gt-abc gastown/crew/max

# 清除 hook
gt hook --clear
```

### 3.4 gt unsling：从 hook 移除工作

```bash
# 从 agent 的 hook 上移除工作
gt unsling gastown/chrome

# 移除自己的 hook 工作
gt unsling
```

### 3.5 gt release：释放卡住的工作

当一个 Bead 长期处于 `in_progress` 但实际无人处理时，可以释放回待分配状态。

```bash
# 释放卡住的 in_progress Bead
gt release gt-abc

# 批量释放某个 Rig 的卡住工作
gt release --rig gastown --days=3
```

#### 对比：hook vs sling vs handoff vs unsling

| 命令 | 行为 | 场景 |
|------|------|------|
| `gt hook <bead>` | 仅挂载，不启动 | 准备工作但暂不执行 |
| `gt sling <bead>` | 挂载 + 立即开始 | 分配并执行 |
| `gt handoff <bead>` | 挂载 + 重启会话 | 上下文满了需要刷新 |
| `gt unsling` | 从 hook 移除工作 | 取消分配或清理 |

### 3.4 Convoy：工作批次追踪

Convoy 用于跟踪一组相关的 Bead，是批次级别的进度追踪单元。

```bash
# 创建 Convoy
gt convoy create "Feature X" gt-abc gt-def

# 创建时订阅通知
gt convoy create "Auth refactor" gt-abc --notify overseer

# 查看所有 Convoy
gt convoy list

# 查看 Convoy 详情
gt convoy status hq-cv-abc

# 向已有 Convoy 添加 Bead
gt convoy add hq-cv-abc gt-ghi

# 订阅完成通知
gt convoy watch hq-cv-abc

# 关闭 Convoy
gt convoy close hq-cv-abc
```

### 3.8 Convoy Synthesis：批次聚合

Synthesis 用于将 Convoy 中完成的 Bead 成果自动聚合。

```bash
# 查看 Convoy 的 synthesis 状态
gt synthesis status hq-cv-abc

# 触发 synthesis
gt synthesis run hq-cv-abc
```

### 3.9 调度器

Scheduler 用于定时或条件触发工作分发。

```bash
# 查看调度器状态
gt scheduler status

# 列出调度规则
gt scheduler list

# 创建调度规则
gt scheduler create --cron "0 9 * * 1-5" --formula mol-daily-check
```

### 3.5 Crew Worker 管理

```bash
# 列出 Crew worker
gt crew list

# 创建 Crew 工作区
gt crew add dave

# 启动 Crew worker
gt crew start dave

# 查看 Crew 状态
gt crew status dave

# 附加到 Crew 会话
gt crew at dave

# 刷新上下文（handoff 邮件循环）
gt crew refresh dave

# 停止 Crew worker
gt crew stop dave

# 移除 Crew 工作区
gt crew remove dave
```

### 3.6 Polecat 管理

```bash
# 列出 Rig 内所有 Polecat
gt polecat list

# 查看 Polecat 状态
gt polecat status chrome

# 检测停滞的 Polecat
gt polecat stale

# 清理 Polecat（销毁 session、worktree、branch、agent bead）
gt polecat nuke chrome
```

### 3.7 gt commit：带身份的 Git 提交

Polecat 和 Crew 使用 `gt commit` 代替 `git commit`，自动附加 agent 身份信息。

```bash
# 带 agent 身份的提交（自动添加 Co-Authored-By）
gt commit -m "fix: resolve auth token refresh (gt-abc)"

# 查看提交差异
gt commit --dry-run
```

---

## 4. 查看进度：Status、Convoy、Trail

### 4.1 全局状态

```bash
# 工作空间总览（Rig、agent、Witness 状态）
gt status

# 详细模式
gt status --verbose

# 持续监控
gt status --watch

# 快速模式（跳过 mail 查询）
gt status --fast
```

### 4.2 Convoy 进度

```bash
# 列出所有 Convoy（dashboard 视图）
gt convoy list

# 查看 Convoy 进度和关联 Bead
gt convoy status hq-cv-abc

# 交互式树形视图
gt convoy list --interactive

# 查找搁浅的 Convoy
gt convoy stranded
```

### 4.3 Polecat 和 Crew 进度

```bash
# Polecat 列表
gt polecat list

# Polecat 状态详情
gt polecat status chrome

# Crew worker 列表
gt crew list

# Crew worker 状态详情
gt crew status dave

# 查看 agent 最近输出
gt peek gastown/chrome
gt peek gastown/chrome -n 50           # 最近 50 行
gt peek beads/crew/dave
```

### 4.4 Bead 进度

```bash
# 查看可开始的工作
gt ready
gt ready --rig=gastown                 # 只看某个 Rig

# 查看 Bead 详情
bd show gt-abc

# 查看 Bead 列表
bd list --status=open
bd list --status=in_progress

# 查看被阻塞的 Bead
bd blocked
```

### 4.5 活动追踪

```bash
# 最近 agent 提交（默认）
gt trail

# 最近 Bead 活动
gt trail beads

# 最近 Hook 活动
gt trail hooks

# 按时间过滤
gt trail --since 1h
gt trail beads --since 24h
gt trail --limit 50

# 查看完成记录
gt changelog
gt changelog --today
gt changelog --week
gt changelog --since 2026-04-01
gt changelog --rig gastown

# 查看某个 actor 的完整工作历史
gt audit --actor=gastown/crew/joe
gt audit --actor=gastown/polecats/chrome
gt audit --since=24h
```

### 4.6 Dashboard

```bash
# 启动 Web Dashboard
gt dashboard

# 指定端口
gt dashboard --port 3000

# 自动打开浏览器
gt dashboard --open

# 统一健康面板
gt vitals
```

---

## 5. 消息通信：Mail、Nudge、Escalate

### 5.1 Mail：持久消息

Mail 消息存储在 Beads 中，会永久保留（每个 `gt mail send` 产生一个 Dolt commit），适合需要持久记录的通信。

```bash
# 查看收件箱
gt mail inbox

# 读取特定消息
gt mail read <msg-id>

# 发送消息
gt mail send gastown/witness -s "Question" -m "Short message"

# 发送给 Mayor
gt mail send mayor/ -s "Need coordination" -m "Context here"

# 多行消息用 --stdin（避免 shell 引号问题）
gt mail send gastown/witness -s "HELP: auth bug" --stdin <<'BODY'
Can't resolve the OAuth token refresh issue.
Tried: clearing cache, rotating keys.
Error: "invalid_grant" on line 42 of auth.go.
BODY

# 回复消息
gt mail reply <msg-id> -m "Got it, working on it"

# 搜索消息
gt mail search "auth bug"

# 查看消息线程
gt mail thread <msg-id>

# 标记已读
gt mail mark-read <msg-id>
```

**通信预算提醒：** Polecat 每个 session 建议只发 0-1 封 mail。优先使用 `gt nudge`。

### 5.2 Nudge：即时消息

Nudge 是零成本的即时消息，不会产生 Dolt commit，适合非紧急协调。

```bash
# 发送 nudge（默认等待对方空闲再投递）
gt nudge gastown/chrome "Check your mail"

# 立即投递（会打断对方工作，慎用）
gt nudge gastown/chrome "Emergency: build is broken" --mode=immediate

# 队列投递（零打断，下次 turn 时读取）
gt nudge gastown/chrome "FYI: new priority work" --mode=queue

# 多行消息
gt nudge gastown/alpha --stdin <<'EOF'
Status update:
- Task 1: complete
- Task 2: in progress
EOF

# 发给 channel
gt nudge channel:workers "New priority work available"

# 强制发送（即使对方开启 DND）
gt nudge gastown/chrome "Critical" --force
```

### 5.3 Escalate：升级处理

当遇到阻塞问题需要人工介入时使用。

```bash
# 创建升级
gt escalate "Build failing" --severity critical --reason "CI blocked"
gt escalate "Need API credentials" --severity high

# 严重程度：critical (P0) > high (P1) > medium (P2) > low (P3)
gt escalate "Code review requested" --reason "PR #123 ready"

# 查看升级列表
gt escalate list

# 确认升级
gt escalate ack hq-abc123

# 关闭升级
gt escalate close hq-abc123 --reason "Fixed in commit abc"

# 重新升级过期的升级
gt escalate stale
```

### 5.4 Resume：接收交接消息

当另一个 agent 向你发送 handoff 时，使用 `gt resume` 读取交接消息。

```bash
# 检查是否有交接消息
gt resume

# 读取并继续工作
gt resume --accept
```

### 5.5 Broadcast 与通知

```bash
# 广播消息给所有 worker
gt broadcast "Deploy starting in 5 minutes"

# 设置通知级别
gt notify --level high

# 切换勿扰模式
gt dnd on
gt dnd off
```

### 5.6 Mail vs Nudge vs Escalate 选择指南

| 场景 | 推荐方式 | 原因 |
|------|----------|------|
| 需要持久记录的通信 | `gt mail send` | 永久保留在 Dolt |
| 日常协调 | `gt nudge` | 零成本，不产生 Dolt commit |
| 紧急打断 | `gt nudge --mode=immediate` | 即时送达 |
| 阻塞需要人工介入 | `gt escalate` | 按严重程度路由，有确认流程 |
| 通知所有人 | `gt broadcast` | 全员广播 |

---

## 6. 诊断排错：Doctor、Vitals、Orphans

### 6.1 gt doctor：全面健康检查

```bash
# 运行所有健康检查
gt doctor

# 只检查某个 Rig
gt doctor --rig gastown

# 自动修复可修复的问题
gt doctor --fix

# 详细输出
gt doctor --verbose

# 高亮慢速检查
gt doctor --slow
```

Doctor 检查项包括：工作空间配置、Town root 保护、基础设施、清理、Clone 分歧、Crew 工作区、路由、生命周期等数十项。

### 6.2 健康面板

```bash
# 统一健康面板
gt vitals

# 综合健康报告
gt health

# Dolt 服务状态
gt dolt status
```

### 6.3 查找孤立项

```bash
# 查找 Polecat 孤立提交和未合并分支
gt orphans
gt orphans --rig=gastown
gt orphans --days=14
gt orphans --all

# 查找 Bead 孤立项（已在 commit 中引用但仍然 open）
bd orphans
```

### 6.4 Polecat 故障诊断

```bash
# 检测停滞的 Polecat
gt polecat stale

# 检查 Polecat 是否需要恢复
gt polecat check-recovery chrome

# 查看 Polecat 的 git 状态
gt polecat git-state chrome

# 清理 Polecat
gt polecat nuke chrome
```

### 6.5 Session 诊断

```bash
# 列出所有 agent session
gt agents

# 查看 session 检查点
gt checkpoint list

# 与前驱 session 对话（获取历史上下文）
gt seance

# 查看 session 成本
gt costs

# 在 session 组之间切换
gt cycle
```

### 6.6 活动监控

```bash
# 实时活动流（持续输出 gt 事件）
gt feed

# 查看城镇活动日志
gt log

# 发射/查看活动事件
gt activity
```

### 6.7 数据修复

```bash
# 修复数据库身份和配置问题
gt repair

# 修复特定 Rig
gt repair --rig gastown
```

### 6.8 常见问题排查流程

```
问题：bd 命令挂起
  → gt dolt status          # 检查 Dolt 是否在线
  → 如果 Dolt 宕机：gt escalate -s HIGH "Dolt: connection refused"

问题：Polecat 似乎卡住
  → gt polecat stale        # 检测停滞 Polecat
  → gt peek <rig>/<name>    # 查看最近输出
  → gt nudge <rig>/<name> "Status?"  # 催促一下

问题：工作丢失
  → gt orphans              # 查找孤立提交
  → gt polecat git-state <name>  # 检查 git 状态
  → gt trail --since 1h     # 查看最近活动

问题：Refinery 未合并
  → gt mq list              # 查看合并队列
  → gt refinery queue       # 查看 Refinery 队列
  → gt refinery status      # 查看 Refinery 状态
```

---

## 7. 高级操作：Mountain、MQ、Formula Run

### 7.1 Mountain：Epic 级自主研磨

Mountain 是带增强监控的 Convoy，适用于大型自主工作。

```bash
# 激活 Mountain（stage + label + launch Wave 1）
gt mountain gt-epic-auth

# 强制启动（忽略 stage 警告）
gt mountain --force gt-epic-x

# 查看 Mountain 进度
gt mountain status

# 暂停 Mountain
gt mountain pause <id>

# 恢复 Mountain
gt mountain resume <id>

# 取消 Mountain（移除 label，保留 Convoy）
gt mountain cancel <id>
```

### 7.2 Merge Queue (MQ)

```bash
# 查看合并队列
gt mq list

# 下一个待合并 MR
gt mq next

# 查看 MR 详情
gt mq status <mr-id>

# 重试失败的 MR
gt mq retry <mr-id>

# 拒绝 MR
gt mq reject <mr-id>

# 提交当前分支到合并队列
gt mq submit

# 也可以用 gt done 提交（Polecat 常用）
gt done
gt done --pre-verified              # 预验证快速合并
gt done --target feat/my-branch     # 指定目标分支
gt done --status ESCALATED          # 遇到阻塞
gt done --status DEFERRED           # 暂停工作
```

### 7.3 Handoff：会话交接

当上下文即将耗尽或需要刷新时，使用 handoff 交接给新 session。

```bash
# 交接当前 session
gt handoff

# 先挂载工作再交接
gt handoff gt-abc

# 带上下文交接
gt handoff -s "Auth refactor" -m "Fixed token refresh, need to add tests"

# 自动收集状态交接
gt handoff --collect

# 交接特定角色
gt handoff mayor
gt handoff crew
```

### 7.4 Rig 管理

```bash
# 列出所有 Rig
gt rig list

# 添加 Rig
gt rig add myproject https://github.com/org/repo.git

# 查看 Rig 配置
gt rig config gastown

# 启动 Rig 的 Witness + Refinery
gt rig start gastown

# 停止 Rig
gt rig stop gastown

# 重启 Rig
gt rig restart gastown

# 停泊 Rig（暂停 agent，daemon 不自动重启）
gt rig park gastown
gt rig unpark gastown

# 入坞 Rig（全局持久关闭）
gt rig dock gastown
gt rig undock gastown
```

### 7.5 Cross-Rig 工作

```bash
# 在另一个 Rig 创建 worktree
gt worktree beads

# 查看 Bead 路由（prefix-based routing）
bd where
```

### 7.6 Agent 会话管理

```bash
# 列出 agent session
gt agents

# 查看 Polecat session
gt session list

# 查看/管理当前角色
gt role
gt role list

# Signal handler（Claude Code hook）
gt signal
gt tap

# 处理 agent 回调
gt callbacks
```

### 7.7 基础设施 Agent

Gas Town 有多个基础设施级 Agent，通常由 Deacon 自动管理。

```bash
# Deacon：城镇级守护进程
gt deacon start
gt deacon stop
gt deacon status

# Boot：Deacon 的看门狗
gt boot status
gt boot restart

# Mayor：全局协调者
gt mayor status
gt mayor nudge         # 催促 Mayor 处理队列

# Dog：跨 Rig 基础设施 worker
gt dog list
gt dog status <name>
gt dog start <name>

# Daemon：Gas Town 后台守护进程
gt daemon start
gt daemon stop
gt daemon status
```

### 7.8 数据维护

```bash
# 完整 Dolt 维护
gt maintain

# Wisp 清理
gt compact

# 清理孤儿 Claude 进程
gt cleanup

# 修剪陈旧的 Polecat 分支
gt prune-branches

# Beads 数据库维护
bd gc
bd compact
bd flatten
bd doctor
```

### 7.9 配置与设置

```bash
# 查看/修改配置
gt config list
gt config set <key> <value>

# 管理角色指令（自定义 agent 行为）
gt directive list
gt directive show <name>

# 管理 Claude Code 账户
gt account list
gt account switch <name>

# 管理 Hook 脚本
gt hooks list
gt hooks add <event> <command>

# 管理当前 issue 显示（状态栏）
gt issue set gt-abc
gt issue clear

# 设置 tmux 主题
gt theme
gt theme set dark

# 管理 shell 集成
gt shell install
gt shell uninstall

# 生成补全脚本
gt completion bash > ~/.gt-completion.bash
gt completion zsh > ~/.gt-completion.zsh

# 版本与升级
gt version
gt upgrade
gt stale                          # 检查二进制是否过时

# 启用/禁用 Gas Town
gt enable
gt disable

# 插件管理
gt plugin list
gt plugin install <name>

# 命令使用统计
gt metrics
```

### 7.10 Town 级操作与特殊命令

```bash
# Town 级操作
gt town status
gt town config

# Wasteland 联邦命令（跨工作空间协作）
gt wl status
gt wl sync

# Key Record Chronicle（临时数据 TTL 管理）
gt krc set <key> <value> --ttl 1h
gt krc get <key>

# Death Warrant（强制终止卡住的 agent）
gt warrant issue gastown/chrome
gt warrant list
gt warrant execute <warrant-id>

# 账户配额轮换
gt quota status
gt quota rotate

# Wisp/issue 清理（Dog 可调用的辅助命令）
gt reaper reap
gt reaper status

# 巡逻摘要
gt patrol
gt patrol status

# 向贡献者致谢
gt thanks
```

### 7.11 名称池管理

Polecat 的名称是从名称池中分配的，确保每个 Polecat 有唯一标识。

```bash
# 查看名称池
gt namepool list

# 查看可用名称
gt namepool available
```

---

## 附录：命令速查表

### 工作管理

| 命令 | 用途 |
|------|------|
| `gt sling <bead> <target>` | 分配工作并立即开始 |
| `gt assign <crew> <title>` | 快速分配给 Crew worker |
| `gt hook [bead]` | 查看/挂载 hook |
| `gt unsling` | 从 hook 移除工作 |
| `gt done` | 提交工作到合并队列 |
| `gt handoff` | 交接会话 |
| `gt commit -m "msg"` | 带 agent 身份的 Git 提交 |
| `gt ready` | 查看可开始的工作 |
| `gt release <bead>` | 释放卡住的 in_progress Bead |
| `gt convoy create/list/status` | 工作批次追踪 |
| `gt synthesis status/run` | Convoy 批次聚合 |
| `gt mountain <epic-id>` | Epic 级自主研磨 |
| `gt mq list/status/retry` | 合并队列操作 |
| `gt changelog` | 完成记录 |
| `gt scheduler list/create` | 调度规则管理 |

### Agent 管理

| 命令 | 用途 |
|------|------|
| `gt polecat list/status/nuke` | Polecat 管理 |
| `gt crew list/start/stop/status` | Crew worker 管理 |
| `gt witness start/stop/status` | Witness 管理 |
| `gt refinery start/stop/status/queue` | Refinery 管理 |
| `gt deacon start/stop/status` | Deacon 守护进程管理 |
| `gt mayor status/nudge` | Mayor 全局协调 |
| `gt dog list/status/start` | Dog 跨 Rig worker |
| `gt boot status/restart` | Boot 看门狗 |
| `gt agents` | 列出所有 session |
| `gt peek <target>` | 查看 agent 输出 |
| `gt role` | 查看/管理当前角色 |

### 通信

| 命令 | 用途 |
|------|------|
| `gt mail inbox/send/read` | 持久消息 |
| `gt nudge <target> <msg>` | 即时消息 |
| `gt escalate <desc>` | 升级处理 |
| `gt broadcast <msg>` | 全员广播 |
| `gt resume` | 接收交接消息 |
| `gt remember/forget/memories` | 记忆管理 |

### 诊断

| 命令 | 用途 |
|------|------|
| `gt status` | 全局状态 |
| `gt doctor [--fix]` | 健康检查与修复 |
| `gt vitals` | 统一健康面板 |
| `gt trail [beads\|hooks]` | 活动追踪 |
| `gt audit --actor=...` | 工作历史 |
| `gt orphans` | 孤立项查找 |
| `gt dolt status` | Dolt 服务状态 |
| `gt feed` | 实时活动流 |
| `gt log` | 城镇活动日志 |
| `gt repair` | 修复数据库问题 |

### Beads (bd)

| 命令 | 用途 |
|------|------|
| `bd create/list/show` | 增删查 |
| `bd update <id> --status=...` | 更新状态 |
| `bd update <id> --notes/--design` | 持久化发现 |
| `bd close <id>` | 关闭 Bead |
| `gt close <id> [<id>...]` | 批量关闭 Bead（gt 命令） |
| `gt show/cat <id>` | 查看 Bead（gt 命令） |
| `bd ready` | 无阻塞可用工作 |
| `bd blocked` | 被阻塞的 Bead |
| `bd dep add/remove` | 依赖管理 |
| `bd graph` | 依赖图 |
| `bd formula list/show` | Formula 管理 |

### 服务与配置

| 命令 | 用途 |
|------|------|
| `gt install <dir>` | 首次安装工作空间 |
| `gt init` | 初始化当前目录为 Rig |
| `gt start / gt up` | 启动服务 |
| `gt shutdown / gt down` | 关闭服务 |
| `gt estop / gt thaw` | 紧急停车/恢复 |
| `gt rig list/add/start/stop` | Rig 管理 |
| `gt config list/set` | 配置管理 |
| `gt directive list/show` | 角色指令管理 |
| `gt hooks list/add` | Hook 脚本管理 |
| `gt account list/switch` | 账户管理 |
| `gt issue set/clear` | 状态栏 issue 显示 |
| `gt version / gt upgrade` | 版本与升级 |
| `gt completion <shell>` | Shell 补全 |
| `gt info` | Gas Town 信息 |

### 特殊与基础设施

| 命令 | 用途 |
|------|------|
| `gt daemon start/stop/status` | 后台守护进程 |
| `gt town status/config` | Town 级操作 |
| `gt wl status/sync` | Wasteland 联邦 |
| `gt krc set/get` | 临时数据 TTL 管理 |
| `gt warrant issue/list/execute` | 强制终止卡住 agent |
| `gt quota status/rotate` | 账户配额轮换 |
| `gt reaper reap/status` | Wisp/issue 清理 |
| `gt patrol` | 巡逻摘要 |
| `gt namepool list/available` | 名称池管理 |
| `gt theme` | tmux 主题管理 |