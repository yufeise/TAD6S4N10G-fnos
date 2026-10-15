#!/bin/bash
APP=/usr/local/apps/@appcenter/tad-module
echo Sseasea123 | sudo -S bash -c "
  cp -a $APP/bin/tad-module $APP/bin/tad-module.bak-fastfan 2>/dev/null || true
  install -m755 /tmp/deploy/tad-module-fastfan $APP/bin/tad-module
  pkill -x tad-module 2>/dev/null
  systemctl restart trim_app_center
  echo RESTARTED
" 2>/dev/null
