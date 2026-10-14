#!/bin/bash
# psu-monitor 1.0.47 部署: 备份 1.0.46 -> 安装新二进制+UI -> 重启 -> 验证
set -e
APP=/usr/local/apps/@appcenter/psu-monitor

echo "=== backup old binary ==="
$SUDO cp -f "$APP/bin/psu-monitor" "$APP/bin/psu-monitor.bak-1.0.46" 2>/dev/null || true

echo "=== install new binary ==="
sudo install -m755 /tmp/deploy/psu-monitor "$APP/bin/psu-monitor"

echo "=== install ui ==="
sudo cp -f /tmp/deploy/ui/app.js /tmp/deploy/ui/index.html /tmp/deploy/ui/styles.css "$APP/ui/static/"
sudo chown root:root "$APP/bin/psu-monitor" "$APP/ui/static/app.js" "$APP/ui/static/index.html" "$APP/ui/static/styles.css"

echo "=== restart ==="
sudo pkill -f "bin/psu-monitor serve" || true
sleep 3
if ! pgrep -f "bin/psu-monitor serve" > /dev/null; then
  echo "NOT_AUTO_RESTARTED: restarting trim_app_center"
  sudo systemctl restart trim_app_center.service || true
  sleep 8
fi
pgrep -af "bin/psu-monitor serve" || echo "STILL_DOWN"
echo "=== installed ==="
md5sum "$APP/bin/psu-monitor" /tmp/deploy/psu-monitor
