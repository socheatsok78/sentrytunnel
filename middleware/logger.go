package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

func RequestLogger(logger log.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			entries := []interface{}{}
			reqID := middleware.GetReqID(r.Context())
			if reqID != "" {
				entries = append(entries, "id", reqID)
			}

			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			entries = append(entries, "method", r.Method, "url", scheme+"://"+r.Host+r.RequestURI)

			entries = append(entries, "proto", r.Proto)
			entries = append(entries, "from", r.RemoteAddr)

			defer func() {
				level.Info(logger).Log(entries...)
			}()

			next.ServeHTTP(w, r)
		})
	}
}
