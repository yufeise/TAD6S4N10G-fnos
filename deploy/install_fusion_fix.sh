#!/bin/bash
set -e
APP=/usr/local/apps/@appcenter/tad-module
# 备份现网二进制并替换为修复版
cp -a $APP/bin/tad-module $APP/bin/tad-module.bak-0.10.21
install -m755 /tmp/deploy/tad-fusion-fix $APP/bin/tad-module
# 重启宿主服务: pkill 后由 trim_app_center 拉起
pkill -x tad-module || true
echo Sseasea123 | sudo -S systemctl restart trim_app_center 2>/dev/null || sudo systemctl restart trim_app_center
echo "replaced & restart requested"
