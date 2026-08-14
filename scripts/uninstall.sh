#!/bin/sh
# Uninstall script for AdGuard Home on OPNsense / FreeBSD

echo "stopping AdGuard Home service..."
service AdGuardHome stop 2>/dev/null || true
pkill -9 AdGuardHome daemon 2>/dev/null || true

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

echo "disabling adguardhome service in rc.conf..."
sysrc -x adguardhome_enable 2>/dev/null || true

echo "AdGuard Home has been completely removed from this system."
