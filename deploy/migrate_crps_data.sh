#!/bin/bash
set -e
OLD=/usr/local/apps/@appconf/psu-monitor
NEW=/usr/local/apps/@appconf/tad-module
echo "---OLD-FILES---"; ls $OLD/
echo "---OLD-GPIO---"; cat $OLD/gpio.json 2>/dev/null
# 数据迁移: 旧独立应用 -> 融合宿主 (crps.json 对应旧 gpio.json, energy.json 同名)
cp -a $OLD/gpio.json $NEW/crps.json
cp -a $OLD/energy.json $NEW/energy.json
chown root:www-data $NEW/crps.json $NEW/energy.json 2>/dev/null || true
chmod 664 $NEW/crps.json $NEW/energy.json
echo "---MIGRATED---"; ls -la $NEW/
# 重启宿主加载配置
pkill -x tad-module || true
systemctl restart trim_app_center
echo "restart requested"
