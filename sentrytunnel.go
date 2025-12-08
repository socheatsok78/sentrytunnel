package sentrytunnel

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/chi-middleware/proxy"
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
	internalMetrics "github.com/socheatsok78/sentrytunnel/metrics"
	internalMiddleware "github.com/socheatsok78/sentrytunnel/middleware"
	"github.com/urfave/cli/v3"
)

var (
	Name    = "sentrytunnel"
	Version = "dev"
)

type SentryTunnel struct {
	Debug            bool
	LoggingLevel     string
	DSN              string
	TracesSampleRate float64

	// Tunnel server
	ListenAddress            string
	AccessControlAllowOrigin []string
	TrustedProxies           []*net.IPNet
	TunnelPath               string
	TunnelTimeout            time.Duration

	// Tunnel metrics
	MetricsAddress string
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
				Name:        "log-level",
				Usage:       "Set the log level",
				Value:       "INFO",
				Sources:     cli.EnvVars("SENTRYTUNNEL_LOG_LEVEL"),
				Destination: &sentrytunnel.LoggingLevel,
				Validator: func(s string) error {
					switch strings.ToUpper(s) {
					case "DEBUG", "INFO", "WARN", "ERROR", "NONE":
						return nil
					default:
						return fmt.Errorf("invalid log level: %s", s)
					}
				},
			},
			&cli.StringFlag{
				Name:        "dsn",
				Usage:       "The Sentry DSN to send monitoring events to",
				Sources:     cli.EnvVars("SENTRYTUNNEL_DSN"),
				Destination: &sentrytunnel.DSN,
				Validator: func(s string) error {
					_, err := sentry.NewDsn(s)
					return err
				},
				Hidden: true,
			},
			&cli.FloatFlag{
				Name:        "trace-sample-rate",
				Usage:       "The Sentry monitoring's trace sample rate for tracing (0.0 - 1.0)",
				Sources:     cli.EnvVars("SENTRYTUNNEL_TRACE_SAMPLE_RATE"),
				Value:       1.0,
				Destination: &sentrytunnel.TracesSampleRate,
				Hidden:      true,
			},

			// Tunnel server
			&cli.StringFlag{
				Name:        "listen-addr",
				Category:    "Tunnel server:",
				Usage:       "The address to listen on",
				Value:       ":8080",
				Sources:     cli.EnvVars("SENTRYTUNNEL_LISTEN_ADDR"),
				Destination: &sentrytunnel.ListenAddress,
			},
			&cli.StringFlag{
				Name:        "tunnel-path",
				Category:    "Tunnel server:",
				Usage:       "The path to accept envelop tunneling requests",
				Value:       "/tunnel",
				Sources:     cli.EnvVars("SENTRYTUNNEL_TUNNEL_PATH"),
				Destination: &sentrytunnel.TunnelPath,
				Validator: func(s string) error {
					if s == "" || s[0] != '/' {
						return fmt.Errorf("tunnel path must start with '/'")
					}
					return nil
				},
			},
			&cli.DurationFlag{
				Name:        "tunnel-timeout",
				Category:    "Tunnel server:",
				Usage:       "The maximum duration for processing the tunneling requests",
				Value:       3 * time.Minute,
				Sources:     cli.EnvVars("SENTRYTUNNEL_TUNNEL_TIMEOUT"),
				Destination: &sentrytunnel.TunnelTimeout,
			},
			&cli.StringSliceFlag{
				Name:     "trusted-proxy",
				Category: "Tunnel server:",
				Usage:    "A list of trusted proxy IPs or CIDRs to extract the client IP from X-Forwarded-For header.",
				Sources:  cli.EnvVars("SENTRYTUNNEL_TRUSTED_PROXY"),
				Action: func(ctx context.Context, c *cli.Command, s []string) error {
					var trustedProxies []*net.IPNet
					for _, cidr := range s {
						_, ipnet, err := net.ParseCIDR(cidr)
						if err != nil {
							// Try to parse as IP address
							ip := net.ParseIP(cidr)
							if ip == nil {
								return fmt.Errorf("invalid trusted proxy CIDR or IP: %s", cidr)
							}
							var mask net.IPMask
							if ip.To4() != nil {
								mask = net.CIDRMask(32, 32)
							} else {
								mask = net.CIDRMask(128, 128)
							}
							ipnet = &net.IPNet{
								IP:   ip,
								Mask: mask,
							}
						}
						trustedProxies = append(trustedProxies, ipnet)
					}
					sentrytunnel.TrustedProxies = trustedProxies
					return nil
				},
			},
			&cli.StringSliceFlag{
				Name:        "allowed-origin",
				Category:    "Tunnel server:",
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

			// Tunnel metrics
			&cli.StringFlag{
				Name:        "metrics-addr",
				Category:    "Tunnel metrics:",
				Usage:       "The address to listen on",
				Value:       ":9091",
				Sources:     cli.EnvVars("SENTRYTUNNEL_METRICS_ADDR"),
				Destination: &sentrytunnel.MetricsAddress,
			},
		},
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			switch strings.ToUpper(c.String("log-level")) {
			case "DEBUG":
				sentrytunnel.Debug = true
				logger = level.NewFilter(logger, level.AllowDebug())
			case "INFO":
				logger = level.NewFilter(logger, level.AllowInfo())
			case "WARN":
				logger = level.NewFilter(logger, level.AllowWarn())
			case "ERROR":
				logger = level.NewFilter(logger, level.AllowError())
			case "NONE":
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
	// Initialize Sentry
	if sentrytunnel.DSN != "" {
		err := sentry.Init(sentry.ClientOptions{
			Debug:          sentrytunnel.Debug,
			Dsn:            sentrytunnel.DSN,
			Release:        Name + "@" + Version,
			SendDefaultPII: true,
			EnableTracing:  true,
			TracesSampler: sentry.TracesSampler(func(ctx sentry.SamplingContext) float64 {
				// Sample only non-heartbeat transactions
				if ctx.Span.Name == "GET /heartbeat" {
					return 0.0
				}
				return sentrytunnel.TracesSampleRate
			}),
		})
		if err != nil {
			level.Error(logger).Log("msg", "error initializing Sentry", "err", err)
			return err
		}
		level.Info(logger).Log("msg", "initialized Sentry for tunnel monitoring")
	}

	// Initialize run group
	var g run.Group

	// Initialize HTTP server with Chi
	r := chi.NewRouter()
	r.Use(middleware.SetHeader("Server", Name+"/"+Version))
	r.Use(internalMiddleware.RequestID)
	r.Use(internalMiddleware.RequestIDHeader)

	// CORS and Trusted Proxies
	r.Use(cors.Handler((cors.Options{
		AllowedOrigins: sentrytunnel.AccessControlAllowOrigin,
	})))
	r.Use(proxy.ForwardedHeaders(
		&proxy.ForwardedHeadersOptions{
			ForwardLimit:       0,
			TrustingAllProxies: len(sentrytunnel.TrustedProxies) == 0,
			TrustedNetworks:    sentrytunnel.TrustedProxies,
		},
	))

	// Metrics
	r.Use(std.HandlerProvider("", metricsMiddleware.New(metricsMiddleware.Config{
		Service:  Name,
		Recorder: metrics.NewRecorder(metrics.Config{}),
	})))

	// Enable logging middleware if the log level is not set to none
	if c.String("log-level") != "NONE" {
		r.Use(middleware.Logger)
	}

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	r.Use(middleware.Timeout(sentrytunnel.TunnelTimeout))

	// Recoverer is a middleware that recovers from panics, logs the panic (and a backtrace),
	// and returns a HTTP 500 (Internal Server Error) status if possible.
	r.Use(middleware.Recoverer)
	r.Use(internalMiddleware.SentryRecoverer)

	// Heartbeat endpoint for liveness probe
	r.Use(middleware.Heartbeat("/heartbeat"))

	// Configure tunnel route
	r.Route(sentrytunnel.TunnelPath, func(r chi.Router) {
		r.Use(SentryTunnelCtx)
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Get the DSN and payload from the context
			dsn := r.Context().Value(contextKeyDSN).(*sentry.Dsn)

			// Prepare the request to upstream Sentry
			req, _ := http.NewRequestWithContext(ctx, "POST", dsn.GetAPIURL().String(), r.Body)
			req.Header.Set("Content-Type", "application/x-sentry-envelope")

			// Set the X-Sentry-Forwarded-For header for preserving client IP
			req.Header.Set("X-Sentry-Forwarded-For", clientIP(r.RemoteAddr))

			// Sending the payload to upstream
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				internalMetrics.SentryEnvelopeForwardErrorCounter.Inc()
				return
			}
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				internalMetrics.SentryEnvelopeForwardErrorCounter.Inc()
				return
			}

			// Proxy the response from upstream back to the client
			internalMetrics.SentryEnvelopeForwardSuccessCounter.Inc()
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
			internalMetrics.SentryEnvelopeRejectedCounter.Inc()
			return
		}

		// Read the envelope from the request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			internalMetrics.SentryEnvelopeRejectedCounter.Inc()
			return
		}

		// Parse the envelope
		payload, err := envelope.Parse(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			internalMetrics.SentryEnvelopeRejectedCounter.Inc()
			return
		}

		// Parse the DSN
		dsn, err := sentry.NewDsn(payload.Header.DSN)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			internalMetrics.SentryEnvelopeRejectedCounter.Inc()
			return
		}

		// Set the DSN and payload to the context
		ctx := r.Context()
		ctx = context.WithValue(ctx, contextKeyDSN, dsn)

		// Reset the request body for the next handler
		r.Body = io.NopCloser(bytes.NewReader(body))

		// Call the next handler
		internalMetrics.SentryEnvelopeAcceptedCounter.Inc()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func clientIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}
