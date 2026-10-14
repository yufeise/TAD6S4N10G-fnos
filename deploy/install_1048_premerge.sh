#!/bin/bash
# v1.0.48 (premerge crps-module) 部署: 用合并后板块构建的二进制替换现网独立版
# 备份现网二进制 -> 安装新二进制(保持服务指向的文件名 psu-monitor 不变) -> 重启托管服务
set -e
APP=/usr/local/apps/@appcenter/psu-monitor
PASS=Sseasea123

echo "$PASS" | sudo -S bash -c "
  set -e
  cp -a '$APP/bin/psu-monitor' '$APP/bin/psu-monitor.bak-1.0.48-premerge'
  install -m755 /tmp/deploy/crps-module-1.0.48 '$APP/bin/psu-monitor'
  pkill -x psu-monitor 2>/dev/null || true
  systemctl restart trim_app_center
  echo INSTALLED
"
echo "$PASS" | sudo -S ls -la "$APP/bin/" | grep psu-monitor
