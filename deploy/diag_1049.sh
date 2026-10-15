#!/bin/bash
SOCK=/usr/local/apps/@appcenter/tad-module/tad-module.sock
echo Sseasea123 | sudo -S bash -c "
  curl -s --unix-socket \$SOCK -o /tmp/st.json -w 'code=%{http_code} size=%{size_download}\n' http://localhost/api/crps/status
  head -c 260 /tmp/st.json; echo
  echo ---crps-root---
  curl -s --unix-socket \$SOCK -o /dev/null -w 'code=%{http_code}\n' http://localhost/api/crps/
  pgrep -a -x tad-module | head -2
  tail -5 /tmp/psu-monitor.log
" 2>/dev/null
