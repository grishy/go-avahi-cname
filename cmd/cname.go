package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/miekg/dns"
	"github.com/urfave/cli/v2"

	"github.com/grishy/go-avahi-cname/avahi"
)

// formatCname formats CNAMEs by ensuring they are fully qualified domain names (FQDNs).
func formatCname(hostnameFqdn string, cnames []string) []string {
	slog.Info("formatting CNAMEs")

	formattedCnames := make([]string, len(cnames))
	for i, cname := range cnames {
		if !dns.IsFqdn(cname) {
			formattedCnames[i] = dns.Fqdn(cname + "." + hostnameFqdn)
			slog.Info("formatted CNAME", "cname", formattedCnames[i], "note", "added FQDN")
		} else {
			formattedCnames[i] = cname
			slog.Info("formatted CNAME", "cname", cname)
		}
	}

	return formattedCnames
}

// publishLoop handles the continuous publishing of CNAME records.
func publishing(ctx context.Context, publisher *avahi.Publisher, cnames []string, ttl, interval uint32) error {
	slog.Info("publishing CNAMEs", "interval", interval, "ttl", ttl)

	resendDuration := time.Duration(interval) * time.Second
	ticker := time.NewTicker(resendDuration)
	defer ticker.Stop()

	// Publish immediately
	// https://github.com/golang/go/issues/17601
	if err := publisher.PublishCNAMES(cnames, ttl); err != nil {
		return fmt.Errorf("failed to publish CNAMEs: %w", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := publisher.PublishCNAMES(cnames, ttl); err != nil {
				return fmt.Errorf("failed to publish CNAMEs: %w", err)
			}
		case <-ctx.Done():
			fmt.Println() // Add new line after ^C
			slog.Info("closing publisher")
			publisher.Close()
			return nil
		}
	}
}

// runCname sets up and starts the CNAME publishing process.
func runCname(
	ctx context.Context,
	publisher *avahi.Publisher,
	cnames []string,
	fqdn string,
	ttl, interval uint32,
) error {
	slog.Info("running CNAME publisher", "fqdn", fqdn)

	formattedCname := formatCname(fqdn, cnames)
	return publishing(ctx, publisher, formattedCname, ttl, interval)
}

func Cname(ctx context.Context) *cli.Command {
	return &cli.Command{
		Name:  "cname",
		Usage: "Announce CNAME records for host via avahi-daemon",
		Flags: []cli.Flag{
			&cli.UintFlag{
				Name:    "ttl",
				Value:   600,
				EnvVars: []string{"TTL"},
				Usage:   "TTL of CNAME record in seconds. How long they will be valid.",
			},
			&cli.UintFlag{
				Name:    "interval",
				Value:   300,
				EnvVars: []string{"INTERVAL"},
				Usage:   "interval of publishing CNAME records in seconds. How often to send records to other machines.",
			},
			&cli.StringFlag{
				Name:        "fqdn",
				EnvVars:     []string{"FQDN"},
				Usage:       "where to redirect. If empty, the Avahi FQDN (current machine) will be used",
				DefaultText: "<hostname>.local.",
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

			slog.Info("creating publisher")
			publisher, err := avahi.NewPublisher()
			if err != nil {
				return fmt.Errorf("failed to create publisher: %w", err)
			}

			if fqdn == "" {
				slog.Info("getting FQDN from Avahi")
				fqdn = publisher.Fqdn()
			}

			return runCname(ctx, publisher, cnames, fqdn, ttl, interval)
		},
	}
}
