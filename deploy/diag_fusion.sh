#!/bin/bash
APP=/usr/local/apps/@appcenter/tad-module
echo "---BIN-MD5---"; md5sum $APP/bin/tad-module
echo "---BIN-CRPS-STRING---"; grep -c 'api/crps' $APP/bin/tad-module 2>/dev/null || echo 0
echo "---UI-CRPS-DIR---"; ls $APP/ui/static/crps/ 2>/dev/null | head -5
echo "---MAINLOG-CRPS---"; grep -i crps /usr/local/apps/@appdata/tad-module/tad-module.log 2>/dev/null | tail -5
echo "---PSU-LOG-HEAD---"; head -5 /tmp/psu-monitor.log 2>/dev/null
echo "---PROC-START---"; ps -o lstart= -p $(pgrep -x tad-module | head -1) 2>/dev/null
