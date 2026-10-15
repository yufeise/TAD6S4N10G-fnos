#!/bin/bash
APP=/usr/local/apps/@appcenter/tad-module
echo Sseasea123 | sudo -S bash -c "
  install -m644 /tmp/deploy/app.js $APP/ui/static/crps/app.js
  cp -a $APP/bin/tad-module $APP/bin/tad-module.bak-0.10.24 2>/dev/null || true
  install -m755 /tmp/deploy/tad-module-0.10.25 $APP/bin/tad-module
  pkill -x tad-module 2>/dev/null
  systemctl restart trim_app_center
  echo RESTARTED
" 2>/dev/null
