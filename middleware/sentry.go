package middleware

import (
	"net/http"

	sentryhttp "github.com/getsentry/sentry-go/http"
)

var defaultSentryHandler *sentryhttp.Handler

// IMPORTANT NOTE: SentryRecoverer should go after middleware.Recoverer. Example:
//
//	r := chi.NewRouter()
//	r.Use(middleware.Recoverer)
//	r.Use(middleware.SentryRecoverer)        // <--<< SentryRecoverer should come after Recoverer
//	r.Get("/", handler)
func SentryRecoverer(next http.Handler) http.Handler {
	return defaultSentryHandler.HandleFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func init() {
	defaultSentryHandler = sentryhttp.New(sentryhttp.Options{
		// Repanice so that the "r.Use(middleware.Recoverer)" still does its job
		Repanic: true,
	})
}
