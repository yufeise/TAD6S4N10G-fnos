#!/bin/bash
# psu-monitor 1.0.46 部署脚本: 备份旧二进制 -> 安装新二进制+UI -> 重启进程
set -e
APP=/usr/local/apps/@appcenter/psu-monitor
SUDO="sudo"

echo "=== backup old binary ==="
$SUDO cp -f "$APP/bin/psu-monitor" "$APP/bin/psu-monitor.bak-1.0.45"

echo "=== install new binary ==="
$SUDO install -m755 /tmp/deploy/psu-monitor "$APP/bin/psu-monitor"

echo "=== install ui ==="
$SUDO cp -f /tmp/deploy/ui/app.js "$APP/ui/static/app.js"
$SUDO cp -f /tmp/deploy/ui/index.html "$APP/ui/static/index.html"
$SUDO cp -f /tmp/deploy/ui/styles.css "$APP/ui/static/styles.css"
$SUDO chown root:root "$APP/bin/psu-monitor" "$APP/ui/static/app.js" "$APP/ui/static/index.html" "$APP/ui/static/styles.css"

echo "=== installed ==="
ls -la "$APP/bin" "$APP/ui/static"

echo "=== restart process ==="
$SUDO pkill -f "bin/psu-monitor serve" || true
sleep 6
if pgrep -f "bin/psu-monitor serve" > /dev/null; then
  echo "RESTARTED_BY_SUPERVISOR"
else
  echo "NOT_RESTARTED: trying trim_app_center restart"
  $SUDO systemctl restart trim_app_center.service || true
  sleep 6
  if pgrep -f "bin/psu-monitor serve" > /dev/null; then
    echo "RESTARTED_BY_TRIM"
  else
    echo "STILL_DOWN"
  fi
fi
pgrep -af "bin/psu-monitor serve" || true
echo "=== done ==="
