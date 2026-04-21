# Mol Mall 设计

> **状态：愿景文档** — 阶段 1（本地 formula）已存在。阶段 2-5（注册表、发布、联邦）尚未实现。

> Gas Town formula 的市场

## 愿景

**Mol Mall** 是一个跨 Gas Town 安装共享 formula 的注册表。可以将其理解为 molecule 的 npm，或工作流的 Terraform Registry。

```
"烹饪一个 formula，将其 sling 到 polecat，witness 观察，refinery 合并。"

如果你能浏览一个 formula 商城，安装一个，然后立即
让你的 polecats 执行世界级的工作流呢？
```

### 网络效应

一个设计良好的"代码审查"或"安全审计"或"部署到 K8s" formula 可以
在数千个 Gas Town 安装中传播。每次采用意味着：
- 更多 agent 执行经过验证的工作流
- 更多结构化、可追踪的工作输出
- 更好的能力路由（在某个 formula 上有业绩记录的 agent 获得类似工作）

## 架构

### 注册表类型

```
┌─────────────────────────────────────────────────────────────────┐
│                      MOL MALL 注册表                             │
└─────────────────────────────────────────────────────────────────┘

公共注册表 (molmall.gastown.io)
├── 社区 formula（MIT 许可）
├── 官方 Gas Town formula（受信）
├── 已验证发布者 formula
└── 开放贡献模式

私有注册表（自托管）
├── 组织特定的 formula
├── 专有工作流
├── 内部部署模式
└── 企业合规 formula

联邦注册表（HOP 未来）
├── 跨组织发现
├── 基于技能的搜索
└── 归属链追踪
└── hop:// URI 解析
```

### URI 方案

```
hop://molmall.gastown.io/formulas/mol-polecat-work@4.0.0
       └──────────────────┘         └──────────────┘ └───┘
           注册表主机               formula 名称    版本

# 短格式
mol-polecat-work                    # 默认注册表，最新版本
mol-polecat-work@4                  # 主版本
mol-polecat-work@4.0.0              # 精确版本
@acme/mol-deploy                    # 按发布者限定
hop://acme.corp/formulas/mol-deploy # 完整 HOP URI
```

### 注册表 API

```yaml
# OpenAPI 风格规范

GET /formulas
  # 列出所有 formula
  查询参数:
    - q: string          # 搜索查询
    - capabilities: string[]   # 按能力标签过滤
    - author: string     # 按作者过滤
    - limit: int
    - offset: int
  响应:
    formulas:
      - name: mol-polecat-work
        version: 4.0.0
        description: "完整的 polecat 工作生命周期..."
        author: steve@gastown.io
        downloads: 12543
        capabilities: [go, testing, code-review]

GET /formulas/{name}
  # 获取 formula 元数据
  响应:
    name: mol-polecat-work
    versions: [4.0.0, 3.2.1, 3.2.0, ...]
    latest: 4.0.0
    author: steve@gastown.io
    repository: https://github.com/steveyegge/gastown
    license: MIT
    capabilities:
      primary: [go, testing]
      secondary: [git, code-review]
    stats:
      downloads: 12543
      stars: 234
      used_by: 89  # 使用此 formula 的 town

GET /formulas/{name}/{version}
  # 获取特定版本
  响应:
    name: mol-polecat-work
    version: 4.0.0
    checksum: sha256:abc123...
    signature: <可选 PGP 签名>
    content: <base64 或指向 .formula.toml 的 URL>
    changelog: "添加了自清理模型..."
    published_at: 2026-01-10T00:00:00Z

POST /formulas
  # 发布 formula（需认证）
  请求体:
    name: mol-my-workflow
    version: 1.0.0
    content: <formula TOML>
    changelog: "初始版本"
  认证: Bearer token（关联到 HOP 身份）

GET /formulas/{name}/{version}/download
  # 下载 formula 内容
  响应: 原始 .formula.toml 内容
```

## Formula 包格式

### 简单情况：单文件

大多数 formula 是单个 `.formula.toml` 文件：

```bash
gt formula install mol-polecat-code-review
# 下载 mol-polecat-code-review.formula.toml 到 ~/gt/.beads/formulas/
```

