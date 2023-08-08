package cmd

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/grishy/go-avahi-cname/avahi"
	"github.com/miekg/dns"
	"github.com/urfave/cli/v2"
)

func formatCname(hostnameFqdn string, cnames []string) []string {
	log.Println("Formating CNAMEs:")

	for i, cname := range cnames {
		if !dns.IsFqdn(cname) {
			cnames[i] = dns.Fqdn(cname + "." + hostnameFqdn)

			log.Printf("  > '%s' (added FQDN)", cnames[i])
			continue
		}

		log.Printf("  > '%s'", cname)
	}

	return cnames
}

func publishing(ctx context.Context, publisher *avahi.Publisher, cnames []string, ttl, interval uint32) error {
	log.Printf("Publishing every %ds and CNAME TTL %ds", interval, ttl)

	resendDuration := time.Duration(interval) * time.Second
	ticker := time.NewTicker(resendDuration)
	defer ticker.Stop()

	// To start publishing immediately
	// https://github.com/golang/go/issues/17601
	if err := publisher.PublishCNAMES(cnames, ttl); err != nil {
		return fmt.Errorf("can't publish CNAMEs: %w", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := publisher.PublishCNAMES(cnames, ttl); err != nil {
				return fmt.Errorf("can't publish CNAMEs: %w", err)
			}
		case <-ctx.Done():
			fmt.Println() // Add new line after ^C
			log.Println("Closing publisher")
			publisher.Close()
			return nil
		}
	}
}

func runCname(ctx context.Context, publisher *avahi.Publisher, cnames []string, fqdn string, ttl, interval uint32) error {
	log.Printf("FQDN: %s", fqdn)

	formattedCname := formatCname(fqdn, cnames)
	return publishing(ctx, publisher, formattedCname, ttl, interval)
}

func CmdCname(ctx context.Context) *cli.Command {
	return &cli.Command{
		Name:  "cname",
		Usage: "Anounce CNAME records for host via avahi-daemon",
		Flags: []cli.Flag{
			&cli.UintFlag{
				Name:    "ttl",
				Value:   600,
				EnvVars: []string{"CNAME_TTL"},
				Usage:   "TTL of CNAME record in seconds. How long will they be valid.",
			},
			&cli.UintFlag{
				Name:    "interval",
				Value:   300,
				EnvVars: []string{"CNAME_INTERVAL"},
				Usage:   "Interval of publishing CNAME records in seconds. How often to send records to other machines.",
			},
			&cli.StringFlag{
				Name:        "fqdn",
				EnvVars:     []string{"SUBDOMAIN_FQDN"},
				Usage:       "Where to redirect. If empty, will be used avahi FQDN (current machine).",
				DefaultText: "hostname.local.",
			},
		},
		Action: func(cCtx *cli.Context) error {
			ttl := uint32(cCtx.Uint("ttl"))
			interval := uint32(cCtx.Uint("interval"))
			fqdn := cCtx.String("fqdn")
			cnames := cCtx.Args().Slice()

			if len(cnames) == 0 {
				return fmt.Errorf("at least one CNAME should be provided")
			}

			log.Println("Creating publisher")
			publisher, err := avahi.NewPublisher()
			if err != nil {
				return fmt.Errorf("can't create publisher: %w", err)
			}

			if fqdn == "" {
				log.Println("Getting FQDN from Avahi")
				fqdn = publisher.Fqdn()
			}

			return runCname(ctx, publisher, cnames, fqdn, ttl, interval)
		},
	}
}
