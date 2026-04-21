# 汉诺塔演示

一个耐久性验证，展示 Gas Town 在任意长的顺序工作流中执行崩溃恢复和会话循环的能力。

## 这证明了什么

1. **大型 Molecule 创建**：在单个工作流中创建 1000+ issue
2. **顺序执行**：依赖在多个步骤间正确链式传递
3. **崩溃恢复**：会话重启后工作正确恢复
4. **非确定性幂等性**：不同会话，相同结果

## 数学原理

汉诺塔需要 `2^n - 1` 步移动 `n` 个盘子：

| 盘子数 | 移动次数 | Formula 大小 | 预估运行时间 |
|-------|---------|-------------|-------------|
| 7     | 127     | ~19 KB      | ~14 秒      |
| 9     | 511     | ~74 KB      | ~1 分钟     |
| 10    | 1,023   | ~149 KB     | ~2 分钟     |
| 15    | 32,767  | ~4.7 MB     | ~1 小时     |
| 20    | 1M+     | ~163 MB     | ~30 小时    |

## 预生成的 Formula

位于 `.beads/formulas/`：

- `towers-of-hanoi-7.formula.toml` - 127 步移动（快速测试）
- `towers-of-hanoi-9.formula.toml` - 511 步移动（中等测试）
- `towers-of-hanoi-10.formula.toml` - 1023 步移动（标准演示）

## 运行演示

### 快速测试（7 个盘子，约 14 秒）

```bash
# 创建 wisp
bd mol wisp towers-of-hanoi-7 --json | jq -r '.new_epic_id'
# 返回: gt-eph-xxx

# 获取所有子项 ID
bd list --parent=gt-eph-xxx --limit=200 --json | jq -r '.[].id' > /tmp/ids.txt

# 关闭所有 issue（串行）
while read id; do bd close "$id" >/dev/null; done < /tmp/ids.txt

# 烧毁 wisp（清理）
bd mol burn gt-eph-xxx --force
```

### 标准演示（10 个盘子，约 2 分钟）

```bash
# 创建 wisp
WISP=$(bd mol wisp towers-of-hanoi-10 --json | jq -r '.new_epic_id')
echo "Created wisp: $WISP"

# 获取所有 1025 个子项 ID（1023 步移动 + setup + verify）
bd list --parent=$WISP --limit=2000 --json | jq -r '.[].id' > /tmp/ids.txt
wc -l /tmp/ids.txt  # 应显示 1025

# 计时执行
START=$(date +%s)
while read id; do bd close "$id" >/dev/null 2>&1; done < /tmp/ids.txt
END=$(date +%s)
echo "Completed in $((END - START)) seconds"

# 验证完成
bd list --parent=$WISP --status=open  # 应为空

# 清理
bd mol burn $WISP --force
```

## 为什么用 Wisp？

演示使用 Wisp（临时 Molecule）因为：

1. **不污染 Git**：Wisp 仅存在于数据库中，保持 git 历史干净
2. **自动清理**：Wisp 可以烧毁而不留墓碑
3. **速度**：快速关闭时无导出开销
4. **语义合适**：这是操作测试，不是可审计的工作

## 关键洞察

### `bd ready` 排除 Wisp

设计上，`bd ready` 过滤掉临时 issue：
```go
"(i.ephemeral = 0 OR i.ephemeral IS NULL)", // Exclude wisps
```

对于 Wisp 执行，直接查询子项：
```bash
bd list --parent=$WISP --status=open
```

### 依赖正确工作

每步移动通过 `needs` 依赖前一步：
```toml
[[steps]]
id = "move-42"
needs = ["move-41"]
```

这创建了正确的 `blocks` 依赖。父子关系仅提供层级结构——不阻塞执行。

### 关闭速度

使用 `bd close`：
- 每次关闭约 109ms（串行）
- 每秒约 9 次关闭

并行化可以提高吞吐量，但需要仔细的依赖排序。

## 生成更大的 Formula

使用生成器脚本：

```bash
# 生成 15 盘子 Formula（32K 步移动）
python3 scripts/gen_hanoi.py 15 > .beads/formulas/towers-of-hanoi-15.formula.toml
```

**警告**：20 盘子 Formula 约 163MB，创建 1M+ issue。仅用于发布后的压力测试。

## 监控进度

对于长时间运行的执行：

```bash
# 计算已关闭的 issue
bd list --parent=$WISP --status=closed --json | jq 'length'

# 计算剩余的 issue
bd list --parent=$WISP --status=open --json | jq 'length'

# 进度百分比
TOTAL=1025
CLOSED=$(bd list --parent=$WISP --status=closed --limit=2000 --json | jq 'length')
echo "$CLOSED / $TOTAL = $((CLOSED * 100 / TOTAL))%"
```

## 会话循环

这个演示的美妙之处：你可以随时停止，稍后恢复。

```bash
# 会话 1：启动 wisp，关闭一些 issue
WISP=$(bd mol wisp towers-of-hanoi-10 --json | jq -r '.new_epic_id')
# ... 关闭一些 issue ...
# 上下文填满，需要循环

gt handoff -s "Hanoi demo" -m "Wisp: $WISP, progress: 400/1025"
```

```bash
# 会话 2：从停止处恢复
# （从 handoff 邮件读取 wisp ID）
bd list --parent=$WISP --status=open --limit=2000 --json | jq -r '.[].id' > /tmp/ids.txt
# ... 继续关闭 ...
```

Molecule 就是状态。无需记住上一个会话。