### 复杂情况：Formula Bundle

某些 formula 需要辅助文件（脚本、模板、配置）：

```
mol-deploy-k8s.formula.bundle/
├── formula.toml              # 主 formula
├── templates/
│   ├── deployment.yaml.tmpl
│   └── service.yaml.tmpl
├── scripts/
│   └── healthcheck.sh
└── README.md
```

Bundle 格式：
```bash
# Bundle 是 tarball
mol-deploy-k8s-1.0.0.bundle.tar.gz
```

安装：
```bash
gt formula install mol-deploy-k8s
# 解压到 ~/gt/.beads/formulas/mol-deploy-k8s/
# formula.toml 位于 mol-deploy-k8s/formula.toml
```

## 安装流程

### 基本安装

```bash
$ gt formula install mol-polecat-code-review

解析中 mol-polecat-code-review...
  注册表: molmall.gastown.io
  版本:   1.2.0（最新）
  作者:    steve@gastown.io
  技能:    code-review, security

下载中... ████████████████████ 100%
校验和中... ✓

安装到: ~/gt/.beads/formulas/mol-polecat-code-review.formula.toml
```

### 版本锁定

```bash
$ gt formula install mol-polecat-work@4.0.0

安装中 mol-polecat-work@4.0.0（锁定）...
✓ 已安装

$ gt formula list --installed
  mol-polecat-work           4.0.0   [pinned]
  mol-polecat-code-review    1.2.0   [latest]
```

### 升级流程

```bash
$ gt formula upgrade mol-polecat-code-review

检查更新...
  当前: 1.2.0
  最新:  1.3.0

1.3.0 变更日志:
  - 添加了安全聚焦选项
  - 改进了测试覆盖步骤

升级? [y/N] y

下载中... ✓
已安装: mol-polecat-code-review@1.3.0
```

### 锁定文件

```json
// ~/gt/.beads/formulas/.lock.json
{
  "version": 1,
  "formulas": {
    "mol-polecat-work": {
      "version": "4.0.0",
      "pinned": true,
      "checksum": "sha256:abc123...",
      "installed_at": "2026-01-10T00:00:00Z",
      "source": "hop://molmall.gastown.io/formulas/mol-polecat-work@4.0.0"
    },
    "mol-polecat-code-review": {
      "version": "1.3.0",
      "pinned": false,
      "checksum": "sha256:def456...",
      "installed_at": "2026-01-10T12:00:00Z",
      "source": "hop://molmall.gastown.io/formulas/mol-polecat-code-review@1.3.0"
    }
  }
}
```

## 发布流程

### 首次设置

```bash
$ gt formula publish --init

设置 Mol Mall 发布...

1. 在 https://molmall.gastown.io/signup 创建账户
2. 在 https://molmall.gastown.io/settings/tokens 生成 API token
3. 运行: gt formula login

$ gt formula login
Token: ********
已登录为: steve@gastown.io
```

### 发布

```bash
$ gt formula publish mol-polecat-work

发布中 mol-polecat-work...

预飞行检查:
  ✓ formula.toml 有效
  ✓ 版本 4.0.0 尚未发布
  ✓ 必需字段存在（name、version、description）
  ✓ 技能已声明

发布到 molmall.gastown.io? [y/N] y

上传中... ✓
已发布: hop://molmall.gastown.io/formulas/mol-polecat-work@4.0.0

查看: https://molmall.gastown.io/formulas/mol-polecat-work
```

### 验证级别

```
┌─────────────────────────────────────────────────────────────────┐
│                    FORMULA 信任级别                               │
└─────────────────────────────────────────────────────────────────┘

未验证（默认）
  任何人都可以发布
  仅基本验证
  显示 ⚠️ 警告

已验证发布者
  发布者身份已确认
  显示 ✓ 勾选标记
  搜索排名更高

官方
  由 Gas Town 团队维护
  显示 🏛️ 徽章
  包含在嵌入默认值中

已审计
  安全审查已完成
  显示 🔒 徽章
  企业注册表必需
```

## 能力标签

### Formula 能力声明

