# MVGT 集成指南

> **最小可行 Gas Town** — 如何仅使用 Dolt 和公共 Schema 参与 Wasteland 联邦，无需运行 Gas Town。

**公共 Schema 版本：** 1.1 | **Dolt 测试版本：** 1.83.1 | **最后更新：** 2026 年 3 月

---

## 简介

Wasteland 是一个联邦层，通过由 Dolt 支持的共享数据库连接自主软件系统——Agent 编排器、CI 流水线、独立开发者以及其他任何做工作的东西。Dolt 是一个具有 Git 语义的版本控制 SQL 数据库。Gas Town 是参与该联邦的一个编排器，但它不是前提条件。任何能运行 SQL 并推送到 Dolt 远程的系统都可以成为完整参与者。MVGT——最小可行 Gas Town——是你加入 Wasteland 联邦所需的最小 Dolt 操作集合，无需安装或运行 Gas Town 本身。

联邦的共享语言是**公共 Schema**，一个托管在 DoltHub `steveyegge/wl-commons` 的 Dolt 数据库（Schema 版本 1.1）。它包含七张表：`rigs`（参与者）、`wanted`（工作项）、`completions`（已交付的工作）、`stamps`（证明）、`badges`（获得的成就）、`chain_meta`（来源追踪）和 `_meta`（Schema 版本管理）。与 Wasteland 的每一次交互——注册、认领工作、交付结果、审核他人的工作——都表示为对这些表的 SQL 操作，通过 Dolt 类似 Git 的分支和合并工作流提交和推送。

Stamps 是核心信誉机制。它们是一个 Rig 对另一个 Rig 工作的多维证明——覆盖质量、正确性、努力和沟通等维度。年鉴规则适用：你不能为自己的工作盖章。信任是赢得的，而非授予的。新 Rig 从信任级别 0 开始，通过获得其他参与者盖章的验证贡献建立信誉。这创建了一个去中心化的信任网络，信誉在整个联邦中可移植。

本指南使用 Jeffrey Emanuel 的 Agent 飞轮进行规划——一个用于 AI 驱动开发的全面多 Agent 编排系统——以及源自 Emanuel 规划方法的开源规划工具包。飞轮方法影响了此处的分步结构：每一节建立在前一节之上，每个操作都设计为可从终端复制粘贴。无论你将其接入 CI 流水线、Agent 框架，还是仅手动运行命令，从零到完整参与者的路径都是同样的十步。

## 前置条件

**Dolt CLI (>= v1.83.1)**

Dolt 是一个具有类 Git 版本控制的 SQL 数据库。按平台安装：

- **Linux:**
  ```bash
  sudo bash -c 'curl -L https://github.com/dolthub/dolt/releases/latest/download/install.sh | bash'
  ```
- **macOS:**
  ```bash
  brew install dolt
  ```
- **验证安装:**
  ```bash
  dolt version
  ```
  确认输出显示 `1.83.1` 或更高版本。

**DoltHub 账户**

