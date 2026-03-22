package cmd

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/net/ipv4"
)

type fakePublisher struct {
	publishCalls int
	publishErr   error
}

func (p *fakePublisher) PublishCNAMES(_ []string, _ uint32) error {
	p.publishCalls++
	return p.publishErr
}

func TestMatchingQuestionNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fqdn     string
		question []dns.Question
		want     []string
	}{
		{
			name: "matching subdomain is returned",
			fqdn: "lab.local.",
			question: []dns.Question{
				{Name: "test.lab.local.", Qtype: dns.TypeA, Qclass: dns.ClassINET},
			},
			want: []string{"test.lab.local."},
		},
		{
			name: "non-matching domain is excluded",
			fqdn: "lab.local.",
			question: []dns.Question{
				{Name: "other.example.com.", Qtype: dns.TypeA, Qclass: dns.ClassINET},
			},
			want: []string{},
		},
		{
			name: "mixed case still matches",
			fqdn: "lab.local.",
			question: []dns.Question{
				{Name: "Test.Lab.LOCAL.", Qtype: dns.TypeA, Qclass: dns.ClassINET},
			},
			want: []string{"Test.Lab.LOCAL."},
		},
		{
			name: "exact fqdn is not a subdomain",
			fqdn: "lab.local.",
			question: []dns.Question{
				// "lab.local." does not have a subdomain prefix before the
				// suffix ".lab.local.", so it must not match.
				{Name: "lab.local.", Qtype: dns.TypeA, Qclass: dns.ClassINET},
			},
			want: []string{},
		},
		{
			name: "multiple questions with mixed matches",
			fqdn: "lab.local.",
			question: []dns.Question{
				{Name: "yes.lab.local.", Qtype: dns.TypeA, Qclass: dns.ClassINET},
				{Name: "no.example.com.", Qtype: dns.TypeA, Qclass: dns.ClassINET},
				{Name: "also.lab.local.", Qtype: dns.TypeAAAA, Qclass: dns.ClassINET},
			},
			want: []string{"yes.lab.local.", "also.lab.local."},
		},
		{
			name:     "empty question list",
			fqdn:     "lab.local.",
			question: []dns.Question{},
			want:     []string{},
		},
		{
			name: "deep subdomain matches",
			fqdn: "lab.local.",
			question: []dns.Question{
				{Name: "a.b.c.lab.local.", Qtype: dns.TypeA, Qclass: dns.ClassINET},
			},
			want: []string{"a.b.c.lab.local."},
		},
		{
			name: "fqdn with upper case letters matches lower case question",
			fqdn: "Lab.Local.",
			question: []dns.Question{
				{Name: "svc.lab.local.", Qtype: dns.TypeA, Qclass: dns.ClassINET},
			},
			want: []string{"svc.lab.local."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := matchingQuestionNames(tt.fqdn, tt.question)

			if len(got) != len(tt.want) {
				t.Fatalf("matchingQuestionNames(%q, ...) returned %d results, want %d\ngot:  %v\nwant: %v",
					tt.fqdn, len(got), len(tt.want), got, tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("result[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestReaderReceivesMessage(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	// Set up a UDP socket on loopback with an ephemeral port.
	listenAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve listen address: %v", err)
	}

	conn, err := net.ListenUDP("udp4", listenAddr)
	if err != nil {
		t.Fatalf("listen UDP: %v", err)
	}
	defer conn.Close()

	msgCh := reader(t.Context(), conn)

	query := new(dns.Msg)
	query.SetQuestion("test.lab.local.", dns.TypeA)

	packed, err := query.Pack()
	if err != nil {
		t.Fatalf("pack DNS query: %v", err)
	}

	senderConn, err := net.DialUDP("udp4", nil, conn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("dial UDP sender: %v", err)
	}
	defer senderConn.Close()

	writeN, writeErr := senderConn.Write(packed)
	if writeErr != nil {
		t.Fatalf("send UDP packet: %v", writeErr)
	}
	if writeN != len(packed) {
		t.Fatalf("short write: wrote %d of %d bytes", writeN, len(packed))
	}

	select {
	case m, ok := <-msgCh:
		if !ok {
			t.Fatal("channel closed before receiving a message")
		}
		if m.err != nil {
			t.Fatalf("reader returned error: %v", m.err)
		}
		if len(m.msg.Question) == 0 {
			t.Fatal("received DNS message has no questions")
		}
		if m.msg.Question[0].Name != "test.lab.local." {
			t.Errorf("question name = %q, want %q", m.msg.Question[0].Name, "test.lab.local.")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for DNS message on channel")
	}
}

func TestReaderClosesChannelOnContextCancel(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	listenAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve listen address: %v", err)
	}

	conn, err := net.ListenUDP("udp4", listenAddr)
	if err != nil {
		t.Fatalf("listen UDP: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	msgCh := reader(ctx, conn)

	cancel()
	conn.Close()

	select {
	case _, ok := <-msgCh:
		if ok {
			for {
				_, ok = <-msgCh
				if !ok {
					break
				}
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("channel was not closed after context cancellation")
	}
}

func TestReaderReportsUnpackError(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	listenAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve listen address: %v", err)
	}

	conn, err := net.ListenUDP("udp4", listenAddr)
	if err != nil {
		t.Fatalf("listen UDP: %v", err)
	}
	defer conn.Close()

	msgCh := reader(t.Context(), conn)

	senderConn, err := net.DialUDP("udp4", nil, conn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("dial UDP sender: %v", err)
	}
	defer senderConn.Close()

	writeN, writeErr := senderConn.Write([]byte{0xFF, 0xFF})
	if writeErr != nil {
		t.Fatalf("send garbage packet: %v", writeErr)
	}
	if writeN != 2 {
		t.Fatalf("short write: wrote %d of 2 bytes", writeN)
	}

	select {
	case m, ok := <-msgCh:
		if !ok {
			t.Fatal("channel closed before receiving error message")
		}
		if m.err == nil {
			t.Fatal("expected unpack error, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for error message on channel")
	}
}

func TestRunSubdomainRetriesListenerErrors(t *testing.T) {
	callCount := 0
	deps := subdomainDeps{
		reconnectDelay: 1 * time.Millisecond,
		listenAndServe: func(_ context.Context, _ subdomainPublisher, _ string, _ uint32) error {
			callCount++
			if callCount == 1 {
				return listenerFailure(errors.New("socket died"))
			}

			return nil
		},
	}

	err := runSubdomainWith(context.Background(), &fakePublisher{}, "lab.local.", 600, deps)
	if err != nil {
		t.Fatalf("runSubdomainWith returned error: %v", err)
	}

	if callCount != 2 {
		t.Fatalf("listenAndServe call count = %d, want 2", callCount)
	}
}

func TestRunSubdomainReturnsPublisherErrors(t *testing.T) {
	expectedErr := errors.New("dbus connection lost")
	callCount := 0
	deps := subdomainDeps{
		reconnectDelay: time.Millisecond,
		listenAndServe: func(_ context.Context, _ subdomainPublisher, _ string, _ uint32) error {
			callCount++
			return publisherFailure(expectedErr)
		},
	}

	err := runSubdomainWith(context.Background(), &fakePublisher{}, "lab.local.", 600, deps)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("runSubdomainWith error = %v, want wrapped %v", err, expectedErr)
	}

	if callCount != 1 {
		t.Fatalf("listenAndServe call count = %d, want 1", callCount)
	}
}

func TestRunSubdomainStopsWaitingWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callCount := 0
	deps := subdomainDeps{
		reconnectDelay: 1 * time.Hour,
		listenAndServe: func(_ context.Context, _ subdomainPublisher, _ string, _ uint32) error {
			callCount++
			return listenerFailure(errors.New("socket died"))
		},
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	startedAt := time.Now()
	err := runSubdomainWith(ctx, &fakePublisher{}, "lab.local.", 600, deps)
	if err != nil {
		t.Fatalf("runSubdomainWith returned error: %v", err)
	}

	if callCount != 1 {
		t.Fatalf("listenAndServe call count = %d, want 1", callCount)
	}

	if time.Since(startedAt) >= 1*time.Second {
		t.Fatal("runSubdomain did not stop promptly after context cancellation")
	}
}

func TestJoinMulticastAllInterfacesNoPanic(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	listenAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve listen address: %v", err)
	}

	conn, err := net.ListenUDP("udp4", listenAddr)
	if err != nil {
		t.Fatalf("listen UDP: %v", err)
	}
	defer conn.Close()

	pc := ipv4.NewPacketConn(conn)

	joinMulticastAllInterfaces(pc)
}
