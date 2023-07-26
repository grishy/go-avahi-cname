package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/grishy/go-avahi-cname/publisher"
	"github.com/miekg/dns"
)

const TTL = uint32(60 * 10) // seconds

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	log.Println("Creating publisher")
	publisher, err := publisher.NewPublisher()
	if err != nil {
		log.Fatalf("Can't create publisher: %v", err)
	}

	log.Println("Formating CNAMEs:")
	cnames := os.Args[1:]
	for i, cname := range cnames {
		if !dns.IsFqdn(cname) {
			cnames[i] = dns.Fqdn(cname + "." + publisher.Fqdn())

			log.Printf("  > '%s' (added current FQDN)", cnames[i])
			continue
		}

		log.Printf("  > '%s'", cname)
	}

	resendDuration := time.Duration(TTL/2) * time.Second
	log.Printf("Publishing every %v and CNAME TTL=%ds.", resendDuration, TTL)

	// To start publishing immediately
	// https://github.com/golang/go/issues/17601
	publisher.PublishCNAMES(os.Args[1:], TTL)

	for {
		select {
		case <-time.Tick(resendDuration):
			publisher.PublishCNAMES(os.Args[1:], TTL)
		case <-ctx.Done():
			log.Println("Closing publisher...")
			if err := publisher.Close(); err != nil {
				log.Fatalf("Can't close publisher: %v", err)
			}
			os.Exit(0)
		}
	}
}
