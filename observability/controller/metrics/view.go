package metrics

import (
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

var reconcileBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.1, 0.15, 0.2, 0.25, 0.3, 0.35, 0.4, 0.45, 0.5,
	0.6, 0.7, 0.8, 0.9, 1.0, 1.25, 1.5, 1.75, 2.0, 2.5, 3.0, 3.5, 4.0, 4.5, 5,
	6, 7, 8, 9, 10, 15, 20, 25, 30, 40, 50, 60,
}

var workqueueBuckets = []float64{
	1e-8, 1e-7, 1e-6, 1e-5, 1e-4, 1e-3, 1e-2, 1e-1, 1, 10, 100, 1000,
}

func ViewOptions() []sdkmetric.Option {
	return []sdkmetric.Option{
		// 1) controller_runtime_reconcile_time_seconds
		sdkmetric.WithView(
			sdkmetric.NewView(
				sdkmetric.Instrument{Name: "controller_runtime_reconcile_time_seconds"},
				sdkmetric.Stream{
					Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
						Boundaries: reconcileBuckets,
					},
				},
			),
		),

		// 2) workqueue_queue_latency_seconds
		sdkmetric.WithView(
			sdkmetric.NewView(
				sdkmetric.Instrument{Name: "workqueue_queue_latency_seconds"},
				sdkmetric.Stream{
					Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
						Boundaries: workqueueBuckets,
					},
				},
			),
		),

		// 3) workqueue_work_duration_seconds
		sdkmetric.WithView(
			sdkmetric.NewView(
				sdkmetric.Instrument{Name: "workqueue_work_duration_seconds"},
				sdkmetric.Stream{
					Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
						Boundaries: workqueueBuckets,
					},
				},
			),
		),
	}
}
