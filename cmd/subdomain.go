package cmd

import (
	"context"

	"github.com/urfave/cli/v2"
)

func CmdSubdomain(ctx context.Context) *cli.Command {
	return &cli.Command{
		Name:  "subdomain",
		Usage: "reply on all subdomains queries",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "domain",
				Value:   "",
				EnvVars: []string{"SUBDOMAIN_DOMAIN"},
				Usage:   "Domain name to publish",
			},
			&cli.StringSliceFlag{
				Name:    "ifaces",
				Value:   nil,
				EnvVars: []string{"SUBDOMAIN_IFACES"},
				Usage:   "Interface for listening and publishing",
			},
			&cli.BoolFlag{
				Name:    "use-avahi",
				Value:   true,
				EnvVars: []string{"SUBDOMAIN_USE_AVAHI"},
				Usage:   "Use avahi for sending CNAMEs or plain DNS",
			},
		},
		Action: func(cCtx *cli.Context) error {

			return nil
		},
	}
}
