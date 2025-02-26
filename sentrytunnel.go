package sentrytunnel

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/socheatsok78/sentrytunnel/envelope"
	"github.com/urfave/cli/v3"
)

var (
	Name                  = "sentrytunnel"
	Version               = "dev"
	HttpHeaderUserAgent   = Name + "/" + Version
	HttpHeaderContentType = "application/x-sentry-envelope"
)

type SentryTunnel struct {
	ListenAddress            string
	TunnelURLPath            string
	LoggingLevel             string
	AccessControlAllowOrigin []string
	TrustedSentryDSN         []string
}

// SentryTunnel
var sentrytunnel = &SentryTunnel{}

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
				Destination: &sentrytunnel.ListenAddress,
			},
			&cli.StringFlag{
				Name:        "tunnel-path",
				Usage:       "The URL path for the tunnel to process the requests",
				Value:       "/tunnel",
				Destination: &sentrytunnel.TunnelURLPath,
			},
			&cli.StringFlag{
				Name:        "log-level",
				Usage:       "Set the log level",
				Value:       "info",
				Destination: &sentrytunnel.LoggingLevel,
			},
			&cli.StringSliceFlag{
				Name:        "allowed-origin",
				Usage:       "A list of origins that are allowed to access the tunnel. e.g. https://example.com",
				Destination: &sentrytunnel.AccessControlAllowOrigin,
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
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// CORS
	r.Use(cors.Handler((cors.Options{
		AllowedOrigins: sentrytunnel.AccessControlAllowOrigin,
		Debug:          true,
	})))

	// Heartbeat
	r.Use(middleware.Heartbeat("/heartbeat"))

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	r.Use(middleware.Timeout(60 * time.Second))

	// Routes
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("welcome"))
	})

	// Sentry Tunnel Route
	r.Route("/tunnel", func(r chi.Router) {
		r.Use(SentryTunnelCtx)
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			// Get the DSN and payload from the context
			dsn := r.Context().Value("dsn").(*sentry.Dsn)
			payload := r.Context().Value("payload").(*envelope.Envelope)

			// Sending the payload to Sentry
			_, err := http.Post(dsn.GetAPIURL().String(), HttpHeaderContentType, bytes.NewReader(payload.Bytes()))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Send the ok response
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("{\"status\": \"ok\"}"))
		})
	})

	// Start the server
	return http.ListenAndServe(sentrytunnel.ListenAddress, r)
}

func SentryTunnelCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Read the envelope from the request body
		envelopeBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Parse the envelope
		payload, err := envelope.Parse(envelopeBytes)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Parse the DSN
		dsn, err := sentry.NewDsn(payload.Header.DSN)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Set the DSN and payload to the context
		ctx = context.WithValue(ctx, "dsn", dsn)
		ctx = context.WithValue(ctx, "payload", payload)

		// Call the next handler
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
