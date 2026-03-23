package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"syscall"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/net/ipv4"
)

const (
	readTimeout             = 30 * time.Second
	multicastRejoinInterval = 60 * time.Second
	mdnsIPv4Addr            = "224.0.0.251"
	mdnsPort                = 5353
)

type dnsMsg struct {
	msg dns.Msg
	err error
}

func joinMulticastAllInterfaces(pc *ipv4.PacketConn) {
	ifaces, err := net.Interfaces()
	if err != nil {
		slog.Debug("failed to list network interfaces", "error", err)
		return
	}

	joined := 0
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 {
			continue
		}

		if joinErr := pc.JoinGroup(&iface, &net.UDPAddr{IP: net.ParseIP(mdnsIPv4Addr)}); joinErr != nil {
			// EADDRINUSE means we are already a member — count as success.
			if !errors.Is(joinErr, syscall.EADDRINUSE) {
				slog.Debug("failed to join multicast group on interface", "interface", iface.Name, "error", joinErr)
				continue
			}
		}

		joined++
		slog.Debug("joined multicast group on interface", "interface", iface.Name)
	}

	if joined == 0 {
		slog.Warn("no interfaces joined the multicast group")
	}
}

func periodicRejoin(ctx context.Context, pc *ipv4.PacketConn) {
	ticker := time.NewTicker(multicastRejoinInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			slog.Debug("periodic multicast re-join")
			joinMulticastAllInterfaces(pc)
		case <-ctx.Done():
			return
		}
	}
}

func listen() (*net.UDPConn, *ipv4.PacketConn, error) {
	addr := &net.UDPAddr{
		IP:   net.ParseIP(mdnsIPv4Addr),
		Port: mdnsPort,
	}

	// We share port 5353 with avahi-daemon, then join the group on every
	// eligible interface so routing quirks do not pin us to the wrong one.
	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		return nil, nil, fmt.Errorf("listen multicast UDP: %w", err)
	}

	pc := ipv4.NewPacketConn(conn)
	if loopbackErr := pc.SetMulticastLoopback(true); loopbackErr != nil {
		slog.Debug("failed to set multicast loopback", "error", loopbackErr)
	}

	joinMulticastAllInterfaces(pc)
	return conn, pc, nil
}

func reader(ctx context.Context, conn *net.UDPConn) <-chan *dnsMsg {
	buf := make([]byte, 1500)
	msgCh := make(chan *dnsMsg)

	go func() {
		defer close(msgCh)

		for {
			// The deadline keeps shutdown and stale-socket recovery bounded.
			if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
				slog.Debug("failed to set read deadline", "error", err)
			}

			bytesRead, remoteAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				if ctx.Err() != nil {
					return
				}

				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					continue
				}

				msgCh <- &dnsMsg{
					err: fmt.Errorf("failed to read from UDP from %s: %w", remoteAddr, err),
				}
				return
			}

			msg := &dnsMsg{}
			if unpackErr := msg.msg.Unpack(buf[:bytesRead]); unpackErr != nil {
				msg.err = fmt.Errorf("failed to unpack message: %w", unpackErr)
				msgCh <- msg
				continue
			}

			msgCh <- msg
		}
	}()

	return msgCh
}
