+++
name = "git-hygiene"
description = "清理所有 rig 仓库中的过期 git 分支、stash 和松散对象"
version = 1

[gate]
type = "cooldown"
duration = "12h"

[tracking]
labels = ["plugin:git-hygiene", "category:cleanup"]
digest = true

[execution]
timeout = "10m"
notify_on_failure = true
severity = "low"
+++

# Git Hygiene

自动清理所有 rig 仓库中的过期 git 分支、stash 和松散对象。
涵盖本地分支（已合并和孤立分支）、GitHub 上的远程分支、
过期的 stash 以及垃圾回收。

前提：需安装 `gh` CLI 并完成认证（`gh auth status`）。

## 步骤 1：枚举 rig 仓库

遍历所有未下线的 rig 以获取它们的仓库路径：

```bash
RIG_JSON=$(gt rig list --json 2>/dev/null)
if [ $? -ne 0 ] || [ -z "$RIG_JSON" ]; then
  echo "SKIP: could not get rig list"
  exit 0
fi

# 提取有仓库路径的 rig
RIG_PATHS=$(echo "$RIG_JSON" | jq -r '.[] | select(.repo_path != null and .repo_path != "") | .repo_path // empty' 2>/dev/null)
if [ -z "$RIG_PATHS" ]; then
  echo "SKIP: no rigs with repo paths found"
  exit 0
fi

RIG_COUNT=$(echo "$RIG_PATHS" | wc -l | tr -d ' ')
echo "Found $RIG_COUNT rig repo(s) to clean"
```

## 步骤 2：处理每个 rig 仓库

对每个 rig 仓库，运行完整的清理流程。跟踪所有 rig 的汇总数据。

