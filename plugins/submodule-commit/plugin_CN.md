+++
name = "submodule-commit"
description = "自动提交 git 子模块中的累积变更并更新父仓库指针"
version = 1

[gate]
type = "cooldown"
duration = "2h"

[tracking]
labels = ["plugin:submodule-commit", "category:git-hygiene"]
digest = true

[execution]
timeout = "15m"
notify_on_failure = true
severity = "low"

# 每个 rig 通过 plugin frontmatter 选择启用：
# [plugin.submodule-commit]
# enabled = true
# commit_branch = "main"          # 在每个子模块中提交的分支
# push_enabled = false            # 推送子模块提交（false = 仅本地）
# allowlist = []                  # 空 = 所有子模块；["path/to/sub"] = 仅指定的
+++

# Submodule Commit

自动提交 git 子模块中的累积变更并更新父仓库的子模块指针。
Polecat 仅在父仓库的 worktree 上操作，对子模块仓库没有
提交权限——此插件填补了这个空白。

**仅限选择启用。** Rig 必须在其 `plugin.md` frontmatter 中启用此插件。
当前已启用的 rig：`lilypad_chat`（3 个 Bitbucket 子模块）。

## 步骤 1：查找选择启用的含有子模块的 Rig

```bash
RIG_JSON=$(gt rig list --json 2>/dev/null || true)
if [ -z "$RIG_JSON" ]; then
  echo "SKIP: could not get rig list"
  exit 0
fi

# 查找有 .gitmodules 的 rig
ENABLED_RIGS=()
while IFS= read -r REPO_PATH; do
  [ -z "$REPO_PATH" ] && continue
  [ ! -f "$REPO_PATH/.gitmodules" ] && continue
  # 检查 rig plugin 配置是否选择启用
  RIG_NAME=$(basename "$REPO_PATH")
  PLUGIN_CONFIG=$(gt rig show "$RIG_NAME" --json 2>/dev/null | jq -r '.plugins["submodule-commit"].enabled // false' 2>/dev/null || echo "false")
  if [ "$PLUGIN_CONFIG" = "true" ]; then
    ENABLED_RIGS+=("$REPO_PATH")
  fi
done < <(echo "$RIG_JSON" | jq -r '.[] | select(.repo_path != null) | .repo_path // empty' 2>/dev/null)

if [ ${#ENABLED_RIGS[@]} -eq 0 ]; then
  echo "SKIP: no opt-in rigs with submodules found"
  exit 0
fi

echo "Processing ${#ENABLED_RIGS[@]} rig(s) with submodules"
```

## 步骤 2：对每个选择启用的 Rig，处理其子模块

