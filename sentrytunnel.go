package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/avast/retry-go/v4"
	humanize "github.com/dustin/go-humanize"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/google/uuid"
	"github.com/socheatsok78/sentrytunnel/envelope"
	"github.com/urfave/cli/v3"
)

var (
	Name                = "sentrytunnel"
	Version             = "dev"
	HttpHeaderUserAgent = Name + "/" + Version
)

var (
	logger       log.Logger
	sentrytunnel = &SentryTunnel{}
)

type SentryTunnel struct {
	ListenAddr               string
	LogLevel                 string
	AccessControlAllowOrigin []string
	TrustedSentryDSN         []string
}

func init() {
	// Set up logging
	logger = log.NewLogfmtLogger(os.Stdout)
	logger = log.With(logger, "ts", log.DefaultTimestampUTC)
	logger = log.With(logger, "caller", log.DefaultCaller)
}

func main() {
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
				Destination: &sentrytunnel.ListenAddr,
			},
			&cli.StringFlag{
				Name:        "log-level",
				Usage:       "Set the log level",
				Value:       "info",
				Destination: &sentrytunnel.LogLevel,
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
			&cli.StringSliceFlag{
				Name:        "trusted-sentry-dsn",
				Usage:       `A list of Sentry DSNs that are trusted by the tunnel, must NOT contain the public/secret keys. e.g. "https://sentry.example.com/1"`,
				Destination: &sentrytunnel.TrustedSentryDSN,
				Config:      cli.StringConfig{TrimSpace: true},
				Validator: func(slices []string) error {
					for _, slice := range slices {
						dsn, err := url.Parse(slice)
						if err != nil {
							return fmt.Errorf("invalid DSN: %s", dsn)
						}

						if dsn.User.String() != "" {
							return fmt.Errorf("DSN must not contain public key and secret key")
						}
					}

					return nil
				},
			},
		},
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			switch c.String("log-level") {
			case "debug":
				logger = level.NewFilter(logger, level.AllowDebug())
			case "info":
				logger = level.NewFilter(logger, level.AllowInfo())
			case "warn":
				logger = level.NewFilter(logger, level.AllowWarn())
			case "error":
				logger = level.NewFilter(logger, level.AllowError())
			default:
				logger = level.NewFilter(logger, level.AllowNone())
			}
			return ctx, nil
		},
		Action: func(ctx context.Context, c *cli.Command) error { return action(ctx, c) },
	}

	if err := cmd.Run(ctx, os.Args); err != nil {
		panic(err)
	}
}

