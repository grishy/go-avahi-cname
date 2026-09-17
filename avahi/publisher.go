package avahi

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/godbus/dbus/v5"
	"github.com/miekg/dns"
)

const (
	// AvahiDNSClassIn from  https://github.com/lathiat/avahi/blob/v0.8/avahi-common/defs.h#L343
	AvahiDNSClassIn = uint16(0x01)
	// AvahiDNSTypeCName from https://github.com/lathiat/avahi/blob/v0.8/avahi-common/defs.h#L331
	AvahiDNSTypeCName = uint16(0x05)

	avahiService        = "org.freedesktop.Avahi"
	serverInterface     = avahiService + ".Server"
	entryGroupInterface = avahiService + ".EntryGroup"
	publishUpdate       = uint32(64)
	maxDynamicNames     = 256
)

type cnameRegistration struct {
	group            dbus.BusObject
	lastUsedSequence uint64
}

type Publisher struct {
	busConn     *dbus.Conn
	avahiServer dbus.BusObject

	fqdn        string
	targetRData []byte

	registrations     map[string]cnameRegistration
	registrationLimit int // Zero means registrations persist until Close.
	useSequence       uint64
}

// NewPublisher keeps explicit CNAME registrations until Close.
func NewPublisher() (*Publisher, error) {
	return newPublisher(0)
}

// NewDynamicPublisher keeps the 256 most recently queried names registered.
// Evicted names can be registered again when another query arrives.
func NewDynamicPublisher() (*Publisher, error) {
	return newPublisher(maxDynamicNames)
}

func newPublisher(registrationLimit int) (*Publisher, error) {
	slog.Debug("creating new publisher")

	// Own the connection: closing it also releases every Avahi registration.
	// No signal subscription is needed for these synchronous D-Bus calls.
	busConn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to system bus: %w", err)
	}

	avahiServer := busConn.Object(avahiService, "/")
	var avahiFQDN string
	err = avahiServer.Call(serverInterface+".GetHostNameFqdn", 0).Store(&avahiFQDN)
	if err != nil {
		_ = busConn.Close()
		return nil, fmt.Errorf("failed to get FQDN from Avahi: %w", err)
	}
	slog.Debug("got FQDN from Avahi", "fqdn", avahiFQDN)

	fqdn := dns.Fqdn(avahiFQDN)

	// CNAME RDATA contains the target hostname in DNS wire format,
	// including its terminating zero byte.
	targetRData := make([]byte, len(fqdn)+1)
	_, err = dns.PackDomainName(fqdn, targetRData, 0, nil, false)
	if err != nil {
		_ = busConn.Close()
		return nil, fmt.Errorf("failed to pack FQDN into RDATA: %w", err)
	}

	slog.Debug("publisher created successfully", "fqdn", fqdn)

	return &Publisher{
		busConn:     busConn,
		avahiServer: avahiServer,

		fqdn:        fqdn,
		targetRData: targetRData,

		registrations:     make(map[string]cnameRegistration),
		registrationLimit: registrationLimit,
	}, nil
}

// Fqdn returns the fully qualified domain name from Avahi.
func (p *Publisher) Fqdn() string {
	return p.fqdn
}

// PublishCNAMES adds or updates CNAME records. Dynamic publishers evict the
// least recently queried name at capacity; explicit registrations persist.
// TTL controls client caching, not registration lifetime. Calls must be serial.
func (p *Publisher) PublishCNAMES(cnames []string, ttl uint32) error {
	slog.Debug("publishing CNAMEs", "count", len(cnames), "ttl", ttl)

	// Reject invalid input before it can evict a working registration.
	if ttl == 0 {
		return errors.New("CNAME TTL must be greater than zero")
	}

	for _, cname := range cnames {
		if _, valid := dns.IsDomainName(cname); !valid {
			return fmt.Errorf("invalid CNAME: %q", cname)
		}
	}

	for _, cname := range cnames {
		if err := p.publishCNAME(cname, ttl); err != nil {
			return err
		}
	}

	return nil
}

func (p *Publisher) publishCNAME(cname string, ttl uint32) error {
	cname = dns.CanonicalName(cname)
	registration, exists := p.registrations[cname]
	flags := publishUpdate

	if !exists {
		if p.registrationLimit > 0 && len(p.registrations) >= p.registrationLimit {
			if err := p.evictLeastRecentlyUsed(); err != nil {
				return err
			}
		}

		// A new name gets its own group so it cannot interrupt another
		// name's registration. Reset/Commit on every query can starve Avahi.
		var groupPath dbus.ObjectPath
		if err := p.avahiServer.Call(serverInterface+".EntryGroupNew", 0).Store(&groupPath); err != nil {
			return fmt.Errorf("failed to create entry group for %s: %w", cname, err)
		}

		registration.group = p.busConn.Object(avahiService, groupPath)
		flags = 0
	}

	err := registration.group.Call(
		entryGroupInterface+".AddRecord",
		0,
		int32(-1), // All interfaces.
		int32(-1), // Both IP protocols.
		flags,
		cname,
		AvahiDNSClassIn,
		AvahiDNSTypeCName,
		ttl,
		p.targetRData,
	).Err

	if err == nil && !exists {
		// UPDATE works without another Commit on an existing group.
		err = registration.group.Call(entryGroupInterface+".Commit", 0).Err
	}

	if err != nil {
		if !exists {
			if freeErr := registration.group.Call(entryGroupInterface+".Free", 0).Err; freeErr != nil {
				slog.Debug("failed to free rejected CNAME", "cname", cname, "error", freeErr)
			}
		}
		return fmt.Errorf("failed to publish CNAME %s: %w", cname, err)
	}

	p.useSequence++
	registration.lastUsedSequence = p.useSequence
	p.registrations[cname] = registration

	if !exists {
		slog.Debug("registered CNAME", "cname", cname, "count", len(p.registrations))
	}

	return nil
}

func (p *Publisher) evictLeastRecentlyUsed() error {
	// ponytail: scan at most 256 entries; use an LRU list only if this limit grows materially.
	var oldestName string
	for name, registration := range p.registrations {
		if oldestName == "" || registration.lastUsedSequence < p.registrations[oldestName].lastUsedSequence {
			oldestName = name
		}
	}

	if err := p.registrations[oldestName].group.Call(entryGroupInterface+".Free", 0).Err; err != nil {
		return fmt.Errorf("failed to evict CNAME %s: %w", oldestName, err)
	}

	delete(p.registrations, oldestName)
	slog.Debug("evicted CNAME", "cname", oldestName, "count", len(p.registrations))
	return nil
}

// Close associated resources.
func (p *Publisher) Close() {
	slog.Debug("closing publisher")

	if err := p.busConn.Close(); err != nil {
		slog.Debug("failed to close D-Bus connection", "error", err)
	}

	clear(p.registrations)
}
