# Tmux 快捷键

Gas Town 覆盖了多个 tmux 快捷键，提供会话导航和操作快捷方式。所有绑定都是有条件的 — 它们仅在 Gas Town 会话中（匹配已注册的 rig 前缀或 `hq-` 的会话）激活。非 GT 会话保留用户的原始绑定。

## 会话循环组（prefix+n / prefix+p）

`gt cycle next` 和 `gt cycle prev` 绑定到 `C-b n` 和 `C-b p`。
它们在基于当前会话类型的组内循环：

| 组 | 包含的会话 | 示例 |
|-------|-------------------|---------|
| **Town** | Mayor + Deacon | `hq-mayor` ↔ `hq-deacon` |
| **Crew** | 同一 rig 中的所有 crew | `gt-crew-max` ↔ `gt-crew-joe` |
| **Rig 运维** | 同一 rig 中的 Witness + Refinery + Polecats | `gt-witness` ↔ `gt-refinery` ↔ `gt-furiosa` ↔ `gt-nux` |

组是按 rig 划分的：`gt-witness` 与 `gt-refinery` 和 gastown polecat 循环，但不与 `bd-witness` 或 `bd-refinery` 循环。

如果组中只有一个会话，prefix+n/p 是无操作。

## 其他绑定

| 键 | 命令 | 用途 |
|-----|---------|--------|
| `C-b a` | `gt feed --window` | 打开/切换到活动动态窗口 |
| `C-b g` | `gt agents menu` | 打开 agent 切换弹窗 |

## 绑定如何设置

绑定由 tmux 包中的 `ConfigureGasTownSession()` 配置，每当创建会话时调用（daemon 为巡逻 agent 创建，witness 为 polecat 创建，`gt crew at` 为 crew 创建）。这意味着：

- 绑定在 tmux 服务器上创建的**第一个** Gas Town 会话上设置
- 它们服务器范围内生效（tmux 快捷键是全局的，不是按会话的）
- `if-shell` 守卫在按键时将它们限定在 GT 会话
- 后续调用是无操作（幂等的）

## 实现细节

### 前缀模式

`if-shell` 守卫使用从所有已注册 rig 前缀构建的正则表达式：

```bash
echo '#{session_name}' | grep -Eq '^(bd|gt|hq)-'
```

模式由 `sessionPrefixPattern()` 从 `config.AllRigPrefixes()` 动态构建。`hq` 和 `gt` 前缀始终包含在内。

### run-shell 上下文

绑定使用 `run-shell`，在 tmux 服务器进程中执行，不在任何会话中。关键变量：

- `#{session_name}` — 在按键时由 tmux 展开（可靠）
- `#{client_tty}` — 标识哪个客户端按下了键（用于多重附加）
- `$TMUX` — 在 run-shell 子进程中设置，指向套接字
- CWD — tmux 服务器的 CWD，通常是 `$HOME`

由于 CWD 是 `$HOME`，`gt` 二进制文件通过 tmux 全局环境中的 `GT_TOWN_ROOT` 查找工作区（由 daemon 在启动时设置）。这由 `gt doctor --check tmux-global-env` 验证。

### 回退保留

首次设置绑定时，捕获每个键的现有绑定，用作 `if-shell` 的 `else` 分支。这为非 GT 会话保留了用户原始的 `C-b n`（next-window）和 `C-b p`（previous-window）。