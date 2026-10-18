#!/bin/sh
# 部署 mfr 修复版 tad-module: 备份 -> 替换 -> 重启
set -e
APP=/usr/local/apps/@appcenter/tad-module
echo Sseasea123 | sudo -S sh -c "cp $APP/bin/tad-module $APP/bin/tad-module.bak-mfrfix 2>/dev/null || true"
echo Sseasea123 | sudo -S install -m755 /tmp/deploy/tad-module-mfrfix $APP/bin/tad-module
echo Sseasea123 | sudo -S pkill -x tad-module || true
sleep 2
echo Sseasea123 | sudo -S systemctl restart trim_app_center
echo INSTALL-OK
