package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/urfave/cli/v3"
)

func main() {
	var logger log.Logger
	logger = log.NewLogfmtLogger(os.Stdout)
	logger = log.With(logger, "ts", log.DefaultTimestampUTC)
	logger = log.With(logger, "caller", log.DefaultCaller)

	cmd := cli.Command{
		Name: "sentry-stub-server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "listen-addr",
				Usage: "The address to listen on",
				Value: ":3000",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			listenAddr := c.String("listen-addr")

			level.Info(logger).Log("msg", "starting server", "addr", listenAddr)

			http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				w.Write([]byte("<h1>" + c.Name + "</h1>"))
			})
			http.HandleFunc("POST /api/{project}/envelope/", func(w http.ResponseWriter, r *http.Request) {
				level.Info(logger).Log("msg", "received envelope", "project", r.PathValue("project"))
				w.WriteHeader(200)
				w.Write([]byte(`{"status":"ok"}`))
			})

			return http.ListenAndServe(listenAddr, nil)
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Println(err)
	}
}
