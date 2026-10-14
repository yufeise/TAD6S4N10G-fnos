#!/bin/bash
# 部署后验证: 进程 / socket / energy API / status API / 日志尾部
SUDO="sudo"
echo "=== process ==="
pgrep -af "bin/psu-monitor serve" || echo "NOT_RUNNING"
echo "=== sock ==="
ls -la "/usr/local/apps/@appcenter/psu-monitor/psu-monitor.sock"
echo "=== energy api ==="
$SUDO curl -s --max-time 10 --unix-socket "/usr/local/apps/@appcenter/psu-monitor/psu-monitor.sock" http://localhost/api/energy
echo
echo "=== status api (head) ==="
$SUDO curl -s --max-time 20 --unix-socket "/usr/local/apps/@appcenter/psu-monitor/psu-monitor.sock" http://localhost/api/status | head -c 500
echo
echo "=== app log tail ==="
tail -8 /tmp/psu-monitor.log 2>/dev/null || echo "(no /tmp/psu-monitor.log)"
