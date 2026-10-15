#!/bin/bash
SOCK=/usr/local/apps/@appcenter/tad-module/tad-module.sock
echo Sseasea123 | sudo -S bash -c "
  T0=\$(date +%s%N)
  S1=\$(curl -s --unix-socket \$SOCK http://localhost/api/crps/status)
  T1=\$(date +%s%N)
  curl -s --unix-socket \$SOCK http://localhost/api/crps/status >/dev/null
  T2=\$(date +%s%N)
  S3=\$(curl -s --unix-socket \$SOCK http://localhost/api/crps/status)
  T3=\$(date +%s%N)
  echo \"first=\$(( (T1-T0)/1000000 ))ms second=\$(( (T3-T2)/1000000 ))ms\"
  echo \"\$S1\" | head -c 200; echo
  echo \"\$S3\" | grep -o '1\.0\.4[0-9]'
  echo \"\$S3\" | grep -o '\"ok\":true'
  curl -s --unix-socket \$SOCK http://localhost/api/crps/energy | head -c 160; echo
  ls /usr/local/apps/@appcenter/tad-module/bin/ | grep bak
" 2>/dev/null
