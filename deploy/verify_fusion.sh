#!/bin/bash
# 验证融合版 tad-module 的 CRPS 板块
SOCK=/usr/local/apps/@appcenter/tad-module/tad-module.sock
echo "---VERSION---"; grep -E '^version|^changelog' /usr/local/apps/@appcenter/tad-module/manifest | head -2
echo "---CRPS-STATUS---"; curl -s --max-time 8 --unix-socket "$SOCK" http://localhost/api/crps/status | head -c 400; echo
echo "---CRPS-ENERGY---"; curl -s --max-time 8 --unix-socket "$SOCK" http://localhost/api/crps/energy | head -c 250; echo
echo "---CRPS-FAN-CLAMP---"; curl -s --max-time 10 --unix-socket "$SOCK" -X POST -d '{"mode":"manual","pct":5}' http://localhost/api/crps/fan; echo
echo "---CRPS-FAN-AUTO---"; curl -s --max-time 10 --unix-socket "$SOCK" -X POST -d '{"mode":"auto"}' http://localhost/api/crps/fan; echo
echo "---CRPS-UI---"; curl -s -o /dev/null -w '%{http_code}' --max-time 5 --unix-socket "$SOCK" http://localhost/crps/index.html; echo
echo "---CRPS-JS---"; curl -s -o /dev/null -w '%{http_code}' --max-time 5 --unix-socket "$SOCK" http://localhost/crps/app.js; echo
echo "---MAIN-UI---"; curl -s -o /dev/null -w '%{http_code}' --max-time 5 --unix-socket "$SOCK" http://localhost/index.html; echo
echo "---CONF-DIR---"; ls /usr/local/apps/@appconf/tad-module/ 2>/dev/null
echo "---CRPS-LOG---"; tail -3 /tmp/psu-monitor.log 2>/dev/null
