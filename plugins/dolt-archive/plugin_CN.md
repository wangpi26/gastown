+++
name = "dolt-archive"
description = "异地备份：JSONL 快照到 git，dolt push 到 GitHub/DoltHub"
version = 1

[gate]
type = "cooldown"
duration = "1h"

[tracking]
labels = ["plugin:dolt-archive", "category:data-safety"]
digest = true

[execution]
timeout = "15m"
notify_on_failure = true
severity = "critical"
+++

# Dolt Archive

将生产数据移出本机。三层保护：

1. **JSONL 导出** — 人类可读的快照（在 Clown Show #13 中救了我们）
2. **Git push** — JSONL 文件提交并推送到 GitHub
3. **Dolt push** — 原生 Dolt 复制到 GitHub/DoltHub（如已配置）

JSONL 是最后一道恢复层。无论其他层是否正常工作，始终维护它。

## 配置

```bash
DOLT_DATA_DIR="$GT_TOWN_ROOT/.dolt-data"
PROD_DBS=("hq" "gt" "mo")
JSONL_EXPORT_DIR="$GT_TOWN_ROOT/.dolt-archive/jsonl"
DOLT_HOST="127.0.0.1"
DOLT_PORT=3307
DOLT_USER="root"
```

## 步骤 1：JSONL 导出

将每个生产数据库的所有 issue 导出为 JSONL 文件。这些文件
人类可读、可 diff，且能在任何存储后端故障后存活。

```bash
echo "=== JSONL Export ==="
EXPORTED=0
EXPORT_FAILED=0

mkdir -p "$JSONL_EXPORT_DIR"

for DB in "${PROD_DBS[@]}"; do
  EXPORT_FILE="$JSONL_EXPORT_DIR/${DB}-$(date +%Y%m%d-%H%M).jsonl"
  LATEST_LINK="$JSONL_EXPORT_DIR/${DB}-latest.jsonl"

  echo "Exporting $DB..."

  # 如果 bd export 可用则使用，否则直接查询
  if bd export --db "$DB" --format jsonl > "$EXPORT_FILE" 2>/dev/null; then
    LINE_COUNT=$(wc -l < "$EXPORT_FILE" | tr -d ' ')
    FILE_SIZE=$(du -h "$EXPORT_FILE" | cut -f1)
    echo "  $DB: $LINE_COUNT issues exported ($FILE_SIZE)"

    # 更新最新符号链接
    ln -sf "$EXPORT_FILE" "$LATEST_LINK"
    EXPORTED=$((EXPORTED + 1))
  else
    # 回退：直接查询 Dolt 获取 issue 数据
    dolt sql -q "SELECT * FROM issues ORDER BY id" \
      --host "$DOLT_HOST" --port "$DOLT_PORT" -u "$DOLT_USER" \
      -d "$DB" --no-auto-commit --result-format json \
      > "$EXPORT_FILE" 2>/dev/null

    if [ $? -eq 0 ] && [ -s "$EXPORT_FILE" ]; then
      LINE_COUNT=$(wc -l < "$EXPORT_FILE" | tr -d ' ')
      echo "  $DB: exported via SQL ($LINE_COUNT lines)"
      ln -sf "$EXPORT_FILE" "$LATEST_LINK"
      EXPORTED=$((EXPORTED + 1))
    else
      echo "  WARN: $DB export failed"
      rm -f "$EXPORT_FILE"
      EXPORT_FAILED=$((EXPORT_FAILED + 1))
    fi
  fi
done

# 清理旧导出（每个数据库保留最近 24 个快照）
for DB in "${PROD_DBS[@]}"; do
  SNAPSHOTS=$(ls -t "$JSONL_EXPORT_DIR/${DB}-2"*.jsonl 2>/dev/null | tail -n +25)
  if [ -n "$SNAPSHOTS" ]; then
    echo "$SNAPSHOTS" | xargs rm -f
    echo "Pruned old $DB snapshots"
  fi
done

echo "Exported: $EXPORTED, failed: $EXPORT_FAILED"
```

## 步骤 2：Git 提交并推送

将 JSONL 快照提交到备份分支并推送到 GitHub。

```bash
echo "=== Git Push ==="
GIT_PUSHED=false

# 检查是否配置了 git 备份仓库
BACKUP_REPO="$HOME/gt/.dolt-archive/git"

if [ -d "$BACKUP_REPO/.git" ]; then
  cd "$BACKUP_REPO"

  # 复制最新的 JSONL 文件
  for DB in "${PROD_DBS[@]}"; do
    LATEST="$JSONL_EXPORT_DIR/${DB}-latest.jsonl"
    if [ -f "$LATEST" ]; then
      cp "$(readlink "$LATEST" || echo "$LATEST")" "$BACKUP_REPO/${DB}.jsonl"
    fi
  done

  # 检查是否有变更
  if git diff --quiet && git diff --staged --quiet; then
    echo "No changes to commit"
  else
    git add *.jsonl
    git commit -m "Archive snapshot $(date +%Y-%m-%d-%H%M)" \
      --author="Gas Town Archive <archive@gastown.local>" 2>/dev/null

    # 推送前检查远程仓库是否存在
    if git remote get-url origin > /dev/null 2>&1; then
      if git push origin main 2>/dev/null; then
        GIT_PUSHED=true
        echo "Pushed to GitHub"
      else
        echo "WARN: Git push to remote failed (check GitHub credentials/permissions)"
      fi
    else
      echo "WARN: No git remote configured for backup repo"
      echo "  To set up: cd $BACKUP_REPO && git remote add origin <github-url>"
    fi
  fi
else
  echo "No git backup repo at $BACKUP_REPO — skipping git push"
  echo "  To set up: git init $BACKUP_REPO && cd $BACKUP_REPO && git remote add origin <url>"
fi
```

