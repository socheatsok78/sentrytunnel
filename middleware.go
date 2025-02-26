package sentrytunnel

import "net/http"

func SentryTunnelMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", HttpHeaderUserAgent)
		next.ServeHTTP(w, r)
	})
}
