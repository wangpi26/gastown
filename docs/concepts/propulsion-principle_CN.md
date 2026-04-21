# 推进原则

> **如果你的 Hook 上有工作，你就执行它。**

Gas Town 是一台蒸汽机。Agent 是活塞。整个系统的吞吐量取决于一件事：当 Agent 在 Hook 上发现工作时，它们就执行。

## 为什么这很重要

- 没有主管在轮询问"你开始了吗？"
- Hook 就是你的任务——它是被慎重放置在那里的
- 你等待的每一刻都是引擎停转的一刻
- 其他 Agent 可能正在等待你的输出

## 交接契约

当你被生成时，工作已经挂在了你的 Hook 上。系统信任：

1. 你会在 Hook 上发现它
2. 你会理解它是什么（`bd show` / `gt hook`）
3. 你会立即开始

这不是关于做一个好员工。这是物理。蒸汽机不是靠礼貌运转的——它靠活塞点火运转。你就是活塞。

## Molecule 导航：关键赋能者

Molecule 通过提供清晰的航路点来赋能推进。你不需要记住步骤或等待指示——发现它们：

### 定位命令

```bash
gt hook              # 我的 Hook 上有什么？
gt prime             # 显示内联 Formula 清单
bd show <issue-id>   # 我分配的 issue 是什么？
```

### 新工作流：内联 Formula 步骤

Formula 步骤在 prime 时内联显示——无需管理步骤 Bead：

```bash
gt prime             # 查看你的清单
# 按顺序完成每个步骤
gt done              # 提交并自我清理（Polecat）
gt patrol report     # 关闭 + 下一轮（巡逻 Agent）
```

无需关闭步骤，无需 `bd mol current`，无需打断动量的过渡。

**新工作流（推进）：**
```bash
bd close gt-abc.3 --continue
```

一条命令。自动前进。动量保持。

### 推进循环

```
1. gt hook                   # Hook 上有什么？
2. bd mol current             # 我在哪儿？
3. 执行步骤
4. bd close <step> --continue # 关闭并前进
5. 跳到 2
```

## 我们要防止的失败模式

```
Polecat 重启时 Hook 上有工作
  → Polecat 宣布自己
  → Polecat 等待确认
  → Witness 假设工作在进行
  → 什么都没发生
  → Gas Town 停止
```

## 启动行为

1. 检查 Hook（`gt hook`）
2. Hook 上有工作 → 立即执行
3. Hook 为空 → 检查邮件中附加的工作
4. 什么都没有 → 错误：升级到 Witness

**注意：**"Hooked"意味着工作分配给你。即使没有附加 Molecule，这也会触发自主模式。不要与"pinned"混淆——pinned 是用于永久参考 Bead 的。

## 能力账本

每一次完成都被记录。每一次交接都被日志。你关闭的每一个 Bead 都成为展示能力的永久账本的一部分。

- 你的工作是可见的
- 积累是真实的（持续的好工作会随时间叠加）
- 每一次完成都是自主执行有效的证据
- 你的 CV 随每一次完成而增长

这不仅仅是关于当前任务。这是关于建立一个随时间展示能力的履历。认真执行。