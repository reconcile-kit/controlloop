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

var (
	meter metric.Meter

	addsCtr    metric.Int64Counter
	retriesCtr metric.Int64Counter

	queueLatencyHist        metric.Float64Histogram
	workDurationHist        metric.Float64Histogram
	depthUpDown             metric.Int64UpDownCounter
	unfinishedGauge         metric.Float64ObservableGauge
	longestRunningProcGauge metric.Float64ObservableGauge
)

type key struct{ name string }

var (
	unfinishedVals sync.Map // map[key]float64
	longestVals    sync.Map // map[key]float64
)

func Init(m metric.Meter) {
	meter = otel.Meter("workqueue")
	if m != nil {
		meter = m
	}

	var err error

	addsCtr, err = meter.Int64Counter("workqueue_adds_total")
	if err != nil {
		panic(err)
	}
	retriesCtr, err = meter.Int64Counter("workqueue_retries_total")
	if err != nil {
		panic(err)
	}

	queueLatencyHist, err = meter.Float64Histogram("workqueue_queue_latency_seconds")
	if err != nil {
		panic(err)
	}
	workDurationHist, err = meter.Float64Histogram("workqueue_work_duration_seconds")
	if err != nil {
		panic(err)
	}

	depthUpDown, err = meter.Int64UpDownCounter("workqueue_depth")
	if err != nil {
		panic(err)
	}

	unfinishedGauge, err = meter.Float64ObservableGauge("workqueue_unfinished_work_seconds")
	if err != nil {
		panic(err)
	}
	longestRunningProcGauge, err = meter.Float64ObservableGauge("workqueue_longest_running_processor_seconds")
	if err != nil {
		panic(err)
	}

	// callback для observable gauges (только label "name")
	_, err = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		unfinishedVals.Range(func(k, v any) bool {
			lbl := k.(key)
			o.ObserveFloat64(unfinishedGauge, v.(float64),
				metric.WithAttributes(attrName.String(lbl.name)),
			)
			return true
		})
		longestVals.Range(func(k, v any) bool {
			lbl := k.(key)
			o.ObserveFloat64(longestRunningProcGauge, v.(float64),
				metric.WithAttributes(attrName.String(lbl.name)),
			)
			return true
		})
		return nil
	}, unfinishedGauge, longestRunningProcGauge)
	if err != nil {
		panic(err)
	}

	workqueue.SetProvider(WorkqueueMetricsProvider{})
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

type otelSettableGauge struct{ key key }  // unfinished
func (g otelSettableGauge) Set(v float64) { unfinishedVals.Store(g.key, v) }

type otelSettableGaugeLongest struct{ key key }  // longest running
func (g otelSettableGaugeLongest) Set(v float64) { longestVals.Store(g.key, v) }

type otelHistogram struct {
	h     metric.Float64Histogram
	attrs []attribute.KeyValue
}

func (h otelHistogram) Observe(v float64) {
	h.h.Record(context.Background(), v, metric.WithAttributes(h.attrs...))
}

type WorkqueueMetricsProvider struct{}

func fixedAttrs(name string) []attribute.KeyValue {
	return []attribute.KeyValue{attrName.String(name)}
}

func (WorkqueueMetricsProvider) NewDepthMetric(name string) workqueue.GaugeMetric {
	return &otelGauge{udCtr: depthUpDown, attrs: fixedAttrs(name)}
}
func (WorkqueueMetricsProvider) NewAddsMetric(name string) workqueue.CounterMetric {
	return otelCounter{ctr: addsCtr, attrs: fixedAttrs(name)}
}
func (WorkqueueMetricsProvider) NewLatencyMetric(name string) workqueue.HistogramMetric {
	return otelHistogram{h: queueLatencyHist, attrs: fixedAttrs(name)}
}
func (WorkqueueMetricsProvider) NewWorkDurationMetric(name string) workqueue.HistogramMetric {
	return otelHistogram{h: workDurationHist, attrs: fixedAttrs(name)}
}
func (WorkqueueMetricsProvider) NewUnfinishedWorkSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return otelSettableGauge{key: key{name: name}}
}
func (WorkqueueMetricsProvider) NewLongestRunningProcessorSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return otelSettableGaugeLongest{key: key{name: name}}
}
func (WorkqueueMetricsProvider) NewRetriesMetric(name string) workqueue.CounterMetric {
	return otelCounter{ctr: retriesCtr, attrs: fixedAttrs(name)}
}
