package avahi

import (
	"fmt"

	"github.com/godbus/dbus/v5"
	"github.com/holoplot/go-avahi"
	"github.com/miekg/dns"
)

const (
	// https://github.com/lathiat/avahi/blob/v0.8/avahi-common/defs.h#L343
	AVAHI_DNS_CLASS_IN = uint16(0x01)
	// https://github.com/lathiat/avahi/blob/v0.8/avahi-common/defs.h#L331
	AVAHI_DNS_TYPE_CNAME = uint16(0x05)
)

type Publisher struct {
	dbusConn    *dbus.Conn
	avahiServer *avahi.Server
	fqdn        string
	rdataField  []byte
}

// NewPublisher creates a new service for Publisher.
func NewPublisher() (*Publisher, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to system bus: %v", err)
	}

	server, err := avahi.ServerNew(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to create Avahi server: %v", err)
	}

	avahiFqdn, err := server.GetHostNameFqdn()
	if err != nil {
		return nil, fmt.Errorf("failed to get FQDN from Avahi: %v", err)
	}

	fqdn := dns.Fqdn(avahiFqdn)

	// RDATA: a variable length string of octets that describes the resource. CNAME in our case
	// Plus 1 because it will add a null byte at the end.
	rdataField := make([]byte, len(fqdn)+1)
	_, err = dns.PackDomainName(fqdn, rdataField, 0, nil, false)
	if err != nil {
		return nil, fmt.Errorf("failed to pack FQDN into RDATA: %v", err)
	}

	return &Publisher{
		dbusConn:    conn,
		avahiServer: server,
		fqdn:        fqdn,
		rdataField:  rdataField,
	}, nil
}

// Fqdn returns the fully qualified domain name from Avahi.
func (p *Publisher) Fqdn() string {
	return p.fqdn
}

// PublishCNAMES send via Avahi-daemon CNAME records with the provided TTL.
func (p *Publisher) PublishCNAMES(cnames []string, ttl uint32) error {
	group, err := p.avahiServer.EntryGroupNew()
	if err != nil {
		return fmt.Errorf("failed to create entry group: %v", err)
	}

	for _, cname := range cnames {
		err := group.AddRecord(
			avahi.InterfaceUnspec,
			avahi.ProtoUnspec,
			uint32(0), // From Avahi Python bindings https://gist.github.com/gdamjan/3168336#file-avahi-alias-py-L42
			cname,
			AVAHI_DNS_CLASS_IN,
			AVAHI_DNS_TYPE_CNAME,
			ttl,
			p.rdataField,
		)
		if err != nil {
			return fmt.Errorf("failed to add record to entry group: %v", err)
		}
	}

	if err := group.Commit(); err != nil {
		return fmt.Errorf("failed to commit entry group: %v", err)
	}

	return nil
}

// Close associated resources.
func (p *Publisher) Close() {
	p.avahiServer.Close() // It also close the DBus connection
}
