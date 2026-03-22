package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/urfave/cli/v2"

	"github.com/grishy/go-avahi-cname/avahi"
)

const reconnectDelay = 5 * time.Second

type subdomainPublisher interface {
	PublishCNAMES(cnames []string, ttl uint32) error
}

type subdomainFailureKind uint8

const (
	subdomainFailureListener subdomainFailureKind = iota + 1
	subdomainFailurePublisher
)

type subdomainError struct {
	kind  subdomainFailureKind
	cause error
}

func (e *subdomainError) Error() string {
	return e.cause.Error()
}

func (e *subdomainError) Unwrap() error {
	return e.cause
}

type subdomainDeps struct {
	listenAndServe func(context.Context, subdomainPublisher, string, uint32) error
	reconnectDelay time.Duration
}

var defaultSubdomainDeps = subdomainDeps{
	listenAndServe: listenAndServe,
	reconnectDelay: reconnectDelay,
}

func listenerFailure(err error) error {
	return &subdomainError{
		kind:  subdomainFailureListener,
		cause: err,
	}
}

func publisherFailure(err error) error {
	return &subdomainError{
		kind:  subdomainFailurePublisher,
		cause: err,
	}
}

func isListenerFailure(err error) bool {
	var failure *subdomainError
	if !errors.As(err, &failure) {
		return false
	}

	return failure.kind == subdomainFailureListener
}

func matchingQuestionNames(fqdn string, questions []dns.Question) []string {
	suffix := strings.ToLower("." + fqdn)
	names := make([]string, 0, len(questions))

	for _, question := range questions {
		if strings.HasSuffix(strings.ToLower(question.Name), suffix) {
			names = append(names, question.Name)
		}
	}

	return names
}

func runSubdomain(ctx context.Context, publisher subdomainPublisher, fqdn string, ttl uint32) error {
	return runSubdomainWith(ctx, publisher, fqdn, ttl, defaultSubdomainDeps)
}

func runSubdomainWith(
	ctx context.Context,
	publisher subdomainPublisher,
	fqdn string,
	ttl uint32,
	deps subdomainDeps,
) error {
	slog.Info("running subdomain publisher", "fqdn", fqdn)

	for ctx.Err() == nil {
		sessionErr := deps.listenAndServe(ctx, publisher, fqdn, ttl)
		if sessionErr == nil {
			return nil
		}
		if !isListenerFailure(sessionErr) {
			return sessionErr
		}

		slog.Warn("listener failed, reconnecting", "error", sessionErr, "delay", deps.reconnectDelay)

		select {
		case <-time.After(deps.reconnectDelay):
		case <-ctx.Done():
			return nil
		}
	}

	return nil
}

func listenAndServe(ctx context.Context, publisher subdomainPublisher, fqdn string, ttl uint32) error {
	slog.Info("creating connection to multicast")

	conn, pc, err := listen()
	if err != nil {
		return listenerFailure(fmt.Errorf("failed to create connection: %w", err))
	}
	defer conn.Close()

	innerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	context.AfterFunc(innerCtx, func() {
		if ctx.Err() != nil {
			fmt.Println()
		}

		slog.Info("closing connection")
		if closeErr := conn.Close(); closeErr != nil {
			slog.Debug("connection close error", "error", closeErr)
		}
	})

	go periodicRejoin(innerCtx, pc)

	slog.Info("start listening")

	var lastErr error
	for message := range reader(innerCtx, conn) {
		if message.err != nil {
			lastErr = message.err
			slog.Error("error processing message", "error", message.err)
			continue
		}

		names := matchingQuestionNames(fqdn, message.msg.Question)
		if len(names) == 0 {
			continue
		}

		slog.Debug("publishing matching CNAMEs", "count", len(names))
		if publishErr := publisher.PublishCNAMES(names, ttl); publishErr != nil {
			return publisherFailure(fmt.Errorf("failed to publish CNAMEs: %w", publishErr))
		}
	}

	if ctx.Err() != nil {
		return nil
	}
	if lastErr != nil {
		return listenerFailure(fmt.Errorf("mDNS reader stopped unexpectedly: %w", lastErr))
	}

	return listenerFailure(errors.New("mDNS reader stopped unexpectedly"))
}

// Subdomain returns the CLI command for the subdomain publisher.
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
			ttlUint := cCtx.Uint("ttl")
			maxUint32 := uint64(^uint32(0))
			if uint64(ttlUint) > maxUint32 {
				return fmt.Errorf("ttl value too large: %d (max allowed: %d)", ttlUint, maxUint32)
			}

			ttl := uint32(ttlUint)
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
