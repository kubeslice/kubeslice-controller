package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MetricRecorder is used to record metrics from a component
type MetricsFactory interface {
	NewCounter(string, string, []string) *prometheus.CounterVec
	NewGauge(string, string, []string) *prometheus.GaugeVec
	NewHistogram(string, string, []string) prometheus.ObserverVec
}

type MetricsFactoryOptions struct {
	Project             string
	Slice               string
	Cluster             string
	Namespace           string
	ReportingController string
	Prefix              string
}

type metricsFactory struct {
	Registerer prometheus.Registerer
	Options    MetricsFactoryOptions
}

func NewMetricsFactory(r prometheus.Registerer, o MetricsFactoryOptions) (MetricsFactory, error) {
	if o.Prefix == "" {
		o.Prefix = "kubeslice"
	}
	mf := &metricsFactory{
		Registerer: r,
		Options:    o,
	}
	return mf, nil
}

// Adds slice labels to the list of labels provided
// Returns the new set of labels and slice labels
func (m *metricsFactory) getCurryLabels(labels []string) ([]string, prometheus.Labels) {
	pl := prometheus.Labels{
		"slice_name":                 m.Options.Slice,
		"slice_project":              m.Options.Project,
		"slice_cluster":              m.Options.Cluster,
		"slice_namespace":            m.Options.Namespace,
		"slice_reporting_controller": m.Options.ReportingController,
	}
	for k, v := range pl {
		// Remove labels if value is empty
		if v == "" {
			delete(pl, k)
			continue
		}
		// add the new label to original list of labels provided
		labels = append(labels, k)
	}
	return labels, pl
}

func (m *metricsFactory) NewCounter(name string, help string, labels []string) *prometheus.CounterVec {
	labels, cl := m.getCurryLabels(labels)
	return promauto.With(m.Registerer).NewCounterVec(prometheus.CounterOpts{
		Namespace: m.Options.Prefix,
		Name:      name,
		Help:      help,
	}, labels).MustCurryWith(cl)
}

func (m *metricsFactory) NewGauge(name string, help string, labels []string) *prometheus.GaugeVec {
	labels, cl := m.getCurryLabels(labels)
	return promauto.With(m.Registerer).NewGaugeVec(prometheus.GaugeOpts{
		Namespace: m.Options.Prefix,
		Name:      name,
		Help:      help,
	}, labels).MustCurryWith(cl)
}

func (m *metricsFactory) NewHistogram(name string, help string, labels []string) prometheus.ObserverVec {
	labels, cl := m.getCurryLabels(labels)
	return promauto.With(m.Registerer).NewHistogramVec(prometheus.HistogramOpts{
		Namespace: m.Options.Prefix,
		Name:      name,
		Help:      help,
	}, labels).MustCurryWith(cl)
}