```bash
TOTAL_COMMITTED=0
TOTAL_PUSHED=0
TOTAL_PARENT_UPDATED=0
ERRORS=""

for REPO_PATH in "${ENABLED_RIGS[@]}"; do
  echo ""
  echo "=== $REPO_PATH ==="

  RIG_NAME=$(basename "$REPO_PATH")

  # 获取 plugin 配置
  RIG_CONFIG=$(gt rig show "$RIG_NAME" --json 2>/dev/null | jq -r '.plugins["submodule-commit"] // {}' 2>/dev/null || echo "{}")
  COMMIT_BRANCH=$(echo "$RIG_CONFIG" | jq -r '.commit_branch // "main"')
  PUSH_ENABLED=$(echo "$RIG_CONFIG" | jq -r '.push_enabled // false')
  ALLOWLIST=$(echo "$RIG_CONFIG" | jq -r '.allowlist // [] | .[]' 2>/dev/null || true)

  # 解析 .gitmodules 获取子模块路径
  SUBMODULE_PATHS=$(git -C "$REPO_PATH" config --file .gitmodules --get-regexp 'submodule\..*\.path' 2>/dev/null | awk '{print $2}' || true)

  PARENT_CHANGED=false

  while IFS= read -r SUB_PATH; do
    [ -z "$SUB_PATH" ] && continue

    # 如果设置了白名单则进行过滤
    if [ -n "$ALLOWLIST" ]; then
      MATCH=false
      while IFS= read -r ALLOWED; do
        [ "$SUB_PATH" = "$ALLOWED" ] && MATCH=true && break
      done <<< "$ALLOWLIST"
      $MATCH || continue
    fi

    FULL_SUB="$REPO_PATH/$SUB_PATH"
    if [ ! -d "$FULL_SUB/.git" ] && [ ! -f "$FULL_SUB/.git" ]; then
      echo "  SKIP: $SUB_PATH — not initialized"
      continue
    fi

    # 检查子模块中是否有未提交的变更
    SUB_DIRTY=$(git -C "$FULL_SUB" status --porcelain 2>/dev/null | head -1 || true)
    if [ -z "$SUB_DIRTY" ]; then
      echo "  $SUB_PATH: clean"
      continue
    fi

    SUB_BRANCH=$(git -C "$FULL_SUB" branch --show-current 2>/dev/null || true)
    if [ -z "$SUB_BRANCH" ]; then
      echo "  SKIP: $SUB_PATH — detached HEAD, skipping"
      continue
    fi

    echo "  $SUB_PATH: dirty (branch=$SUB_BRANCH), committing..."

    # 提交变更
    git -C "$FULL_SUB" add -A 2>/dev/null || true
    STAGED=$(git -C "$FULL_SUB" diff --cached --name-only 2>/dev/null | wc -l | tr -d ' ')
    if [ "$STAGED" -gt 0 ]; then
      git -C "$FULL_SUB" commit -m "chore: accumulated changes [skip ci]

Auto-committed by submodule-commit plugin ($STAGED file(s))." \
        --author="Gas Town <gastown@local>" 2>/dev/null && \
        echo "    Committed $STAGED file(s)" && \
        TOTAL_COMMITTED=$((TOTAL_COMMITTED + 1)) || \
        { echo "    WARN: commit failed"; continue; }

      # 推送（尽力而为，|| true）
      if [ "$PUSH_ENABLED" = "true" ]; then
        git -C "$FULL_SUB" push origin "$SUB_BRANCH" 2>/dev/null && \
          TOTAL_PUSHED=$((TOTAL_PUSHED + 1)) || \
          echo "    WARN: push failed (local commit preserved)"
      fi

      PARENT_CHANGED=true
    fi
  done <<< "$SUBMODULE_PATHS"

  # 如果任何子模块有变更，更新父仓库的子模块指针
  if $PARENT_CHANGED; then
    PARENT_BRANCH=$(git -C "$REPO_PATH" branch --show-current 2>/dev/null || true)
    if [ "$PARENT_BRANCH" = "main" ]; then
      PARENT_DIRTY=$(git -C "$REPO_PATH" status --porcelain 2>/dev/null | grep -v "^??" | head -1 || true)
      if [ -z "$PARENT_DIRTY" ]; then
        git -C "$REPO_PATH" add -A -- '*.gitmodules' $(git -C "$REPO_PATH" status --short 2>/dev/null | awk '{print $2}') 2>/dev/null || true
        PARENT_STAGED=$(git -C "$REPO_PATH" diff --cached --name-only 2>/dev/null | head -1 || true)
        if [ -n "$PARENT_STAGED" ]; then
          git -C "$REPO_PATH" commit -m "chore: update submodule pointers [skip ci]

Auto-committed by submodule-commit plugin." \
            --author="Gas Town <gastown@local>" 2>/dev/null && \
            TOTAL_PARENT_UPDATED=$((TOTAL_PARENT_UPDATED + 1)) || true
          git -C "$REPO_PATH" push origin main 2>/dev/null || echo "  WARN: parent push failed (local commit preserved)"
        fi
      else
        echo "  SKIP: parent repo dirty, not updating submodule pointer"
      fi
    else
      echo "  SKIP: parent repo on $PARENT_BRANCH (not main), not updating pointer"
    fi
  fi
done
```

## 记录结果

```bash
SUMMARY="submodule-commit: $TOTAL_COMMITTED submodule(s) committed, $TOTAL_PUSHED pushed, $TOTAL_PARENT_UPDATED parent pointer(s) updated"
echo ""
echo "=== Submodule Commit Summary ==="
echo "$SUMMARY"

RESULT="success"
[ -n "$ERRORS" ] && RESULT="warning"

bd create "$SUMMARY" -t chore --ephemeral \
  -l "type:plugin-run,plugin:submodule-commit,result:$RESULT" \
  -d "$SUMMARY" --silent 2>/dev/null || true
```