package controlloop

import (
	"context"
	"errors"
	"fmt"
	"github.com/reconcile-kit/api/resource"
	"github.com/reconcile-kit/controlloop/observability/controller/metrics"
	"sync"
	"sync/atomic"
	"time"
)

const defaultReconcileTime = time.Second * 30
const errorReconcileTime = time.Second * 5

type ControlLoop[T resource.Object[T]] struct {
	r           Reconcile[T]
	name        string
	stopChannel chan struct{}
	exitChannel chan struct{}
	l           Logger
	concurrency int
	Storage     *MemoryStorage[T]
	Queue       *Queue[T]
}

func New[T resource.Object[T]](r Reconcile[T], name string, storage ReconcileStorage[T], options ...ClOption) *ControlLoop[T] {
	currentOptions := &opts{}
	for _, o := range options {
		o(currentOptions)
	}
	memoryStorage := storage.GetMemoryStorage()
	controlLoop := &ControlLoop[T]{
		r:           r,
		name:        name,
		stopChannel: make(chan struct{}),
		exitChannel: make(chan struct{}),
		Storage:     memoryStorage,
		Queue:       memoryStorage.Queue,
	}

	if currentOptions.logger != nil {
		controlLoop.l = currentOptions.logger
	} else {
		controlLoop.l = &SimpleLogger{}
	}

	if currentOptions.concurrency > 0 {
		controlLoop.concurrency = currentOptions.concurrency
	} else {
		controlLoop.concurrency = 1
	}

	return controlLoop
}

func (cl *ControlLoop[T]) Run() {
	stopping := atomic.Bool{}
	stopping.Store(false)

	cl.initMetrics()

	go func() {
		<-cl.stopChannel
		delayQueueLen := cl.Queue.len()
		if delayQueueLen > 0 {
			stopping.Store(true)
			for object, _ := range cl.Queue.getExistedItems() {
				cl.Queue.queue.Add(object)
			}
		} else {
			cl.Queue.queue.ShutDownWithDrain()
		}
	}()

	f := func(wg *sync.WaitGroup) {
		defer wg.Done()

		r := cl.r
		ctx := context.Background()
		for {

			name, object, exit, err := cl.Storage.getLast()
			if exit {
				return
			}
			if err != nil {
				// object already deleted
				if errors.Is(err, KeyNotExist) {
					cl.Queue.queue.Done(name)
					continue
				}
			}

			if stopping.Load() && object.GetKillTimestamp() == "" {
				object.SetKillTimestamp(time.Now())
				err := cl.Storage.Update(object)
				if errors.Is(err, AlreadyUpdated) {
					cl.Queue.add(object)
					cl.Queue.done(object)
				}
				continue
			}

			metrics.ActiveWorkers.WithLabelValues(cl.name).Add(1)
			result, err := cl.reconcile(ctx, r, object)
			metrics.ActiveWorkers.WithLabelValues(cl.name).Add(-1)

			switch {
			case err != nil:
				cl.Queue.addRateLimited(object)
				metrics.ReconcileErrors.WithLabelValues(cl.name).Inc()
				metrics.ReconcileTotal.WithLabelValues(cl.name, labelError).Inc()
				cl.l.Error(err.Error())
			case result.RequeueAfter > 0:
				cl.Queue.addAfter(object, result.RequeueAfter)
				metrics.ReconcileTotal.WithLabelValues(cl.name, labelRequeueAfter).Inc()
			case result.Requeue:
				cl.Queue.add(object)
				metrics.ReconcileTotal.WithLabelValues(cl.name, labelRequeue).Inc()
			default:
				if object.GetDeletionTimestamp() != "" {
					cl.Storage.Delete(object.GetName())
				} else {
					cl.Queue.finalize(object.GetName())
				}
				metrics.ReconcileTotal.WithLabelValues(cl.name, labelSuccess).Inc()
			}

			cl.Queue.done(object)
			if stopping.Load() && cl.Queue.len() == 0 {
				cl.Queue.queue.ShutDownWithDrain()
			}
		}
	}

	wg := &sync.WaitGroup{}

	wg.Add(cl.concurrency)
	go func(wg *sync.WaitGroup) {
		wg.Wait()
		cl.exitChannel <- struct{}{}
	}(wg)

	for range cl.concurrency {
		go f(wg)
	}
}

func (cl *ControlLoop[T]) Stop() {
	cl.stopChannel <- struct{}{}
	<-cl.exitChannel
}

func (cl *ControlLoop[T]) reconcile(ctx context.Context, r Reconcile[T], object T) (result Result, reterr error) {
	defer func() {
		if r := recover(); r != nil {
			metrics.ReconcilePanics.WithLabelValues(cl.name).Inc()
			err := fmt.Errorf("recovered from panic: %v", r)
			cl.l.Error(err)
			reterr = err
		}
	}()
	return r.Reconcile(ctx, object)
}

const (
	labelError        = "error"
	labelRequeueAfter = "requeue_after"
	labelRequeue      = "requeue"
	labelSuccess      = "success"
)

func (cl *ControlLoop[T]) initMetrics() {
	metrics.ReconcileTotal.WithLabelValues(cl.name, labelError).Add(0)
	metrics.ReconcileTotal.WithLabelValues(cl.name, labelRequeueAfter).Add(0)
	metrics.ReconcileTotal.WithLabelValues(cl.name, labelRequeue).Add(0)
	metrics.ReconcileTotal.WithLabelValues(cl.name, labelSuccess).Add(0)
	metrics.ReconcileErrors.WithLabelValues(cl.name).Add(0)
	metrics.ReconcilePanics.WithLabelValues(cl.name).Add(0)
	metrics.WorkerCount.WithLabelValues(cl.name).Set(float64(cl.concurrency))
	metrics.ActiveWorkers.WithLabelValues(cl.name).Set(0)
}
