package avahi

import (
	"fmt"
	"log"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/holoplot/go-avahi"
	"github.com/miekg/dns"
)

const (
	// From Avahi Python bindings
	// AVAHI_DBUS_NAME                  = "org.freedesktop.Avahi"
	// AVAHI_DBUS_INTERFACE_ENTRY_GROUP = AVAHI_DBUS_NAME + ".EntryGroup"

	// https://github.com/lathiat/avahi/blob/v0.8/avahi-common/defs.h#L343
	AVAHI_DNS_CLASS_IN = uint16(0x01)
	// https://github.com/lathiat/avahi/blob/v0.8/avahi-common/defs.h#L331
	AVAHI_DNS_TYPE_CNAME = uint16(0x05)

	TTL = uint32(60)
)

func PublishCNAME(cname string) {
	fmt.Println("Publishing CNAME:", cname)

	conn, err := dbus.SystemBus()
	if err != nil {
		log.Fatalf("Cannot get system bus: %v", err)
	}

	server, err := avahi.ServerNew(conn)
	if err != nil {
		log.Fatalf("Avahi new failed: %v", err)
	}

	fqdn, err := server.GetHostNameFqdn()
	if err != nil {
		log.Fatalf("GetHostNameFqdn() failed: %v", err)
	}
	log.Println("GetHostNameFqdn()", fqdn)

	for {

		group, err := server.EntryGroupNew()
		if err != nil {
			panic(err)
		}

		fmt.Println("Created EntryGroup")

		// https://lists.freedesktop.org/archives/avahi/2008-July/001380.html
		// rdata := []byte(fqdn + "\x00")
		// p := idna.New()
		// rdataS, _ := p.ToASCII(cname)

		// RDATA: a variable length string of octets that describes the resource.
		// The format of this information varies according to the TYPE and CLASS of the resource record.
		// For example, the if the TYPE is A (IPv4) and the CLASS is IN, the RDATA field is a 4 octet ARPA Internet address.

		fqdnFull := fqdn + "."
		rdata := make([]byte, len(fqdnFull)+1)
		_, err = dns.PackDomainName(fqdnFull, rdata, 0, nil, false)
		if err != nil {
			panic(err)
		}

		fmt.Println("Rdata", rdata)
		cnameFull := cname + "." + fqdnFull

		fmt.Println("fqdnFull", fqdnFull)
		fmt.Println("cnameFull", cnameFull)
		err1 := group.AddRecord(
			avahi.InterfaceUnspec,
			avahi.ProtoUnspec,
			uint32(0), // From Avahi Python bindings https://gist.github.com/gdamjan/3168336#file-avahi-alias-py-L42
			cnameFull,
			AVAHI_DNS_CLASS_IN,
			AVAHI_DNS_TYPE_CNAME,
			TTL,
			rdata,
		)
		if err1 != nil {
			panic(err)
		}

		fmt.Println("Added record")

		err2 := group.Commit()
		if err2 != nil {
			panic(err2)
		}
		fmt.Println("Committed")

		time.Sleep(time.Second * 5)
	}

	// serverObj := conn.Object(AVAHI_DBUS_NAME, AVAHI_DBUS_PATH_SERVER)
	// _ = serverObj

	// // var s string
	// err = serverObj.Call(AVAHI_DBUS_INTERFACE_SERVER, 0).Store(&s)
	// if err != nil {
	// 	fmt.Fprintln(os.Stderr, "Failed to call Foo function (is the server example running?):", err)
	// 	os.Exit(1)
	// }

	// fmt.Println("Result from calling Foo function on com.github.guelfey.Demo interface:")
	// fmt.Println(s)
}
