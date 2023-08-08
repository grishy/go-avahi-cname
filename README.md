<img src="./docs/logo_x3.png" width="350">

![GitHub tag (with filter)](https://img.shields.io/github/v/tag/grishy/go-avahi-cname)
[![Go Report Card](https://goreportcard.com/badge/github.com/grishy/go-avahi-cname)](https://goreportcard.com/report/github.com/grishy/go-avahi-cname)
![Build Status](https://github.com/grishy/go-avahi-cname/actions/workflows/release.yml/badge.svg)

# What is go-avahi-cname?

It is a simple and lightweight project that allows you to publish CNAME records pointing to the local host over multicast DNS using the **Avahi** daemon, which is widely available in most Linux distributions. This means that you can access your local host using different names from any device on the same network, as long as they support Apple’s Bonjour protocol, which is compatible with Avahi.

## TLDR

Forward all subdomains from machine to machine( `*.hostname.local` -> `hostname.local`)

1. _Binary_ `./go-avahi-cname subdomain`
2. _Docker_ `docker run -d -v "/var/run/dbus/system_bus_socket:/var/run/dbus/system_bus_socket" ghcr.io/grishy/go-avahi-cname:v2.0.0`

## Modes

- **Subdomain reply** - _I think you want this._ We listen to the traffic and if someone asks `*.hostname.local` (example: `name1.hostname.local`), we "redirect" to `hostname.local`.
- **Interval publishing** - With some frequency, we send out information about all `name1.hostname.local`, `name2.hostname.local`...

## Goals

Here are some of the benefits of using go-avahi-cname:

- **✅ No dependencies**  
   You only need the Avahi daemon running on your host, no other libraries or packages are required.
- **✅ Small footprint**
- **✅ Support x86_64 and ARM**
- **✅ Install as binaries or containers**

---

# How does it work?

_go-avahi-cname_ communicates with the Avahi daemon via DBus, and publishes the CNAME records. Different modes are only the way of how we select records to publish. The Avahi daemon then broadcasts these records over multicast DNS, so that other devices on the same network can resolve them.

## Subdomain CNAME reply

![Architecture](./docs/arch_subdomain.excalidraw.svg)

```plain
> ./go-avahi-cname subdomain -h
NAME:
   go-avahi-cname subdomain - Listen for all queries and publish CNAMEs for subdomains

USAGE:
   go-avahi-cname subdomain [command options] [arguments...]

OPTIONS:
   --ttl value   TTL of CNAME record in seconds (default: 600) [$TTL]
   --fqdn value  FQDN which will be used for CNAME. If empty, will be used current FQDN (default: hostname.local.) [$FQDN]
   --help, -h    show help
```

In this variant, we listen to the traffic with avahi-daemon for all questions with names and if they match ours, we send a command to avahi to answer it (send CNAME). The standard can be run without parameters, then we will resolve all requests that contain our hostname. For example, `git.lab.local` will be redirected to `lab.local`

## Interval publishing of CNAME records

![Architecture](./docs/arch_cname.excalidraw.svg)

As you can see, _go-avahi-cname_ communicates with the Avahi daemon via DBus, and publishes the CNAME records that you specify as arguments.

```plain
> ./go-avahi-cname cname -h
NAME:
   go-avahi-cname cname - Announce CNAME records for host via avahi-daemon

USAGE:
   go-avahi-cname cname [command options] [arguments...]

OPTIONS:
   --ttl value       TTL of CNAME record in seconds. How long they will be valid. (default: 600) [$TTL]
   --interval value  Interval of publishing CNAME records in seconds. How often to send records to other machines. (default: 300) [$INTERVAL]
   --fqdn value      Where to redirect. If empty, the Avahi FQDN (current machine) will be used (default: hostname.local.) [$FQDN]
   --help, -h        show help
```

You can specify any number of CNAMEs as arguments when running go-avahi-cname, with no length limit.
You can use either just the name (`name1`), which will create a record as a subdomain for the current machine, or you can write the full FQDN (`name1.hostname.local.` domain with a dot on the end) format.

For example, if your machine’s hostname is lab, you can run:

```plain
> ./go-avahi-cname cname git photo.local. example.lab.local.
2023/08/08 14:51:21 Creating publisher
2023/08/08 14:51:21 Getting FQDN from Avahi
2023/08/08 14:51:21 FQDN: lab.local.
2023/08/08 14:51:21 Formatting CNAMEs:
2023/08/08 14:51:21   > 'git.lab.local.' (added FQDN)
2023/08/08 14:51:21   > 'photo.local.'
2023/08/08 14:51:21   > 'example.lab.local.'
2023/08/08 14:51:21 Publishing every 300s and CNAME TTL 600s
```

This will create three CNAME records pointing to your local host:

- `git.lab.local.`
- `photo.local.`
- `example.lab.local.`

You can then access your local host using any of these names from other devices on the same network.

## How to use and install?

You can specify any number of CNAMEs as arguments when running go-avahi-cname, with no length limit.
You can use either just the name (`name1`), which will create a record as a subdomain for the current machine, or you can write the full FQDN (`name1.hostname.local.` domain with a dot on the end) format.

For example, if your machine’s hostname is lab, you can run:

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

This will create three CNAME records pointing to your local host:

- `git.lab.local.`
- `photo.local.`
- `example.lab.local.`

You can then access your local host using any of these names from other devices on the same network.

### Installation options

There are two ways to install and run go-avahi-cname:

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

- https://github.com/nfam/mdns-subdomain/tree/master - I used this approach for version 2.
- https://web.archive.org/web/20151016190620/http://www.avahi.org/wiki/Examples/PythonPublishAlias
- https://pypi.org/project/mdns-publisher/

## License

Copyright © 2023 [Sergei G.](https://github.com/grishy)  
This project is [MIT](./LICENSE) licensed.
