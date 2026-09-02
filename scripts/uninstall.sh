#!/bin/sh
# Uninstall script for AdGuard Home on OPNsense / FreeBSD

echo "stopping AdGuard Home service..."
service AdGuardHome stop 2>/dev/null || true

# Match only the actual AdGuardHome binary's full path. A bare "daemon"
# pattern would also kill every other service on this box that happens to
# run under /usr/sbin/daemon (a generic wrapper OPNsense uses widely), since
# pkill on FreeBSD treats multiple patterns as alternatives, not as a
# single AND'd match.
pkill -9 -f "/usr/local/bin/AdGuardHome" 2>/dev/null || true

echo "removing service binaries and configuration files..."
rm -rf /usr/local/bin/AdGuardHome \
       /usr/local/bin/AdGuardHome.yaml \
       /usr/local/bin/data \
       /usr/local/AdGuardHome \
       /var/db/adguardhome \
       /var/run/AdGuardHome* \
       /etc/AdGuardHome* \
       /usr/local/etc/AdGuardHome* \
       /usr/local/etc/rc.d/AdGuardHome \
       /etc/rc.conf.d/adguardhome \
       /usr/local/etc/rc.syshook.d/start/99-adguardhome* \
       /usr/local/opnsense/service/conf/actions.d/actions_adguardhome.conf \
       /usr/local/etc/inc/plugins.inc.d/adguardhome.inc

echo "disabling AdGuardHome service in rc.conf..."
sysrc -x AdGuardHome_enable 2>/dev/null || true
sysrc -x adguardhome_enable 2>/dev/null || true

echo "AdGuard Home has been completely removed from this system."
