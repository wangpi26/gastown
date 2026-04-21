+++
name = "github-sheriff"
description = "监控开放 PR 的 GitHub CI 检查状态，并为失败的检查创建 Bead"
version = 1

[gate]
type = "cooldown"
duration = "2h"

[tracking]
labels = ["plugin:github-sheriff", "category:ci-monitoring"]
digest = true

[execution]
timeout = "2m"
notify_on_failure = true
severity = "low"
+++

# GitHub Sheriff

轮询 GitHub 上的开放 Pull Request，按就绪程度分类，
并为新出现的失败创建 `ci-failure` Bead。实现了
[Gas Town 用户手册](https://steve-yegge.medium.com/gas-town-emergency-user-manual-cf0e4556d74b)
中的 PR Sheriff 模式，作为 Deacon 插件运行。

将每个 PR 分类为：
- **Easy win**：CI 通过、改动量小（<200 LOC 变更）、无合并冲突
- **Needs review**：CI 失败、改动量大或有冲突

前提：需安装 `gh` CLI 并完成认证（`gh auth status`）。

## 检测

验证 `gh` 是否可用且已认证：

```bash
gh auth status 2>/dev/null
if [ $? -ne 0 ]; then
  echo "SKIP: gh CLI not authenticated"
  exit 0
fi
```

从 rig 的 git 远程仓库检测仓库信息。如果自动检测失败，则
回退到显式配置：

```bash
REPO=$(git -C "$GT_RIG_ROOT" remote get-url origin 2>/dev/null \
  | sed -E 's|.*github\.com[:/]||; s|\.git$||')

if [ -z "$REPO" ]; then
  echo "SKIP: could not detect GitHub repo from rig remote"
  exit 0
fi
```

## 执行

### 步骤 1：获取开放 PR 的完整详情

通过 `gh` 在单次 GraphQL 调用中获取所有开放 PR。返回新增行数、
删除行数、可合并状态和 CI 检查结果，无需逐个 PR 调用 API：

```bash
SINCE=$(date -d '7 days ago' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -v-7d +%Y-%m-%dT%H:%M:%SZ)
PRS=$(gh pr list --repo "$REPO" --state open \
  --json number,title,author,additions,deletions,mergeable,statusCheckRollup,url,updatedAt \
  --limit 100 | jq --arg since "$SINCE" '[.[] | select(.updatedAt >= $since)]')

PR_COUNT=$(echo "$PRS" | jq length)
if [ "$PR_COUNT" -eq 0 ]; then
  echo "No open PRs found for $REPO"
  exit 0
fi
```

### 步骤 2：对每个 PR 分类

使用进程替换（而非管道）处理每个 PR，以确保数组修改
在循环结束后仍然有效：

```bash
EASY_WINS=()
NEEDS_REVIEW=()
FAILURES=()

while IFS= read -r PR_JSON; do
  [ -z "$PR_JSON" ] && continue

  PR_NUM=$(echo "$PR_JSON" | jq -r '.number')
  PR_TITLE=$(echo "$PR_JSON" | jq -r '.title')
  AUTHOR=$(echo "$PR_JSON" | jq -r '.author.login')
  ADDITIONS=$(echo "$PR_JSON" | jq -r '.additions // 0')
  DELETIONS=$(echo "$PR_JSON" | jq -r '.deletions // 0')
  MERGEABLE=$(echo "$PR_JSON" | jq -r '.mergeable')
  TOTAL_CHANGES=$((ADDITIONS + DELETIONS))

  # 从 statusCheckRollup 确定 CI 状态
  TOTAL_CHECKS=$(echo "$PR_JSON" | jq '.statusCheckRollup | length')
  PASSING_CHECKS=$(echo "$PR_JSON" | jq '[.statusCheckRollup[] | select(
    .conclusion == "SUCCESS" or .conclusion == "NEUTRAL" or
    .conclusion == "SKIPPED" or .state == "SUCCESS"
  )] | length')

  if [ "$TOTAL_CHECKS" -gt 0 ] && [ "$TOTAL_CHECKS" -eq "$PASSING_CHECKS" ]; then
    CI_PASS=true
  else
    CI_PASS=false
  fi

  # 收集各检查失败项以创建 Bead
  while IFS= read -r CHECK; do
    [ -z "$CHECK" ] && continue
    CHECK_NAME=$(echo "$CHECK" | jq -r '.name')
    CHECK_URL=$(echo "$CHECK" | jq -r '.detailsUrl // .targetUrl // empty')
    FAILURES+=("$PR_NUM|$PR_TITLE|$CHECK_NAME|$CHECK_URL")
  done < <(echo "$PR_JSON" | jq -c '.statusCheckRollup[] | select(
    .conclusion == "FAILURE" or .conclusion == "CANCELLED" or
    .conclusion == "TIMED_OUT" or .state == "FAILURE" or .state == "ERROR"
  )')

  # 对 PR 分类
  if [ "$MERGEABLE" = "MERGEABLE" ] && [ "$CI_PASS" = true ] && [ "$TOTAL_CHANGES" -lt 200 ]; then
    EASY_WINS+=("PR #$PR_NUM: $PR_TITLE (by $AUTHOR, +$ADDITIONS/-$DELETIONS)")
  else
    REASONS=""
    [ "$MERGEABLE" != "MERGEABLE" ] && REASONS+="conflicts "
    [ "$CI_PASS" != true ] && REASONS+="ci-failing "
    [ "$TOTAL_CHANGES" -ge 200 ] && REASONS+="large(${TOTAL_CHANGES}loc) "
    NEEDS_REVIEW+=("PR #$PR_NUM: $PR_TITLE (by $AUTHOR, ${REASONS% })")
  fi
done < <(echo "$PRS" | jq -c '.[]')

# 报告分类结果
if [ ${#EASY_WINS[@]} -gt 0 ]; then
  echo "Easy wins (${#EASY_WINS[@]}):"
  printf '  %s\n' "${EASY_WINS[@]}"
fi
if [ ${#NEEDS_REVIEW[@]} -gt 0 ]; then
  echo "Needs review (${#NEEDS_REVIEW[@]}):"
  printf '  %s\n' "${NEEDS_REVIEW[@]}"
fi
```

## 记录结果

```bash
SUMMARY="$REPO: $PR_COUNT PRs — ${#EASY_WINS[@]} easy win(s), ${#NEEDS_REVIEW[@]} need review, ${#FAILURES[@]} CI failure(s) detected"
echo "$SUMMARY"
```

成功时：
```bash
bd create "github-sheriff: $SUMMARY" -t chore --ephemeral \
  -l type:plugin-run,plugin:github-sheriff,result:success \
  -d "$SUMMARY" --silent 2>/dev/null || true
```

失败时：
```bash
bd create "github-sheriff: FAILED" -t chore --ephemeral \
  -l type:plugin-run,plugin:github-sheriff,result:failure \
  -d "GitHub sheriff failed: $ERROR" --silent 2>/dev/null || true

gt escalate "Plugin FAILED: github-sheriff" \
  --severity low \
  --reason "$ERROR"
```