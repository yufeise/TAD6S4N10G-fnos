#!/bin/bash
# 1.0.48 部署验证: 版本号 + 风扇钳制(pct<10 -> 10) + 回归
APP=/usr/local/apps/@appcenter/psu-monitor
SOCK="$APP/psu-monitor.sock"
C="sudo curl -s --max-time 15 --unix-socket $SOCK"

echo "=== wait for process ==="
for i in $(seq 1 20); do
  if pgrep -f "bin/psu-monitor serve" > /dev/null; then
    echo "RUNNING (waited ${i}x5s)"
    break
  fi
  sleep 5
done
pgrep -af "bin/psu-monitor serve" || { echo "STILL_DOWN"; exit 1; }
sleep 2

echo "=== version+status ==="
$C http://localhost/api/status | head -c 300
echo
echo "=== fan clamp test: pct=5 should become 10 ==="
$C -X POST -H 'Content-Type: application/json' -d '{"mode":"manual","pct":5}' http://localhost/api/fan
echo
echo "=== fan normal: pct=10 ==="
$C -X POST -H 'Content-Type: application/json' -d '{"mode":"manual","pct":10}' http://localhost/api/fan
echo
echo "=== fan restore auto ==="
$C -X POST -H 'Content-Type: application/json' -d '{"mode":"auto"}' http://localhost/api/fan
echo
echo "=== energy (回归) ==="
$C http://localhost/api/energy | head -c 220
echo
echo "=== config (回归) ==="
$C http://localhost/api/config | head -c 200
echo
echo "=== ui files md5 ==="
md5sum "$APP/ui/static/app.js" "$APP/ui/static/index.html" "$APP/ui/static/styles.css"
echo "=== cleanup ==="
rm -rf /tmp/deploy
echo done
