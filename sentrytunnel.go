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

	humanize "github.com/dustin/go-humanize"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/socheatsok78/sentry-tunnel/envelope"
	"github.com/urfave/cli/v3"
)

var (
	Version                  = "dev"
	HttpHeaderUserAgent      = "sentry-tunnel/" + Version
	HttpHeaderSentryTunnelID = "X-Sentry-Tunnel-Id"
)

var (
	logger log.Logger

	// ErrTunnelingToSentry is an error message for when there is an error tunneling to Sentry
	ErrTunnelingToSentry = fmt.Errorf("error tunneling to sentry")
)

var (
	// SentryEnvelopeAccepted is a Prometheus counter for the number of envelopes accepted by the tunnel
	SentryEnvelopeAccepted = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sentry_envelope_accepted",
		Help: "The number of envelopes accepted by the tunnel",
	})
	// SentryEnvelopeRejected is a Prometheus counter for the number of envelopes rejected by the tunnel
	SentryEnvelopeRejected = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sentry_envelope_rejected",
		Help: "The number of envelopes rejected by the tunnel",
	})
	// SentryEnvelopeForwardedSuccess is a Prometheus counter for the number of envelopes successfully forwarded by the tunnel
	SentryEnvelopeForwardedSuccess = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sentry_envelope_forward_success",
		Help: "The number of envelopes successfully forwarded by the tunnel",
	})
	// SentryEnvelopeForwardedError is a Prometheus counter for the number of envelopes that failed to be forwarded by the tunnel
	SentryEnvelopeForwardedError = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sentry_envelope_forward_error",
		Help: "The number of envelopes that failed to be forwarded by the tunnel",
	})
)

func init() {
	// Set up logging
	logger = log.NewLogfmtLogger(os.Stdout)
	logger = log.With(logger, "ts", log.DefaultTimestampUTC)
	logger = log.With(logger, "caller", log.DefaultCaller)

	// Register Prometheus metrics
	prometheus.MustRegister(SentryEnvelopeAccepted)
	prometheus.MustRegister(SentryEnvelopeRejected)
	prometheus.MustRegister(SentryEnvelopeForwardedSuccess)
	prometheus.MustRegister(SentryEnvelopeForwardedError)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := cli.Command{
		Name:    "sentry-tunnel",
		Usage:   "A tunneling service for Sentry",
		Version: Version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "listen-addr",
				Usage: "The address to listen on",
				Value: ":8080",
			},
			&cli.StringFlag{
				Name:  "log-level",
				Usage: "Set the log level",
				Value: "info",
			},
			&cli.StringSliceFlag{
				Name:  "trusted-sentry-dsn",
				Usage: `A list of Sentry DSNs that are trusted by the tunnel. The DSNs must not contain the public key and secret key. e.g. "https://public@sentry.example.com/1"`,
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
	listenAddrFlag := cmd.String("listen-addr")
	trustedSentryDSNFlags := cmd.StringSlice("trusted-sentry-dsn")

	level.Info(logger).Log("msg", "Starting the "+cmd.Name, "version", cmd.Version)

	if len(trustedSentryDSNFlags) == 0 {
		level.Warn(logger).Log("msg", "You are trusting all Sentry DSNs. We recommend you to specify the DSNs you trust. Please see --help for more information.")
	}

	// Register Prometheus metrics handler
	http.Handle("GET /metrics", promhttp.Handler())

	// Register the tunnel handler
	http.Handle("POST /tunnel", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		envelopeBytes, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(fmt.Sprintf(`{"error":"%s"}`, err.Error())))
			return

		}

		// Generate a new tunnel ID
		tunnelID := uuid.New()

		// Set the CORS header
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set(HttpHeaderSentryTunnelID, tunnelID.String())

		envelopeBytesPretty := humanize.Bytes(uint64(r.ContentLength))
		level.Debug(logger).Log("msg", "Envelope received", "tunnel_id", tunnelID.String(), "size", envelopeBytesPretty)

		envelope, err := envelope.Parse(envelopeBytes)
		if err != nil {
			SentryEnvelopeRejected.Inc()
			w.WriteHeader(500)
			w.Write([]byte(fmt.Sprintf(`{"error":"%s"}`, err.Error())))
			level.Debug(logger).Log("msg", "Failed to parse envelope", "tunnel_id", tunnelID.String(), "error", err)
			return
		}

		// Parse the DSN into a URL object
		dsn, err := url.Parse(envelope.Header.DSN)
		if err != nil {
			SentryEnvelopeRejected.Inc()
			w.WriteHeader(500)
			w.Write([]byte(fmt.Sprintf(`{"error":"%s"}`, err.Error())))
			level.Error(logger).Log("msg", "Failed to parse envelope DSN", "tunnel_id", tunnelID.String(), "error", err)
			return
		}

		// Check if the DSN is trusted, it is possible for trustedDSNs to be empty
		// If trustedDSNs is empty, we trust all DSNs
		if len(trustedSentryDSNFlags) > 0 {
			level.Debug(logger).Log("msg", "Checking if the DSN is trusted", "tunnel_id", tunnelID.String(), "dsn", dsn.Host+dsn.Path)
			if err := isTrustedDSN(dsn, trustedSentryDSNFlags); err != nil {
				SentryEnvelopeRejected.Inc()
				w.WriteHeader(500)
				w.Write([]byte(fmt.Sprintf(`{"error":"%s"}`, err.Error())))
				level.Error(logger).Log("msg", "Rejected envelope", "tunnel_id", tunnelID.String(), "error", err)
				return
			}
		}

		level.Info(logger).Log("msg", "Forwarding envelope to Sentry", "tunnel_id", tunnelID.String(), "dsn", dsn.Host+dsn.Path, "size", envelopeBytesPretty)
		SentryEnvelopeAccepted.Inc()

		// Tunnel the envelope to Sentry
		data := []byte(envelope.String())
		if err := tunnel(tunnelID.String(), dsn, data); err != nil {
			SentryEnvelopeForwardedError.Inc()
			w.WriteHeader(500)
			w.Write([]byte(fmt.Sprintf(`{"error":"%s"}`, err.Error())))
			level.Error(logger).Log("msg", "Failed to forward envelope to Sentry", "tunnel_id", tunnelID.String(), "error", err)
			return
		}

		level.Debug(logger).Log("msg", "Successfully forwarded envelope to Sentry", "tunnel_id", tunnelID.String(), "dsn", dsn.Host+dsn.Path, "size", envelopeBytesPretty)
		SentryEnvelopeForwardedSuccess.Inc()

		w.WriteHeader(200)
		w.Write([]byte(`{"status":"ok"}`))
	}))

	// Start the server
	level.Info(logger).Log("msg", "The server is listening on "+listenAddrFlag)
	return http.ListenAndServe(listenAddrFlag, nil)
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

func tunnel(tunnelID string, dsn *url.URL, data []byte) error {
	project := strings.TrimPrefix(dsn.Path, "/")
	endpoint := dsn.Scheme + "://" + dsn.Host + "/api/" + project + "/envelope/"

	// Create a new HTTP request
	req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(data))
	req.Header.Set("User-Agent", HttpHeaderUserAgent)
	req.Header.Set(HttpHeaderSentryTunnelID, tunnelID)

	// Forward the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to forward envelope: %w", err)
	}

	// Check the status code
	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
