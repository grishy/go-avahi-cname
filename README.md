<img src="./docs/logo_x3.png" width="350">

![GitHub tag (with filter)](https://img.shields.io/github/v/tag/grishy/go-avahi-cname)
[![Go Report Card](https://goreportcard.com/badge/github.com/grishy/go-avahi-cname)](https://goreportcard.com/report/github.com/grishy/go-avahi-cname)
![Build Status](https://github.com/grishy/go-avahi-cname/actions/workflows/release.yml/badge.svg)

## About go-avahi-cname

This project publishe CNAME records pointing to the local host over multicast DNS using the **Avahi** daemon found in all major Linux distributions.
Since Avahi is compatible with Apple's Bonjour, these names are usable from MacOS X and Windows too.

### Goals

- ✅ No dependencies
- ✅ Small footprint, less than 3MB binary
- ✅ Support x86_64 and ARM
- ✅ Release binaries and containers

### Architecture

![Architecture](./docs/arch.excalidraw.svg)

## Usege and installation

All CNAMEs are specified as program arguments, no length limit.  
You can use either just the name (`name1`), which will create a record as a subdomain for the current machine, or you can write the full FQDN (`name1.hostname.local.` domain with a dot on the end) format.

Example output on machine with hostname `lab`:

```plain
> ./go-avahi-cname git photo.local. example.lab.local.
2023/07/27 08:37:14 Creating publisher
2023/07/27 08:37:14 Formating CNAMEs:
2023/07/27 08:37:14   > 'git.lab.local.' (added current FQDN)
2023/07/27 08:37:14   > 'photo.local.'
2023/07/27 08:37:14   > 'example.lab.local.'
2023/07/27 08:37:14 Publishing every 5m0s and CNAME TTL=600s.
^C
2023/07/27 08:37:16 Closing publisher...
```

### Installation

#### Binary

Binary files can be taken as artifacts for [the Release](https://github.com/grishy/go-avahi-cname/releases). In this case, it would be better to create a systemd service.

#### Container

The images for each version are in [the Packages section](https://github.com/grishy/go-avahi-cname/pkgs/container/go-avahi-cname).  
You need to provide the `/var/run/dbus/system_bus_socket` file to the container to be able to communicate with the host's Avahi daemon.

One-liner to run the container `v0.3.1`:

```bash
> docker run --restart=unless-stopped -d -v /var/run/dbus/system_bus_socket:/var/run/dbus/system_bus_socket ghcr.io/grishy/go-avahi-cname:v0.3.1 name1 name2.lab.local.
5a19790e06cca93016af6651d7af4046c24095a6909ace2fe26c3451fb98ceee

> docker logs 5a19790e06cca93016af6651d7af4046c24095a6909ace2fe26c3451fb98ceee
2023/07/27 08:49:02 Creating publisher
2023/07/27 08:49:02 Formating CNAMEs:
2023/07/27 08:49:02   > 'name1.lab.local.' (added current FQDN)
2023/07/27 08:49:02   > 'name2.lab.local.'
2023/07/27 08:49:02 Publishing every 5m0s ans CNAME TTL=600s.
```

Ansible task to run the container:

```yaml
- name: go-avahi-cname | Start container
  community.docker.docker_container:
    name: "go-avahi-cname"
    image: "ghcr.io/grishy/go-avahi-cname:v0.3.1"
    restart_policy: unless-stopped
    volumes:
      - "/var/run/dbus/system_bus_socket:/var/run/dbus/system_bus_socket" # access to avahi-daemon
    command: "name1 name2 git"
```

## Source of inspiration

- https://web.archive.org/web/20151016190620/http://www.avahi.org/wiki/Examples/PythonPublishAlias
- https://pypi.org/project/mdns-publisher/

## License

Copyright © 2022 [Sergei G.](https://github.com/grishy)  
This project is [MIT](./LICENSE) licensed.
