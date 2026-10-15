#!/bin/bash
SOCK=/usr/local/apps/@appcenter/tad-module/tad-module.sock
echo "---MAIN-STATUS---"; curl -s --max-time 8 --unix-socket "$SOCK" http://localhost/api/status | head -c 200; echo
echo "---DISABLE-OLD-APP---"
OLD=/usr/local/apps/@appcenter/psu-monitor
if [ -f $OLD/bin/psu-monitor ]; then
  sudo mv $OLD/bin/psu-monitor $OLD/bin/psu-monitor.disabled-1.0.48
  echo "old app binary disabled (prevent GPIO bus conflict with fused crps)"
else
  echo "already disabled"
fi
pgrep -x psu-monitor && pkill -x psu-monitor || echo "old app not running"
