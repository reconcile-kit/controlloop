package controlloop

import "github.com/reconcile-kit/controlloop/metrics"

type opts struct {
	logger          Logger
	concurrency     int
	storages        *StorageSet
	metricsProvider metrics.MetricsProvider
}

type ClOption func(*opts)

func WithLogger(logger Logger) ClOption {
	return func(o *opts) {
		o.logger = logger
	}
}

func WithConcurrentReconciles(count int) ClOption {
	return func(o *opts) {
		o.concurrency = count
	}
}

func WithStorageSet(sr *StorageSet) ClOption {
	return func(o *opts) {
		o.storages = sr
	}
}

func WithMetricsProvider(mp metrics.MetricsProvider) ClOption {
	return func(o *opts) {
		o.metricsProvider = mp
	}
}
