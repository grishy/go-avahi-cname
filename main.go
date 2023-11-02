package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/carlmjohnson/versioninfo"
	"github.com/urfave/cli/v2"

	"github.com/grishy/go-avahi-cname/cmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer stop()

	app := &cli.App{
		Name:    "go-avahi-cname",
		Usage:   "A tool for publishing CNAME records with Avahi",
		Version: versioninfo.Short(),
		Commands: []*cli.Command{
			cmd.Cname(ctx),
			cmd.Subdomain(ctx),
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println("Error:")
		fmt.Printf(" > %+v\n", err)
	}
}
