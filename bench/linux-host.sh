#!/bin/bash
# Prepares an Ubuntu desktop host for a long benchmark run and undoes it:
#   sudo bench/linux-host.sh apply
#   sudo bench/linux-host.sh restore
# apply waits for running apt upgrades, stops background services and
# timers, sets the CPU to performance, disables Docker's userland proxy (so
# the host load generator reaches published ports through iptables, not a
# proxy process), widens ephemeral ports (kept above the bench host ports,
# 25680-28081, so a client socket never takes a port Docker must bind) and
# disables GNOME idle suspend.
set -eu

[ "$(id -u)" = 0 ] || { echo "run with sudo" >&2; exit 1; }
user=${SUDO_USER:-}

services="unattended-upgrades packagekit fwupd cups cups-browsed avahi-daemon
  ModemManager kerneloops smartmontools anacron cron snapd"
sockets="snapd.socket cups.socket avahi-daemon.socket"
timers="apt-daily.timer apt-daily-upgrade.timer fwupd-refresh.timer
  motd-news.timer man-db.timer anacron.timer snapd.snap-repair.timer"
daemon_json=/etc/docker/daemon.json

governor() {
  for f in /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor; do echo "$1" > "$f"; done
  for f in /sys/devices/system/cpu/cpu*/cpufreq/energy_performance_preference; do echo "$2" > "$f" 2>/dev/null || true; done
}

as_user() {
  [ -n "$user" ] || return 0
  sudo -u "$user" DBUS_SESSION_BUS_ADDRESS="unix:path=/run/user/$(id -u "$user")/bus" "$@" || true
}

case "${1:-}" in
  apply)
    # Never kill apt mid-upgrade: wait for it to finish.
    while pgrep -f '/usr/bin/unattended-upgrade$' >/dev/null || fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1; do
      echo "waiting for unattended-upgrade/apt to finish..."; sleep 10
    done
    systemctl stop $timers $sockets $services 2>/dev/null || true
    powerprofilesctl set performance 2>/dev/null || true
    governor performance performance
    sysctl -q -w net.ipv4.ip_local_port_range="30000 65535" net.ipv4.tcp_tw_reuse=1
    if [ ! -f "$daemon_json" ]; then
      echo '{"userland-proxy": false}' > "$daemon_json"
      systemctl restart docker
    fi
    as_user gsettings set org.gnome.settings-daemon.plugins.power sleep-inactive-ac-type nothing
    pkill -x snap-store || true
    sync; echo 3 > /proc/sys/vm/drop_caches
    echo "applied: governor=$(cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor)" \
      "docker=$(cat "$daemon_json")"
    ;;
  restore)
    sysctl -q -w net.ipv4.ip_local_port_range="32768 60999" net.ipv4.tcp_tw_reuse=2
    powerprofilesctl set balanced 2>/dev/null || true
    governor powersave balance_performance
    systemctl start $sockets $services $timers 2>/dev/null || true
    as_user gsettings set org.gnome.settings-daemon.plugins.power sleep-inactive-ac-type suspend
    echo "restored (docker userland-proxy stays disabled; remove $daemon_json and restart docker to undo)"
    ;;
  *) echo "usage: sudo $0 apply|restore" >&2; exit 2 ;;
esac
