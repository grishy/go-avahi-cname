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

type dnsMsg struct {
	msg dns.Msg
	err error
}

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

				dnsMsg.err = errors.Join(dnsMsg.err, fmt.Errorf("can't read from UDP from %s: %w", remoteAddress, err))
				msgCh <- dnsMsg
				return
			}

			if err := dnsMsg.msg.Unpack(buf[:bytesRead]); err != nil {
				dnsMsg.err = errors.Join(dnsMsg.err, fmt.Errorf("can't unpack message: %w", err))
				msgCh <- dnsMsg
				continue
			}

			msgCh <- dnsMsg
		}
	}()

	return msgCh
}

func selectQuestion(fqdn string, qs []dns.Question) (res []string) {
	for _, q := range qs {
		if strings.HasSuffix(q.Name, fqdn) {
			res = append(res, q.Name)
		}
	}

	return res
}

func runSubdomain(ctx context.Context, publisher *avahi.Publisher, fqdn string, ttl uint32) error {
	log.Printf("FQDN: %s", fqdn)

	log.Println("Create connection to multicast")
	conn, err := listen()
	if err != nil {
		return fmt.Errorf("can't create connection: %w", err)
	}

	msgCh := reader(ctx, conn)

	go func() {
		<-ctx.Done()
		fmt.Println() // Add new line after ^C
		log.Println("Closing connection")
		if err := conn.Close(); err != nil {
			log.Printf("Can't close connection: %v", err)
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
				log.Printf("Can't publish CNAMEs: %v", err)
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
				EnvVars: []string{"CNAME_TTL"},
				Usage:   "TTL of CNAME record in seconds",
			},
			&cli.StringFlag{
				Name:        "fqdn",
				EnvVars:     []string{"SUBDOMAIN_FQDN"},
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
				return fmt.Errorf("can't create publisher: %w", err)
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
