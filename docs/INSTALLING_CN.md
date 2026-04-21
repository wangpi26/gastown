# 安装 Gas Town

Gas Town 多代理编排器的完整安装指南。

## 前提条件

### 必需

| 工具 | 版本 | 检查方式 | 安装方式 |
|------|------|----------|----------|
| **Go** | 1.24+ | `go version` | 参见 [golang.org](https://go.dev/doc/install) |
| **Git** | 2.20+ | `git --version` | 见下方 |
| **Dolt** | >= 1.82.4 | `dolt version` | 参见 [dolthub/dolt](https://github.com/dolthub/dolt?tab=readme-ov-file#installation) |
| **Beads** | >= 0.55.4 | `bd version` | `go install github.com/steveyegge/beads/cmd/bd@latest` |

### 可选（完整堆栈模式）

| 工具 | 版本 | 检查方式 | 安装方式 |
|------|------|----------|----------|
| **tmux** | 3.0+ | `tmux -V` | 见下方 |
| **Claude Code**（默认） | >= 2.0.20 | `claude --version` | 参见 [claude.ai/claude-code](https://claude.ai/claude-code) |
| **Codex CLI**（可选） | 最新 | `codex --version` | 参见 [developers.openai.com/codex/cli](https://developers.openai.com/codex/cli) |
| **OpenCode CLI**（可选） | 最新 | `opencode --version` | 参见 [opencode.ai](https://opencode.ai) |
| **GitHub Copilot CLI**（可选） | 最新 | `copilot --version` | 参见 [cli.github.com](https://cli.github.com)（需要 Copilot 席位） |

## 安装前提条件

### macOS

```bash
# 如需安装 Homebrew
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# 必需
brew install go git
# 安装 Dolt：参见 https://github.com/dolthub/dolt?tab=readme-ov-file#installation

# 可选（完整堆栈模式）
brew install tmux
```

### Linux（Debian/Ubuntu）

```bash
# 必需
sudo apt update
sudo apt install -y git

# 安装 Go（apt 版本可能过旧，建议使用官方安装器）
wget https://go.dev/dl/go1.24.12.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.24.12.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc
source ~/.bashrc

# 安装 Dolt：参见 https://github.com/dolthub/dolt?tab=readme-ov-file#installation

# 可选（完整堆栈模式）
sudo apt install -y tmux
```

### Linux（Fedora/RHEL）

```bash
# 必需
sudo dnf install -y git golang
# 安装 Dolt：参见 https://github.com/dolthub/dolt?tab=readme-ov-file#installation

# 可选
sudo dnf install -y tmux
```

### 验证前提条件

```bash
# 检查所有前提条件
go version        # 应显示 go1.24 或更高
git --version     # 应显示 2.20 或更高
dolt version      # 应显示 1.82.4 或更高
tmux -V           # （可选）应显示 3.0 或更高
```

## 安装 Gas Town

### 第 1 步：安装二进制文件

```bash
# 安装 Gas Town CLI
go install github.com/steveyegge/gastown/cmd/gt@latest

# 安装 Beads（问题跟踪器）
go install github.com/steveyegge/beads/cmd/bd@latest

# 验证安装
gt version
bd version
```

如果找不到 `gt`，请确保 `$GOPATH/bin`（通常是 `~/go/bin`）在你的 PATH 中：

```bash
# 添加到 ~/.bashrc、~/.zshrc 或等效文件
export PATH="$PATH:$HOME/go/bin"
```

### 第 2 步：创建工作空间

```bash
# 创建 Gas Town 工作空间（HQ）
gt install ~/gt --shell

# 这会创建：
#   ~/gt/
#   ├── CLAUDE.md          # 身份锚点（运行 gt prime）
#   ├── mayor/             # Mayor 配置和状态
#   ├── rigs/              # 项目容器（初始为空）
#   └── .beads/            # Town 级问题跟踪
```

### 第 3 步：添加项目（Rig）

```bash
# 添加你的第一个项目
gt rig add myproject https://github.com/you/repo.git

# 这会克隆仓库并设置：
#   ~/gt/myproject/
#   ├── .beads/            # 项目问题跟踪
#   ├── mayor/rig/         # Mayor 的克隆（权威副本）
#   ├── refinery/rig/      # 合并队列处理器
#   ├── witness/           # 工作者监控
#   └── polecats/          # 工作者克隆（按需创建）
```

### 第 4 步：验证安装

```bash
cd ~/gt

gt enable              # 全局启用 Gas Town
gt git-init            # 为 HQ 初始化 git 仓库
gt up                  # 启动所有服务。使用 gt down 或 gt shutdown 停止。

gt doctor              # 运行健康检查
gt status              # 显示工作空间状态
```

### 第 5 步：配置代理（可选）

Gas Town 支持内置运行时（`claude`、`gemini`、`codex`、`cursor`、`auggie`、`amp`、`opencode`、`copilot`）以及自定义代理别名。

```bash
# 列出可用代理
gt config agent list

# 创建别名（别名可以编码模型/思考模式标志）
gt config agent set codex-low "codex --thinking low"
gt config agent set claude-haiku "claude --model haiku --dangerously-skip-permissions"

# 设置全镇默认代理（当 Rig 未指定时使用）
gt config default-agent codex-low
```

你也可以在不更改默认值的情况下按命令覆盖代理：

```bash
gt start --agent codex-low
gt sling gt-abc12 myproject --agent claude-haiku
```

## 最小模式 vs 完整堆栈模式

Gas Town 支持两种运行模式：

### 最小模式（无守护进程）

手动运行单个运行时实例。Gas Town 仅跟踪状态。

```bash
# 创建并分配工作
gt convoy create "Fix bugs" gt-abc12
gt sling gt-abc12 myproject

# 手动运行运行时
cd ~/gt/myproject/polecats/<worker>
claude --resume          # Claude Code
# 或：codex              # Codex CLI

# 检查进度
gt convoy list
```

**适用场景**：测试、简单工作流，或当你倾向于手动控制时。

### 完整堆栈模式（带守护进程）

代理在 tmux 会话中运行。守护进程自动管理生命周期。

```bash
# 启动守护进程
gt daemon start

# 创建并分配工作（工作者自动生成）
gt convoy create "Feature X" gt-abc12 gt-def34
gt sling gt-abc12 myproject
gt sling gt-def34 myproject

# 在仪表板上监控
gt convoy list

# 附加到任何代理会话
gt mayor attach
gt witness attach myproject
```

**适用场景**：多个并发代理的生产工作流。

### 选择角色

Gas Town 是模块化的。仅启用你需要的功能：

| 配置 | 角色 | 用例 |
|------|------|------|
| **仅 Polecat** | 工作者 | 手动生成，无监控 |
| **+ Witness** | + 监控器 | 自动生命周期，卡住检测 |
| **+ Refinery** | + 合并队列 | MR 审查，代码集成 |
| **+ Mayor** | + 协调器 | 跨项目协调 |

## 故障排除

### `gt: command not found`

你的 Go bin 目录不在 PATH 中：

```bash
# 添加到你的 Shell 配置（~/.bashrc, ~/.zshrc）
export PATH="$PATH:$HOME/go/bin"
source ~/.bashrc  # 或重启终端
```

### `bd: command not found`

Beads CLI 未安装：

```bash
go install github.com/steveyegge/beads/cmd/bd@latest
```

### `gt doctor` 显示错误

使用 `--fix` 自动修复常见问题：

```bash
gt doctor --fix
```

对于持续性问题，检查具体错误：

```bash
gt doctor --verbose
```

### 守护进程未启动

检查 tmux 是否已安装并正常工作：

```bash
tmux -V                    # 应显示版本
tmux new-session -d -s test && tmux kill-session -t test  # 快速测试
```

### Git 认证问题

确保 SSH 密钥或凭据已配置：

```bash
# 测试 SSH 访问
ssh -T git@github.com

# 或配置凭据助手
git config --global credential.helper cache
```

### Beads 问题

如果遇到 Beads 问题：

```bash
cd ~/gt/myproject/mayor/rig
bd status                  # 检查数据库健康状态
bd doctor                  # 运行 Beads 健康检查
```

## 更新

更新 Gas Town 和 Beads：

```bash
go install github.com/steveyegge/gastown/cmd/gt@latest
go install github.com/steveyegge/beads/cmd/bd@latest
gt doctor --fix            # 修复更新后的问题
```

## 卸载

```bash
# 移除二进制文件
rm $(which gt) $(which bd)

# 移除工作空间（注意：会删除所有工作）
rm -rf ~/gt
```

## 下一步

安装完成后：

1. **阅读 README** — 核心概念和工作流
2. **尝试简单工作流** — `bd create "Test task"` 然后 `gt convoy create "Test" <bead-id>`
3. **浏览文档** — `docs/reference.md` 命令参考
4. **定期运行 doctor** — `gt doctor` 可及早发现问题
5. **加入 Wasteland** — `gt wl join hop/wl-commons` 浏览并领取联邦工作（参见 [WASTELAND.md](WASTELAND.md)）