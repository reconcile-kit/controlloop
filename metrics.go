package controlloop

import (
	"github.com/reconcile-kit/controlloop/metrics"
	"k8s.io/utils/clock"
	"time"
)

func newControllerMetrics(mp metrics.MetricsProvider, name string, clock clock.Clock) *defaultControllerMetrics {
	if mp == nil {
		return &defaultControllerMetrics{
			reconcilePanics: noopMetric{},
			workerCount:     noopMetric{},
			reconcileTime:   noopMetric{},
			reconcileErrors: noopMetric{},
			activeWorkers:   noopMetric{},
			reconcileTotal:  noopMetric{},
			clock:           clock,
		}
	}

	return &defaultControllerMetrics{
		clock:           clock,
		activeWorkers:   mp.NewActiveWorkersMetric(name),
		reconcileTotal:  mp.NewReconcileTotalMetric(name),
		reconcileErrors: mp.NewReconcileErrorsMetric(name),
		reconcilePanics: mp.NewReconcilePanicsMetric(name),
		reconcileTime:   mp.NewReconcileTimeMetric(name),
		workerCount:     mp.NewWorkerCountMetrics(name),
	}
}

type defaultControllerMetrics struct {
	clock clock.Clock

	activeWorkers   metrics.GaugeMetric
	reconcileTotal  metrics.CounterMetricLabeled
	reconcileErrors metrics.CounterMetric
	reconcilePanics metrics.CounterMetric

	reconcileTime metrics.HistogramMetric

	workerCount metrics.SettableGaugeMetric
}

func (m *defaultControllerMetrics) sinceInSeconds(start time.Time) float64 {
	return m.clock.Since(start).Seconds()
}

type noopMetric struct{}

func (noopMetric) Inc() {}
func (noopMetric) Dec() {}

func (noopMetric) Set(_ float64) {}

func (noopMetric) IncLabeled(_ string) {}

func (noopMetric) Observe(_ float64) {}