```toml
[formula.capabilities]
# 此 formula 运用哪些能力？用于 agent 路由。
primary = ["go", "testing", "code-review"]
secondary = ["git", "ci-cd"]

# 能力权重（可选，用于精细路由）
[formula.capabilities.weights]
go = 0.3           # 30% 的 formula 工作是 Go
testing = 0.4      # 40% 是测试
code-review = 0.3  # 30% 是代码审查
```

### 基于能力的搜索

```bash
$ gt formula search --capabilities="security,go"

匹配能力的 formula: security, go

  mol-security-audit           v2.1.0   ⭐ 4.8   📥 8,234
    能力: security, go, code-review
    "全面安全审查工作流"

  mol-dependency-scan          v1.0.0   ⭐ 4.2   📥 3,102
    能力: security, go, supply-chain
    "扫描 Go 依赖的漏洞"
```

### Agent 问责

当 polecat 完成一个 formula 时，执行被追踪：

```
Polecat: beads/amber
Formula: mol-polecat-code-review@1.3.0
完成于: 2026-01-10T15:30:00Z
运用的能力:
  - code-review（主要）
  - security（次要）
  - go（次要）
```

此执行记录支持：
1. **路由** — 有成功业绩记录的 agent 获得类似工作
2. **调试** — 追踪哪个 agent 做了什么、什么时候
3. **质量指标** — 按 agent 和 formula 追踪成功率

## 私有注册表

### 企业部署

```yaml
# ~/.gtconfig.yaml
registries:
  - name: acme
    url: https://molmall.acme.corp
    auth: token
    priority: 1  # 优先检查

  - name: public
    url: https://molmall.gastown.io
    auth: none
    priority: 2  # 回退
```

### 自托管注册表

```bash
# Docker 部署
docker run -d \
  -p 8080:8080 \
  -v /data/formulas:/formulas \
  -e AUTH_PROVIDER=oidc \
  gastown/molmall-registry:latest

# 配置
MOLMALL_STORAGE=s3://bucket/formulas
MOLMALL_AUTH=oidc
MOLMALL_OIDC_ISSUER=https://auth.acme.corp
```

## 联邦

联邦通过 Highway Operations Protocol（HOP）实现跨组织的 formula 共享。

### 跨注册表发现

```bash
$ gt formula search "deploy kubernetes" --federated

搜索联邦注册表...

  molmall.gastown.io:
    mol-deploy-k8s           v3.0.0   🏛️ 官方

  molmall.acme.corp:
    @acme/mol-deploy-k8s     v2.1.0   ✓ 已验证

  molmall.bigco.io:
    @bigco/k8s-workflow      v1.0.0   ⚠️ 未验证
```

### HOP URI 解析

`hop://` URI 方案提供跨注册表的实体引用：

```bash
# 完整 HOP URI
gt formula install hop://molmall.acme.corp/formulas/@acme/mol-deploy@2.1.0

# 通过 HOP 解析（Highway Operations Protocol）
1. 解析 hop:// URI
2. 解析注册表端点（DNS/HOP 发现）
3. 认证（如果需要）
4. 下载 formula
5. 验证校验和/签名
6. 安装到 town 级
```

## 实现阶段

### 阶段 1：本地命令（当前）

参见 [Formula 解析](formula-resolution.md) 了解已实现的三层解析系统。

### 阶段 2：手动共享

- Formula 导出/导入
- `gt formula export mol-polecat-work > mol-polecat-work.formula.toml`
- `gt formula import < mol-polecat-work.formula.toml`
- 锁定文件格式

### 阶段 3：公共注册表

- molmall.gastown.io 上线
- 从注册表 `gt formula install`
- `gt formula publish` 流程
- 基本搜索和浏览

### 阶段 4：企业功能

- 私有注册表支持
- 认证集成
- 验证级别
- 审计日志

### 阶段 5：联邦（HOP）

- Schema 中的能力标签
- 联邦协议（Highway Operations Protocol）
- 跨注册表搜索
- Agent 执行追踪用于问责

## 相关文档

- [Formula 解析](formula-resolution.md) - 本地解析顺序
- [Molecules](../concepts/molecules.md) - Formula 生命周期（cook、pour、squash）