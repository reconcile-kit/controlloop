package observability

import (
	controller "github.com/reconcile-kit/controlloop/observability/controller/metrics"
	workqueue "github.com/reconcile-kit/controlloop/observability/workqueue/metrics"
	"go.opentelemetry.io/otel/metric"
)

func Init(opt Options) {
	controller.Init(opt.Meter)
	workqueue.Init(opt.Meter)
}

type Options struct {
	Meter metric.Meter
}
