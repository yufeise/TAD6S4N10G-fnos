#!/bin/bash
echo Sseasea123 | sudo -S bash -c '
  SOCK=/usr/local/apps/@appcenter/tad-module/tad-module.sock
  curl -s --unix-socket $SOCK -X POST -H "Content-Type: application/json" -d "{\"mode\":\"manual\",\"pct\":30}" http://localhost/api/crps/fan; echo
  for t in 1 2 3 4 5; do
    sleep 1
    echo -n "t+${t}s: "
    curl -s --unix-socket $SOCK http://localhost/api/crps/status | grep -o "\"fan_mode\":\"[a-z]*\""
  done
  curl -s --unix-socket $SOCK -X POST -H "Content-Type: application/json" -d "{\"mode\":\"auto\"}" http://localhost/api/crps/fan > /dev/null
' 2>/dev/null