## 步骤 3：Dolt 原生推送

通过 `dolt push` 将生产数据库推送到 GitHub/DoltHub 远程仓库。

```bash
echo "=== Dolt Push ==="
DOLT_PUSHED=0
DOLT_PUSH_FAILED=0

for DB in "${PROD_DBS[@]}"; do
  DB_DIR="$DOLT_DATA_DIR/$DB"

  if [ ! -d "$DB_DIR/.dolt" ]; then
    echo "  $DB: no .dolt directory, skipping"
    continue
  fi

  # 检查是否配置了远程仓库
  REMOTES=$(cd "$DB_DIR" && dolt remote -v 2>/dev/null | grep -v "^$" | head -5)

  if [ -z "$REMOTES" ]; then
    echo "  $DB: no remotes configured, skipping dolt push"
    continue
  fi

  echo "  $DB: pushing to remotes..."

  # 推送到每个远程仓库
  cd "$DB_DIR"
  for REMOTE_NAME in $(dolt remote -v 2>/dev/null | awk '{print $1}' | sort -u); do
    if timeout 120 dolt push "$REMOTE_NAME" main 2>/dev/null; then
      echo "    $REMOTE_NAME: pushed"
      DOLT_PUSHED=$((DOLT_PUSHED + 1))
    else
      echo "    $REMOTE_NAME: FAILED"
      DOLT_PUSH_FAILED=$((DOLT_PUSH_FAILED + 1))
    fi
  done
done

echo "Dolt push: $DOLT_PUSHED succeeded, $DOLT_PUSH_FAILED failed"
```

## 步骤 4：验证远程数据

验证备份数据已成功到达远程仓库且可访问。

```bash
echo "=== Verification ==="
VERIFY_PASSED=0
VERIFY_FAILED=0

# 验证 git 备份中的 JSONL
if [ -d "$BACKUP_REPO/.git" ]; then
  echo "Verifying git remote..."
  if cd "$BACKUP_REPO" && git ls-remote origin HEAD > /dev/null 2>&1; then
    # 尝试克隆到临时目录以验证
    TEMP_CLONE=$(mktemp -d)
    if git clone --depth 1 origin "$TEMP_CLONE" 2>/dev/null; then
      for DB in "${PROD_DBS[@]}"; do
        if [ -f "$TEMP_CLONE/${DB}.jsonl" ]; then
          REMOTE_COUNT=$(wc -l < "$TEMP_CLONE/${DB}.jsonl" | tr -d ' ')
          echo "  git: $DB verified ($REMOTE_COUNT lines in remote)"
          VERIFY_PASSED=$((VERIFY_PASSED + 1))
        else
          echo "  git: $DB MISSING from remote"
          VERIFY_FAILED=$((VERIFY_FAILED + 1))
        fi
      done
    else
      echo "  git: Clone verification failed"
      VERIFY_FAILED=$((VERIFY_FAILED + 1))
    fi
    rm -rf "$TEMP_CLONE"
  else
    echo "  git: Remote not accessible"
  fi
fi

# 验证 dolt push（检查远程仓库是否包含我们的提交）
for DB in "${PROD_DBS[@]}"; do
  DB_DIR="$DOLT_DATA_DIR/$DB"
  if [ -d "$DB_DIR/.dolt" ]; then
    # 检查是否有可达的 dolt 远程仓库
    REMOTE_HEADS=$(cd "$DB_DIR" && dolt remote -v 2>/dev/null | awk '{print $1}' | sort -u)
    if [ -n "$REMOTE_HEADS" ]; then
      cd "$DB_DIR"
      # 验证至少一个远程仓库有数据
      for REMOTE in $REMOTE_HEADS; do
        if dolt log "$REMOTE/main" -n 1 > /dev/null 2>&1; then
          echo "  dolt: $DB on $REMOTE verified"
          VERIFY_PASSED=$((VERIFY_PASSED + 1))
          break
        fi
      done
    fi
  fi
done

echo "Verified: $VERIFY_PASSED, failed: $VERIFY_FAILED"
```

## 记录结果

```bash
SUMMARY="Archive: jsonl=$EXPORTED/$((EXPORTED + EXPORT_FAILED)), git=${GIT_PUSHED}, dolt_push=$DOLT_PUSHED/$((DOLT_PUSHED + DOLT_PUSH_FAILED)), verify=$VERIFY_PASSED/$((VERIFY_PASSED + VERIFY_FAILED))"
echo "=== $SUMMARY ==="

RESULT="success"
if [ "$EXPORT_FAILED" -gt 0 ] || [ "$DOLT_PUSH_FAILED" -gt 0 ] || [ "$VERIFY_FAILED" -gt 0 ]; then
  RESULT="warning"
fi

bd create "$SUMMARY" -t chore --ephemeral \
  -l type:plugin-run,plugin:dolt-archive,result:$RESULT \
  -d "$SUMMARY" --silent 2>/dev/null || true

if [ "$EXPORT_FAILED" -gt 0 ]; then
  gt escalate "JSONL export failed for $EXPORT_FAILED databases" \
    --severity critical \
    --reason "JSONL is our last-resort recovery layer. $EXPORT_FAILED databases failed to export."
fi
```