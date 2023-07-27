package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/grishy/go-avahi-cname/publisher"
	"github.com/miekg/dns"
)

const TTL = uint32(10 * 60) // in seconds

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

func publishing(ctx context.Context, publisher *publisher.Publisher, cnames []string) {
	resendDuration := time.Duration(TTL/2) * time.Second
	log.Printf("Publishing every %v and CNAME TTL=%ds.", resendDuration, TTL)

	// To start publishing immediately
	// https://github.com/golang/go/issues/17601
	if err := publisher.PublishCNAMES(cnames, TTL); err != nil {
		log.Fatalf("can't publish CNAMEs: %v", err)
	}

	for {
		select {
		case <-time.Tick(resendDuration):
			if err := publisher.PublishCNAMES(cnames, TTL); err != nil {
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

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	log.Println("Creating publisher")
	publisher, err := publisher.NewPublisher()
	if err != nil {
		log.Fatalf("Can't create publisher: %v", err)
	}

	cnames := formatCname(publisher.Fqdn(), os.Args[1:])
	publishing(ctx, publisher, cnames)
}
