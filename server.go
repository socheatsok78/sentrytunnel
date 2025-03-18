package sentrytunnel

import (
	"context"
	"io"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-kit/log/level"
	"github.com/google/uuid"
	"github.com/socheatsok78/sentrytunnel/envelope"
)

func SentryTunnelRoutes(r chi.Router) {
	r.Use(SentryTunnelContextHandler)

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
}

func SentryTunnelContextHandler(next http.Handler) http.Handler {
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
