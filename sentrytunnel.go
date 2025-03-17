package sentrytunnel

import (
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
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/google/uuid"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	metrics "github.com/slok/go-http-metrics/metrics/prometheus"
	metricsMiddleware "github.com/slok/go-http-metrics/middleware"
	"github.com/slok/go-http-metrics/middleware/std"
	"github.com/socheatsok78/sentrytunnel/envelope"
	"github.com/urfave/cli/v3"
)

var (
	Name    = "sentrytunnel"
	Version = "dev"
)

type SentryTunnel struct {
	ListenAddress            string
	MetricsAddress           string
	LoggingLevel             string
	AccessControlAllowOrigin []string
	TrustedSentryDSN         []string

	// Tunnel monitoring
	Debug            bool
	DSN              string
	TracesSampleRate float64
}

// SentryTunnel
var (
	logger       log.Logger
	sentrytunnel = &SentryTunnel{}
)

type contextKey string

const (
	contextKeyID      contextKey = "id"
	contextKeyDSN     contextKey = "dsn"
	contextKeyPayload contextKey = "payload"
)

// Setup logger
func init() {
	logger = log.NewLogfmtLogger(os.Stdout)
	logger = log.With(logger, "ts", log.DefaultTimestampUTC)
}

// Run the main application
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
				Sources:     cli.EnvVars("SENTRYTUNNEL_LISTEN_ADDR"),
				Destination: &sentrytunnel.ListenAddress,
			},
			&cli.StringFlag{
				Name:        "metrics-addr",
				Usage:       "The address to listen on",
				Value:       ":9091",
				Sources:     cli.EnvVars("SENTRYTUNNEL_METRICS_ADDR"),
				Destination: &sentrytunnel.MetricsAddress,
			},
			&cli.StringFlag{
				Name:        "log-level",
				Usage:       "Set the log level",
				Value:       "info",
				Sources:     cli.EnvVars("SENTRYTUNNEL_LOG_LEVEL"),
				Destination: &sentrytunnel.LoggingLevel,
			},

			// Tunnel monitoring
			&cli.StringFlag{
				Name:        "dsn",
				Usage:       "The Sentry DSN for monitoring the tunnel",
				Sources:     cli.EnvVars("SENTRY_DSN"),
				Destination: &sentrytunnel.DSN,
				Validator: func(s string) error {
					_, err := sentry.NewDsn(s)
					return err
				},
			},
			&cli.FloatFlag{
				Name:        "trace-sample-rate",
				Usage:       "The Sentry tunnel sample rate for sampling traces in the range [0.0, 1.0]",
				Sources:     cli.EnvVars("SENTRY_TRACE_SAMPLE_RATE"),
				Value:       1.0,
				Destination: &sentrytunnel.TracesSampleRate,
			},

			// CORS
			&cli.StringSliceFlag{
				Name:        "allowed-origin",
				Usage:       "A list of origins that are allowed to access the tunnel. e.g. https://example.com",
				Sources:     cli.EnvVars("SENTRYTUNNEL_ALLOWED_ORIGIN"),
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
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			switch c.String("log-level") {
			case "debug":
				sentrytunnel.Debug = true
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
		Action: func(ctx context.Context, c *cli.Command) error {
			level.Info(logger).Log("msg", "starting the "+Name, "version", Version)
			return action(ctx, c)
		},
	}

	return cmd.Run(ctx, os.Args)
}

func action(_ context.Context, _ *cli.Command) error {
	// Initialize run group
	var g run.Group

	// Initialize Sentry
	err := sentry.Init(sentry.ClientOptions{
		Debug:         sentrytunnel.Debug,
		Dsn:           sentrytunnel.DSN,
		Release:       Name + "@" + Version,
		EnableTracing: true,
		// Set TracesSampleRate to 1.0 to capture 100%
		// of transactions for tracing.
		TracesSampleRate: sentrytunnel.TracesSampleRate,
	})
	if err != nil {
		level.Error(logger).Log("msg", "error initializing Sentry", "err", err)
		return err
	}
	// Flush buffered events before the program terminates.
	// Set the timeout to the maximum duration the program can afford to wait.
	defer sentry.Flush(2 * time.Second)

	// Initialize HTTP server with Chi
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.SetHeader("Server", Name+"/"+Version))
	r.Use(middleware.Heartbeat("/heartbeat"))

	// CORS
	r.Use(cors.Handler((cors.Options{
		AllowedOrigins: sentrytunnel.AccessControlAllowOrigin,
	})))

	// Metrics
	r.Use(std.HandlerProvider("", metricsMiddleware.New(metricsMiddleware.Config{
		Service:  Name,
		Recorder: metrics.NewRecorder(metrics.Config{}),
	})))

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	r.Use(middleware.Timeout(60 * time.Second))

	// Configure tunnel route
	r.Route("/tunnel", func(r chi.Router) {
		r.Use(SentryTunnelCtx)
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			id := r.Context().Value(contextKeyID).(string)

			// Get the DSN and payload from the context
			dsn := r.Context().Value(contextKeyDSN).(*sentry.Dsn)
			payload := r.Context().Value(contextKeyPayload).(*envelope.Envelope)

			// Sending the payload to upstream
			level.Info(logger).Log("id", id, "msg", "sending envelope to sentry endpoint", "dsn", dsn.GetAPIURL().String())
			res, err := http.Post(dsn.GetAPIURL().String(), "application/x-sentry-envelope", payload.NewReader())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer res.Body.Close()

			// Read the response body
			body, err := io.ReadAll(res.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Respond to the client with the upstream's response
			level.Info(logger).Log("id", id, "msg", "received response from sentry", "dsn", dsn.GetAPIURL().String(), "status", res.StatusCode)

			// Set the response status code and body
			w.WriteHeader(res.StatusCode)
			w.Write(body)
		})
	})

	// Serve metrics
	g.Add(func() error {
		level.Info(logger).Log("msg", fmt.Sprintf("metrics listening at %s", sentrytunnel.MetricsAddress))
		return http.ListenAndServe(sentrytunnel.MetricsAddress, promhttp.Handler())
	}, func(err error) {})

	// Serve Sentry Tunnel
	g.Add(func() error {
		level.Info(logger).Log("msg", fmt.Sprintf("server listening at %s", sentrytunnel.ListenAddress))
		return http.ListenAndServe(sentrytunnel.ListenAddress, r)
	}, func(err error) {})

	return g.Run()
}

func SentryTunnelCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the request is a POST request
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		// Process the request
		// Check if the request is a POST request
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		// Process the request
		id := uuid.New().String()
		level.Info(logger).Log("id", id, "msg", "received request", "method", r.Method, "url", r.URL.String())

		// Set the tunnel ID to the response header
		w.Header().Set("X-Sentry-Tunnel-ID", id)

		// Read the envelope from the request body
		envelopeBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			level.Error(logger).Log("id", id, "msg", "error reading request body", "err", err)
			return
		}

		// Parse the envelope
		payload, err := envelope.Parse(envelopeBytes)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			level.Error(logger).Log("id", id, "msg", "error parsing envelope", "err", err)
			return
		}

		// Parse the DSN
		dsn, err := sentry.NewDsn(payload.Header.DSN)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			level.Error(logger).Log("id", id, "msg", "error parsing Sentry DSN", "err", err)
			return
		}

		// Set the DSN and payload to the context
		ctx := r.Context()
		ctx = context.WithValue(ctx, contextKeyID, id)
		ctx = context.WithValue(ctx, contextKeyDSN, dsn)
		ctx = context.WithValue(ctx, contextKeyPayload, payload)

		// Call the next handler
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
