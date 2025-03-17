package sentrymiddleware

import (
	"net/http"

	sentryhttp "github.com/getsentry/sentry-go/http"
)

func Sentry(options *sentryhttp.Options) func(http.Handler) http.Handler {
	if options == nil {
		options = &sentryhttp.Options{}
	}
	sentryHandler := sentryhttp.New(*options)
	return func(h http.Handler) http.Handler {
		return sentryHandler.HandleFunc(h.ServeHTTP)
	}
}
