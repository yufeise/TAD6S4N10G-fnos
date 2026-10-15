#!/bin/bash
echo Sseasea123 | sudo -S bash -c '
  SOCK=/usr/local/apps/@appcenter/tad-module/tad-module.sock
  echo "--- set manual 20 ---"
  curl -s --unix-socket $SOCK -X POST -H "Content-Type: application/json" -d "{\"mode\":\"manual\",\"pct\":20}" http://localhost/api/crps/fan > /dev/null
  sleep 3
  curl -s --unix-socket $SOCK http://localhost/api/crps/status | grep -o "\"fan_mode\":\"[a-z]*\""
  echo "--- set auto ---"
  curl -s --unix-socket $SOCK -X POST -H "Content-Type: application/json" -d "{\"mode\":\"auto\"}" http://localhost/api/crps/fan > /dev/null
  sleep 3
  curl -s --unix-socket $SOCK http://localhost/api/crps/status | grep -o "\"fan_mode\":\"[a-z]*\""
' 2>/dev/null
