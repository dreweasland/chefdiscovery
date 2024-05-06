package chef

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/prometheus/prometheus/discovery"
)

var _ discovery.DiscovererMetrics = (*chefMetrics)(nil)

type chefMetrics struct {
	refreshMetrics discovery.RefreshMetricsInstantiator

	bootstrapFailure *prometheus.GaugeVec

	metricRegisterer discovery.MetricRegisterer
}

func newDiscovererMetrics(reg prometheus.Registerer, rmi discovery.RefreshMetricsInstantiator) discovery.DiscovererMetrics {
	m := &chefMetrics{
		refreshMetrics: rmi,
		bootstrapFailure: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "sd_chef_bootstrap_fail",
				Help:      "Outputs a 1 if a node is detected that appears to not be correctly bootstrapped",
			},
			[]string{"node"},
		),
	}

	m.metricRegisterer = discovery.NewMetricRegisterer(reg, []prometheus.Collector{
		m.bootstrapFailure,
	})

	return m
}

// Register implements discovery.DiscovererMetrics.
func (m *chefMetrics) Register() error {
	return m.metricRegisterer.RegisterMetrics()
}

// Unregister implements discovery.DiscovererMetrics.
func (m *chefMetrics) Unregister() {
	m.metricRegisterer.UnregisterMetrics()
}
