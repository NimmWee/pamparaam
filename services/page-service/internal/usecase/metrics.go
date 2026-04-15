package usecase

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricsOnce sync.Once

	autosaveDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "wiki_editor",
		Subsystem: "page_service",
		Name:      "draft_save_duration_seconds",
		Help:      "Autosave duration grouped by result.",
	}, []string{"result"})
	autosaveConflicts = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "wiki_editor",
		Subsystem: "page_service",
		Name:      "autosave_conflicts_total",
		Help:      "Number of stale autosave attempts rejected by the page service.",
	})
	publishTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "wiki_editor",
		Subsystem: "page_service",
		Name:      "publish_total",
		Help:      "Number of publish operations accepted by the page service.",
	})
)

func ensureMetricsRegistered() {
	metricsOnce.Do(func() {
		prometheus.MustRegister(autosaveDuration, autosaveConflicts, publishTotal)
	})
}
