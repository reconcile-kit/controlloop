package metrics

type MetricsProvider interface {
	NewActiveWorkersMetric(name string) GaugeMetric
	NewReconcileTotalMetric(name string) CounterMetricLabeled
	NewReconcilePanicsMetric(name string) CounterMetric
	NewReconcileErrorsMetric(name string) CounterMetric
	NewReconcileTimeMetric(name string) HistogramMetric
	NewWorkerCountMetrics(name string) SettableGaugeMetric
}

type GaugeMetric interface {
	Inc()
	Dec()
}

type SettableGaugeMetric interface {
	Set(float64)
}

type CounterMetric interface {
	Inc()
}

type CounterMetricLabeled interface {
	IncLabeled(label string)
}

type SummaryMetric interface {
	Observe(float64)
}

type HistogramMetric interface {
	Observe(float64)
}
