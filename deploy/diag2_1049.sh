#!/bin/bash
echo Sseasea123 | sudo -S bash -c "
  ls -la /usr/local/apps/@appcenter/tad-module/tad-module.sock 2>&1
  curl -sv --unix-socket /usr/local/apps/@appcenter/tad-module/tad-module.sock http://localhost/api/crps/status 2>&1 | grep -E 'Connected|Failed|refused|HTTP/1' | head -4
  curl -s --unix-socket /usr/local/apps/@appcenter/tad-module/tad-module.sock http://localhost/api/crps/status | head -c 240; echo
" 2>/dev/null
