#!/bin/bash
echo Sseasea123 | sudo -S bash -c '
  SOCK=/usr/local/apps/@appcenter/tad-module/tad-module.sock
  pgrep -x tad-module >/dev/null && echo PROC-UP || echo PROC-DOWN
  echo "--- auto 切换耗时 ---"
  curl -s --unix-socket $SOCK -o /dev/null -w "%{time_total}s\n" -X POST -H "Content-Type: application/json" -d "{\"mode\":\"auto\"}" http://localhost/api/crps/fan
  sleep 2
  curl -s --unix-socket $SOCK http://localhost/api/crps/status | grep -o "\"fan_mode\":\"[a-z]*\""
  echo "--- manual 30 切换耗时 ---"
  curl -s --unix-socket $SOCK -o /dev/null -w "%{time_total}s\n" -X POST -H "Content-Type: application/json" -d "{\"mode\":\"manual\",\"pct\":30}" http://localhost/api/crps/fan
  sleep 2
  curl -s --unix-socket $SOCK http://localhost/api/crps/status | grep -o "\"fan_mode\":\"[a-z]*\""
  curl -s --unix-socket $SOCK -X POST -H "Content-Type: application/json" -d "{\"mode\":\"auto\"}" http://localhost/api/crps/fan > /dev/null
' 2>/dev/null
