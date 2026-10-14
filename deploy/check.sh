#!/bin/bash
# 部署完整性校验: md5 对比 + app.js 语法检查 (用 NAS 上的 node)
APP=/usr/local/apps/@appcenter/psu-monitor
echo "=== md5 deployed ui ==="
md5sum "$APP/ui/static/app.js" "$APP/ui/static/index.html" "$APP/ui/static/styles.css"
echo "=== md5 staged ==="
md5sum /tmp/deploy/ui/app.js /tmp/deploy/ui/index.html /tmp/deploy/ui/styles.css
echo "=== binary md5 ==="
md5sum "$APP/bin/psu-monitor" /tmp/deploy/psu-monitor
echo "=== node check app.js ==="
NODE=$(ls /usr/local/apps/nodejs_v*/bin/node 2>/dev/null | sort | tail -1)
if [ -z "$NODE" ]; then NODE=$(which node 2>/dev/null); fi
if [ -n "$NODE" ]; then
  echo "node: $NODE"
  "$NODE" --check "$APP/ui/static/app.js" && echo "JS_SYNTAX_OK" || echo "JS_SYNTAX_FAIL"
else
  echo "node not found, skip"
fi
echo "=== energy api again (累计应增长) ==="
$SUDO="sudo"
sudo curl -s --max-time 10 --unix-socket "$APP/psu-monitor.sock" http://localhost/api/energy
echo
