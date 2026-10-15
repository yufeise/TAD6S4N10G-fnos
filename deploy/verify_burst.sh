#!/bin/bash
echo Sseasea123 | sudo -S bash -c '
  SOCK=/usr/local/apps/@appcenter/tad-module/tad-module.sock
  pgrep -x tad-module >/dev/null && echo PROC-UP || echo PROC-DOWN
  echo "--- 连续 5 次快速切换, 逐次计时 ---"
  for m in auto manual auto manual auto; do
    if [ $m = manual ]; then D="{\"mode\":\"manual\",\"pct\":40}"; else D="{\"mode\":\"auto\"}"; fi
    T=$(curl -s --unix-socket $SOCK -o /dev/null -w "%{time_total}" -X POST -H "Content-Type: application/json" -d "$D" http://localhost/api/crps/fan)
    echo "$m: ${T}s"
  done
  curl -s --unix-socket $SOCK http://localhost/api/crps/status | grep -o "\"fan_mode\":\"[a-z]*\""
' 2>/dev/null
