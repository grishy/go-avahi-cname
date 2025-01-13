---
name: Bug report
about: Create a report to help us improve
title: ''
labels: ''
assignees: grishy

---

**Describe the bug**
A clear and concise description of what the bug is.

**To Reproduce**
Steps to reproduce the behavior:
1. Start go-avahi-cname as '...'
2. Send ping from '...'
3. ...

**Server (with go-avahi-cname):**
 - OS and version:
 - go-avahi-cname version:
 - avahi version:

**Client (from where you send request):**
 - OS and version:
 - How mDNS configured: 

**Network:**
Describe topology of network and how Server and Client communicate.

**go-avahi-cname log:**
Start with `--debug` option and attach logs to the issue:
(attach here)

**Network dump:**
Like: `sudo tcpdump -n host 224.0.0.251 and port 5353 -w /tmp/dump.pcap`
(attach here `/tmp/dump.pcap`)