func action(_ context.Context, cmd *cli.Command) error {
	allowedOrigins := cmd.StringSlice("allowed-origin")
	level.Info(logger).Log("msg", "Starting the "+cmd.Name, "version", cmd.Version)

	if len(allowedOrigins) == 0 {
		sentrytunnel.AccessControlAllowOrigin = []string{"*"}
		level.Warn(logger).Log("msg", "You are allowing all origins. We recommend you to specify the origins you trust. Please specify the --allowed-origin flag.")
	}

	if len(sentrytunnel.TrustedSentryDSN) == 0 {
		level.Warn(logger).Log("msg", "You are trusting all Sentry DSNs. We recommend you to specify the DSNs you trust. Please specify the --trusted-sentry-dsn flag.")
	}

	// Register the tunnel handler
	http.Handle("POST /tunnel", cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generate a new tunnel ID
		tunnelID := uuid.New()
		w.Header().Set("X-Sentry-Tunnel-Id", tunnelID.String())
		level.Debug(logger).Log("msg", "Tunnel request received", "tunnel_id", tunnelID.String())

		envelopeBytes, err := io.ReadAll(r.Body)
		if err != nil {
			level.Debug(logger).Log("msg", "Failed to read envelope", "tunnel_id", tunnelID.String(), "error", err)
			w.WriteHeader(500)
			w.Write([]byte(fmt.Sprintf(`{"error":"%s"}`, err.Error())))
			return
		}

		envelopeBytesPretty := humanize.Bytes(uint64(len(envelopeBytes)))
		level.Debug(logger).Log("msg", "Reading envelope", "tunnel_id", tunnelID.String(), "size", envelopeBytesPretty)

		envelope, err := envelope.Parse(envelopeBytes)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(fmt.Sprintf(`{"error":"%s"}`, err.Error())))
			level.Error(logger).Log("msg", "Failed to parse envelope", "tunnel_id", tunnelID.String(), "error", err)
			return
		}

		// Parse the DSN into a URL object
		upstreamSentryDSN, err := url.Parse(envelope.Header.DSN)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(fmt.Sprintf(`{"error":"%s"}`, err.Error())))
			level.Error(logger).Log("msg", "Failed to parse envelope DSN", "tunnel_id", tunnelID.String(), "error", err)
			return
		}

		// Check if the DSN is trusted, it is possible for trustedDSNs to be empty
		// If trustedDSNs is empty, we trust all DSNs
		if len(sentrytunnel.TrustedSentryDSN) > 0 {
			level.Debug(logger).Log("msg", "Checking if the DSN is trusted", "tunnel_id", tunnelID.String(), "dsn", sanatizeDsn(upstreamSentryDSN))
			if err := isTrustedDSN(upstreamSentryDSN, sentrytunnel.TrustedSentryDSN); err != nil {
				w.WriteHeader(500)
				w.Write([]byte(fmt.Sprintf(`{"error":"%s"}`, err.Error())))
				level.Error(logger).Log("msg", "Rejected envelope", "tunnel_id", tunnelID.String(), "error", err)
				return
			}
		}

		// Repack the envelope
		data := envelope.Bytes()
		dataBytesPretty := humanize.Bytes(uint64(len(data)))
		level.Debug(logger).Log("msg", "Repackaging envelope", "tunnel_id", tunnelID.String(), "type", envelope.Type.Type, "dsn", sanatizeDsn(upstreamSentryDSN), "size", dataBytesPretty)

		// Tunnel the envelope to Sentry
		level.Info(logger).Log("msg", "Sending envelope to Sentry", "tunnel_id", tunnelID.String(), "dsn", sanatizeDsn(upstreamSentryDSN), "type", envelope.Type.Type, "size", envelopeBytesPretty)
		if err := tunnel(tunnelID.String(), upstreamSentryDSN, data); err != nil {
			w.WriteHeader(500)
			w.Write([]byte(fmt.Sprintf(`{"error":"%s"}`, err.Error())))
			level.Error(logger).Log("msg", "Failed to send the envelope to Sentry", "tunnel_id", tunnelID.String(), "dsn", sanatizeDsn(upstreamSentryDSN), "error", err)
			return
		}

		level.Debug(logger).Log("msg", "Successfully sent the envelope to Sentry", "tunnel_id", tunnelID.String(), "dsn", sanatizeDsn(upstreamSentryDSN), "size", envelopeBytesPretty)

		w.WriteHeader(200)
		w.Write([]byte(`{"status":"ok"}`))
	})))

	// Start the server
	level.Info(logger).Log("msg", "The server is listening on "+sentrytunnel.ListenAddr)
	return http.ListenAndServe(sentrytunnel.ListenAddr, nil)
}

func tunnel(tunnelID string, dsn *url.URL, data []byte) error {
	project := strings.TrimPrefix(dsn.Path, "/")
	endpoint := dsn.Scheme + "://" + dsn.Host + "/api/" + project + "/envelope/"

	// Create a new HTTP request
	req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(data))
	req.Header.Set("User-Agent", HttpHeaderUserAgent)
	req.Header.Set("X-Sentry-Tunnel-Id", tunnelID)

	// Sending the request, with retries
	err := retry.Do(func() error {
		resp, err := http.DefaultClient.Do(req)

		// Check if there was an error
		if err != nil {
			return fmt.Errorf("failed to send the envelope to upstream: %s", err)
		}

		// Check the status code
		if resp.StatusCode != 200 {
			return fmt.Errorf("upstream return unexpected status code: %d", resp.StatusCode)
		}

		return nil
	}, retry.Attempts(5))

	return err
}

func sanatizeDsn(dsn *url.URL) string {
	return dsn.Host + dsn.Path
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "" {
			origin := r.Header.Get("Origin")
			for _, allowedOrigin := range sentrytunnel.AccessControlAllowOrigin {
				if allowedOrigin == "*" || allowedOrigin == origin {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					break
				}
				http.Error(w, "Request from an untrusted origin", http.StatusForbidden)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isTrustedDSN(dsn *url.URL, trustedDSNs []string) error {
	for _, trustedDSN := range trustedDSNs {
		trustedUrl, err := url.Parse(trustedDSN)
		if err != nil {
			return fmt.Errorf("invalid trusted DSN: %s", trustedDSN)
		}
		if dsn.Host+dsn.Path == trustedUrl.Host+trustedUrl.Path {
			return nil
		}
	}
	return fmt.Errorf("untrusted DSN: %s", dsn)
}
