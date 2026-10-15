#!/bin/bash
SOCK=/usr/local/apps/@appcenter/tad-module/tad-module.sock
echo Sseasea123 | sudo -S bash -c "
  for i in 1 2 3; do
    curl -s --unix-socket \$SOCK -o /dev/null -w \"status: %{time_total}s\n\" http://localhost/api/crps/status
  done
  curl -s --unix-socket \$SOCK -o /dev/null -w 'energy: %{time_total}s\n' http://localhost/api/crps/energy
  curl -s --unix-socket \$SOCK -o /dev/null -w 'ui: %{time_total}s\n' http://localhost/crps/
" 2>/dev/null
