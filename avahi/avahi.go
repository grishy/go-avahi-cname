package avahi

import (
	"log"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/holoplot/go-avahi"
	"github.com/miekg/dns"
)

const (
	// https://github.com/lathiat/avahi/blob/v0.8/avahi-common/defs.h#L343
	AVAHI_DNS_CLASS_IN = uint16(0x01)
	// https://github.com/lathiat/avahi/blob/v0.8/avahi-common/defs.h#L331
	AVAHI_DNS_TYPE_CNAME = uint16(0x05)

	TTL = uint32(120)
)

func PublishCNAME(cnames []string) {
	log.Println("Publishing CNAME:", cnames)

	conn, err := dbus.SystemBus()
	if err != nil {
		log.Fatalf("Cannot get system bus: %v", err)
	}
	defer conn.Close()

	server, err := avahi.ServerNew(conn)
	if err != nil {
		log.Fatalf("Avahi new failed: %v", err)
	}
	defer server.Close()

	fqdn, err := server.GetHostNameFqdn()
	if err != nil {
		log.Fatalf("GetHostNameFqdn() failed: %v", err)
	}

	for {
		group, err := server.EntryGroupNew()
		if err != nil {
			log.Fatalf("EntryGroupNew() failed: %v", err)
		}

		for _, cname := range cnames {
			// RDATA: a variable length string of octets that describes the resource.
			// The format of this information varies according to the TYPE and CLASS of the resource record.
			// For example, the if the TYPE is A (IPv4) and the CLASS is IN, the RDATA field is a 4 octet ARPA Internet address.
			fqdnFull := fqdn + "."
			rdata := make([]byte, len(fqdnFull)+1)
			_, err = dns.PackDomainName(fqdnFull, rdata, 0, nil, false)
			if err != nil {
				log.Fatalf("dns.PackDomainName() failed: %v", err)
			}

			cnameFull := cname + "." + fqdnFull
			err := group.AddRecord(
				avahi.InterfaceUnspec,
				avahi.ProtoUnspec,
				uint32(0), // From Avahi Python bindings https://gist.github.com/gdamjan/3168336#file-avahi-alias-py-L42
				cnameFull,
				AVAHI_DNS_CLASS_IN,
				AVAHI_DNS_TYPE_CNAME,
				TTL,
				rdata,
			)
			if err != nil {
				log.Fatalf("AddRecord() failed: %v", err)
			}
		}

		if err := group.Commit(); err != nil {
			log.Fatalf("Commit() failed: %v", err)
		}

		time.Sleep(time.Second * time.Duration(TTL))
	}
}
