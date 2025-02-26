package sentrytunnel

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/urfave/cli/v3"
)

var (
	Name                = "sentrytunnel"
	Version             = "dev"
	HttpHeaderUserAgent = Name + "/" + Version
)

type sentrytunnel struct {
	ListenAddress            string
	TunnelURLPath            string
	LoggingLevel             string
	AccessControlAllowOrigin []string
	TrustedSentryDSN         []string
}

var SentryTunnel = &sentrytunnel{}

func Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := cli.Command{
		Name:    Name,
		Usage:   "A tunneling service for Sentry",
		Version: Version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "listen-addr",
				Usage:       "The address to listen on",
				Value:       ":8080",
				Destination: &SentryTunnel.ListenAddress,
			},
			&cli.StringFlag{
				Name:        "tunnel-path",
				Usage:       "The URL path for the tunnel to process the requests",
				Value:       "/tunnel",
				Destination: &SentryTunnel.TunnelURLPath,
			},
			&cli.StringFlag{
				Name:        "log-level",
				Usage:       "Set the log level",
				Value:       "info",
				Destination: &SentryTunnel.LoggingLevel,
			},
			&cli.StringSliceFlag{
				Name:        "allowed-origin",
				Usage:       "A list of origins that are allowed to access the tunnel. e.g. https://example.com",
				Destination: &SentryTunnel.AccessControlAllowOrigin,
				Validator: func(s []string) error {
					for _, origin := range s {
						if origin == "*" {
							return nil
						}
						origin, err := url.Parse(origin)
						if err != nil {
							return fmt.Errorf("invalid origin: %s", origin)
						}
						if origin.Scheme == "" || origin.Host == "" {
							return fmt.Errorf("invalid origin: %s", origin)
						}
					}
					return nil
				},
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error { return action(ctx, c) },
	}

	return cmd.Run(ctx, os.Args)
}

func action(_ context.Context, _ *cli.Command) error {
	return ListenAndServe(SentryTunnel.ListenAddress)
}
