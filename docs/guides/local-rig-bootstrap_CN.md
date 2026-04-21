# 本地 Rig 引导

对于 NightRider 风格的本地设置，推荐使用干净的引导而非 `gt rig add --adopt`。

`--adopt` 用于注册一个已经组装好的 Rig 目录。它信任现有的结构，这使得它不适合手动组装的本地 Rig——在那种情况下 `.repo.git`、worktree 和元数据可能已经不一致了。

使用引导脚本代替：

```bash
./scripts/bootstrap-local-rig.sh \
  --town-root /gt \
  --rig nightrider_local \
  --local-repo /gt/nightRider \
  --prefix nr \
  --polecat-agent claude \
  --witness-agent codex \
  --refinery-agent codex
```

如果省略 `--remote`，脚本会使用 `file://<local-repo>` 注册 Rig。这通常是本地或私有仓库在 Gastown 容器内的正确选择，因为上游远程可能不可达或未认证。

此脚本的作用：

- 使用 `gt rig add <name> <git-url> --local-repo <path>`，让 Gas Town 创建一个全新的标准 Rig 容器，而非继承手工搭建的。
- 复用本地仓库的对象，因此引导保持快速且不修改源仓库。
- 让生成的 Rig 具有 Gas Town 期望的标准 `.repo.git`、`mayor/rig`、`refinery/rig`、`settings/` 和 `.beads/` 布局。
- 可选地在 `settings/config.json` 中固定每个 Rig 的角色 Agent。

何时仍应使用 `--adopt`：

- 你已有一个真正在其他地方创建的 Gas Town Rig 目录，只需要在一个 Town 中注册它。