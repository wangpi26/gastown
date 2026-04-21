# Gas Town 发布指南

## 分发渠道

| 渠道 | 机制 | 是否自动？ |
|------|------|-----------|
| **GitHub Release** | 标签推送时通过 Actions 的 GoReleaser | 是 |
| **Homebrew** (homebrew-core) | Homebrew bot 检测新发布 | 是（24-48小时延迟） |
| **npm** (`@gastown/gt`) | Actions 工作流，OIDC 受信发布 | 是（组织设置完成后） |

## 如何发布

### 方式 A：自动化（推荐）

使用发布 formula，它会处理所有步骤：

```bash
gt mol wisp create gastown-release --var version=X.Y.Z
```

### 方式 B：版本升级脚本

```bash
cd gastown/mayor/rig
./scripts/bump-version.sh X.Y.Z --commit --tag --push --install
```

### 方式 C：手动

1. 更新 CHANGELOG.md 的 `[Unreleased]` 部分
2. 更新 `internal/cmd/info.go` 的 `versionChanges` 切片
3. 运行 `./scripts/bump-version.sh X.Y.Z`（更新 version.go、package.json、CHANGELOG 头部）
4. 提交、打标签、推送：

```bash
git add -A
git commit -m "chore: Bump version to X.Y.Z"
git tag -a vX.Y.Z -m "Release vX.Y.Z"
git push origin main
git push origin vX.Y.Z
```

5. 本地重新构建：

```bash
make install        # 构建、代码签名、安装到 ~/.local/bin
gt daemon stop && gt daemon start
```

## 标签推送后会发生什么

`release.yml` 工作流会自动触发：

1. **验证标签与 Version 常量匹配** —— 运行 `make check-version-tag`，
   如果推送的标签（`vX.Y.Z`）与 `internal/cmd/version.go` 中的 `Version`
   常量不匹配，则中止发布。防止
   [#3459](https://github.com/steveyegge/gastown/issues/3459) 再次发生，
   即 v0.13.0 发布时仍报告 0.12.1 的问题。
2. **goreleaser** 任务为所有平台构建二进制文件并创建 GitHub Release
3. **publish-npm** 任务发布到 npm（尽力而为，`continue-on-error: true`）

Homebrew 不会由工作流更新。参见下文。

### 本地运行标签/版本检查

```bash
make check-version-tag
```

该目标在未打标签的 HEAD 上是空操作，所以在任何检出上运行都是安全的。
只有当 HEAD 被标记为 `vX.Y.Z` 且 `Version` 常量不匹配时才会失败。
在 `scripts/bump-version.sh` 之后、推送标签之前运行此命令，
可以在 CI 之前捕获偏差。

## Homebrew (homebrew-core)

Gastown 在 **homebrew-core** 中（不是自定义 tap）。Formula 位于：
`https://github.com/Homebrew/homebrew-core/blob/HEAD/Formula/g/gastown.rb`

### 更新方式

Homebrew 的 `BrewTestBot` 会自动检测新的 GitHub 发布并向
homebrew-core 提交 PR。Gastown 在自动更新列表上——bot 每 ~3 小时检查一次。

### 如果 bot 没有自动更新

Gastown 在自动更新列表上，因此 `brew bump-formula-pr` 会拒绝
提交手动 PR。如果 6 小时以上 bot 仍未更新，请查看
https://github.com/Homebrew/homebrew-core/pulls?q=gastown 是否有卡住的 PR。

### 验证

```bash
brew update
brew info gastown    # 检查版本
brew upgrade gastown # 如已安装则升级
```

## npm (`@gastown/gt`)

### 工作原理

工作流使用 **OIDC 受信发布**（npm 来源证明）。不需要 NPM_TOKEN
密钥——作业上的 `id-token: write` 权限生成短期 OIDC token，
npm 信任该 token 是因为 GitHub 仓库已与 npm 包关联。

### 前置条件

`@gastown` npm 组织必须存在并与本仓库关联：

1. 前往 https://www.npmjs.com 创建（或加入）`@gastown` 组织
2. 在组织设置中，启用 "Require 2FA" 并配置受信发布
3. 将 `steveyegge/gastown` 关联为 `@gastown/gt` 的受信发布者

### 当前状态（截至 2026-03-06）

`@gastown` npm 组织由社区成员（Ivan Casco Valero,
ivan@ivancasco.com）保护性注册，以防止命名空间抢注。所有权转让待处理。
在组织转让完成之前，npm 发布将优雅地失败而不会阻塞
发布流程（工作流中设置了 `continue-on-error: true`）。

### 验证

```bash
npm view @gastown/gt version
npm install -g @gastown/gt
gt version
```

## 发布期间更新的文件

| 文件 | 变更内容 |
|------|---------|
| `CHANGELOG.md` | 带日期的新版本章节 |
| `internal/cmd/info.go` | `versionChanges` 条目（用于 `gt info --whats-new`） |
| `internal/cmd/version.go` | `Version` 常量 |
| `npm-package/package.json` | `version` 字段 |
| `flake.nix` | version + vendorHash（仅在 PATH 中有 `nix` 时） |

## 故障排除

### GoReleaser 因 "replace directives" 失败

工作流拒绝包含 `replace` 指令的 `go.mod` 文件（它们会导致
`go install` 失败）。在打标签之前删除 replace 指令并提交。

### npm 发布返回 404

`@gastown` npm 组织不存在或你没有发布权限。
参见上方 npm 部分。发布仍会成功——npm 是尽力而为的。

### Homebrew 6 小时后仍显示旧版本

Gastown 在 BrewTestBot 的自动更新列表上（每 ~3 小时检查）。请查看
https://github.com/Homebrew/homebrew-core/pulls?q=gastown 是否有卡住的 PR。
对自动更新 formula，手动 `brew bump-formula-pr` 被阻止。

### `make install` 显示 `-dirty` 后缀

`.beads/` 目录有未暂存的变更。这仅影响外观——版本号是正确的。
`-dirty` 来自 `git describe` 检测到任何未暂存的修改。

### 版本升级脚本后 version.go 中的版本仍是旧的

版本升级脚本从 version.go 读取当前版本并替换它。
如果 version.go 已被手动编辑为不同的版本，脚本的 sed
模式将无法匹配。请手动修复 version.go 后重新运行。