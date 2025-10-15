package controlloop

import (
	"context"
	"errors"
	"fmt"
	"github.com/reconcile-kit/api/resource"
	"github.com/reconcile-kit/controlloop/assertions"
	_ "github.com/reconcile-kit/controlloop/testdata/workqueue/metrics"
	"k8s.io/utils/clock"
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
	metrics     *defaultControllerMetrics
}

func New[T resource.Object[T]](r Reconcile[T], storage ReconcileStorage[T], options ...ClOption) *ControlLoop[T] {
	currentOptions := &opts{}
	for _, o := range options {
		o(currentOptions)
	}
	t := assertions.TypeOf[T]()
	memoryStorage := storage.GetMemoryStorage()
	controlLoop := &ControlLoop[T]{
		r:           r,
		name:        t.Name(),
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

	controlLoop.metrics = newControllerMetrics(currentOptions.metricsProvider, t.Name(), clock.RealClock{})

	return controlLoop
}

func (cl *ControlLoop[T]) Run() {
	stopping := atomic.Bool{}
	stopping.Store(false)

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
					cl.Queue.queue.Add(object.GetName())
					cl.Queue.queue.Done(object.GetName())
				}
				cl.Queue.queue.Done(object.GetName())
				continue
			}

			cl.metrics.activeWorkers.Inc()
			result, err := cl.reconcile(ctx, r, object)
			cl.metrics.activeWorkers.Dec()

			switch {
			case err != nil:
				cl.Queue.addRateLimited(object)
				cl.metrics.reconcileErrors.Inc()
				cl.metrics.reconcileTotal.IncLabeled(labelError)
				cl.l.Error(err.Error())
			case result.RequeueAfter > 0:
				cl.Queue.addAfter(object, result.RequeueAfter)
				cl.metrics.reconcileTotal.IncLabeled(labelRequeueAfter)
			case result.Requeue:
				cl.Queue.add(object)
				cl.metrics.reconcileTotal.IncLabeled(labelRequeue)

			default:
				if object.GetDeletionTimestamp() != "" {
					cl.Storage.Delete(object.GetName())
				} else {
					cl.Queue.finalize(object.GetName())
				}
				cl.metrics.reconcileTotal.IncLabeled(labelSuccess)
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
	cl.metrics.workerCount.Set(float64(cl.concurrency))
}

func (cl *ControlLoop[T]) Stop() {
	cl.stopChannel <- struct{}{}
	<-cl.exitChannel
}

func (cl *ControlLoop[T]) reconcile(ctx context.Context, r Reconcile[T], object T) (result Result, reterr error) {
	reconcileStartTS := time.Now()
	defer func() {
		cl.metrics.reconcileTime.Observe(cl.metrics.sinceInSeconds(reconcileStartTS))
	}()
	defer func() {
		if r := recover(); r != nil {
			cl.metrics.reconcilePanics.Inc()
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
