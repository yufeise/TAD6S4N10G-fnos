#!/bin/bash
set -e
APP=/usr/local/apps/@appcenter/tad-module
echo Sseasea123 | sudo -S bash -c "
  cp -a $APP/bin/tad-module $APP/bin/tad-module.bak-1.0.48
  install -m755 /tmp/deploy/tad-module $APP/bin/tad-module
  install -m644 /tmp/deploy/crps-app.js $APP/ui/static/crps/app.js
  pkill -f 'tad-module' || true
  systemctl restart trim_app_center
" 2>/dev/null
echo INSTALLED
