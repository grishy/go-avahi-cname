package avahi

import (
	"fmt"
	"log/slog"

	"github.com/godbus/dbus/v5"
	"github.com/holoplot/go-avahi"
	"github.com/miekg/dns"
)

const (
	// AvahiDNSClassIn from  https://github.com/lathiat/avahi/blob/v0.8/avahi-common/defs.h#L343
	AvahiDNSClassIn = uint16(0x01)
	// AvahiDNSTypeCName from https://github.com/lathiat/avahi/blob/v0.8/avahi-common/defs.h#L331
	AvahiDNSTypeCName = uint16(0x05)
)

type Publisher struct {
	dbusConn        *dbus.Conn
	avahiServer     *avahi.Server
	avahiEntryGroup *avahi.EntryGroup
	fqdn            string
	rdataField      []byte
}

// NewPublisher creates a new service for Publisher.
func NewPublisher() (*Publisher, error) {
	slog.Debug("creating new publisher")

	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to system bus: %w", err)
	}

	server, err := avahi.ServerNew(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to create Avahi server: %w", err)
	}

	avahiFqdn, err := server.GetHostNameFqdn()
	if err != nil {
		return nil, fmt.Errorf("failed to get FQDN from Avahi: %w", err)
	}
	slog.Debug("got FQDN from Avahi", "fqdn", avahiFqdn)

	group, err := server.EntryGroupNew()
	if err != nil {
		return nil, fmt.Errorf("failed to create entry group: %w", err)
	}

	fqdn := dns.Fqdn(avahiFqdn)

	// RDATA: a variable length string of octets that describes the resource. CNAME in our case
	// Plus 1 because it will add a null byte at the end.
	rdataField := make([]byte, len(fqdn)+1)
	_, err = dns.PackDomainName(fqdn, rdataField, 0, nil, false)
	if err != nil {
		return nil, fmt.Errorf("failed to pack FQDN into RDATA: %w", err)
	}

	slog.Debug("publisher created successfully", "fqdn", fqdn)
	return &Publisher{
		dbusConn:        conn,
		avahiServer:     server,
		avahiEntryGroup: group,
		fqdn:            fqdn,
		rdataField:      rdataField,
	}, nil
}

// Fqdn returns the fully qualified domain name from Avahi.
func (p *Publisher) Fqdn() string {
	return p.fqdn
}

// PublishCNAMES send via Avahi-daemon CNAME records with the provided TTL.
func (p *Publisher) PublishCNAMES(cnames []string, ttl uint32) error {
	slog.Debug("publishing CNAMEs", "count", len(cnames), "ttl", ttl)

	// Reset the entry group to remove all records.
	// Because we can't update records without it after the `Commit`.
	if err := p.avahiEntryGroup.Reset(); err != nil {
		return fmt.Errorf("failed to reset entry group: %w", err)
	}

	for _, cname := range cnames {
		slog.Debug("adding CNAME record", "cname", cname)
		err := p.avahiEntryGroup.AddRecord(
			avahi.InterfaceUnspec,
			avahi.ProtoUnspec,
			uint32(0), // From Avahi Python bindings https://gist.github.com/gdamjan/3168336#file-avahi-alias-py-L42
			cname,
			AvahiDNSClassIn,
			AvahiDNSTypeCName,
			ttl,
			p.rdataField,
		)
		if err != nil {
			return fmt.Errorf("failed to add record to entry group: %w", err)
		}
	}

	if err := p.avahiEntryGroup.Commit(); err != nil {
		return fmt.Errorf("failed to commit entry group: %w", err)
	}

	slog.Debug("successfully published CNAMEs")
	return nil
}

// Close associated resources.
func (p *Publisher) Close() {
	slog.Debug("closing publisher")
	p.avahiServer.Close() // It also closes the DBus connection and free the entry group
}
