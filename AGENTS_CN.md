# Agent 指令

完整的 Agent 上下文和指令请参见 **CLAUDE.md**。

此文件用于兼容查找 AGENTS.md 的工具。

> **恢复**：在 compaction、clear 或新会话后运行 `gt prime`

完整上下文在会话启动时由 `gt prime` 注入。

<!-- beads-agent-instructions-v2 -->

---

## Beads 工作流集成

本项目使用 [beads](https://github.com/steveyegge/beads) 进行问题追踪。Issue 存放在 `.beads/` 目录中，并通过 git 进行追踪。

两个 CLI：**bd**（Issue CRUD）和 **bv**（图感知分流，只读）。

### bd：Issue 管理

```bash
bd ready              # 查看已解除阻塞、可立即开始的工作
bd list --status=open # 查看所有 open 状态的 issue
bd show <id>          # 查看完整详情及依赖关系
bd create --title="..." --type=task --priority=2
bd update <id> --status=in_progress
bd close <id>         # 标记完成
bd close <id1> <id2>  # 批量关闭
bd dep add <a> <b>    # a 依赖于 b
bd sync               # 与 git 同步
```

### bv：图分析（只读）

**绝对不要直接运行 `bv`** —— 它会启动交互式 TUI。请始终使用 `--robot-*` 标志：

```bash
bv --robot-triage     # 排名推荐、快速取胜项、阻塞项、健康度
bv --robot-next       # 单个最优推荐 + 认领命令
bv --robot-plan       # 并行执行轨道
bv --robot-alerts     # 过期 issue、级联问题、不匹配项
bv --robot-insights   # 完整图指标：PageRank、介数、环路
```

### 工作流

1. **开始**：`bd ready`（或 `bv --robot-triage` 进行图分析）
2. **认领**：`bd update <id> --status=in_progress`
3. **工作**：实现任务
4. **完成**：`bd close <id>`
5. **同步**：会话结束时运行 `bd sync`

### 会话结束协议

```bash
git status            # 检查变更内容
git add <files>       # 暂存代码变更
bd sync               # 提交 beads 变更
git commit -m "..."   # 提交代码
bd sync               # 提交任何新的 beads 变更
git push              # 推送到远程
```

### 关键概念

- **优先级**：P0=关键, P1=高, P2=中, P3=低, P4=积压（仅使用数字）
- **类型**：task, bug, feature, epic, question, docs
- **依赖**：`bd ready` 仅显示未受阻塞的工作

<!-- end-beads-agent-instructions -->

<!-- gastown-agent-instructions-v1 -->

---

## Gas Town 多 Agent 通信

本工作空间是 **Gas Town** 多 Agent 环境的一部分。你使用 `gt` 命令与其他 Agent 通信——绝不通过打印文本或使用原始 tmux。

### Nudging Agent（即时送达）

`gt nudge` 将消息直接发送到另一个 Agent 的活跃会话：

```bash
gt nudge mayor "Status update: PR review complete"
gt nudge laneassist/crew/dom "Check your mail — PR ready for review"
gt nudge witness "Polecat health check needed"
gt nudge refinery "Merge queue has items"
```

**目标格式：**
- 角色快捷名：`mayor`, `deacon`, `witness`, `refinery`
- 完整路径：`<rig>/crew/<name>`, `<rig>/polecats/<name>`

**重要：** `gt nudge` 是向另一个 Agent 会话发送文本的唯一方式。不要打印 "Hey @name"——另一个 Agent 无法看到你的终端输出。

### 发送邮件（持久消息）

`gt mail` 发送的消息在会话重启后仍然保留：

```bash
# 读取
gt mail inbox                    # 列出消息
gt mail read <id>                # 读取指定消息

# 发送（多行内容使用 --stdin）
gt mail send mayor/ -s "Subject" -m "Short message"
gt mail send laneassist/crew/dom -s "PR Review" --stdin <<'BODY'
Multi-line message content here.
Details about the PR and what to look for.
BODY
gt mail send --human -s "Subject" -m "Message to overseer"
```

### 何时使用哪种方式

| 想要... | 命令 | 原因 |
|---------|------|------|
| 唤醒休眠的 Agent | `gt nudge <target> "msg"` | 即时送达 |
| 发送详细的任务/信息 | `gt mail send <target> -s "..." --stdin` | 重启后仍保留 |
| 两者兼顾：发送 + 唤醒 | 先 `gt mail send` 再 `gt nudge` | 邮件承载内容，nudge 唤醒 |

### 上下文恢复

在 compaction 或新会话后，运行 `gt prime` 以重新加载完整的角色上下文、身份和待处理工作。

```bash
gt prime              # 完整上下文重新加载
gt hook               # 检查已分配的工作
gt mail inbox         # 检查消息
```

<!-- end-gastown-agent-instructions -->

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:ca08a54f -->
## Beads 问题追踪器

本项目使用 **bd (beads)** 进行问题追踪。运行 `bd prime` 查看完整的工作流上下文和命令。

### 快速参考

```bash
bd ready              # 查找可用工作
bd show <id>          # 查看 issue 详情
bd update <id> --claim  # 认领工作
bd close <id>         # 完成工作
```

### 规则

- 所有任务追踪使用 `bd` —— 不要使用 TodoWrite、TaskCreate 或 markdown TODO 列表
- 运行 `bd prime` 获取详细命令参考和会话结束协议
- 持久化知识使用 `bd remember` —— 不要使用 MEMORY.md 文件

## 会话完成

**结束工作会话时**，你必须完成以下所有步骤。在 `git push` 成功之前，工作不算完成。

**强制工作流：**

1. **为剩余工作创建 issue** —— 为需要后续跟进的事项创建 issue
2. **运行质量门检查**（如果代码有变更）—— 测试、代码检查、构建
3. **更新 issue 状态** —— 关闭已完成的工作，更新进行中的项目
4. **推送到远程** —— 这是必须的：
   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status  # 必须显示 "up to date with origin"
   ```
5. **清理** —— 清除暂存，修剪远程分支
6. **验证** —— 所有变更已提交且已推送
7. **交接** —— 为下一个会话提供上下文

**关键规则：**
- 在 `git push` 成功之前，工作不算完成
- 绝不在推送前停止 —— 那会让工作滞留在本地
- 绝不说 "ready to push when you are" —— 你必须自行推送
- 如果推送失败，解决后重试直到成功
<!-- END BEADS INTEGRATION -->