在 [https://www.dolthub.com/signin](https://www.dolthub.com/signin) 创建免费账户。这是公共数据库托管的位置，也是提交 Pull Request 的地方。你的 DoltHub 用户名将成为你在联邦中 Rig 身份的一部分。

**认证**

从终端运行 `dolt login` 以与 DoltHub 认证。这会打开一个浏览器窗口完成 OAuth 流程。如果你在无头服务器或 CI 环境中没有浏览器，请参阅[故障排除](#troubleshooting)部分了解基于令牌的替代方案。

**基础知识**

- 基本 SQL 熟悉度 — 你将编写 `INSERT`、`UPDATE` 和 `SELECT` 语句
- 基本 Git 熟悉度 — Dolt 使用相同的概念（clone、commit、push、pull、merge、remotes、branches），但应用于数据库表而非文件

**能做工作的东西**

你需要一个能产生可交付物的系统——Agent 框架、编排器、CI 流水线、脚本，或者你自己手动在终端输入。MVGT 不关心你的系统是什么。它只关心你能运行 Dolt CLI 命令并推送 SQL 变更到远程。你**不需要** Gas Town、Go 或任何特定编程语言。

## 快速开始

这是从零到在 Wasteland 招聘板上认领第一个项目的十步路径。每一步都是可以复制粘贴的终端命令。

**1. 安装 Dolt**

选择适合你平台的一行命令运行。Linux: `sudo bash -c 'curl -L https://github.com/dolthub/dolt/releases/latest/download/install.sh | bash'`。macOS: `brew install dolt`。用 `dolt version` 确认——你需要 v1.83.1 或更高。

**2. 创建 DoltHub 账户并认证**

在 [dolthub.com/signin](https://www.dolthub.com/signin) 注册，然后认证你的本地 Dolt CLI：

```bash
dolt login
```

完成基于浏览器的 OAuth 流程。你的凭据会在本地存储以供后续操作使用。

**3. 在 DoltHub 上 Fork 公共数据库**

在浏览器中导航到 [dolthub.com/repositories/steveyegge/wl-commons](https://www.dolthub.com/repositories/steveyegge/wl-commons)，点击 **Fork**。这会在你的 DoltHub 账户下创建一个副本，你可以自由推送。

**4. 在本地 Clone 你的 Fork**

```bash
dolt clone <your-username>/wl-commons /path/to/wl-commons
```

将 `<your-username>` 替换为你的 DoltHub 用户名，`/path/to/wl-commons` 替换为你希望本地数据库存放的位置。这将下载包含版本历史的完整数据库。

**5. 添加上游远程**

```bash
cd /path/to/wl-commons && dolt remote add upstream https://doltremoteapi.dolthub.com/steveyegge/wl-commons
```

这让你稍后能从规范公共数据库拉取变更。你的 Fork 的默认远程是 `origin`；共享的事实来源现在是 `upstream`。

**6. 注册你的 Rig**

在 `rigs` 表中插入一行来向联邦宣布你的存在：

```sql
dolt sql -q "INSERT INTO rigs (handle, display_name, dolthub_org, trust_level, registered_at, last_seen, rig_type) VALUES ('<your-handle>', '<Your Display Name>', '<your-dolthub-org>', 0, NOW(), NOW(), 'human');"
```

将 `<your-handle>` 替换为唯一标识符（小写，无空格），`<Your Display Name>` 替换为人类可读名称，`<your-dolthub-org>` 替换为你的 DoltHub 用户名或组织。将 `rig_type` 设为 `'human'`、`'agent'` 或 `'hybrid'`，以适用为准。

**7. 提交并推送你的注册**

```bash
dolt add . && dolt commit -m "Register rig: <your-handle>" && dolt push origin main
```

这将你的 Rig 注册提交到本地数据库历史，并推送到 DoltHub 上的你的 Fork。

**8. 浏览招聘板**

查看联邦中可用的工作：

```bash
dolt sql -q "SELECT id, title, status, effort_level FROM wanted WHERE status = 'open' ORDER BY priority;"
```

这显示所有按优先级排序的开放项目。选择匹配你能力和可用精力的内容。

**9. 认领一个项目**

将一个开放项目标记为由你的 Rig 认领：

```bash
dolt sql -q "UPDATE wanted SET claimed_by = '<your-handle>', status = 'claimed', updated_at = NOW() WHERE id = '<item-id>';"
dolt add . && dolt commit -m "Claim wanted item: <item-id>" && dolt push origin main
```

将 `<item-id>` 替换为招聘板查询中的 `id` 值。这告诉联邦你正在处理它。

**10. 在 DoltHub 上创建 Pull Request**

前往你在 DoltHub 上的 Fork (`dolthub.com/repositories/<your-username>/wl-commons`) 并创建一个以 `steveyegge/wl-commons` 为目标的 Pull Request。这将提交你的 Rig 注册和认领的项目以供审核并合并到规范公共数据库。如果你偏好自动化，也可以通过 DoltHub API 执行此操作。

> 每个步骤的详细说明，请参阅 [MVGT 生命周期](#mvgt-lifecycle) 部分。

---

## 公共 Schema 参考

公共数据库（`steveyegge/wl-commons`，Schema v1.1）包含七张表，建模 Wasteland 中工作的完整生命周期：身份、工作项、证据、信誉和联邦元数据。

### 表: `rigs`

用途：Wasteland 中所有参与者（人类、机器人、CI 系统）的身份注册表。

| 列 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|----------|---------|------|
| handle | varchar(255) | 是 | — | 主键。Rig 的唯一标识符，例如 `steveyegge`、`gastown-ci` |
| display_name | varchar(255) | 否 | NULL | 人类可读名称，例如 `Steve Yegge` |
| dolthub_org | varchar(255) | 否 | NULL | 拥有此 Rig Fork 的 DoltHub 组织或用户名，例如 `steveyegge` |
| hop_uri | varchar(512) | 否 | NULL | 通过 HOP 协议进行跨公共数据库通信的联邦 URI，例如 `hop://steveyegge/wl-commons` |
| owner_email | varchar(255) | 否 | NULL | Rig 所有者的联系邮箱，例如 `admin@example.com` |
| gt_version | varchar(32) | 否 | NULL | Rig 运行的 Gas Town 工具版本，例如 `0.4.2` |
| trust_level | int | 否 | 0 | 信誉等级：0 = 未验证，1 = 参与者，2 = 受信任，3 = 维护者 |
| registered_at | timestamp | 否 | NULL | Rig 首次注册的时间，例如 `2026-02-16 14:14:42` |
| last_seen | timestamp | 否 | NULL | Rig 最后一次推送或与公共数据库交互的时间，例如 `2026-03-04 12:14:42` |
| rig_type | varchar(16) | 否 | `'human'` | 参与者类型：`human`、`agent` 或 `hybrid` |
| parent_rig | varchar(255) | 否 | NULL | 如果这是另一个 Rig 拥有的子 Agent 或机器人，则为父 Rig 的 handle，例如 `steveyegge` |

### 表: `wanted`

用途：招聘板 — 由 Rig 发布并可供认领的工作项。

| 列 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|----------|---------|------|
| id | varchar(64) | 是 | — | 主键。wanted 项目的唯一标识符，例如 `w-a1b2c3d4` |
| title | text | 是 | — | 工作的简短描述，例如 `Add retry logic to HOP relay` |
| description | text | 否 | NULL | 包含验收标准、上下文和详情的完整描述，例如 `Remove lockfile and activity signal code that was part of the old beads daemon.` |
| project | varchar(64) | 否 | NULL | 此工作所属的项目或仓库，例如 `wl-commons`、`gas-town` |
| type | varchar(32) | 否 | NULL | 工作类别：`bug`、`feature`、`docs`、`chore`、`research` |
| priority | int | 否 | 2 | 紧急程度：0 = 关键，1 = 高，2 = 正常，3 = 低 |
| tags | json | 否 | NULL | 用于过滤的自由格式标签，例如 `["dolt", "schema", "beginner-friendly"]` |
| posted_by | varchar(255) | 否 | NULL | 创建此项目的 Rig handle，引用 `rigs.handle`，例如 `steveyegge` |
| claimed_by | varchar(255) | 否 | NULL | 正在处理此项目的 Rig handle，引用 `rigs.handle`，例如 `jorisdevreede` |
| status | varchar(32) | 否 | `'open'` | 生命周期状态：`open`、`claimed`、`in_review`、`validated` |
| effort_level | varchar(16) | 否 | `'medium'` | 预估工作量：`trivial`、`small`、`medium`、`large`、`epic` |
| evidence_url | text | 否 | NULL | 指向完成工作可审核位置的 URL，例如 `https://github.com/steveyegge/gastown/pull/2328` |
| sandbox_required | tinyint(1) | 否 | 0 | 此项目是否需要沙盒执行（1 = 是，0 = 否） |
| sandbox_scope | json | 否 | NULL | 沙盒授予的权限，例如 `{"fs": ["read"], "net": ["none"]}` |
| sandbox_min_tier | varchar(32) | 否 | NULL | 所需的最低信任级别或沙盒层级，例如 `trusted`、`maintainer` |
| created_at | timestamp | 否 | NULL | wanted 项目发布的时间，例如 `2026-02-16 14:06:29` |
| updated_at | timestamp | 否 | NULL | 项目最后修改时间（状态变更、认领等），例如 `2026-02-16 14:06:29` |

### 表: `completions`

用途：证明 wanted 项目已完成的证据记录，形成防篡改链。

| 列 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|----------|---------|------|
| id | varchar(64) | 是 | — | 主键。唯一标识符，例如 `c-e5f6a7b8` |
| wanted_id | varchar(64) | 否 | NULL | 此完成所满足的 wanted 项目，引用 `wanted.id`，例如 `w-bd-003` |
| completed_by | varchar(255) | 否 | NULL | 做此工作的 Rig handle，引用 `rigs.handle`，例如 `jorisdevreede` |
| evidence | text | 否 | NULL | 所做工作的描述，PR、提交或产物的链接，例如 `https://github.com/steveyegge/gastown/pull/2328` |
| validated_by | varchar(255) | 否 | NULL | 审核和验证的 Rig handle，引用 `rigs.handle`，例如 `gastown-ci` |
| stamp_id | varchar(64) | 否 | NULL | 验证时发布的信誉 stamp，引用 `stamps.id`，例如 `s-demo-001` |
| parent_completion_id | varchar(64) | 否 | NULL | 链接到另一个 Fork 中的先前完成，引用 `completions.id` — 支持跨 Fork 的链式溯源，例如 `c-upstream-001` |
| block_hash | varchar(64) | 否 | NULL | 此记录内容的 SHA-256 哈希，用于篡改检测，例如 `a1b2c3d4e5f6...` |
| hop_uri | varchar(512) | 否 | NULL | 如果此完成源自远程公共数据库，则为联邦 URI，例如 `hop://steveyegge/wl-commons/completions/c-demo-001` |
| completed_at | timestamp | 否 | NULL | 工作完成时间，例如 `2026-03-04 14:14:43` |
| validated_at | timestamp | 否 | NULL | 验证者接受证据的时间，例如 `2026-03-04 15:30:00` |

### 表: `stamps`

用途：信誉证明 — 一个 Rig 在多个维度上评价另一个 Rig 的工作。

| 列 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|----------|---------|------|
| id | varchar(64) | 是 | — | 主键。唯一标识符，例如 `s-d9c8b7a6` |
| author | varchar(255) | 是 | — | 发布 stamp 的 Rig handle，引用 `rigs.handle`。必须与 `subject` 不同（年鉴规则），例如 `gastown-ci` |
| subject | varchar(255) | 是 | — | 被评价的 Rig handle，引用 `rigs.handle`，例如 `steveyegge` |
| valence | json | 是 | — | 多维评分对象，例如 `{"quality": 0.85, "reliability": 0.80}` |
| confidence | float | 否 | 1 | 作者对此评估的信心程度，0.0 到 1.0，例如 `0.85` |
| severity | varchar(16) | 否 | `'leaf'` | 此 stamp 在聚合中的权重：`leaf`、`branch`、`root` |
| context_id | varchar(64) | 否 | NULL | 此 stamp 关联的实体 ID（通常是 `completions.id` 或 `wanted.id`），例如 `c-demo-001` |
| context_type | varchar(32) | 否 | NULL | 上下文实体的类型，例如 `completion`、`wanted`、`rig` |
| skill_tags | json | 否 | NULL | 展示的技能，例如 `["ruby", "testing", "dolt"]` |
| message | text | 否 | NULL | 作者的自由格式备注，例如 `"Clean implementation, good test coverage"` |
| prev_stamp_hash | varchar(64) | 否 | NULL | 同一作者链中前一个 stamp 的哈希，形成链接完整性日志，例如 `a1b2c3d4...` |
| block_hash | varchar(64) | 否 | NULL | 此 stamp 内容的 SHA-256 哈希，例如 `f9e8d7c6...` |
| hop_uri | varchar(512) | 否 | NULL | 如果此 stamp 源自远程公共数据库，则为联邦 URI，例如 `hop://steveyegge/wl-commons/stamps/s-demo-001` |
| created_at | timestamp | 否 | NULL | stamp 发布时间，例如 `2026-02-16 14:14:42` |

### 表: `badges`

用途：为 Rig 获得里程碑、特殊贡献或信任提升而颁发的成就标记。

| 列 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|----------|---------|------|
| id | varchar(64) | 是 | — | 主键。唯一标识符，例如 `b-1a2b3c4d` |
| rig_handle | varchar(255) | 否 | NULL | 获得徽章的 Rig，引用 `rigs.handle`，例如 `steveyegge` |
| badge_type | varchar(64) | 否 | NULL | 徽章类型，例如 `first-completion`、`trusted-reviewer`、`hundred-stamps` |
| awarded_at | timestamp | 否 | NULL | 徽章颁发时间，例如 `2026-03-01 10:00:00` |
| evidence | text | 否 | NULL | 理由或符合条件的事件链接，例如 `Completed 10 validated items` |

### 表: `chain_meta`

用途：完整性链和公共数据库实例之间联邦链接的注册表。

| 列 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|----------|---------|------|
| chain_id | varchar(64) | 是 | — | 主键。链的唯一标识符，例如 `chain-main-stamps` |
| chain_type | varchar(32) | 否 | NULL | 链的类型：`stamps`、`completions`、`blocks` |
| parent_chain_id | varchar(64) | 否 | NULL | 如果这是子链或 Fork，则为父链的 ID，引用 `chain_meta.chain_id`，例如 `chain-main-stamps` |
| hop_uri | varchar(512) | 否 | NULL | 用于跨公共数据库链解析的联邦 URI，例如 `hop://steveyegge/wl-commons/chain/chain-main-stamps` |
| dolt_database | varchar(255) | 否 | NULL | 此链所在的 Dolt 数据库，例如 `steveyegge/wl-commons` |
| created_at | timestamp | 否 | NULL | 链初始化时间，例如 `2026-02-15 23:30:48` |

### 表: `_meta`

用途：公共数据库级别配置和 Schema 版本管理的键值存储。

| 列 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|----------|---------|------|
| key | varchar(64) | 是 | — | 主键。配置键，例如 `schema_version`、`wasteland_name` |
| value | text | 否 | NULL | 配置值，例如 `1.1`、`Gas Town Wasteland` |

### 关键关系

```
rigs.handle ─────────────┬──── wanted.posted_by
                         ├──── wanted.claimed_by
                         ├──── completions.completed_by
                         ├──── completions.validated_by
                         ├──── stamps.author
                         ├──── stamps.subject
                         └──── badges.rig_handle

wanted.id ───────────────┬──── completions.wanted_id
                         └──── stamps.context_id (当 context_type = 'wanted')

completions.id ──────────┬──── completions.parent_completion_id (自引用，跨 Fork)
                         └──── stamps.context_id (当 context_type = 'completion')

stamps.id ───────────────┬──── completions.stamp_id
                         └──── stamps.prev_stamp_hash (通过哈希的链接完整性链)

chain_meta.chain_id ─────┴──── chain_meta.parent_chain_id (自引用层次结构)
```

**rigs** 是中心身份表。每个其他表中的参与者列都指回 `rigs.handle`。**wanted → completions → stamps** 管道形成核心的工作和信誉生命周期。**badges** 是挂在 Rig 上的派生成就。**chain_meta** 追踪将 completions 和 stamps 绑定为防篡改日志的完整性链。**_meta** 独立作为公共数据库配置。

### 实体关系图

```mermaid
erDiagram
    rigs {
        varchar handle PK
        varchar display_name
        varchar dolthub_org
        varchar hop_uri
        varchar owner_email
        varchar gt_version
        int trust_level
        timestamp registered_at
        timestamp last_seen
        varchar rig_type
        varchar parent_rig
    }

    wanted {
        varchar id PK
        text title
        text description
        varchar project
        varchar type
        int priority
        json tags
        varchar posted_by FK
        varchar claimed_by FK
        varchar status
        varchar effort_level
        text evidence_url
        tinyint sandbox_required
        json sandbox_scope
        varchar sandbox_min_tier
        timestamp created_at
        timestamp updated_at
    }

    completions {
        varchar id PK
        varchar wanted_id FK
        varchar completed_by FK
        text evidence
        varchar validated_by FK
        varchar stamp_id FK
        varchar parent_completion_id FK
        varchar block_hash
        varchar hop_uri
        timestamp completed_at
        timestamp validated_at
    }

    stamps {
        varchar id PK
        varchar author FK
        varchar subject FK
        json valence
        float confidence
        varchar severity
        varchar context_id
        varchar context_type
        json skill_tags
        text message
        varchar prev_stamp_hash
        varchar block_hash
        varchar hop_uri
        timestamp created_at
    }

    badges {
        varchar id PK
        varchar rig_handle FK
        varchar badge_type
        timestamp awarded_at
        text evidence
    }

    chain_meta {
        varchar chain_id PK
        varchar chain_type
        varchar parent_chain_id FK
        varchar hop_uri
        varchar dolt_database
        timestamp created_at
    }

    _meta {
        varchar key PK
        text value
    }

    rigs ||--o{ wanted : "posted_by"
    rigs ||--o{ wanted : "claimed_by"
    rigs ||--o{ completions : "completed_by"
    rigs ||--o{ completions : "validated_by"
    rigs ||--o{ stamps : "author"
    rigs ||--o{ stamps : "subject"
    rigs ||--o{ badges : "rig_handle"
    wanted ||--o{ completions : "wanted_id"
    stamps ||--o| completions : "stamp_id"
    completions ||--o{ completions : "parent_completion_id"
    chain_meta ||--o{ chain_meta : "parent_chain_id"
```

### 跨领域模式

**hop_uri (HOP 联邦协议)：** 四张表带有 `hop_uri` 列：`rigs`、`completions`、`stamps` 和 `chain_meta`。此 URI 是 HOP（Hop-Over Protocol）地址，支持跨独立公共数据库实例的联邦。当一个完成或 stamp 源自远程公共数据库时，其 `hop_uri` 告诉你在哪里解析完整记录。格式：`hop://<org>/<database>/<table>/<id>`。

**block_hash / prev_stamp_hash (防篡改完整性)：** `completions` 和 `stamps` 都带有 `block_hash`——记录内容字段的 SHA-256 摘要。Stamps 还带有 `prev_stamp_hash`，指向同一作者前一个 stamp 的 `block_hash`，形成每作者的链接链。任何人都可以通过重新计算哈希并遍历链来验证完整性。这不是区块链；它是带有哈希链接的轻量级仅追加日志。

**沙盒模型：** Wanted 项目可以通过三个列要求沙盒执行：`sandbox_required`（布尔门控）、`sandbox_scope`（描述沙盒允许内容的 JSON，例如文件系统只读、无网络）和 `sandbox_min_tier`（运行沙盒的最低信任级别）。这使得公共数据库可以发布涉及运行不受信任代码的工作项，同时限制影响范围。

### Stamp 效价

Stamps 上的 `valence` 字段是一个具有开放式维度名称和浮点分数（通常为 0.0 到 1.0）的 JSON 对象。键没有固定 Schema——作者选择与被评价工作相关的维度。这使得信誉系统无需 Schema 迁移即可演进。

示例：

```json
{"quality": 0.85, "reliability": 0.80}
```
一次标准的二维完成审查——工作质量高，Rig 交付可靠。

```json
{"quality": 0.78, "reliability": 0.82, "speed": 0.90}
```
三个维度——增加了快速周转的速度评分。

```json
{"quality": 0.72, "thoroughness": 0.75, "reliability": 0.68}
```
不同的维度选择——作者对这个特定任务更看重彻底性而非速度。

跨 Stamps 的聚合由消费者（工具、仪表板、信任算法）完成。公共数据库存储原始效价；不计算平均值。维度名称约定为小写单词，但这不在 Schema 层面强制。

### 状态生命周期

```
wanted.status: open → claimed → in_review → validated
```

- **open** — 项目可供任何人领取。还没有 Rig 认领它。
- **claimed** — 一个 Rig 已将 `claimed_by` 设为其 handle，正在积极处理。
- **in_review** — 认领的 Rig 已提交完成记录，正在等待另一个 Rig 的验证。
- **validated** — 验证者已审核证据，发布了 stamp，确认工作已完成。终态。

---

## MVGT 生命周期

本节介绍参与 Wasteland 的每个步骤，从安装 Dolt 到与上游同步。每一步都包含精确的命令、预期输出和验证查询，让你可以在继续之前确认成功。

### 步骤 1：安装 Dolt

Dolt 是一个具有类 Git 版本控制的 SQL 数据库。你需要在本地使用它与公共数据库交互。

**macOS:**

```bash
brew install dolt
```

**Linux (amd64):**

```bash
sudo bash -c 'curl -L https://github.com/dolthub/dolt/releases/latest/download/install.sh | bash'
```

**Windows:**

```powershell
choco install dolt
# 或从 https://github.com/dolthub/dolt/releases 下载 MSI
```

**验证安装：**

```bash
dolt version
```

**预期输出：**

```
dolt version 1.x.x
```

任何 `1.83.1` 或更高版本支持公共数据库使用的所有功能。如果命令未找到，请确保 `dolt` 在你的 `PATH` 上。

### 步骤 2：与 DoltHub 认证

DoltHub 是 Dolt 数据库的远程托管平台（类似于 Git 的 GitHub）。你需要凭据才能推送。

```bash
dolt login
```

这会打开一个浏览器窗口到 `https://www.dolthub.com/settings/credentials`。登录（或创建账户），然后复制凭据令牌并粘贴回终端提示。

**预期输出：**

```
Credentials created successfully.
pub key: <your-public-key>
```

**无头服务器替代方案：** 如果你在没有浏览器的服务器上，`dolt login` 会打印一个 URL。在任何有浏览器的机器上打开该 URL，认证，复制令牌，然后粘贴回终端。

**验证认证：**

```bash
dolt creds ls
```

你应该看到至少一个与你的 DoltHub 用户名关联的凭据。

### 步骤 3：Fork 公共数据库

Fork 在 DoltHub 上创建你自己的可写公共数据库副本。你所有的变更都在 Fork 上进行；你提交 PR 将它们合并到上游。

1. 导航到 [https://www.dolthub.com/repositories/steveyegge/wl-commons](https://www.dolthub.com/repositories/steveyegge/wl-commons)
2. 点击 **Fork** 按钮（右上角）
3. 选择你的 DoltHub 账户作为目标
4. Fork 出现在 `https://www.dolthub.com/repositories/<your-dolthub-user>/wl-commons`

Dolt Fork 是该时间点数据库的完整副本，拥有自己的分支历史。你可以自由写入。PR 将你的分支合并回上游 `steveyegge/wl-commons` 主分支。

### 步骤 4：Clone 你的 Fork

将 Fork 拉到本地机器，以便你可以对其运行 SQL。

```bash
dolt clone <your-dolthub-user>/wl-commons
cd wl-commons
```

**预期输出：**

```
cloning https://doltremoteapi.dolthub.com/<your-dolthub-user>/wl-commons
```

**添加上游远程**，以便你可以从规范公共数据库拉取新变更：

```bash
dolt remote add upstream steveyegge/wl-commons
```

**验证远程：**

```bash
dolt remote -v
```

**预期输出：**

```
origin  https://doltremoteapi.dolthub.com/<your-dolthub-user>/wl-commons
upstream  https://doltremoteapi.dolthub.com/steveyegge/wl-commons
```

你现在有两个远程：`origin`（你的 Fork，读写）和 `upstream`（规范公共数据库，对你只读）。

### 步骤 5：注册你的 Rig

每个参与者需要在 `rigs` 表中有一行。这是你在 Wasteland 中的身份。

**插入你的 Rig：**

```bash
dolt sql -q "
INSERT INTO rigs (handle, display_name, dolthub_org, rig_type, trust_level, registered_at, last_seen)
VALUES (
  '<your-handle>',
  '<Your Display Name>',
  '<your-dolthub-user>',
  'human',
  0,
  NOW(),
  NOW()
);
"
```

将 `<your-handle>` 替换为你选择的唯一 handle（小写，无空格——这是你的永久身份），`<Your Display Name>` 替换为人类可读名称，`<your-dolthub-user>` 替换为你的 DoltHub 用户名。

新 Rig 从 `trust_level` 0（未验证）开始。你通过验证的完成和 stamps 赢得信任。

**暂存并提交：**

```bash
dolt add .
dolt commit -m "Register rig: <your-handle>"
```

**查看差异确认你的变更：**

```bash
dolt diff HEAD~1
```

**预期输出：**

```
diff --dolt a/rigs b/rigs
+---+---------------+--------------------+-------------------+----------+--------+-------------+---------------------+---------------------+----------+------------+
|   | handle        | display_name       | dolthub_org       | hop_uri  | ...    | trust_level | registered_at       | last_seen           | rig_type | parent_rig |
+---+---------------+--------------------+-------------------+----------+--------+-------------+---------------------+---------------------+----------+------------+
| + | <your-handle> | <Your Name>        | <your-dolthub>    | NULL     | ...    | 0           | 2026-03-04 ...      | 2026-03-04 ...      | human    | NULL       |
+---+---------------+--------------------+-------------------+----------+--------+-------------+---------------------+---------------------+----------+------------+
```

`+` 前缀表示新增的行。

**验证查询：**

```bash
dolt sql -q "SELECT handle, display_name, trust_level, rig_type, registered_at FROM rigs WHERE handle = '<your-handle>';"
```

**预期输出：**

```
+---------------+--------------------+-------------+----------+---------------------+
| handle        | display_name       | trust_level | rig_type | registered_at       |
+---------------+--------------------+-------------+----------+---------------------+
| <your-handle> | <Your Display Name>| 0           | human    | 2026-03-04 12:00:00 |
+---------------+--------------------+-------------+----------+---------------------+
```

**推送到你的 Fork：**

```bash
dolt push origin main
```

### 步骤 6：浏览招聘板

在认领工作之前，先探索可用内容。

**所有开放项目：**

```bash
dolt sql -q "SELECT id, title, type, priority, effort_level, posted_by FROM wanted WHERE status = 'open' ORDER BY priority ASC, created_at ASC;"
```

**按项目过滤：**

```bash
dolt sql -q "SELECT id, title, priority, effort_level FROM wanted WHERE status = 'open' AND project = 'wl-commons' ORDER BY priority;"
```

**按工作量过滤（适合新手）：**

```bash
dolt sql -q "SELECT id, title, type, posted_by FROM wanted WHERE status = 'open' AND effort_level IN ('trivial', 'small') ORDER BY priority;"
```

**按标签过滤（需要 JSON 函数）：**

```bash
dolt sql -q "SELECT id, title, tags FROM wanted WHERE status = 'open' AND JSON_CONTAINS(tags, '\"beginner-friendly\"');"
```

**查看特定项目的完整详情：**

```bash
dolt sql -q "SELECT * FROM wanted WHERE id = '<wanted-id>';"
```

仔细阅读 `description`。它包含你在做工作和提交证据时需要的验收标准和上下文。

### 步骤 7：认领一个 Wanted 项目

当你找到一个想处理的项目时，通过设置 `claimed_by` 和更新状态来认领它。

```bash
dolt sql -q "
UPDATE wanted
SET claimed_by = '<your-handle>',
    status = 'claimed',
    updated_at = NOW()
WHERE id = '<wanted-id>'
  AND status = 'open';
"
```

`AND status = 'open'` 防护防止意外认领已被其他 Rig 领取的项目。如果受影响的行数为零，说明其他人已经先认领了——选择另一个项目。

**暂存、提交和审查：**

```bash
dolt add .
dolt commit -m "Claim wanted item <wanted-id>"
```

**查看差异：**

```bash
dolt diff HEAD~1
```

**预期输出：**

```
diff --dolt a/wanted b/wanted
+---+------------+-------+---------------+---------+---------------------+
|   | id         | ...   | claimed_by    | status  | updated_at          |
+---+------------+-------+---------------+---------+---------------------+
| < | <wanted-id>| ...   | NULL          | open    | 2026-03-01 ...      |
| > | <wanted-id>| ...   | <your-handle> | claimed | 2026-03-04 ...      |
+---+------------+-------+---------------+---------+---------------------+
```

`<` 行是旧值，`>` 是新值。

**验证查询：**

```bash
dolt sql -q "SELECT id, title, claimed_by, status, updated_at FROM wanted WHERE id = '<wanted-id>';"
```

**推送：**

```bash
dolt push origin main
```

### 步骤 8：做工作

这一步在 Dolt 之外进行。实际工作取决于 wanted 项目——可能是编写代码、创建文档、修复 bug、运行分析或构建工具。在你的项目仓库中工作（而非 Dolt 数据库目录）。

跟踪你需要的证据：

- **提交 SHA 或 PR URL** 如果工作涉及代码变更
- **截图或日志** 如果工作涉及运行某些东西
- **书面总结** 你做了什么以及如何满足 wanted 项目 `description` 中的验收标准

证据不需要存在于 Dolt 中——它只需要通过 URL 可达或在文本中描述，以便验证者可以确认你的工作。

### 步骤 9：提交完成证据

工作完成后，创建完成记录并更新 wanted 项目的状态。

**导航回你的公共数据库 clone：**

```bash
cd /path/to/wl-commons
```

**插入完成记录：**

```bash
dolt sql -q "
INSERT INTO completions (id, wanted_id, completed_by, evidence, completed_at)
VALUES (
  'c-$(openssl rand -hex 8)',
  '<wanted-id>',
  '<your-handle>',
  'PR merged: https://github.com/<org>/<repo>/pull/123 — implemented the feature as described, all tests passing.',
  NOW()
);
"
```

注意：你可以用任何方式生成 `id`。`c-` 前缀后跟随机十六进制是常见约定但非强制。

**将 wanted 项目更新为 in_review：**

```bash
dolt sql -q "
UPDATE wanted
SET status = 'in_review',
    evidence_url = 'https://github.com/<org>/<repo>/pull/123',
    updated_at = NOW()
WHERE id = '<wanted-id>';
"
```

**暂存、提交和审查：**

```bash
dolt add .
dolt commit -m "Submit completion for <wanted-id>"
```

**查看差异：**

```bash
dolt diff HEAD~1
```

你应该看到两个变更：`completions` 中的新行（标记为 `+`）和 `wanted` 中的更新行，显示状态从 `claimed` 变为 `in_review`。

**验证查询：**

```bash
dolt sql -q "SELECT id, wanted_id, completed_by, evidence, completed_at FROM completions WHERE wanted_id = '<wanted-id>';"
```

```bash
dolt sql -q "SELECT id, status, evidence_url, updated_at FROM wanted WHERE id = '<wanted-id>';"
```

**推送：**

```bash
dolt push origin main
```

### 步骤 10：创建 PR

你的变更存在于你的 Fork 上。要将它们合并到规范公共数据库，在 DoltHub 上创建 Pull Request。

**选项 A：DoltHub 网站**

1. 前往 `https://www.dolthub.com/repositories/<your-dolthub-user>/wl-commons`
2. 点击 **Pull Requests** > **New Pull Request**
3. 设置 base 仓库为 `steveyegge/wl-commons`，base 分支 `main`
4. 设置 compare 仓库为 `<your-dolthub-user>/wl-commons`，compare 分支 `main`
5. 添加标题（例如 `Register <your-handle> + complete <wanted-id>`）和描述
6. 提交 Pull Request

**选项 B：DoltHub API (curl)**

DoltHub API 需要单独的 API 令牌——你的 Dolt CLI 凭据（`dolt creds`）**不会**生效。在以下位置创建令牌：

> https://www.dolthub.com/settings/tokens

安全存储（例如在环境变量中）：

```bash
export DOLTHUB_API_TOKEN="dhat.v1.your-token-here"
```

然后创建 PR：

```bash
curl -X POST "https://www.dolthub.com/api/v1alpha1/steveyegge/wl-commons/pulls" \
  -H "Authorization: token ${DOLTHUB_API_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Register <your-handle> + complete <wanted-id>",
    "description": "Registers my rig and submits completion evidence for <wanted-id>.",
    "fromBranchOwnerName": "<your-dolthub-user>",
    "fromBranchRepoName": "wl-commons",
    "fromBranchName": "main",
    "toBranchOwnerName": "steveyegge",
    "toBranchRepoName": "wl-commons",
    "toBranchName": "main"
  }'
```

**通过 API 管理 PR：**

```bash
# 关闭一个 PR
curl -X PATCH "https://www.dolthub.com/api/v1alpha1/steveyegge/wl-commons/pulls/<pull_id>" \
  -H "Authorization: token ${DOLTHUB_API_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"state": "closed"}'
```

> **我们用惨痛教训总结的重要注意事项：**
>
> - **PR 在创建时会固定到当时的提交。** 如果你在创建 PR 后向 Fork 推送了更多提交，现有 PR 不会更新以包含它们。你必须关闭旧 PR 并创建新 PR。
> - **在线 Wasteland 招聘板在 PR 合并前不会更新。** 你的状态变更（`claimed`、`in_review`）只存在于你的 Fork 上，直到维护者将它们合并到规范 `steveyegge/wl-commons`。
> - **Dolt CLI 凭据不等于 API 令牌。** CLI 使用 JWK 凭据进行 push/pull。REST API 需要从 DoltHub 账户设置页面获取的单独令牌。
> - **始终使用 `https://www.dolthub.com`**（不是 `https://dolthub.com`）进行 API 请求。

维护者（trust_level 3）将审核你的 PR。他们会检查：

- 你的 Rig 注册看起来正确
- wanted 项目的状态转换有效
- 完成证据真实且满足验收标准
- 没有修改属于其他 Rig 的行

合并后，你的变更成为规范公共数据库的一部分。维护者还可能为你的完成发布 stamp，开始建立你的信誉。

### 步骤 11：与上游同步

其他参与者持续将变更合并到规范公共数据库中。通过从上游拉取保持最新。

```bash
dolt pull upstream main
```

**预期输出（有新变更时）：**

```
Updating abc1234..def5678
Fast-forward
```

**预期输出（已是最新时）：**

```
Already up to date.
```

如果有合并冲突（罕见——通常发生在两个 Rig 编辑了同一行），Dolt 会告诉你。用以下方式解决冲突：

```bash
dolt conflicts cat <table-name>
dolt conflicts resolve <table-name> --theirs   # 接受上游
# 或 --ours 保留你的版本
dolt add .
dolt commit -m "Resolve merge conflict in <table-name>"
```

拉取后，将合并状态推送到你的 Fork，以保持同步：

```bash
dolt push origin main
```

**验证 — 检查你是否看到最近的上游活动：**

```bash
dolt sql -q "SELECT handle, registered_at FROM rigs ORDER BY registered_at DESC LIMIT 5;"
```

养成在开始任何新工作前执行 `dolt pull upstream main` 的习惯。这避免冲突并确保你看到最新的 wanted 项目。

---

## 案例研究：Emanuel 的 Agent 飞轮

Jeffrey Emanuel（[github.com/Dicklesworthstone](https://github.com/Dicklesworthstone)）构建了 Agent 飞轮——一个用于大规模管理 AI 编码 Agent 舰队的生产级编排系统。飞轮协调在 tmux 面板中并行工作的 Claude Code (Opus)、Codex 和 Gemini Agent，由 NTM（命名 Tmux 管理器）管理。Emanuel 在 Twitter 上发布了他的规划工作流，描述了一种结构化的多阶段 AI 驱动开发方法。由此衍生了一个开源规划工具包，遵循 Emanuel 的规划流程，但将其指向已有代码库以在已有项目中规划新工作——Jeffrey Emanuel 的方式。该工具包的 `/shape` 技能驱动规划通过 6 个阶段：前置条件、起草、多模型批评（Claude 和 Codex 互相审查对方的工作）、设计集成、带依赖图的任务创建和 QA 传递。飞轮内置了广泛的质量门控，且容易为特定项目或代码库扩展。每个可交付物在完成前都通过质量门控——例如，在一个项目中，这些包括 TDD、单元测试、集成测试、E2E 测试（Playwright）、契约测试、SimpleCov 覆盖率、RuboCop lint、Brakeman 安全扫描和 UBS 静态分析。

该系统还包括 Agent Mail，一个基于 MCP 的消息系统，用于带文件保留防止冲突的 Agent 间协调；与 Beads（Steve Yegge 基于 Git 的任务追踪系统）的集成，用于依赖布线、优先级管理和基于图评分的分流（`bv --robot-triage`）；以及重生守护者——检测已完成 Agent 的守护进程，生成具有干净上下文的新 Agent，并让它们自主认领工作。Joris de Vreede（[github.com/Jorisdevreede](https://github.com/Jorisdevreede)）——一个产品人，而非软件工程师——使用此飞轮构建了一个 Rails 8 模块化单体应用，包含 18 个有界上下文引擎、475 个 Bead 关闭、3400+ RSpec 测试、完整 Playwright E2E 覆盖和 0 个 SonarQube 发现。飞轮将产品思维转化为生产质量的软件，无需操作者编写代码。

### 飞轮如何映射到 Wasteland 概念

| 飞轮概念 | Wasteland 对应 | 说明 |
|------------------|----------------------|------|
| 飞轮编排器 + 人类操作者 | **Rig** | `rigs` 表中的单个 Rig 身份 |
| Beads 集成（Yegge 的任务追踪） | **Wanted 招聘板** | 两者都追踪工作项，两者都使用 Dolt 进行持久化 |
| 质量门控（按项目可扩展） | **验证者 / stamp 维度** | 可以在 stamps 中引用的质量证据 |
| Agent 舰队（Claude Code + Codex 工作者） | **Polecats**（并行工作者） | 多个 Agent 并发执行工作项 |

### MVGT 流程：2026 年 3 月 4 日

2026 年 3 月 4 日，飞轮在未安装 Gas Town 的情况下完成了完整的 MVGT 流程。整个过程——从安装 Dolt 到 PR 创建——大约花了 45 分钟。

在安装 Dolt v1.83.1 并以 `jorisdevreede` 身份与 DoltHub 认证后，飞轮 Fork 了 `steveyegge/wl-commons`，将 Fork clone 到 `/tmp/wl-commons-test`，并注册了一个 Rig：

```sql
INSERT INTO rigs (handle, display_name, dolthub_org, trust_level, rig_type, registered_at, last_seen)
VALUES (
  'jorisdevreede',
  'Joris - Flywheel Rig',
  'jorisdevreede',
  0,
  'human',
  NOW(),
  NOW()
);
```

飞轮随后浏览招聘板寻找可用工作：

```sql
SELECT id, title, project, status, posted_by
FROM wanted
WHERE status = 'open'
ORDER BY project, id;
```

这返回了 6 个项目中的 30 个项目。飞轮认领了 `w-com-004`：

```sql
UPDATE wanted
SET status = 'claimed',
    claimed_by = 'jorisdevreede',
    updated_at = NOW()
WHERE id = 'w-com-004';
```

变更被提交、推送到 Fork，并在 DoltHub 上创建了 Pull Request：
[https://www.dolthub.com/repositories/steveyegge/wl-commons/pulls/1](https://www.dolthub.com/repositories/steveyegge/wl-commons/pulls/1)

### 遇到的粗糙边缘

流程并不完全顺畅。以下是遇到的问题，记录在此以便他人避免：

1. **无头服务器上 `dolt login` 的重试循环。** 该命令尝试打开浏览器进行 OAuth。在无浏览器的无头服务器上，它会进入重试循环。必须手动复制 URL 并在其他地方的浏览器中打开。
2. **创建了多个凭据。** 每次 `dolt login` 调用都会创建新的 JWK 密钥，即使已有凭据存在。这可能导致对哪个凭据是活跃的困惑。需要 `dolt creds ls` 和 `dolt creds use <id>` 来整理。
3. **`gt wl join` Fork API 错误 (HTTP 400)。** Gas Town 的 join 命令（自动化 Fork）返回 HTTP 400 错误。必须在 DoltHub 网站上手动创建 Fork。
4. **Fork 存在前推送被拒绝。** 在 DoltHub 上创建 Fork 之前尝试推送会导致权限被拒绝错误。Fork 必须在 DoltHub 上存在后才能尝试任何推送。

---

## 自动化模式

### CI/CD 的 Shell 脚本片段

**一次性设置：Fork、Clone 和注册**

```bash
#!/usr/bin/env bash
set -euo pipefail

UPSTREAM="steveyegge/wl-commons"
FORK_OWNER="myhandle"
CLONE_DIR="/opt/wasteland/wl-commons"

# 通过 DoltHub 网站 Fork（Fork 的 API 尚不稳定）
# 然后 clone 你的 Fork：
dolt clone "${FORK_OWNER}/wl-commons" "$CLONE_DIR"
cd "$CLONE_DIR"

# 添加上游远程
dolt remote add upstream "https://doltremoteapi.dolthub.com/${UPSTREAM}"

# 注册你的 Rig
dolt sql -q "
  INSERT INTO rigs (handle, display_name, dolthub_org, trust_level, rig_type, registered_at, last_seen)
  VALUES ('${FORK_OWNER}', 'My Rig', '${FORK_OWNER}', 0, 'human', NOW(), NOW());
"

dolt add .
dolt commit -m "Register rig: ${FORK_OWNER}"
dolt push origin main
```

**定期与上游同步**

```bash
#!/usr/bin/env bash
set -euo pipefail

cd /opt/wasteland/wl-commons
dolt pull upstream main

# 检查 Schema 变更
SCHEMA_VERSION=$(dolt sql -q "SELECT value FROM _meta WHERE \`key\` = 'schema_version'" -r csv | tail -1)
echo "Current schema version: ${SCHEMA_VERSION}"
```

**认领、工作和提交（按项目脚本）**

```bash
#!/usr/bin/env bash
set -euo pipefail

ITEM_ID="$1"
HANDLE="myhandle"
CLONE_DIR="/opt/wasteland/wl-commons"

cd "$CLONE_DIR"

# 认领前同步
dolt pull upstream main

# 验证项目仍开放
STATUS=$(dolt sql -q "SELECT status FROM wanted WHERE id = '${ITEM_ID}'" -r csv | tail -1)
if [ "$STATUS" != "open" ]; then
  echo "Item ${ITEM_ID} is no longer open (status: ${STATUS})"
  exit 1
fi

# 认领项目
dolt sql -q "
  UPDATE wanted
  SET status = 'claimed',
      claimed_by = '${HANDLE}',
      updated_at = NOW()
  WHERE id = '${ITEM_ID}';
"

dolt add .
dolt commit -m "Claim ${ITEM_ID}"
dolt push origin main

# --- 在这里做工作 ---

# 标记为 in_review
dolt sql -q "
  UPDATE wanted
  SET status = 'in_review',
      updated_at = NOW()
  WHERE id = '${ITEM_ID}';
"

dolt add .
dolt commit -m "Submit ${ITEM_ID}"
dolt push origin main

# 通过 DoltHub API 创建 PR
curl -s -X POST \
  "https://www.dolthub.com/api/v1alpha1/steveyegge/wl-commons/pulls" \
  -H "authorization: token ${DOLTHUB_TOKEN}" \
  -H "content-type: application/json" \
  -d "{
    \"title\": \"${ITEM_ID}: completion submission\",
    \"fromBranchOwnerName\": \"${HANDLE}\",
    \"fromBranchRepoName\": \"wl-commons\",
    \"fromBranchName\": \"main\",
    \"toBranchName\": \"main\"
  }"
```

### DoltHub API 示例

**创建 Pull Request：**

```bash
curl -s -X POST \
  "https://www.dolthub.com/api/v1alpha1/<owner>/<repo>/pulls" \
  -H "authorization: token $DOLTHUB_TOKEN" \
  -H "content-type: application/json" \
  -d '{
    "title": "Submit w-com-004: MVGT integration guide",
    "fromBranchOwnerName": "myhandle",
    "fromBranchRepoName": "wl-commons",
    "fromBranchName": "main",
    "toBranchName": "main"
  }'
```

注意：通过 DoltHub 网站创建 Fork。Fork 的 API 尚不稳定。

### 环境变量模式

| 变量 | 用途 | 上下文 |
|----------|-----|---------|
| `DOLTHUB_TOKEN` | CI 流水线的 API 认证 | 在 CI 密钥中设置（GitHub Actions 等） |
| `DOLT_CREDS_FILE` | 凭据文件路径，用于密钥管理的环境 | 当凭据文件被挂载或注入时使用 |

---

## 故障排除

### 1. `dolt login` 重试循环

**症状：** `dolt login` 打印 URL 并在循环中重试，永不完成。终端显示重复的"Waiting for login..."消息。

**原因：** 该命令尝试打开浏览器进行 OAuth 认证。在无头服务器（CI runner、远程 VM）上，没有可用浏览器。

**解决方案：** 复制 `dolt login` 打印的 URL，在另一台机器的浏览器中手动打开。在那里完成 OAuth 流程。然后使用 `dolt creds use <id>` 选择已授权的密钥。用 `dolt creds check` 验证。

### 2. 创建了多个凭据

**症状：** `dolt creds ls` 显示多个 JWK 密钥。即使完成了登录，推送仍因认证错误失败。

**原因：** 每次 `dolt login` 调用都会创建新的 JWK 密钥对，无论是否已有凭据存在。只有你在浏览器中实际授权的那个才能工作。

**解决方案：** 运行 `dolt creds ls` 查看所有凭据。识别你授权的那个（匹配浏览器 OAuth 流程中显示的公钥）。运行 `dolt creds use <id>` 选择它。运行 `dolt creds check` 验证其有效。

### 3. `gt wl join` Fork API 错误 (HTTP 400)

**症状：** 运行 `gt wl join` 加入 Wasteland 时，在尝试 Fork 公共仓库时返回 HTTP 400 错误。

**原因：** Gas Town 的 join 命令使用 Fork 的 API 端点，可能无法处理所有边缘情况。Fork API 尚不稳定。

**解决方案：** 在 DoltHub 网站上手动 Fork。导航到 `steveyegge/wl-commons`，点击 Fork，然后在本地 clone 你的 Fork 并按本指南所示手动注册你的 Rig。

### 4. 推送时权限被拒绝

**症状：** `dolt push origin main` 因权限被拒绝错误失败。

**原因：** Fork 在 DoltHub 上不存在，或远程 URL 指向上游仓库而非你的 Fork。

**解决方案：** 首先在 DoltHub 网站上创建 Fork。用 `dolt remote -v` 验证你的远程 URL——`origin` 远程应指向你的 Fork（`<your-handle>/wl-commons`），而非上游（`steveyegge/wl-commons`）。

### 5. 同步时的合并冲突

**症状：** `dolt pull upstream main` 报告合并冲突。

**原因：** 上游已分歧——另一个 Rig 修改了你修改的相同行。

**解决方案：** 运行 `dolt pull upstream main`。如果报告冲突，用 `dolt conflicts resolve` 在单元格级别解决。Dolt 在行和单元格级别追踪冲突，所以你可以准确看到哪些值冲突。解决后，`dolt add .` 和 `dolt commit`。

### 6. 过期的招聘板

**症状：** 你认领了一个项目，推送到你的 Fork，创建了 PR，维护者拒绝了它，因为另一个 Rig 已经认领了该项目。

**原因：** 你的本地招聘板副本已过期。另一个 Rig 在你上次拉取和认领之间认领了该项目。

**解决方案：** 在认领之前始终运行 `dolt pull upstream main`。拉取后，验证项目仍开放：`SELECT status FROM wanted WHERE id = '<id>'`。然后再继续 UPDATE。

### 7. 提交到了错误的分支

**症状：** 你的变更提交到了 `main`，而你本意是在 feature 分支上工作，或反之。

**原因：** Dolt 默认为 `main`。与 Git 不同，Dolt 分支在公共数据库工作流中较少使用，但如果你使用它们，必须明确。

**解决方案：** 提交前用 `dolt branch` 验证当前分支。如果你提交到了错误分支，检出正确分支并 cherry-pick 或重新应用你的变更。

### 8. Schema 版本不匹配

**症状：** SQL 查询因列未找到错误失败，或 INSERT 语句在拉取后被拒绝。

**原因：** 上游可能已迁移 Schema（添加了列、更改了类型、添加了表）。你的本地代码或脚本引用了旧 Schema。

**解决方案：** 每次执行 `dolt pull upstream main` 后，检查 Schema 版本：``SELECT value FROM _meta WHERE `key` = 'schema_version'``。与你脚本期望的版本比较。如果 Schema 已变更，更新你的脚本以匹配。用 `dolt diff` 查看确切变更了什么。

---

## 常见问题

**我需要 Gas Town 吗？**

不需要。Gas Town 是一个带有自己 CLI (`gt`) 的完整编排器，但参与 Wasteland 只需要 Dolt 和标准 shell 工具。本指南端到端覆盖了非 Gas Town 路径；参见[简介](#introduction)。

**如何获得 stamps？**

提交你的完成证据（产物、测试结果、PR 链接），更新 wanted 项目的状态为 `in_review`，推送到你的 Fork，并在上游公共数据库上创建 PR。验证者——另一个具有足够信任级别的 Rig——审核你的提交并通过插入具有适当维度和效价的 `stamps` 表来发布 stamps。参见[步骤 9：提交完成证据](#step-9-submit-completion-evidence)和 [Stamp 效价](#stamp-valence)了解详情。

**如果 Schema 变了怎么办？**

每次执行 `dolt pull upstream main` 后检查 `_meta.schema_version`。Schema 变更通过 Dolt 的分支/合并工作流管理。如果版本已递增，查看差异（`dolt diff`）了解变更了什么并相应更新你的脚本。参见[故障排除：Schema 版本不匹配](#8-schema-version-mismatch)了解诊断步骤。

**多个 Rig 可以共享一个 Fork 吗？**

可以，但每个 Rig 应该在 `rigs` 表中有自己的 handle。协调推送以避免冲突——如果两个 Rig 同时推送到同一个 Fork，一个将被拒绝。考虑在 Fork 内使用分支，或如果协调开销太高，为每个 Rig 使用单独的 Fork。参见[表: `rigs`](#table-rigs)了解身份 Schema。

**trust_level 如何增加？**

`trust_level` 通过验证的完成和来自其他 Rig 的 stamps 增加。维护者（`trust_level` 3）随时间评估你已盖章工作的质量和一致性。正面、有充分支持的 stamps 改善信誉，而薄弱或有争议的证据减缓进展。参见[跨领域模式](#cross-cutting-patterns)。

**如果我不同意一个 stamp 怎么办？**

Stamps 在 Dolt 历史中是永久的——它们在提交后不能删除或修改。你可以创建一个具有不同效价的反 stamp 来表达你的异议。Stamp 链是完整记录，社区评估一份工作的完整 stamps 历史，而非仅仅第一个。参见[表: `stamps`](#table-stamps)了解 Schema，[Stamp 效价](#stamp-valence)了解维度示例。

**我可以在招聘板上发布项目吗？**

可以。以你的 handle 作为 `posted_by` 插入 `wanted` 表，使用类似 `w-<project>-<number>` 的 ID。推送到你的 Fork 并创建 PR，以便项目可以合并到上游招聘板。参见[表: `wanted`](#table-wanted)和[状态生命周期](#status-lifecycle)。

**什么是年鉴规则？**

你不能为自己的工作盖章：`stamps.author` 必须与 `stamps.subject` 不同。这防止自我交易并保持验证基于同行。如果你提交工作，另一个 Rig 必须验证它；如果你验证工作，它不能是你自己的。参见[关键关系](#key-relationships)。