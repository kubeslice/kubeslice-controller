package metrics

import (
	"fmt"
	mfm "github.com/kubeslice/kubeslice-monitoring/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"math/rand"
	"net/http"
	"os"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	ctrl "sigs.k8s.io/controller-runtime"
)

// create latency metrics which has to be populated when we receive latency from tunnel
var (
	KubeSliceEventsCounter *prometheus.CounterVec

	controllerNamespace = "kubeslice_controller"

	log = ctrl.Log.WithName("setup")
)

// StartMetricsCollector registers metrics to prometheus
func StartMetricsCollector(metricCollectorPort string) {
	metricCollectorPort = ":" + metricCollectorPort
	//log.Info("Starting metric collector @ %s", metricCollectorPort)
	rand.Seed(time.Now().Unix())
	histogramVec := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "prom_request_time",
		Help: "Time it has taken to retrieve the metrics",
	}, []string{"time"})

	prometheus.Register(histogramVec)

	mf, err := mfm.NewMetricsFactory(ctrlmetrics.Registry, mfm.MetricsFactoryOptions{
		Prefix: controllerNamespace,
	})
	if err != nil {
		log.Error(err, "unable to create metrics factory")
		os.Exit(1)
	}

	KubeSliceEventsCounter = mf.NewCounter(
		"events_counter",
		"The events counter metrics",
		append([]string{
			"action",
			"event",
			"object_name",
			"object_kind",
		}, getDefaultLabels()...),
	)

	prometheus.MustRegister(KubeSliceEventsCounter)

	http.Handle("/metrics", newHandlerWithHistogram(promhttp.Handler(), histogramVec))

	err = http.ListenAndServe(metricCollectorPort, nil)
	if err != nil {
		log.Error(err, "Failed to start metric collector @ %s")
	}
}

func getDefaultLabels() []string {
	return []string{"slice_project", "slice_name", "slice_namespace", "slice_cluster", "slice_reporting_controller"}
}

// send http request
func newHandlerWithHistogram(handler http.Handler, histogram *prometheus.HistogramVec) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		status := http.StatusOK

		defer func() {
			histogram.WithLabelValues(fmt.Sprintf("%d", status)).Observe(time.Since(start).Seconds())
		}()

		if req.Method == http.MethodGet {
			handler.ServeHTTP(w, req)
			return
		}
		status = http.StatusBadRequest

		w.WriteHeader(status)
	})
}
