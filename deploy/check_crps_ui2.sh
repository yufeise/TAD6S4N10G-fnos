#!/bin/bash
SOCK=/usr/local/apps/@appcenter/tad-module/tad-module.sock
echo "---RAW /crps/index.html---"
curl -si --unix-socket "$SOCK" http://localhost/crps/index.html | head -12
echo "---RAW /crps/---"
curl -si --unix-socket "$SOCK" http://localhost/crps/ | head -8
