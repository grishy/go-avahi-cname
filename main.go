package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/earthboundkid/versioninfo/v2"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	"github.com/urfave/cli/v2"

	"github.com/grishy/go-avahi-cname/cmd"
)

const forceExitTimeout = 3 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go handleGracefulShutdown(ctx)

	if err := run(ctx); err != nil {
		fmt.Println("Error:")
		fmt.Printf(" > %+v\n", err)
	}
}

// run starts the CLI application
func run(ctx context.Context) error {
	app := &cli.App{
		Name:    "go-avahi-cname",
		Usage:   "A tool for publishing CNAME records with Avahi",
		Version: versioninfo.Short(),
		Authors: []*cli.Author{{
			Name: "grishy",
		}},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"d"},
				Usage:   "Enable debug logging",
				EnvVars: []string{"DEBUG"},
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

// handleGracefulShutdown handles graceful shutdown with timeout
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

// setupLogger configures the global logger with appropriate settings
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
