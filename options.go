package controlloop

type opts struct {
	logger      Logger
	concurrency int
	storages    *StorageSet
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
