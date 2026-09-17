package main

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/urfave/cli/v3"
)

// Keep each invocation and its observable parsing results together.
//
//nolint:gocognit
func TestCLICompatibility(t *testing.T) {
	originalLogger := slog.Default()
	t.Cleanup(func() { slog.SetDefault(originalLogger) })

	tests := []struct {
		name     string
		args     []string
		env      map[string]string
		ttl      uint
		interval uint
		fqdn     string
		debug    bool
		names    []string
	}{
		{
			name:     "cname defaults",
			args:     []string{"cname", "first", "second"},
			ttl:      600,
			interval: 300,
			names:    []string{"first", "second"},
		},
		{
			name: "subdomain defaults",
			args: []string{"subdomain"},
			ttl:  600,
		},
		{
			name:     "environment",
			args:     []string{"cname", "first"},
			env:      map[string]string{"TTL": "42", "INTERVAL": "12", "FQDN": "env.local.", "DEBUG": "true"},
			ttl:      42,
			interval: 12,
			fqdn:     "env.local.",
			debug:    true,
			names:    []string{"first"},
		},
		{
			name: "flags override environment",
			args: []string{
				"--debug=false",
				"cname",
				"--ttl",
				"2",
				"--interval",
				"3",
				"--fqdn",
				"flag.local.",
				"first",
			},
			env:      map[string]string{"TTL": "42", "INTERVAL": "12", "FQDN": "env.local.", "DEBUG": "true"},
			ttl:      2,
			interval: 3,
			fqdn:     "flag.local.",
			names:    []string{"first"},
		},
		{
			name:  "debug alias and subdomain flags",
			args:  []string{"-d", "subdomain", "--ttl=2", "--fqdn=flag.local."},
			ttl:   2,
			fqdn:  "flag.local.",
			debug: true,
		},
		{
			name:     "flags after a name are parsed",
			args:     []string{"cname", "first", "--ttl", "2"},
			ttl:      2,
			interval: 300,
			names:    []string{"first"},
		},
		{
			name:     "double dash",
			args:     []string{"cname", "--", "--help"},
			ttl:      600,
			interval: 300,
			names:    []string{"--help"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range []string{"TTL", "INTERVAL", "FQDN", "DEBUG"} {
				t.Setenv(key, tt.env[key])
			}

			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()

			app := newCommand()
			app.Writer = io.Discard
			app.ErrWriter = io.Discard
			var parsed *cli.Command
			var actionContext context.Context
			for _, command := range app.Commands {
				command.Action = func(ctx context.Context, c *cli.Command) error {
					parsed, actionContext = c, ctx
					return nil
				}
			}

			if err := app.Run(ctx, append([]string{appName}, tt.args...)); err != nil {
				t.Fatal(err)
			}
			if parsed == nil {
				t.Fatal("command action was not called")
			}

			deadline, ok := actionContext.Deadline()
			expected, _ := ctx.Deadline()
			if !ok || !deadline.Equal(expected) {
				t.Error("action lost the shutdown context deadline")
			}
			if parsed.Uint("ttl") != tt.ttl || parsed.String("fqdn") != tt.fqdn || parsed.Bool("debug") != tt.debug {
				t.Errorf(
					"flags: ttl=%d fqdn=%q debug=%v",
					parsed.Uint("ttl"),
					parsed.String("fqdn"),
					parsed.Bool("debug"),
				)
			}
			if parsed.Name == "cname" && parsed.Uint("interval") != tt.interval {
				t.Errorf("interval=%d, want %d", parsed.Uint("interval"), tt.interval)
			}
			if !slices.Equal(parsed.Args().Slice(), tt.names) {
				t.Errorf("names=%q, want %q", parsed.Args().Slice(), tt.names)
			}
			if slog.Default().Enabled(actionContext, slog.LevelDebug) != tt.debug {
				t.Error("debug flag was not applied to the logger")
			}
		})
	}
}
