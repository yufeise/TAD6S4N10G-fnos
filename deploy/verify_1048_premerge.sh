#!/bin/bash
# v1.0.48 premerge 部署后验证: 版本/状态/耗电/风扇钳制(5->10)/恢复自动
SOCK=/usr/local/apps/@appcenter/psu-monitor/psu-monitor.sock
C="curl -s --max-time 8 --unix-socket $SOCK"
echo "=== /api/status ==="
$C http://localhost/api/status | head -c 400; echo
echo "=== /api/energy ==="
$C http://localhost/api/energy | head -c 300; echo
echo "=== POST /api/fan pct=5 (期望钳到 10) ==="
$C -X POST -H 'Content-Type: application/json' -d '{"mode":"manual","pct":5}' http://localhost/api/fan; echo
echo "=== 恢复自动 ==="
$C -X POST -H 'Content-Type: application/json' -d '{"mode":"auto"}' http://localhost/api/fan; echo
echo "=== md5 对照 ==="
md5sum /usr/local/apps/@appcenter/psu-monitor/bin/psu-monitor 2>/dev/null || echo "Sseasea123" | sudo -S md5sum /usr/local/apps/@appcenter/psu-monitor/bin/psu-monitor
