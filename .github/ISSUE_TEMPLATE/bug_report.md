---
name: Bug report
about: Create a report to help us improve
title: ''
labels: ''
assignees: grishy

---

**Describe the Bug**  
A clear and concise description of the bug.  
Please, check already existing issues here - https://github.com/grishy/go-avahi-cname/issues?q=

**To Reproduce**  
Steps to reproduce the behavior:  
1. Start `go-avahi-cname` as '...'  
2. Send a ping from '...'  
3. ...  

**Server (with go-avahi-cname):**  
- OS and version:  
- Avahi version:  
- go-avahi-cname version:  

**Client (from where you send the request):**  
- OS and version:  
- How mDNS is configured, is it build-in into OS:  

**Network:**  
Describe the topology of the network and how the Server and Client communicate.  

**go-avahi-cname Log:**  
Start with the `--debug` option and attach logs to the issue:  
(attach here)  

**Network Dump:**  
Example: `sudo tcpdump -i any udp port 5353 -w mdns_capture.pcap`  
(attach here: `/tmp/dump.pcap`)
