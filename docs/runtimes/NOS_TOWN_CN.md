# NOS Town 运行时集成

本文档描述如何将 Gas Town 的核心编排与 NOS Town Groq 原生运行时结合使用。

## 概述

NOS Town 通过 Groq 托管的开源模型支持、多模型路由、共识委员会和通过 Historian 的机构记忆扩展 Gas Town。两个系统共享相同的核心概念（Hook、Bead、Convoy、Mayor/Witness/Deacon 角色），但在运行时和模型选择上有所分歧。

## 架构

```
Gas Town 核心（本仓库）       NOS Town 运行时 (kab0rn/nostown)
│                              │
├── Hook 生命周期              ├── Groq API 客户端
├── Beads 集成                 ├── 多模型路由表
├── Convoy 管理                ├── Council 编排
├── Mayor/Witness/Deacon       ├── Historian (Batch job)
├── Refinery 合并队列          ├── Safeguard 集成
└── gt CLI                     └── nos CLI（包装 gt + Groq）
```

## Fork 策略

**NOS Town 不是 Gas Town 的 git fork。** 而是：

1. **kab0rn/gastown** 通过常规 fork/sync 工作流追踪 `gastownhall/gastown` 上游
2. **kab0rn/nostown** 将 Gas Town 核心作为依赖导入（Go modules 或 submodule）
3. NOS 特定的逻辑（Groq 运行时、路由、council）仅存在于 `kab0rn/nostown`

### 为什么采用这种方式？

- **保留上游演进**：Steve Yegge 持续在 Gas Town 上迭代。真正的 fork 会产生分歧。
- **干净分离**：Gas Town 核心不需要 Groq 依赖；NOS 不重复编排逻辑。
- **方便贡献上游**：对 Hook、Convoy 或角色的改进可以 PR 回 Gas Town，不带 Groq 特定的包袱。

## 配置

将 NOS Town 与此 Gas Town fork 一起使用：

### 1. 安装前置条件

```bash
# Gas Town 依赖（与标准安装相同）
go install github.com/kab0rn/gastown/cmd/gt@latest
go install github.com/steveyegge/beads/cmd/bd@latest

# NOS Town CLI
go install github.com/kab0rn/nostown/cmd/nos@latest

# 设置 Groq API 密钥
export GROQ_API_KEY="your-api-key"
```

### 2. 初始化工作区

```bash
# 使用 nos CLI（内部包装 gt）
nos install ~/nos --git
cd ~/nos

# 或使用 gt 并手动配置 Groq 运行时
gt install ~/nos --git
cd ~/nos
gt config set runtime.provider groq
gt config set runtime.base_url https://api.groq.com/openai/v1
```

### 3. 配置每个 Rig 的运行时

编辑 `<rig>/settings/config.json`：

```json
{
  "runtime": {
    "provider": "groq",
    "base_url": "https://api.groq.com/openai/v1",
    "api_key_env": "GROQ_API_KEY"
  },
  "routing": {
    "mayor":    { "default": "llama-3.3-70b-versatile" },
    "crew":     { "default": "llama-3.3-70b-versatile" },
    "polecat":  { "default": "llama-3.1-8b-instant", "boosted": "llama-3.3-70b-versatile" },
    "witness":  { "default": "llama-3.3-70b-versatile", "council": ["llama-3.3-70b-versatile", "openai/gpt-oss-120b"] },
    "refinery": { "default": "llama-3.3-70b-versatile", "fast_path": "llama-3.1-8b-instant" },
    "deacon":   { "default": "llama-3.1-8b-instant" },
    "dogs":     { "default": "llama-3.1-8b-instant" }
  }
}
```

## 工作流

### 使用 nos CLI

`nos` CLI 包装了所有 `gt` 命令并添加了 Groq 特定的扩展：

```bash
# 与 gt 相同
nos rig add myproject https://github.com/you/repo.git
nos crew add yourname --rig myproject
nos mayor attach

# NOS 特定：路由配置
nos config route show
nos config route set polecat.consistency high  # 启用 N 路自一致模式

# NOS 特定：historian 状态
nos historian status
nos historian rebuild  # 强制从 Beads 重建 Playbook
```

### 使用 gt CLI 配合 Groq

如果你在 `settings/config.json` 中配置了 Groq 运行时，也可以直接使用 `gt`。所有核心命令正常工作：

```bash
gt mayor attach
gt convoy create "Feature X" gt-abc12 gt-def34
gt sling gt-abc12 myproject
```

主要区别：`gt` 不知道 NOS 特定的功能如 council、Historian 或路由表管理。对那些功能使用 `nos`。

## 与标准 Gas Town 的主要区别

| 功能 | Gas Town (Claude Code) | NOS Town (Groq) |
|------|------------------------|------------------|
| **运行时** | Claude Code IDE | Groq OpenAI 兼容 API |
| **模型选择** | 单模型（Opus/Sonnet/Haiku） | 按角色多模型路由 |
| **Polecat 模式** | 每 Bead 单实例 | 标准 / 自一致 / Power |
| **Witness** | 单一判断 | 可选 Council（N 个评判者） |
| **Refinery** | 仅实时合并队列 | + 离线 Batch 合并模拟 |
| **机构记忆** | 每 Rig 的 CLAUDE.md | + Historian 从所有 Bead 挖掘 Playbook |
| **安全** | Claude 内置 guardrails | + Safeguard-20B 显式哨兵 |
| **成本概况** | ~$15/M 输入, $75/M 输出 | ~$0.10–$0.80/M tokens |
| **吞吐量** | ~50–100 tok/s 每 Agent | ~500+ tok/s 每 Agent |

## 贡献

### 向 Gas Town 核心贡献

如果你发现对 Hook、Bead、Convoy 生命周期或核心角色的改进：

1. Fork `gastownhall/gastown`
2. 在你的 fork 中修改
3. PR 回 `gastownhall/gastown`
4. `kab0rn/gastown` 将从上游同步
5. `kab0rn/nostown` 拉取更新的核心

### 向 NOS Town 运行时贡献

Groq 特定的功能（路由、council、Historian、Safeguard）：

1. Fork `kab0rn/nostown`
2. 修改
3. PR 到 `kab0rn/nostown`

## 文档

- **Gas Town**：[README.md](../../README.md), [docs/](../)
- **NOS Town**：[github.com/kab0rn/nostown](https://github.com/kab0rn/nostown)
  - [docs/ROLES.md](https://github.com/kab0rn/nostown/blob/main/docs/ROLES.md) — Groq 特定角色设计
  - [docs/ROUTING.md](https://github.com/kab0rn/nostown/blob/main/docs/ROUTING.md) — 多模型路由
  - [docs/HISTORIAN.md](https://github.com/kab0rn/nostown/blob/main/docs/HISTORIAN.md) — Playbook 挖掘

## 许可证

Gas Town 和 NOS Town 均为 MIT 许可。