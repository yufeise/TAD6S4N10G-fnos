#!/bin/bash
SOCK=/usr/local/apps/@appcenter/tad-module/tad-module.sock
echo "---LINE-104-LOG-MASK---"
curl -sL --unix-socket "$SOCK" http://localhost/crps/index.html | grep -n 'log-mask\|dl-log\|<form'
echo "---GPIO-AREA---"
curl -sL --unix-socket "$SOCK" http://localhost/crps/index.html | sed -n '84,96p'
