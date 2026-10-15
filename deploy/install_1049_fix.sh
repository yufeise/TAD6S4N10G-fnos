#!/bin/bash
APP=/usr/local/apps/@appcenter/tad-module
echo Sseasea123 | sudo -S bash -c "
  ls -la $APP/bin/ | head -6
  md5sum $APP/bin/tad-module /tmp/deploy/tad-module 2>/dev/null
  pgrep -a -x tad-module || echo 'NO-PROCESS'
  install -m644 /tmp/deploy/crps-app.js $APP/ui/static/crps/app.js 2>/dev/null || echo 'UI-SKIP'
  P=\$APP/bin/tad-module
  pkill -x tad-module 2>/dev/null
  systemctl restart trim_app_center
  echo RESTARTED
" 2>/dev/null
