# 联邦架构

> **状态：部分实现** — 基础设施（Dolt 远程）已存在。核心联邦功能（URI 方案、跨工作空间查询、委托）尚未实现。

Gas Town 和 Beads 的多工作空间协调。

## 概述

联邦使多个 Gas Town 实例能够引用彼此的工作、跨组织协调，
并追踪分布式项目。

## 实体模型

### 三个层级

```
层级 1: 实体    - 人或组织（扁平命名空间）
层级 2: 链      - 每个实体的工作空间/town
层级 3: 工作单元 - 链上的 issue、task、molecule
```

### URI 方案

完整工作单元引用（HOP 协议）：

```
hop://entity/chain/rig/issue-id
hop://steve@example.com/main-town/greenplace/gp-xyz
```

跨仓库引用（同平台）：

```
beads://platform/org/repo/issue-id
beads://github/acme/backend/ac-123
```

工作空间内，优先使用短格式：

```
gp-xyz             # 本地（通过 routes.jsonl 前缀路由）
greenplace/gp-xyz  # 不同 rig，同链
./gp-xyz           # 显式当前 rig 引用
```

完整 URI 规范参见 `~/gt/docs/hop/GRAPH-ARCHITECTURE.md`。

## 关系类型（尚未实现）

计划的关系原语：**雇佣**（实体到组织的成员关系）、
**交叉引用**（跨工作空间的 `depends_on` 链接）和**委托**
（跨工作空间的工作分发，带有条款和截止日期）。

## Agent 来源

每个 agent 操作都有归属。完整 BD_ACTOR 格式约定参见
[identity.md](../concepts/identity.md)。

### Git 提交

```bash
# 按 agent 会话设置
GIT_AUTHOR_NAME="greenplace/crew/joe"
GIT_AUTHOR_EMAIL="steve@example.com"  # 工作空间所有者
```

结果：`abc123 Fix bug (greenplace/crew/joe <steve@example.com>)`

### Beads 操作

```bash
BD_ACTOR="greenplace/crew/joe"  # 在 agent 环境中设置
bd create --title="Task"        # Actor 自动填充
```

### 事件日志

所有事件包含 actor：

```json
{
  "ts": "2025-01-15T10:30:00Z",
  "type": "sling",
  "actor": "greenplace/crew/joe",
  "payload": { "bead": "gp-xyz", "target": "greenplace/polecats/Toast" }
}
```

## 发现（尚未实现）

工作空间元数据存储在 `~/gt/.town.json`（owner、name、public_name）。
计划中的命令：`gt remote add/list` 用于远程注册、
`bd show hop://...` 和 `bd list --remote=...` 用于跨工作空间查询。

## 实现状态

- [x] Git 提交中的 Agent 身份
- [x] Beads 创建中的 BD_ACTOR 默认值
- [x] 工作空间元数据文件（.town.json）
- [x] 跨工作空间 URI 方案（hop://、beads://、本地形式）
- [x] Dolt 远程已配置（DoltHub 端点）
- [x] 本地 remotesapi 已启用（端口 8000）
- [ ] DoltHub 认证（`dolt login`）
- [ ] 远程注册（gt remote add）
- [ ] 跨工作空间查询
- [ ] 委托原语

## Dolt 联邦配置

### 当前设置

Town 级 Dolt 数据库已配置指向 DoltHub 的远程：

```bash
# 检查 town 数据库的已配置远程
cd ~/gt/.dolt-data/town && dolt remote -v
# origin https://doltremoteapi.dolthub.com/steveyegge/gastown-town {}
# local  http://localhost:8000/town {}
```

### 已配置远程

| 数据库 | 远程名 | URL | 用途 |
|--------|--------|-----|------|
| town | origin | `steveyegge/gastown-town` | DoltHub 公共联邦 |
| town | local | `http://localhost:8000/town` | 本地开发/测试 |
| gastown | origin | `steveyegge/gastown-rig` | DoltHub 公共联邦 |
| beads | origin | `steveyegge/gastown-beads` | DoltHub 公共联邦 |

### 联邦端点选项

**1. DoltHub（推荐用于公共联邦）**

类似 GitHub 的 Dolt 版本 — 公共、托管、零基础设施：

```bash
# 登录 DoltHub（一次性设置）
dolt login

# 推送到远程
cd ~/gt/.dolt-data/town
dolt push origin main
```

**2. 本地 Remotesapi（开发/测试）**

已在 `~/gt/.dolt-data/config.yaml` 中启用：
- 端口：8000
- 模式：只读（设置 `read_only: false` 以启用完整联邦）

```bash
# 测试本地远程
dolt push local main
```

**3. 自托管 DoltLab（企业）**

用于组织内部的私有联邦：
- 部署 DoltLab 实例
- 配置远程：`dolt remote add corp https://doltlab.corp.example.com/org/repo`

**4. 直接 Town 对 Town（高级）**

两个 Gas Town 实例直接联邦：
- Town A 在可访问的端点上运行 remotesapi
- Town B 将 Town A 添加为远程：`dolt remote add town-a http://town-a.example.com:8000/town`

### 启用完整联邦

向/从已配置的远程推送/拉取：

1. **DoltHub 认证：**
   ```bash
   dolt login
   # 打开浏览器进行 OAuth
   # 在 ~/.dolt/creds/ 中创建凭证
   ```

2. **创建 DoltHub 仓库：**
   - 访问 https://www.dolthub.com
   - 创建与远程名匹配的仓库（如 `steveyegge/gastown-town`）

3. **初始推送：**
   ```bash
   cd ~/gt/.dolt-data/town
   dolt push -u origin main
   ```

4. **为本地 Remotesapi 启用写入：**
   编辑 `~/gt/.dolt-data/config.yaml`：
   ```yaml
   remotesapi:
     port: 8000
     read_only: false  # 启用写入
   ```
   重启 daemon：`gt down && gt up`

### 安全考量

- **DoltHub**：默认公开；敏感数据使用私有仓库
- **本地 remotesapi**：仅绑定到 localhost；网络访问时使用 TLS
- **认证**：DoltHub 使用 OAuth；自托管可使用 TLS 客户端证书

## 未来用例

- **多仓库项目**：通过跨工作空间引用追踪跨多个仓库的工作
- **分布式团队**：不同工作空间中的团队成员贡献同一项目，各自拥有独立审计跟踪
- **承包商协调**：跨组织的委托链，带有级联完成和保留的归属
- **跨工作空间查询**：跨组织的工作聚合视图（`bd list --org=...`）