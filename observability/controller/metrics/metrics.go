// package metrics — OTel-эквивалент Prometheus-варианта
package metrics

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"sync"
)

var (
	attrController = attribute.Key("controller")
	attrResult     = attribute.Key("result")
)

type key struct{ controller string }

var (
	workerCountVals sync.Map
)

var (
	meter metric.Meter

	ReconcileTotal     metric.Int64Counter         // controller_runtime_reconcile_total{controller,result}
	ReconcileErrors    metric.Int64Counter         // controller_runtime_reconcile_errors_total{controller}
	ReconcilePanics    metric.Int64Counter         // controller_runtime_reconcile_panics_total{controller}
	ReconcileTime      metric.Float64Histogram     // controller_runtime_reconcile_time_seconds{controller}
	WorkerCountGauge   metric.Int64ObservableGauge // controller_runtime_max_concurrent_reconciles{controller}
	ActiveWorkersCount metric.Int64UpDownCounter   // controller_runtime_active_workers{controller}
)

func IncReconcileTotal(ctx context.Context, controller, result string, n int64) {
	ReconcileTotal.Add(ctx, n,
		metric.WithAttributes(attrController.String(controller), attrResult.String(result)))
}

func IncReconcileErrors(ctx context.Context, controller string, n int64) {
	ReconcileErrors.Add(ctx, n, metric.WithAttributes(attrController.String(controller)))
}

func IncReconcilePanics(ctx context.Context, controller string, n int64) {
	ReconcilePanics.Add(ctx, n, metric.WithAttributes(attrController.String(controller)))
}

func ObserveReconcileTime(ctx context.Context, controller string, seconds float64) {
	ReconcileTime.Record(ctx, seconds, metric.WithAttributes(attrController.String(controller)))
}

func SetWorkerCount(controller string, v int64) {
	workerCountVals.Store(key{controller}, v)
}

func AddActiveWorkers(ctx context.Context, controller string, delta int64) {
	ActiveWorkersCount.Add(ctx, delta, metric.WithAttributes(attrController.String(controller)))
}

func init() {
	meter = otel.Meter("controller-runtime")

	var err error

	ReconcileTotal, err = meter.Int64Counter("controller_runtime_reconcile_total")
	if err != nil {
		panic(err)
	}

	ReconcileErrors, err = meter.Int64Counter("controller_runtime_reconcile_errors_total")
	if err != nil {
		panic(err)
	}

	ReconcilePanics, err = meter.Int64Counter("controller_runtime_reconcile_panics_total")
	if err != nil {
		panic(err)
	}

	ReconcileTime, err = meter.Float64Histogram("controller_runtime_reconcile_time_seconds")
	if err != nil {
		panic(err)
	}

	WorkerCountGauge, err = meter.Int64ObservableGauge("controller_runtime_max_concurrent_reconciles")
	if err != nil {
		panic(err)
	}

	ActiveWorkersCount, err = meter.Int64UpDownCounter("controller_runtime_active_workers")
	if err != nil {
		panic(err)
	}

	_, err = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		workerCountVals.Range(func(k, v any) bool {
			lbl := k.(key)
			o.ObserveInt64(WorkerCountGauge, v.(int64),
				metric.WithAttributes(attrController.String(lbl.controller)))
			return true
		})
		return nil
	}, WorkerCountGauge)
	if err != nil {
		panic(err)
	}
}
