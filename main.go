package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/grishy/go-avahi-cname/cmd"
	"github.com/urfave/cli/v2"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	app := &cli.App{
		Name:  "go-avahi-cname",
		Usage: "make an explosive entrance",
		Commands: []*cli.Command{
			cmd.CmdCname(ctx),
			cmd.CmdSubdomain(ctx),
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
