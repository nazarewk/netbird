#!/usr/bin/env sh
# put at /root/netbird-configure.sh
set -euo pipefail
set -x

svc="/etc/init.d/netbird"
log="/var/log/netbird/client.log"

ln -sf /root/netbird /usr/bin/netbird
cp /root/config.json /etc/netbird/config.json
if test -e "${svc}" ; then
  "${svc}" stop || :
fi
if test -e "${log}" ; then
  rm "${log}"
fi
if test -e "${svc}" ; then
  netbird service uninstall
fi
netbird service install

"${svc}" start

tail -F "${log}"
