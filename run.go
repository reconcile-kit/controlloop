package controlloop

import (
	"context"
	"errors"
	"fmt"
	resource "github.com/reconcile-kit/api"
	"sync"
	"sync/atomic"
	"time"
)

const defaultReconcileTime = time.Second * 30
const errorReconcileTime = time.Second * 5

type ControlLoop[T resource.Object[T]] struct {
	r           Reconcile[T]
	stopChannel chan struct{}
	exitChannel chan struct{}
	l           Logger
	concurrency int
	Storage     *MemoryStorage[T]
	Queue       *Queue[T]
}

func New[T resource.Object[T]](r Reconcile[T], storage ReconcileStorage[T], options ...ClOption) *ControlLoop[T] {
	currentOptions := &opts{}
	for _, o := range options {
		o(currentOptions)
	}
	memoryStorage := storage.GetMemoryStorage()
	controlLoop := &ControlLoop[T]{
		r:           r,
		stopChannel: make(chan struct{}),
		exitChannel: make(chan struct{}),
		Storage:     memoryStorage,
		Queue:       memoryStorage.Queue,
	}

	if currentOptions.logger != nil {
		controlLoop.l = currentOptions.logger
	} else {
		controlLoop.l = &logger{}
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

			object, exit, err := cl.Storage.getLast()
			if exit {
				return
			}
			if err != nil {
				// object already deleted
				if errors.Is(err, KeyNotExist) {
					continue
				}
			}

			if stopping.Load() && object.GetKillTimestamp() == "" {
				object.SetKillTimestamp(time.Now())
				err := cl.Storage.Update(object)
				if errors.Is(err, AlreadyUpdated) {
					cl.Queue.queue.Add(object.GetName())
				}
				continue
			}

			result, err := cl.reconcile(ctx, r, object)

			switch {
			case err != nil:
				cl.Queue.addRateLimited(object)
			case result.RequeueAfter > 0:
				cl.Queue.addAfter(object, result.RequeueAfter)
			case result.Requeue:
				cl.Queue.add(object)
			default:
				if object.GetDeletionTimestamp() != "" {
					cl.Storage.Delete(object.GetName())
				} else {
					cl.Queue.finalize(object.GetName())
				}
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

func (cl *ControlLoop[T]) reconcile(ctx context.Context, r Reconcile[T], object T) (Result, error) {
	defer func() {
		if r := recover(); r != nil {
			cl.l.Error(fmt.Errorf("Recovered from panic: %v ", r))
		}
	}()
	return r.Reconcile(ctx, object)
}
