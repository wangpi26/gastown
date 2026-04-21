+++
name = "dolt-log-rotate"
description = "当 Dolt 服务器日志文件超过大小阈值时进行轮转"
version = 1

[gate]
type = "cooldown"
duration = "6h"

[tracking]
labels = ["plugin:dolt-log-rotate", "category:maintenance"]
digest = true

[execution]
timeout = "2m"
notify_on_failure = true
severity = "medium"
+++

# Dolt Log Rotate

Dolt 服务器将 stdout/stderr 写入 `daemon/dolt.log`。此文件可能
增长到数 GB，导致磁盘压力或拖慢 `gt dolt logs`。

此插件每 6 小时检查一次日志大小，超过 100MB 时进行轮转
（可通过 `GT_DOLT_LOG_MAX_MB` 配置）。保留 3 份压缩后的
轮转副本。

轮转在 Dolt 运行期间是安全的——服务器持有打开的文件描述符，
因此重命名日志文件并创建新文件可以工作（Unix fd 语义）。
不过，新的日志输出仍会写入旧的 fd，直到 Dolt 重启。
为了将输出重定向到新文件，插件会发送 SIGHUP（如支持），
或注明完整轮转将在下次 Dolt 重启时完成。