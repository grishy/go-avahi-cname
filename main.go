package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	goversion "github.com/caarlos0/go-version"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	"github.com/urfave/cli/v2"

	"github.com/grishy/go-avahi-cname/cmd"
)

const (
	forceExitTimeout = 5 * time.Second
	appName          = "go-avahi-cname"
)

// Version information set during build
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go handleGracefulShutdown(ctx)

	if err := run(ctx); err != nil {
		fmt.Println("Error:")
		fmt.Printf(" > %+v\n", err)
		os.Exit(1)
	}

	// To avoid graceful shutdown timeout
	os.Exit(0)
}

// run starts and configures the CLI application
func run(ctx context.Context) error {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Print(buildVersion().String())
	}

	app := &cli.App{
		Name:    appName,
		Usage:   "Create local domain names using Avahi daemon",
		Version: version,
		Description: `A tool that helps you create local domain names for your computer by using the Avahi daemon.

It works in two ways:
1. Automatic mode (use 'subdomain' command): 
   Any subdomain you try to use (like myapp.computer.local) will automatically point to your computer

2. Manual mode (use 'cname' command):
   You can create your own domain names that point to your computer and keep them active

Need help? Visit https://github.com/grishy/go-avahi-cname`,
		Authors: []*cli.Author{{
			Name:  "Sergei G.",
			Email: "mail@grishy.dev",
		}},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"d"},
				Usage:   "enable debug logging",
				EnvVars: []string{"DEBUG"},
				Value:   false,
			},
		},
		Before: setupLogger,
		Commands: []*cli.Command{
			cmd.Cname(ctx),
			cmd.Subdomain(ctx),
		},
	}

	return app.Run(os.Args)
}

// handleGracefulShutdown manages graceful shutdown with timeout
func handleGracefulShutdown(ctx context.Context) {
	<-ctx.Done()
	slog.Info("initiating graceful shutdown...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), forceExitTimeout)
	defer cancel()

	// Wait for cleanup or timeout
	<-shutdownCtx.Done()
	if shutdownCtx.Err() == context.DeadlineExceeded {
		slog.Error("failed to shutdown gracefully, forcing exit")
		os.Exit(1)
	}
}

// setupLogger configures the global structured logger with appropriate settings
func setupLogger(c *cli.Context) error {
	w := os.Stdout
	level := slog.LevelInfo
	if c.Bool("debug") {
		level = slog.LevelDebug
	}

	slog.SetDefault(slog.New(
		tint.NewHandler(w, &tint.Options{
			Level:      level,
			NoColor:    !isatty.IsTerminal(w.Fd()),
			TimeFormat: time.TimeOnly,
		}),
	))

	return nil
}

// buildVersion constructs version information for the application
func buildVersion() goversion.Info {
	return goversion.GetVersionInfo(
		func(i *goversion.Info) {
			i.GitCommit = commit
			i.BuildDate = date
			i.GitVersion = version
		},
	)
}
