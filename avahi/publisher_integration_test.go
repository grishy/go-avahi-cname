//go:build integration

package avahi

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/miekg/dns"
	"golang.org/x/net/ipv4"
)

// Run against an isolated Avahi daemon; see CONTRIBUTING.md.
func TestPublishCNAMESPreservesRegistrations(t *testing.T) {
	address := os.Getenv("AVAHI_TEST_ADDRESS")
	if address == "" {
		t.Fatal("AVAHI_TEST_ADDRESS must point to an isolated Avahi daemon's UDP port 5353")
	}

	publisher, err := NewPublisher()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(publisher.Close)

	git := "git." + publisher.Fqdn()
	immich := "immich." + publisher.Fqdn()

	// Alternating packets and duplicate A/AAAA questions must not withdraw records
	// or keep restarting Avahi's registration holdoff.
	for range 30 {
		for _, names := range [][]string{
			{git, git},
			{immich},
			{"GIT." + publisher.Fqdn()},
		} {
			if err = publisher.PublishCNAMES(names, 600); err != nil {
				t.Fatal(err)
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Invalid TTLs must not withdraw existing records or prevent a later retry.
	retry := "retry." + publisher.Fqdn()
	for _, name := range []string{git, retry} {
		if err = publisher.PublishCNAMES([]string{name}, 0); err == nil {
			t.Fatalf("expected rejection of TTL 0 for %s", name)
		}
	}

	if err = publisher.PublishCNAMES([]string{retry}, 600); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{git, immich} {
		assertCNAME(t, address, name, publisher.Fqdn())
	}

	time.Sleep(time.Second) // Allow the newly retried group to establish.
	assertCNAME(t, address, retry, publisher.Fqdn())
}

func TestDynamicPublisherEvictsOldest(t *testing.T) {
	address := os.Getenv("AVAHI_TEST_ADDRESS")
	if address == "" {
		t.Fatal("AVAHI_TEST_ADDRESS is required")
	}

	publisher, err := NewDynamicPublisher()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(publisher.Close)

	frequentlyQueriedName := "hot." + publisher.Fqdn()
	firstRegisteredName := "alias-0." + publisher.Fqdn()
	var firstRegistrationGroup dbus.BusObject

	// Exceed Avahi's default 1,024-object limit while keeping one name active.
	for i := range 1100 {
		name := fmt.Sprintf("alias-%d.%s", i, publisher.Fqdn())

		// Case, duplicates, and an omitted final dot must refresh the same entry.
		names := []string{
			strings.ToUpper(frequentlyQueriedName),
			strings.TrimSuffix(frequentlyQueriedName, "."),
			name,
		}
		if err = publisher.PublishCNAMES(names, 600); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			firstRegistrationGroup = publisher.registrations[firstRegisteredName].group
		}
		if len(publisher.registrations) > maxDynamicNames {
			t.Fatal("dynamic limit exceeded")
		}
	}

	// Eviction must release both our registration and the daemon's entry group.
	if len(publisher.registrations) != maxDynamicNames {
		t.Fatalf("got %d registrations", len(publisher.registrations))
	}
	if _, exists := publisher.registrations[firstRegisteredName]; exists {
		t.Fatal("oldest name was not evicted")
	}
	if err = firstRegistrationGroup.Call(entryGroupInterface+".GetState", 0).Err; err == nil {
		t.Fatal("evicted group still exists in Avahi")
	}

	// Invalid input at capacity must not evict another valid registration.
	sequenceBeforeRejection := publisher.useSequence
	invalidRequests := []struct {
		name string
		ttl  uint32
	}{
		{
			name: "invalid." + publisher.Fqdn(),
			ttl:  0,
		},
		{
			name: strings.Repeat("x", 64) + "." + publisher.Fqdn(),
			ttl:  600,
		},
	}
	for _, invalid := range invalidRequests {
		if err = publisher.PublishCNAMES([]string{invalid.name}, invalid.ttl); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
	if len(publisher.registrations) != maxDynamicNames || publisher.useSequence != sequenceBeforeRejection {
		t.Fatal("invalid input changed registrations")
	}

	// Eviction is temporary: a later query can register the name again.
	if err = publisher.PublishCNAMES([]string{firstRegisteredName}, 600); err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)
	for _, name := range []string{frequentlyQueriedName, firstRegisteredName, "alias-1099." + publisher.Fqdn()} {
		assertCNAME(t, address, name, publisher.Fqdn())
	}
}

func TestExplicitPublisherKeepsNamesBeyondTTLAndDynamicLimit(t *testing.T) {
	address := os.Getenv("AVAHI_TEST_ADDRESS")
	if address == "" {
		t.Fatal("AVAHI_TEST_ADDRESS is required")
	}

	publisher, err := NewPublisher()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(publisher.Close)

	names := make([]string, maxDynamicNames+1)
	for i := range names {
		names[i] = fmt.Sprintf("static-%d.%s", i, publisher.Fqdn())
	}

	if err = publisher.PublishCNAMES(names, 1); err != nil {
		t.Fatal(err)
	}

	time.Sleep(2 * time.Second)
	if len(publisher.registrations) != len(names) {
		t.Fatal("explicit names were evicted")
	}

	assertCNAME(t, address, names[0], publisher.Fqdn())
	assertCNAME(t, address, names[len(names)-1], publisher.Fqdn())

	// Existing registrations also accept a changed TTL without another Commit.
	if err = publisher.PublishCNAMES(names, 600); err != nil {
		t.Fatal(err)
	}
	if ttl := assertCNAME(t, address, names[0], publisher.Fqdn()); ttl <= 1 {
		t.Fatalf("TTL update did not reach Avahi: got %d", ttl)
	}
}

//nolint:gocognit // Keep the state-change and shutdown assertions in one lifecycle check.
func TestPublisherCloseAfterStateChanges(t *testing.T) {
	publisher, err := NewPublisher()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(publisher.Close)

	name := "shutdown." + publisher.Fqdn()
	if err = publisher.PublishCNAMES([]string{name}, 600); err != nil {
		t.Fatal(err)
	}
	group := publisher.registrations[name].group

	// Force more than ten events on ONE group: the old wrapper blocked here.
	for range 20 {
		if err = group.Call(entryGroupInterface+".Reset", 0).Err; err != nil {
			t.Fatal(err)
		}

		err = group.Call(
			entryGroupInterface+".AddRecord",
			0,
			int32(-1),
			int32(-1),
			uint32(0),
			name,
			AvahiDNSClassIn,
			AvahiDNSTypeCName,
			uint32(600),
			publisher.targetRData,
		).Err
		if err != nil {
			t.Fatal(err)
		}

		if err = group.Call(entryGroupInterface+".Commit", 0).Err; err != nil {
			t.Fatal(err)
		}
	}

	observer, err := dbus.ConnectSystemBus()
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Close()

	publisherBusName := publisher.busConn.Names()[0]
	done := make(chan struct{})
	go func() {
		publisher.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close blocked after state changes")
	}

	if publisher.busConn.Connected() {
		t.Fatal("publisher connection is still open")
	}

	// A private publisher connection must not close other users of the bus.
	if err = observer.BusObject().Call("org.freedesktop.DBus.GetId", 0).Err; err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for {
		var hasOwner bool
		err = observer.BusObject().Call(
			"org.freedesktop.DBus.NameHasOwner", 0, publisherBusName,
		).Store(&hasOwner)
		if err != nil {
			t.Fatal(err)
		}
		if !hasOwner {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("publisher still owns a D-Bus name")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err = observer.Object(avahiService, group.Path()).Call(entryGroupInterface+".GetState", 0).Err; err == nil {
		t.Fatal("registration survived Close")
	}

	if err = publisher.PublishCNAMES([]string{name}, 600); err == nil {
		t.Fatal("publication on a closed connection succeeded")
	}
}

func TestPublisherInitializationFailureClosesConnection(t *testing.T) {
	// A separate bus with no Avahi exercises the constructor's error path
	// without stopping the real daemon used by the other integration checks.
	daemon := exec.Command("dbus-daemon", "--session", "--nofork", "--print-address")
	daemonStdout, err := daemon.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err = daemon.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = daemon.Process.Kill()
		_ = daemon.Wait()
	})

	address, err := bufio.NewReader(daemonStdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", strings.TrimSpace(address))

	observer, err := dbus.ConnectSystemBus()
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Close()

	var busNamesBefore []string
	if err = observer.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&busNamesBefore); err != nil {
		t.Fatal(err)
	}

	for range 20 {
		publisher, initErr := NewPublisher()
		if initErr == nil {
			publisher.Close()
			t.Fatal("initialization without Avahi succeeded")
		}
	}

	deadline := time.Now().Add(time.Second)
	for {
		var busNamesAfter []string
		if err = observer.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&busNamesAfter); err != nil {
			t.Fatal(err)
		}
		if len(busNamesAfter) == len(busNamesBefore) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("failed constructors leaked connections: before %v, after %v", busNamesBefore, busNamesAfter)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRejectedRecordReleasesGroup(t *testing.T) {
	publisher, err := NewPublisher()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(publisher.Close)

	name := "rejected." + publisher.Fqdn()

	validTargetRData := publisher.targetRData
	publisher.targetRData = []byte{0xff} // Force a real Avahi AddRecord rejection.
	for range 1030 {
		if err = publisher.PublishCNAMES([]string{name}, 600); err == nil {
			t.Fatal("Avahi accepted invalid RDATA")
		}
	}

	publisher.targetRData = validTargetRData
	if err = publisher.PublishCNAMES([]string{name}, 600); err != nil {
		t.Fatalf("rejected records leaked groups: %v", err)
	}

	time.Sleep(time.Second)
	assertCNAME(t, os.Getenv("AVAHI_TEST_ADDRESS"), name, publisher.Fqdn())
}

func TestPublisherCLI(t *testing.T) {
	binaryPath := os.Getenv("AVAHI_TEST_BINARY")
	if binaryPath == "" {
		t.Skip("AVAHI_TEST_BINARY is not set")
	}

	publisher, err := NewPublisher()
	if err != nil {
		t.Fatal(err)
	}
	fqdn := publisher.Fqdn()
	publisher.Close()

	for _, mode := range []string{"cname", "subdomain"} {
		t.Run(mode, func(t *testing.T) {
			args := []string{"--debug", mode, "--ttl", "1", "--fqdn", "cli.local."}
			if mode == "cname" {
				args = append(args, "--interval", "1", "first", "second")
			}

			command := exec.Command(binaryPath, args...)
			var logs bytes.Buffer
			command.Stdout, command.Stderr = &logs, &logs
			if startErr := command.Start(); startErr != nil {
				t.Fatal(startErr)
			}

			done := make(chan struct{})
			var exitErr error
			go func() {
				exitErr = command.Wait()
				close(done)
			}()
			t.Cleanup(func() {
				_ = command.Process.Kill()
				<-done
			})

			assertMulticastCNAME(t, "first.cli.local.", fqdn)
			assertMulticastCNAME(t, "second.cli.local.", fqdn)
			time.Sleep(1100 * time.Millisecond) // Cross the static mode's refresh interval.
			assertMulticastCNAME(t, "first.cli.local.", fqdn)

			if signalErr := command.Process.Signal(syscall.SIGTERM); signalErr != nil {
				t.Fatal(signalErr)
			}
			select {
			case <-done:
				if exitErr != nil {
					t.Errorf("CLI shutdown: %v", exitErr)
				}
				t.Logf("CLI output:\n%s", logs.String())
			case <-time.After(3 * time.Second):
				t.Fatal("CLI did not shut down promptly")
			}
		})
	}
}

func assertCNAME(t *testing.T, address, name, target string) uint32 {
	t.Helper()

	client := dns.Client{Timeout: time.Second}
	query := new(dns.Msg)
	query.SetQuestion(name, dns.TypeCNAME)
	response, _, err := client.Exchange(query, address)
	if err != nil {
		t.Errorf("resolve %s after repeated publication: %v", name, err)
		return 0
	}

	for _, record := range response.Answer {
		cname, ok := record.(*dns.CNAME)
		if ok &&
			dns.CanonicalName(cname.Hdr.Name) == name &&
			cname.Target == target &&
			cname.Hdr.Ttl > 0 {
			t.Logf("%s", response)
			return cname.Hdr.Ttl
		}
	}

	t.Errorf("missing CNAME %s -> %s: %s", name, target, response)
	return 0
}

//nolint:gocognit // Keep multicast retries, deadlines, and reply filtering together.
func assertMulticastCNAME(t *testing.T, name, target string) {
	t.Helper()

	address := &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}
	conn, err := net.ListenMulticastUDP("udp4", nil, address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if err = ipv4.NewPacketConn(conn).SetMulticastLoopback(true); err != nil {
		t.Fatal(err)
	}

	query := new(dns.Msg)
	query.SetQuestion(name, dns.TypeCNAME)
	query.Id, query.RecursionDesired = 0, false
	packet, err := query.Pack()
	if err != nil {
		t.Fatal(err)
	}

	buffer := make([]byte, 9000)
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if _, err = conn.WriteToUDP(packet, address); err != nil {
			t.Fatal(err)
		}
		if err = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
			t.Fatal(err)
		}

		// Use genuine mDNS replies: legacy unicast on shared port 5353 may be
		// delivered to the CLI listener instead of Avahi on the same machine.
		for {
			size, _, readErr := conn.ReadFromUDP(buffer)
			if readErr != nil {
				if !os.IsTimeout(readErr) {
					t.Fatal(readErr)
				}
				break
			}

			var response dns.Msg
			if err = response.Unpack(buffer[:size]); err != nil {
				t.Fatal(err)
			}
			if !response.Response {
				continue
			}

			for _, record := range response.Answer {
				if cname, ok := record.(*dns.CNAME); ok &&
					cname.Hdr.Name == name &&
					cname.Target == target &&
					cname.Hdr.Ttl > 0 {
					t.Logf("multicast answer: %s", cname)
					return
				}
			}
		}
	}

	t.Errorf("no multicast CNAME %s -> %s", name, target)
}
