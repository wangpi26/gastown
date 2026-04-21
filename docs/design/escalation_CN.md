# Gas Town 升级协议

> Gas Town 统一升级系统的参考文档。

## 概述

Gas Town agent 在自动解决不可能时升级 issue。升级按严重性路由，
以 bead 追踪，并支持过期检测与自动再升级。

## 严重性级别

| 级别 | 优先级 | 描述 | 默认路由 |
|------|--------|------|----------|
| **CRITICAL** | P0（紧急） | 系统威胁性，需立即关注 | bead + mail + email + SMS |
| **HIGH** | P1（高） | 重要阻塞，需要人工尽快处理 | bead + mail + email |
| **MEDIUM** | P2（普通） | 标准升级，人工方便时处理 | bead + mail mayor |

## 分级升级流程

```
Agent -> gt escalate -s <SEVERITY> "description"
           |
           v
     [Deacon 接收]
           |
           +-- 可解决 --> 更新 issue，重新 sling 工作
           +-- 无法解决 --> 转发给 Mayor
                              +-- 可解决 --> 更新 issue，重新 sling
                              +-- 无法解决 --> 转发给 Overseer --> 解决
```

每个层级可以解决或转发。链路通过 bead 评论追踪。

## 配置

配置文件：`~/gt/settings/escalation.json`

### 默认配置

```json
{
  "type": "escalation",
  "version": 1,
  "routes": {
    "medium": ["bead", "mail:mayor"],
    "high": ["bead", "mail:mayor", "email:human"],
    "critical": ["bead", "mail:mayor", "email:human", "sms:human"]
  },
  "contacts": {
    "human_email": "",
    "human_sms": "",
    "slack_webhook": "",
    "smtp_host": "",
    "smtp_port": "587",
    "smtp_from": "",
    "smtp_user": "",
    "smtp_pass": "",
    "sms_webhook": ""
  },
  "stale_threshold": "4h",
  "max_reescalations": 2
}
```

### 动作类型

| 动作 | 格式 | 行为 |
|------|------|------|
| `bead` | `bead` | 创建升级 bead（始终第一，隐式的） |
| `mail:<target>` | `mail:mayor` | 向目标发送 gt mail |
| `email:human` | `email:human` | 向 `contacts.human_email` 发送邮件 |
| `sms:human` | `sms:human` | 向 `contacts.human_sms` 发送短信 |
| `slack` | `slack` | 发送到 `contacts.slack_webhook` |
| `log` | `log` | 写入升级日志文件 |

## 升级 Bead

升级 bead 使用 `type: escalation`，带有结构化标签用于追踪。

### 标签 Schema

| 标签 | 值 | 用途 |
|------|---|------|
| `severity:<level>` | MEDIUM、HIGH、CRITICAL | 当前严重性 |
| `source:<type>:<name>` | plugin:rebuild-gt、patrol:deacon | 触发来源 |
| `acknowledged:<bool>` | true、false | 人工是否已确认 |
| `reescalated:<bool>` | true、false | 是否已被再升级 |
| `reescalation_count:<n>` | 0、1、2、... | 再升级次数 |
| `original_severity:<level>` | MEDIUM、HIGH | 初始严重性 |

## 分类路由（未来）

分类基于升级的性质提供结构化路由。
尚未作为 CLI 标志实现；目前使用 `--to` 进行显式路由。

| 类别 | 描述 | 默认路由 |
|------|------|----------|
| `decision` | 多种有效路径，需要选择 | Deacon -> Mayor |
| `help` | 需要指导或专业知识 | Deacon -> Mayor |
| `blocked` | 等待无法解决的依赖 | Mayor |
| `failed` | 意外错误，无法继续 | Deacon |
| `emergency` | 安全或数据完整性问题 | Overseer（直接） |
| `gate_timeout` | Gate 未在时间内解决 | Deacon |
| `lifecycle` | Worker 卡住或需要回收 | Witness |

## 命令

### gt escalate

创建新的升级。

```bash
gt escalate -s <MEDIUM|HIGH|CRITICAL> "简短描述" \
  [-m "详细说明"] [--source="plugin:rebuild-gt"]
```

标志：`-s` 严重性（必需）、`-m` 正文、`--source` 来源标识符、
`--to` 路由到层级（deacon/mayor/overseer）、`--dry-run`、`--json`。

### gt escalate ack

确认升级（防止再升级）。

```bash
gt escalate ack <bead-id> [--note="正在调查"]
```

### gt escalate list

```bash
gt escalate list [--severity=...] [--stale] [--unacked] [--all] [--json]
```

### gt escalate stale

对过期（超过 `stale_threshold` 未确认）的升级进行再升级。
提升严重性（MEDIUM->HIGH->CRITICAL），重新执行路由，
遵守 `max_reescalations`。

```bash
gt escalate stale [--dry-run]
```

### gt escalate close

```bash
gt escalate close <bead-id> [--reason="在 commit abc123 中修复"]
```

## 集成点

### 插件系统

插件使用升级进行故障通知：

```bash
gt escalate -s MEDIUM "插件失败: rebuild-gt" \
  -m "$ERROR" --source="plugin:rebuild-gt"
```

### Deacon 巡逻

Deacon 使用升级报告健康问题：

```bash
if [ $unresponsive_cycles -ge 5 ]; then
  gt escalate -s HIGH "Witness 无响应: gastown" \
    -m "Witness 已 $unresponsive_cycles 个周期无响应" \
    --source="patrol:deacon:health-scan"
fi
```

Deacon 巡逻还定期运行 `gt escalate stale` 以捕获未确认的
升级并进行再升级。

## 何时升级

### Agent 应该升级的情况：

- **系统错误**：数据库损坏、磁盘满、网络故障
- **安全问题**：未授权访问尝试、凭证暴露
- **无法解决的冲突**：无法自动解决的合并冲突
- **需求模糊**：规格不清晰，多种合理解释
- **设计决策**：需要人工判断的架构选择
- **卡住循环**：Agent 卡住无法前进
- **Gate 超时**：异步条件未在预期时间内解决

### Agent 不应该升级的情况：

- **正常工作流**：无需人工输入即可进行的常规工作
- **可恢复错误**：会自动重试的瞬时故障
- **信息查询**：可以从上下文中回答的问题

## Mayor 启动检查

在 `gt prime` 时，Mayor 显示按严重性分组的待处理升级。
操作：使用 `bd list --tag=escalation` 查看，使用 `bd close <id> --reason "..."` 关闭。


## 查看升级

```bash
# 列出所有打开的升级
bd list --status=open --tag=escalation

# 按类别过滤
bd list --tag=escalation --tag=decision

# 查看特定升级
bd show <escalation-id>

# 关闭已解决的升级
bd close <id> --reason "通过修复 X 解决"
```