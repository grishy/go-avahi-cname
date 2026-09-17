#!/bin/sh
set -eu

# Isolated container only: no host network, D-Bus socket, or writable repo mount.
mkdir -p /run/dbus /results
dbus-daemon --system --fork
avahi-daemon --no-drop-root --debug > /results/avahi.log 2>&1 &
avahi_pid=$!

cleanup() {
    kill "$avahi_pid" "${monitor_pid:-}" "${capture_pid:-}" 2>/dev/null || true
    wait || true
}
trap cleanup EXIT

attempt=0
until dbus-send --system --print-reply --dest=org.freedesktop.Avahi / \
    org.freedesktop.Avahi.Server.GetState 2>/dev/null | grep -q 'int32 2'; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 10 ]; then
        cat /results/avahi.log
        exit 1
    fi
    sleep 1
done

dbus-monitor --system > /results/dbus.log 2>&1 &
monitor_pid=$!
tcpdump -U -i any -w /results/mdns.pcap udp port 5353 > /results/tcpdump.log 2>&1 &
capture_pid=$!
sleep 1

export AVAHI_TEST_ADDRESS="$(hostname -i):5353"
status=0
/publisher.test -test.v -test.timeout=30s > /results/test.log 2>&1 || status=$?
cat /results/test.log
exit "$status"
