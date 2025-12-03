package sentrytunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	metrics "github.com/slok/go-http-metrics/metrics/prometheus"
	metricsMiddleware "github.com/slok/go-http-metrics/middleware"
	"github.com/slok/go-http-metrics/middleware/std"
	"github.com/socheatsok78/sentrytunnel/envelope"
	smiddleware "github.com/socheatsok78/sentrytunnel/middleware"
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
				Value:       "none",
				Sources:     cli.EnvVars("SENTRYTUNNEL_LOG_LEVEL"),
				Destination: &sentrytunnel.LoggingLevel,
			},

			// Tunnel monitoring
			&cli.StringFlag{
				Name:        "dsn",
				Usage:       "The Sentry DSN for monitoring the tunnel",
				Sources:     cli.EnvVars("SENTRYTUNNEL_DSN"),
				Destination: &sentrytunnel.DSN,
				Validator: func(s string) error {
					_, err := sentry.NewDsn(s)
					return err
				},
			},
			&cli.FloatFlag{
				Name:        "trace-sample-rate",
				Usage:       "The Sentry tunnel sample rate for sampling traces in the range [0.0, 1.0]",
				Sources:     cli.EnvVars("SENTRYTUNNEL_TRACE_SAMPLE_RATE"),
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
			case "none":
				logger = level.NewFilter(logger, level.AllowNone())
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

func action(_ context.Context, c *cli.Command) error {
	// Initialize run group
	var g run.Group

	// Initialize Sentry
	if sentrytunnel.DSN != "" {
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
	}

	// Initialize HTTP server with Chi
	r := chi.NewRouter()
	r.Use(smiddleware.RequestID)
	r.Use(smiddleware.RequestIDHeader)

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

	// Enable logging middleware if the log level is not set to none
	if c.String("log-level") != "none" {
		r.Use(middleware.Logger)
		// r.Use(middleware.RequestLogger(smiddleware.StructuredFormatter{
		// 	Logger: logger,
		// }))
	}

	// Recoverer is a middleware that recovers from panics, logs the panic (and a backtrace),
	// and returns a HTTP 500 (Internal Server Error) status if possible.
	r.Use(middleware.Recoverer)
	r.Use(smiddleware.SentryRecoverer)

	// Configure tunnel route
	r.Route("/tunnel", func(r chi.Router) {
		r.Use(SentryTunnelCtx)
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			// Get the DSN and payload from the context
			dsn := r.Context().Value(contextKeyDSN).(*sentry.Dsn)
			payload := r.Context().Value(contextKeyPayload).(*envelope.Envelope)

			// Sending the payload to upstream
			res, err := http.Post(dsn.GetAPIURL().String(), "application/x-sentry-envelope", payload.NewReader())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Proxy the response from upstream back to the client
			w.WriteHeader(res.StatusCode)
			w.Write(body)
		})
	})

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	g.Add(func() error {
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit
		level.Info(logger).Log("msg", fmt.Sprintf("received signal %s", s))
		return nil
	}, func(err error) {
		level.Info(logger).Log("msg", "shutting down", "err", err)
		close(quit)
	})

	// Serve metrics
	{
		ln, _ := net.Listen("tcp", sentrytunnel.MetricsAddress)
		g.Add(func() error {
			level.Info(logger).Log("msg", fmt.Sprintf("metrics listening at %s", sentrytunnel.MetricsAddress))
			return http.Serve(ln, promhttp.Handler())
		}, func(err error) {
			ln.Close()
		})
	}

	// Serve Sentry Tunnel
	{
		ln, _ := net.Listen("tcp", sentrytunnel.ListenAddress)
		g.Add(func() error {
			level.Info(logger).Log("msg", fmt.Sprintf("server listening at %s", sentrytunnel.ListenAddress))
			return http.Serve(ln, r)
		}, func(err error) {
			ln.Close()

			// Flush buffered events before the program terminates.
			// Set the timeout to the maximum duration the program can afford to wait.
			sentry.Flush(2 * time.Second)
		})
	}

	return g.Run()
}

func SentryTunnelCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Process the request
		// Check if the request is a POST request
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

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
		ctx := r.Context()
		ctx = context.WithValue(ctx, contextKeyDSN, dsn)
		ctx = context.WithValue(ctx, contextKeyPayload, payload)

		// Call the next handler
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
