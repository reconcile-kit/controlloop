package metrics

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"k8s.io/client-go/util/workqueue"
)

var attrName = attribute.Key("name")

type key struct{ name string }

func NewWorkqueueMetricsProvider(m metric.Meter) *WorkqueueMetricsProvider {
	meter := otel.Meter("workqueue")
	if m != nil {
		meter = m
	}

	var err error

	wmp := &WorkqueueMetricsProvider{
		meter: meter,
	}

	addsCtr, err := meter.Int64Counter("workqueue_adds")
	if err != nil {
		panic(err)
	}
	wmp.addsCtr = addsCtr

	retriesCtr, err := meter.Int64Counter("workqueue_retries")
	if err != nil {
		panic(err)
	}
	wmp.retriesCtr = retriesCtr

	queueLatencyHist, err := meter.Float64Histogram("workqueue_queue_latency_seconds")
	if err != nil {
		panic(err)
	}
	wmp.queueLatencyHist = queueLatencyHist

	workDurationHist, err := meter.Float64Histogram("workqueue_work_duration_seconds")
	if err != nil {
		panic(err)
	}
	wmp.workDurationHist = workDurationHist

	depthUpDown, err := meter.Int64UpDownCounter("workqueue_depth")
	if err != nil {
		panic(err)
	}
	wmp.depthUpDown = depthUpDown

	unfinishedGauge, err := meter.Float64ObservableGauge("workqueue_unfinished_work_seconds")
	if err != nil {
		panic(err)
	}
	wmp.unfinishedGauge = unfinishedGauge

	longestRunningProcGauge, err := meter.Float64ObservableGauge("workqueue_longest_running_processor_seconds")
	if err != nil {
		panic(err)
	}
	wmp.longestRunningProcGauge = longestRunningProcGauge

	// callback для observable gauges (только label "name")
	_, err = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		wmp.unfinishedVals.Range(func(k, v any) bool {
			lbl := k.(key)
			o.ObserveFloat64(wmp.unfinishedGauge, v.(float64),
				metric.WithAttributes(attrName.String(lbl.name)),
			)
			return true
		})
		wmp.longestVals.Range(func(k, v any) bool {
			lbl := k.(key)
			o.ObserveFloat64(wmp.longestRunningProcGauge, v.(float64),
				metric.WithAttributes(attrName.String(lbl.name)),
			)
			return true
		})
		return nil
	}, wmp.unfinishedGauge, wmp.longestRunningProcGauge)
	if err != nil {
		panic(err)
	}

	return wmp
}

type otelCounter struct {
	ctr   metric.Int64Counter
	attrs []attribute.KeyValue
}

func (c otelCounter) Inc() {
	c.ctr.Add(context.Background(), 1, metric.WithAttributes(c.attrs...))
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

type otelSettableGauge struct {
	key            key
	unfinishedVals *sync.Map
}

func (g otelSettableGauge) Set(v float64) { g.unfinishedVals.Store(g.key, v) }

type otelSettableGaugeLongest struct {
	key         key
	longestVals *sync.Map
}

func (g otelSettableGaugeLongest) Set(v float64) { g.longestVals.Store(g.key, v) }

type otelHistogram struct {
	h     metric.Float64Histogram
	attrs []attribute.KeyValue
}

func (h otelHistogram) Observe(v float64) {
	h.h.Record(context.Background(), v, metric.WithAttributes(h.attrs...))
}

type WorkqueueMetricsProvider struct {
	meter metric.Meter

	addsCtr    metric.Int64Counter
	retriesCtr metric.Int64Counter

	queueLatencyHist        metric.Float64Histogram
	workDurationHist        metric.Float64Histogram
	depthUpDown             metric.Int64UpDownCounter
	unfinishedGauge         metric.Float64ObservableGauge
	longestRunningProcGauge metric.Float64ObservableGauge

	unfinishedVals sync.Map
	longestVals    sync.Map
}

func fixedAttrs(name string) []attribute.KeyValue {
	return []attribute.KeyValue{attrName.String(name)}
}

func (w *WorkqueueMetricsProvider) NewDepthMetric(name string) workqueue.GaugeMetric {
	return &otelGauge{udCtr: w.depthUpDown, attrs: fixedAttrs(name)}
}
func (w *WorkqueueMetricsProvider) NewAddsMetric(name string) workqueue.CounterMetric {
	return otelCounter{ctr: w.addsCtr, attrs: fixedAttrs(name)}
}
func (w *WorkqueueMetricsProvider) NewLatencyMetric(name string) workqueue.HistogramMetric {
	return otelHistogram{h: w.queueLatencyHist, attrs: fixedAttrs(name)}
}
func (w *WorkqueueMetricsProvider) NewWorkDurationMetric(name string) workqueue.HistogramMetric {
	return otelHistogram{h: w.workDurationHist, attrs: fixedAttrs(name)}
}
func (w *WorkqueueMetricsProvider) NewUnfinishedWorkSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return otelSettableGauge{key: key{name: name}, unfinishedVals: &w.unfinishedVals}
}
func (w *WorkqueueMetricsProvider) NewLongestRunningProcessorSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return otelSettableGaugeLongest{key: key{name: name}, longestVals: &w.longestVals}
}
func (w *WorkqueueMetricsProvider) NewRetriesMetric(name string) workqueue.CounterMetric {
	return otelCounter{ctr: w.retriesCtr, attrs: fixedAttrs(name)}
}
