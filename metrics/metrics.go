package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// SentryEnvelopeAcceptedCounter is a Prometheus counter for the number of envelopes accepted by the tunnel
	SentryEnvelopeAcceptedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sentry_envelope_accepted",
		Help: "The number of envelopes accepted by the tunnel",
	})
	// SentryEnvelopeRejectedCounter is a Prometheus counter for the number of envelopes rejected by the tunnel
	SentryEnvelopeRejectedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sentry_envelope_rejected",
		Help: "The number of envelopes rejected by the tunnel",
	})
	// SentryEnvelopeForwardSuccessCounter is a Prometheus counter for the number of envelopes successfully forwarded by the tunnel
	SentryEnvelopeForwardSuccessCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sentry_envelope_forward_success",
		Help: "The number of envelopes successfully forwarded by the tunnel",
	})
	// SentryEnvelopeForwardErrorCounter is a Prometheus counter for the number of envelopes that failed to be forwarded by the tunnel
	SentryEnvelopeForwardErrorCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sentry_envelope_forward_error",
		Help: "The number of envelopes that failed to be forwarded by the tunnel",
	})
)

func init() {
	prometheus.MustRegister(SentryEnvelopeAcceptedCounter)
	prometheus.MustRegister(SentryEnvelopeRejectedCounter)
	prometheus.MustRegister(SentryEnvelopeForwardSuccessCounter)
	prometheus.MustRegister(SentryEnvelopeForwardErrorCounter)
}