```bash
TOTAL_LOCAL_MERGED=0
TOTAL_LOCAL_ORPHAN=0
TOTAL_REMOTE=0
TOTAL_STASHES=0
TOTAL_GC=0
ERRORS=()

while IFS= read -r REPO_PATH; do
  [ -z "$REPO_PATH" ] && continue

  # 验证是否为 git 仓库
  if ! git -C "$REPO_PATH" rev-parse --git-dir >/dev/null 2>&1; then
    echo "SKIP: $REPO_PATH is not a git repo"
    continue
  fi

  echo ""
  echo "=== Cleaning: $REPO_PATH ==="

  # 检测默认分支（main 或 master）
  DEFAULT_BRANCH=$(git -C "$REPO_PATH" symbolic-ref refs/remotes/origin/HEAD 2>/dev/null \
    | sed 's|refs/remotes/origin/||')
  if [ -z "$DEFAULT_BRANCH" ]; then
    DEFAULT_BRANCH="main"
  fi

  CURRENT_BRANCH=$(git -C "$REPO_PATH" branch --show-current 2>/dev/null)

  ### 步骤 2a：清理远程跟踪引用
  echo "  Pruning remote tracking refs..."
  git -C "$REPO_PATH" fetch --prune --all 2>/dev/null || true

  ### 步骤 2b：删除已合并的本地分支
  echo "  Deleting merged local branches..."
  MERGED_BRANCHES=$(git -C "$REPO_PATH" branch --merged "$DEFAULT_BRANCH" 2>/dev/null \
    | grep -v "^\*" \
    | grep -v "^+" \
    | grep -v -E "^\s*(main|master)$" \
    | sed 's/^[[:space:]]*//')

  LOCAL_MERGED=0
  while IFS= read -r BRANCH; do
    [ -z "$BRANCH" ] && continue
    # 绝不删除当前分支或默认分支
    if [ "$BRANCH" = "$CURRENT_BRANCH" ] || [ "$BRANCH" = "$DEFAULT_BRANCH" ]; then
      continue
    fi
    # 绝不删除基础设施分支
    case "$BRANCH" in
      refinery-patrol|merge/*) continue ;;
    esac
    echo "    Deleting merged: $BRANCH"
    git -C "$REPO_PATH" branch -d "$BRANCH" 2>/dev/null && LOCAL_MERGED=$((LOCAL_MERGED + 1))
  done <<< "$MERGED_BRANCHES"
  TOTAL_LOCAL_MERGED=$((TOTAL_LOCAL_MERGED + LOCAL_MERGED))

  ### 步骤 2c：删除过期的未合并孤立分支
  # 仅删除匹配已知代理/临时模式且满足以下条件的分支：
  # - 没有活跃的 worktree（不以 + 开头）
  # - 没有对应的远程跟踪分支
  echo "  Deleting stale orphan branches..."
  STALE_PATTERNS="polecat/|dog/|fix/|pr-|integration/|worktree-agent-"
  ALL_BRANCHES=$(git -C "$REPO_PATH" branch 2>/dev/null \
    | grep -v "^\*" \
    | grep -v "^+" \
    | sed 's/^[[:space:]]*//')

  LOCAL_ORPHAN=0
  while IFS= read -r BRANCH; do
    [ -z "$BRANCH" ] && continue
    # 必须匹配某个过期模式
    if ! echo "$BRANCH" | grep -qE "^($STALE_PATTERNS)"; then
      continue
    fi
    # 绝不删除当前分支、默认分支或基础设施分支
    if [ "$BRANCH" = "$CURRENT_BRANCH" ] || [ "$BRANCH" = "$DEFAULT_BRANCH" ]; then
      continue
    fi
    case "$BRANCH" in
      main|master|refinery-patrol|merge/*) continue ;;
    esac
    # 检查远程跟踪分支是否存在
    if git -C "$REPO_PATH" rev-parse --verify "refs/remotes/origin/$BRANCH" >/dev/null 2>&1; then
      continue  # 远程分支仍存在，跳过
    fi
    echo "    Deleting orphan: $BRANCH"
    git -C "$REPO_PATH" branch -D "$BRANCH" 2>/dev/null && LOCAL_ORPHAN=$((LOCAL_ORPHAN + 1))
  done <<< "$ALL_BRANCHES"
  TOTAL_LOCAL_ORPHAN=$((TOTAL_LOCAL_ORPHAN + LOCAL_ORPHAN))

  ### 步骤 2d：删除 GitHub 上已合并的远程分支
  echo "  Deleting merged remote branches..."
  REMOTE_DELETED=0

  # 从远程仓库检测 GitHub 仓库
  GH_REPO=$(git -C "$REPO_PATH" remote get-url origin 2>/dev/null \
    | sed -E 's|.*github\.com[:/]||; s|\.git$||')

  if [ -n "$GH_REPO" ]; then
    REMOTE_BRANCHES=$(git -C "$REPO_PATH" branch -r 2>/dev/null \
      | grep -v HEAD \
      | grep -v "origin/$DEFAULT_BRANCH" \
      | grep -v "origin/dependabot/" \
      | grep -v "origin/refinery-patrol" \
      | grep -vE "origin/merge/" \
      | sed 's|^[[:space:]]*origin/||')

    REMOTE_PATTERNS="polecat/|fix/|pr-|integration/|worktree-agent-"

    while IFS= read -r RBRANCH; do
      [ -z "$RBRANCH" ] && continue
      # 必须匹配清理模式
      if ! echo "$RBRANCH" | grep -qE "^($REMOTE_PATTERNS)"; then
        continue
      fi
      # 检查是否已合并到默认分支
      if git -C "$REPO_PATH" merge-base --is-ancestor "origin/$RBRANCH" "origin/$DEFAULT_BRANCH" 2>/dev/null; then
        echo "    Deleting remote: origin/$RBRANCH"
        # 使用 gh api 因为 git push --delete 可能被 pre-push 钩子阻止
        gh api "repos/$GH_REPO/git/refs/heads/$RBRANCH" -X DELETE 2>/dev/null && REMOTE_DELETED=$((REMOTE_DELETED + 1))
      fi
    done <<< "$REMOTE_BRANCHES"
  else
    echo "    SKIP: could not detect GitHub repo from remote"
  fi
  TOTAL_REMOTE=$((TOTAL_REMOTE + REMOTE_DELETED))

  ### 步骤 2e：清理过期 stash
  echo "  Clearing stashes..."
  STASH_COUNT=$(git -C "$REPO_PATH" stash list 2>/dev/null | wc -l | tr -d ' ')
  if [ "$STASH_COUNT" -gt 0 ]; then
    echo "    Clearing $STASH_COUNT stash(es)"
    git -C "$REPO_PATH" stash clear 2>/dev/null
    TOTAL_STASHES=$((TOTAL_STASHES + STASH_COUNT))
  fi

  ### 步骤 2f：垃圾回收
  echo "  Running git gc..."
  git -C "$REPO_PATH" gc --prune=now --quiet 2>/dev/null && TOTAL_GC=$((TOTAL_GC + 1))

  echo "  Done: $LOCAL_MERGED merged, $LOCAL_ORPHAN orphan, $REMOTE_DELETED remote, $STASH_COUNT stash(es)"
done <<< "$RIG_PATHS"
```

## 记录结果

```bash
SUMMARY="$RIG_COUNT rig(s): $TOTAL_LOCAL_MERGED merged branch(es), $TOTAL_LOCAL_ORPHAN orphan branch(es), $TOTAL_REMOTE remote branch(es), $TOTAL_STASHES stash(es) cleared, $TOTAL_GC gc run(s)"
echo ""
echo "=== Git Hygiene Summary ==="
echo "$SUMMARY"
```

成功时：
```bash
bd create "git-hygiene: $SUMMARY" -t chore --ephemeral \
  -l type:plugin-run,plugin:git-hygiene,result:success \
  -d "$SUMMARY" --silent 2>/dev/null || true
```

失败时：
```bash
bd create "git-hygiene: FAILED" -t chore --ephemeral \
  -l type:plugin-run,plugin:git-hygiene,result:failure \
  -d "Git hygiene failed: $ERROR" --silent 2>/dev/null || true

gt escalate "Plugin FAILED: git-hygiene" \
  --severity low \
  --reason "$ERROR"
```