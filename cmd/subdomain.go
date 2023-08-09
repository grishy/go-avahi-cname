package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/grishy/go-avahi-cname/avahi"
	"github.com/miekg/dns"
	"github.com/urfave/cli/v2"
)

type dnsMsg struct {
	msg dns.Msg
	err error
}

// listen creates a UDP connection to multicast for listening to DNS messages.
func listen() (*net.UDPConn, error) {
	addr := &net.UDPAddr{
		IP:   net.ParseIP("224.0.0.251"),
		Port: 5353,
	}

	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// reader reads DNS messages from the UDP connection. Assume that context is canceled before closing the connection.
func reader(ctx context.Context, conn *net.UDPConn) chan *dnsMsg {
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
					log.Println("Closing reader")
					close(msgCh)
					return
				}

				dnsMsg.err = errors.Join(dnsMsg.err, fmt.Errorf("failed to read from UDP from %s: %w", remoteAddress, err))
				msgCh <- dnsMsg
				return
			}

			if err := dnsMsg.msg.Unpack(buf[:bytesRead]); err != nil {
				dnsMsg.err = errors.Join(dnsMsg.err, fmt.Errorf("failed to unpack message: %w", err))
				msgCh <- dnsMsg
				continue
			}

			msgCh <- dnsMsg
		}
	}()

	return msgCh
}

// selectQuestion filters and selects questions with the given FQDN suffix.
func selectQuestion(fqdn string, qs []dns.Question) (res []string) {
	suffix := "." + fqdn
	for _, q := range qs {
		if strings.HasSuffix(q.Name, suffix) {
			res = append(res, q.Name)
		}
	}

	return res
}

// runSubdomain starts listening for DNS messages, filters relevant questions, and publishes corresponding CNAMEs.
func runSubdomain(ctx context.Context, publisher *avahi.Publisher, fqdn string, ttl uint32) error {
	log.Printf("FQDN: %s", fqdn)

	log.Println("Create connection to multicast")
	conn, err := listen()
	if err != nil {
		return fmt.Errorf("failed to create connection: %w", err)
	}

	msgCh := reader(ctx, conn)

	go func() {
		<-ctx.Done()
		fmt.Println() // Add new line after ^C
		log.Println("Closing connection")
		if err := conn.Close(); err != nil {
			log.Printf("Failed to close connection: %v", err)
		}
	}()

	log.Println("Start listening")
	for m := range msgCh {
		msg := m.msg
		if m.err != nil {
			log.Printf("Error: %v", m.err)
			continue
		}

		found := selectQuestion(fqdn, msg.Question)

		if len(found) > 0 {
			if err := publisher.PublishCNAMES(found, ttl); err != nil {
				log.Printf("Failed to publish CNAMEs: %v", err)
				continue
			}
		}
	}

	return nil
}

func CmdSubdomain(ctx context.Context) *cli.Command {
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
				Usage:       "FQDN which will be used for CNAME. If empty, will be used current FQDN",
				DefaultText: "hostname.local.",
			},
		},
		Action: func(cCtx *cli.Context) error {
			ttl := uint32(cCtx.Uint("ttl"))
			fqdn := cCtx.String("fqdn")

			log.Println("Creating publisher")
			publisher, err := avahi.NewPublisher()
			if err != nil {
				return fmt.Errorf("failed to create publisher: %w", err)
			}
			defer publisher.Close()

			if fqdn == "" {
				log.Println("Getting FQDN from Avahi")
				fqdn = publisher.Fqdn()
			}

			return runSubdomain(ctx, publisher, fqdn, ttl)
		},
	}
}
