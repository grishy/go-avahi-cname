package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/grishy/go-avahi-cname/publisher"
	"github.com/miekg/dns"
	"github.com/urfave/cli/v2"
)

func formatCname(hostnameFqdn string, cnames []string) []string {
	log.Println("Formating CNAMEs:")

	for i, cname := range cnames {
		if !dns.IsFqdn(cname) {
			cnames[i] = dns.Fqdn(cname + "." + hostnameFqdn)

			log.Printf("  > '%s' (added current FQDN)", cnames[i])
			continue
		}

		log.Printf("  > '%s'", cname)
	}

	return cnames
}

func publishing(ctx context.Context, publisher *publisher.Publisher, ttl, interval uint32, cnames []string) {
	resendDuration := time.Duration(interval) * time.Second
	log.Printf("Publishing every %v and CNAME TTL=%ds.", resendDuration, ttl)

	// To start publishing immediately
	// https://github.com/golang/go/issues/17601
	if err := publisher.PublishCNAMES(cnames, ttl); err != nil {
		log.Fatalf("can't publish CNAMEs: %v", err)
	}

	for {
		select {
		case <-time.Tick(resendDuration):
			if err := publisher.PublishCNAMES(cnames, ttl); err != nil {
				log.Fatalf("can't publish CNAMEs: %v", err)
			}
		case <-ctx.Done():
			fmt.Println()
			log.Println("Closing publisher...")
			if err := publisher.Close(); err != nil {
				log.Fatalf("Can't close publisher: %v", err)
			}
			os.Exit(0)
		}
	}
}

func cnameCmd(ctx context.Context, ttl, interval uint32, cnames []string) {

}

func CmdCname(ctx context.Context) *cli.Command {
	return &cli.Command{
		Name:  "cname",
		Usage: "anonse CNAME via Avahi",
		Flags: []cli.Flag{
			&cli.UintFlag{
				Name:    "ttl",
				Value:   600,
				EnvVars: []string{"CNAME_TTL"},
				Usage:   "TTL of CNAME record in seconds",
			},
			&cli.UintFlag{
				Name:    "interval",
				Value:   300,
				EnvVars: []string{"CNAME_INTERVAL"},
				Usage:   "Interval of sending CNAME record in seconds",
			},
		},
		Action: func(cCtx *cli.Context) error {
			ttl := uint32(cCtx.Uint("ttl"))
			interval := uint32(cCtx.Uint("interval"))
			cnames := cCtx.Args().Slice()

			if len(cnames) == 0 {
				log.Fatal("CNAMEs are not specified")
			}

			log.Println("Creating publisher")
			publisher, err := publisher.NewPublisher()
			if err != nil {
				log.Fatalf("Can't create publisher: %v", err)
			}

			formattedCname := formatCname(publisher.Fqdn(), cnames)
			publishing(ctx, publisher, ttl, interval, formattedCname)
			return nil
		},
	}
}
