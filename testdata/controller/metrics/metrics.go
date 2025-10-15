package metrics

import (
	"context"
	"github.com/reconcile-kit/controlloop/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"sync"
)

var attrName = attribute.Key("controller")

var (
	attrController = attribute.Key("controller")
	attrResult     = attribute.Key("result")
)

type key struct{ controller string }

func NewControllerMetrics(m metric.Meter) *ControllerMetricsProvider {
	meter := otel.Meter("controlloop")
	if m != nil {
		meter = m
	}

	var err error

	wmp := &ControllerMetricsProvider{
		meter: meter,
	}

	ReconcileTotal, err := meter.Int64Counter("controlloop_reconcile")
	if err != nil {
		panic(err)
	}
	wmp.ReconcileTotal = ReconcileTotal

	ReconcileErrors, err := meter.Int64Counter("controlloop_reconcile_errors")
	if err != nil {
		panic(err)
	}
	wmp.ReconcileErrors = ReconcileErrors

	ReconcilePanics, err := meter.Int64Counter("controlloop_reconcile_panics")
	if err != nil {
		panic(err)
	}
	wmp.ReconcilePanics = ReconcilePanics

	ReconcileTime, err := meter.Float64Histogram("controlloop_reconcile_time_seconds")
	if err != nil {
		panic(err)
	}
	wmp.ReconcileTime = ReconcileTime

	WorkerCountGauge, err := meter.Float64ObservableGauge("controlloop_max_concurrent_reconciles")
	if err != nil {
		panic(err)
	}
	wmp.WorkerCountGauge = WorkerCountGauge

	ActiveWorkersCount, err := meter.Int64UpDownCounter("controlloop_active_workers")
	if err != nil {
		panic(err)
	}
	wmp.ActiveWorkersCount = ActiveWorkersCount

	_, err = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		wmp.workerCountVals.Range(func(k, v any) bool {
			lbl := k.(key)
			o.ObserveFloat64(wmp.WorkerCountGauge, v.(float64),
				metric.WithAttributes(attrController.String(lbl.controller)))
			return true
		})
		return nil
	}, wmp.WorkerCountGauge)
	if err != nil {
		panic(err)
	}

	return wmp
}

type ControllerMetricsProvider struct {
	meter metric.Meter

	workerCountVals sync.Map

	ReconcileTotal     metric.Int64Counter
	ReconcileErrors    metric.Int64Counter
	ReconcilePanics    metric.Int64Counter
	ReconcileTime      metric.Float64Histogram
	WorkerCountGauge   metric.Float64ObservableGauge
	ActiveWorkersCount metric.Int64UpDownCounter
}

func fixedAttrs(name string) []attribute.KeyValue {
	return []attribute.KeyValue{attrName.String(name)}
}

type otelGauge struct {
	udCtr  metric.Int64UpDownCounter
	attrs  []attribute.KeyValue
	lastMu sync.Mutex
	last   int64
}

func (g *otelGauge) Inc() { g.Add(1) }
func (g *otelGauge) Dec() { g.Add(-1) }
func (g *otelGauge) Add(n int64) {
	g.lastMu.Lock()
	defer g.lastMu.Unlock()
	g.udCtr.Add(context.Background(), n, metric.WithAttributes(g.attrs...))
	g.last += n
}
func (g *otelGauge) Set(v float64) {
	g.lastMu.Lock()
	defer g.lastMu.Unlock()
	newVal := int64(v)
	delta := newVal - g.last
	if delta != 0 {
		g.udCtr.Add(context.Background(), delta, metric.WithAttributes(g.attrs...))
		g.last = newVal
	}
}

type otelCounter struct {
	ctr   metric.Int64Counter
	attrs []attribute.KeyValue
}

func (c otelCounter) Inc() {
	c.ctr.Add(context.Background(), 1, metric.WithAttributes(c.attrs...))
}

type otelCounterLabled struct {
	ctr   metric.Int64Counter
	attrs []attribute.KeyValue
}

func (c otelCounterLabled) IncLabeled(label string) {
	c.attrs = append(c.attrs, attrResult.String(label))
	c.ctr.Add(context.Background(), 1, metric.WithAttributes(c.attrs...))
}

type otelHistogram struct {
	h     metric.Float64Histogram
	attrs []attribute.KeyValue
}

func (h otelHistogram) Observe(v float64) {
	h.h.Record(context.Background(), v, metric.WithAttributes(h.attrs...))
}

type otelSettableGauge struct {
	key             key
	workerCountVals *sync.Map
}

func (g otelSettableGauge) Set(v float64) { g.workerCountVals.Store(g.key, v) }

func (w *ControllerMetricsProvider) NewActiveWorkersMetric(name string) metrics.GaugeMetric {
	return &otelGauge{udCtr: w.ActiveWorkersCount, attrs: fixedAttrs(name)}
}
func (w *ControllerMetricsProvider) NewReconcileTotalMetric(name string) metrics.CounterMetricLabeled {
	return &otelCounterLabled{ctr: w.ReconcileTotal, attrs: fixedAttrs(name)}
}
func (w *ControllerMetricsProvider) NewReconcileTimeMetric(name string) metrics.HistogramMetric {
	return &otelHistogram{h: w.ReconcileTime, attrs: fixedAttrs(name)}
}
func (w *ControllerMetricsProvider) NewWorkerCountMetrics(name string) metrics.SettableGaugeMetric {
	return &otelSettableGauge{key: key{controller: name}, workerCountVals: &w.workerCountVals}
}
func (w *ControllerMetricsProvider) NewReconcilePanicsMetric(name string) metrics.CounterMetric {
	return &otelCounter{ctr: w.ReconcilePanics, attrs: fixedAttrs(name)}
}
func (w *ControllerMetricsProvider) NewReconcileErrorsMetric(name string) metrics.CounterMetric {
	return &otelCounter{ctr: w.ReconcileErrors, attrs: fixedAttrs(name)}
}
