#!/bin/bash
# 1.0.47 部署验证: 保留功能正常 + 被删端点 404
APP=/usr/local/apps/@appcenter/psu-monitor
SOCK="$APP/psu-monitor.sock"
C="sudo curl -s --max-time 15 --unix-socket $SOCK"
echo "=== process ==="
pgrep -af "bin/psu-monitor serve" || echo "NOT_RUNNING"
echo "=== version+status (保留) ==="
$C http://localhost/api/status | head -c 300
echo
echo "=== energy (保留) ==="
$C http://localhost/api/energy | head -c 220
echo
echo "=== config (保留) ==="
$C http://localhost/api/config | head -c 200
echo
echo "=== removed endpoints should be 404 ==="
echo -n "curve:    "; $C http://localhost/api/curve
echo
echo -n "fan-test: "; $C http://localhost/api/fan-test
echo
echo -n "temp-test:"; $C http://localhost/api/temp-test
echo
echo "=== ui files md5 ==="
md5sum "$APP/ui/static/app.js" "$APP/ui/static/index.html" "$APP/ui/static/styles.css"
echo "=== cleanup ==="
rm -rf /tmp/deploy
echo done
