// Package metrics defines the Prometheus metrics the service exports.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds every collector, registered on its own registry.
type Metrics struct {
	registry         *prometheus.Registry
	PullDuration     *prometheus.HistogramVec
	VersionsServed   prometheus.Counter
	Resets           *prometheus.CounterVec
	ActiveStreams    prometheus.Gauge
	PokesSent        prometheus.Counter
	PokeFanOut       prometheus.Counter
	BusNotifications *prometheus.CounterVec
}

// New returns Metrics registered on a fresh registry together with the Go
// runtime and process collectors.
func New() *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		PullDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "store_pull_duration_seconds",
			Help:    "Time to answer a Pull, by outcome (entries, reset, error).",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 14),
		}, []string{"outcome"}),
		VersionsServed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "store_pull_versions_served_total",
			Help: "Versions returned by Pull.",
		}),
		Resets: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "store_pull_resets_total",
			Help: "Pulls answered with Reset, by reason.",
		}, []string{"reason"}),
		ActiveStreams: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "store_poke_streams_active",
			Help: "Open Pokes streams.",
		}),
		PokesSent: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "store_pokes_sent_total",
			Help: "Poke messages written to streams, keep-alives included.",
		}),
		PokeFanOut: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "store_poke_fanout_total",
			Help: "Scope notifications handed to subscribed streams.",
		}),
		BusNotifications: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "store_poke_bus_notifications_total",
			Help: "Scope notifications received from the poke bus, by source.",
		}, []string{"source"}),
	}
	m.registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.PullDuration, m.VersionsServed, m.Resets, m.ActiveStreams, m.PokesSent, m.PokeFanOut, m.BusNotifications,
	)
	return m
}

// Handler serves the metrics in the Prometheus exposition format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{Registry: m.registry})
}
