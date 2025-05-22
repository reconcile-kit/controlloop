package registry

import "github.com/prometheus/client_golang/prometheus"

// RegistererGatherer combines both parts of the API of a Prometheus
// registry, both the Registerer and the Gatherer interfaces.
type RegistererGatherer interface {
	prometheus.Registerer
	prometheus.Gatherer
}

// Registry is a prometheus registry for storing observability within the
// controlloop.
var Registry RegistererGatherer = prometheus.NewRegistry()
