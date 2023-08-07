package cmd

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/godbus/dbus/v5"
	"github.com/holoplot/go-avahi"
	"github.com/miekg/dns"
	"github.com/urfave/cli/v2"
)

func getFqdn() (string, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return "nil", fmt.Errorf("can't connect to system bus: %v", err)
	}

	server, err := avahi.ServerNew(conn)
	if err != nil {
		return "nil", fmt.Errorf("can't create Avahi server: %v", err)
	}

	avahiFqdn, err := server.GetHostNameFqdn()
	if err != nil {
		return "nil", fmt.Errorf("can't get FQDN from Avahi: %v", err)
	}

	return avahiFqdn, nil
}

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

func readMessage(ctx context.Context, conn *net.UDPConn) (chan *dns.Msg, chan error) {
	buf := make([]byte, 1500)

	msgCh := make(chan *dns.Msg)
	errCh := make(chan error)

	go func() {
		<-ctx.Done()
		fmt.Println() // Add new line after ^C
		log.Println("Closing reader")

		conn.Close()
		close(msgCh)
		close(errCh)
	}()

	go func() {
		for {
			read, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				errCh <- fmt.Errorf("can't read from UDP from %s: %w", addr, err)
				continue
			}

			msg := new(dns.Msg)
			if err := msg.Unpack(buf[:read]); err != nil {
				errCh <- fmt.Errorf("can't unpack message from %s: %w", addr, err)
				continue
			}

			msgCh <- msg
		}
	}()

	return msgCh, errCh
}

func runSubdomain(ctx context.Context, fqdn string) error {
	log.Println("Starting subdomain...")

	l, err := listen()
	if err != nil {
		return fmt.Errorf("can't listen: %w", err)
	}
	defer l.Close()

	msgCh, errCh := readMessage(ctx, l)
	// TODO: Pack to one struct, msg and error
	_ = errCh

	for msg := range msgCh {
		if len(msg.Question) > 0 {
			log.Printf("Received question: %s", msg.Question[0].Name)
		}
	}

	return nil
}

func CmdSubdomain(ctx context.Context) *cli.Command {
	return &cli.Command{
		Name:  "subdomain",
		Usage: "Listen for all queries and publish CNAMEs for subdomains",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "fqdn",
				EnvVars:     []string{"SUBDOMAIN_FQDN"},
				Usage:       "FQDN which will be used for CNAME. If empty, will be used current FQDN",
				DefaultText: "hostname.local.",
			},
		},
		Action: func(cCtx *cli.Context) error {
			fqdn := cCtx.String("fqdn")

			if fqdn == "" {
				var err error
				fqdn, err = getFqdn()
				if err != nil {
					return fmt.Errorf("can't get FQDN: %w", err)
				}
			}

			log.Printf("FQDN: %s", fqdn)

			return runSubdomain(ctx, fqdn)
		},
	}
}
