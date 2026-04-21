# 第 4 阶段最小修复 - 验收命令和 PR 草稿

本文档冻结了第 4 阶段最小修复批次的最终验证命令和可直接粘贴的 PR 摘要。

## 修复后的验证命令

从 `gastown-main` 运行：

```bash
# 1) 完整容器验收
make test-e2e-container

# 2) 针对角色路径回归的最小 WSL 冒烟测试
go test ./internal/cmd -tags=integration -run TestRoleHomeCwdDetection\|TestRoleEnvCwdMismatchFromIncompleteDir -count=1 -v
```

## 最新运行结果

- `make test-e2e-container`：**通过**
- `go test ./internal/cmd -tags=integration -run TestRoleHomeCwdDetection\|TestRoleEnvCwdMismatchFromIncompleteDir -count=1 -v`：**通过**

## PR 描述（可直接粘贴）

### 摘要

- 修复调度器集成 JSON 污染问题，保持测试辅助程序成功路径仅输出到 stdout，仅在失败时显示 stderr。
- 使 Dolt 元数据写入幂等，保持 Mayor 工作树整洁（`.beads/metadata.json` 在内容未变时为无操作）。
- 通过固定工具链版本（`bd` 和 `dolt`）、重试循环和 Docker 上下文优化来增强端到端容器可靠性。
- 稳定嘈杂环境中的安装/角色集成行为（公式配置弹性、角色命令预运行噪声抑制、以及在 Dolt 启动竞争条件出现的场景下的容许在线冒烟测试）。

### 验证

- `make test-e2e-container` ✅
- `go test ./internal/cmd -tags=integration -run TestRoleHomeCwdDetection\|TestRoleEnvCwdMismatchFromIncompleteDir -count=1 -v` ✅

### 备注

- 部分集成测试用例在该测试环境中缺少特定外部二进制文件时会故意 `SKIP`。
- 集成日志中显示的 Doctor 警告在合成测试环境中是预期行为，不影响通过/失败判定。