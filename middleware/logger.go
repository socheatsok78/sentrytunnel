package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

type StructuredFormatter struct {
	Logger log.Logger
}

func (l StructuredFormatter) NewLogEntry(r *http.Request) middleware.LogEntry {
	entry := &StructuredLogEntry{
		StructuredFormatter: &l,
		request:             r,
		entries:             []interface{}{},
	}

	reqID := middleware.GetReqID(r.Context())
	if reqID != "" {
		entry.entries = append(entry.entries, "id", reqID)
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	entry.entries = append(entry.entries, "msg", fmt.Sprintf("%s %s://%s%s %s", r.Method, scheme, r.Host, r.RequestURI, r.Proto))
	entry.entries = append(entry.entries, "from", r.RemoteAddr)

	return entry
}

type StructuredLogEntry struct {
	*StructuredFormatter
	request *http.Request
	entries []interface{}
}

func (l *StructuredLogEntry) Write(status, bytes int, header http.Header, elapsed time.Duration, extra interface{}) {
	l.entries = append(l.entries, "status", status)
	l.entries = append(l.entries, "bytes", fmt.Sprintf("%dB", bytes))
	l.entries = append(l.entries, "elapsed", elapsed)
	level.Info(l.Logger).Log(l.entries...)
}

func (l *StructuredLogEntry) Panic(v interface{}, stack []byte) {
	middleware.PrintPrettyStack(v)
}
