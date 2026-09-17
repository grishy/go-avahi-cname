FROM ubuntu:24.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    avahi-daemon dbus tcpdump && rm -rf /var/lib/apt/lists/*

COPY run.sh /run-test.sh
ENTRYPOINT ["sh", "/run-test.sh"]
