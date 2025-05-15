package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/miekg/dns"
	"github.com/urfave/cli/v2"

	"github.com/grishy/go-avahi-cname/avahi"
)

type dnsMsg struct {
	msg dns.Msg
	err error
}

// listen creates a UDP connection to multicast for listening to DNS messages.
func listen() (*net.UDPConn, error) {
	slog.Debug("creating multicast UDP connection", "ip", "224.0.0.251", "port", 5353)
	addr := &net.UDPAddr{
		IP:   net.ParseIP("224.0.0.251"),
		Port: 5353,
	}

	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		return nil, err
	}

	slog.Debug("multicast UDP connection created successfully")
	return conn, nil
}

// reader reads DNS messages from the UDP connection. Assume that context is canceled before closing the connection.
func reader(ctx context.Context, conn *net.UDPConn) chan *dnsMsg {
	slog.Debug("starting DNS message reader")
	buf := make([]byte, 1500)

	msgCh := make(chan *dnsMsg)

	go func() {
		for {
			dnsMsg := &dnsMsg{
				msg: dns.Msg{},
			}

			bytesRead, remoteAddress, err := conn.ReadFromUDP(buf)
			if err != nil {
				if ctx.Err() != nil {
					slog.Info("closing reader")
					close(msgCh)
					return
				}

				dnsMsg.err = errors.Join(
					dnsMsg.err,
					fmt.Errorf("failed to read from UDP from %s: %w", remoteAddress, err),
				)
				msgCh <- dnsMsg
				return
			}
			slog.Debug("received UDP message", "bytes", bytesRead, "from", remoteAddress)

			if err := dnsMsg.msg.Unpack(buf[:bytesRead]); err != nil {
				dnsMsg.err = errors.Join(dnsMsg.err, fmt.Errorf("failed to unpack message: %w", err))
				msgCh <- dnsMsg
				continue
			}
			slog.Debug("unpacked DNS message successfully")

			msgCh <- dnsMsg
		}
	}()

	return msgCh
}

// selectQuestion filters and selects questions with the given FQDN suffix.
func selectQuestion(fqdn string, qs []dns.Question) (res []string) {
	suffix := strings.ToLower("." + fqdn)
	slog.Debug("filtering DNS questions", "suffix", suffix, "questions", len(qs))

	for _, q := range qs {
		slog.Debug("processing question", "name", q.Name, "type", dns.TypeToString[q.Qtype])

		if strings.HasSuffix(strings.ToLower(q.Name), suffix) {
			slog.Debug("found matching question", "name", q.Name, "type", dns.TypeToString[q.Qtype])
			res = append(res, q.Name)
		}
	}

	slog.Debug("filtered questions", "matches", len(res))
	return res
}

// runSubdomain starts listening for DNS messages, filters relevant questions, and publishes corresponding CNAMEs.
func runSubdomain(ctx context.Context, publisher *avahi.Publisher, fqdn string, ttl uint32) error {
	slog.Info("running subdomain publisher", "fqdn", fqdn)

	slog.Info("creating connection to multicast")
	conn, err := listen()
	if err != nil {
		return fmt.Errorf("failed to create connection: %w", err)
	}

	msgCh := reader(ctx, conn)

	go func() {
		<-ctx.Done()
		fmt.Println() // Add new line after ^C
		slog.Info("closing connection")
		if err := conn.Close(); err != nil {
			slog.Error("failed to close connection", "error", err)
		}
	}()

	slog.Info("start listening")
	for m := range msgCh {
		msg := m.msg
		if m.err != nil {
			slog.Error("error processing message", "error", m.err)
			continue
		}
		slog.Debug("processing DNS message", "questions", len(msg.Question))

		found := selectQuestion(fqdn, msg.Question)

		if len(found) > 0 {
			slog.Debug("publishing matching CNAMEs", "count", len(found))
			if err := publisher.PublishCNAMES(found, ttl); err != nil {
				return fmt.Errorf("failed to publish CNAMEs: %w", err)
			}
		}
	}

	return nil
}

func Subdomain(ctx context.Context) *cli.Command {
	return &cli.Command{
		Name:  "subdomain",
		Usage: "Listen for all queries and publish CNAMEs for subdomains",
		Flags: []cli.Flag{
			&cli.UintFlag{
				Name:    "ttl",
				Value:   600,
				EnvVars: []string{"TTL"},
				Usage:   "TTL of CNAME record in seconds",
			},
			&cli.StringFlag{
				Name:        "fqdn",
				EnvVars:     []string{"FQDN"},
				Usage:       "FQDN which will be used for CNAME. If empty, will be used current FQDN from Avahi",
				DefaultText: "<hostname>.local.",
			},
		},
		Action: func(cCtx *cli.Context) error {
			ttl := uint32(cCtx.Uint("ttl"))
			fqdn := cCtx.String("fqdn")

			slog.Info("creating publisher")
			publisher, err := avahi.NewPublisher()
			if err != nil {
				return fmt.Errorf("failed to create publisher: %w", err)
			}
			defer publisher.Close()

			if fqdn == "" {
				slog.Info("getting FQDN from Avahi")
				fqdn = publisher.Fqdn()
			}

			return runSubdomain(ctx, publisher, fqdn, ttl)
		},
	}
}
