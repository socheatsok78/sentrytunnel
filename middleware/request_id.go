package middleware

import (
	"context"
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// RequestID is a middleware that injects a request ID into the context of each
// request.
func RequestID(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		requestID := r.Header.Get(chimiddleware.RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}
		ctx = context.WithValue(ctx, chimiddleware.RequestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

// RequestIDHeader is a middleware that injects a request ID into the response
// header of each request.
func RequestIDHeader(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		requestID := chimiddleware.GetReqID(r.Context())
		if requestID != "" {
			w.Header().Set("X-Request-ID", requestID)
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
