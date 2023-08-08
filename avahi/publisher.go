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

func NewPublisher() (*Publisher, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("can't connect to system bus: %v", err)
	}

	server, err := avahi.ServerNew(conn)
	if err != nil {
		return nil, fmt.Errorf("can't create Avahi server: %v", err)
	}

	avahiFqdn, err := server.GetHostNameFqdn()
	if err != nil {
		return nil, fmt.Errorf("can't get FQDN from Avahi: %v", err)
	}

	fqdn := dns.Fqdn(avahiFqdn)

	// RDATA: a variable length string of octets that describes the resource. CNAME in our case
	// Plus 1 because it will add null byte at the end
	rdataField := make([]byte, len(fqdn)+1)
	_, err = dns.PackDomainName(fqdn, rdataField, 0, nil, false)
	if err != nil {
		return nil, fmt.Errorf("can't pack FQDN into RDATA: %v", err)
	}

	return &Publisher{
		dbusConn:    conn,
		avahiServer: server,
		fqdn:        fqdn,
		rdataField:  rdataField,
	}, nil
}

func (p *Publisher) Fqdn() string {
	return p.fqdn
}

func (p *Publisher) PublishCNAMES(cnames []string, ttl uint32) error {
	group, err := p.avahiServer.EntryGroupNew()
	if err != nil {
		return fmt.Errorf("can't create entry group: %v", err)
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
			return fmt.Errorf("can't add record to entry group: %v", err)
		}
	}

	if err := group.Commit(); err != nil {
		return fmt.Errorf("can't commit entry group: %v", err)
	}

	return nil
}

func (p *Publisher) Close() {
	p.avahiServer.Close() // It also close the DBus connection
}
