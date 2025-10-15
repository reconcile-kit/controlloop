package controlloop

import (
	"sync"
	"time"

	"github.com/reconcile-kit/api/resource"
	"github.com/reconcile-kit/controlloop/assertions"
	"k8s.io/client-go/util/workqueue"
)

type Queue[T resource.Object[T]] struct {
	queue        workqueue.TypedRateLimitingInterface[resource.ObjectKey]
	existedItems map[resource.ObjectKey]resource.Object[T]
	m            *sync.RWMutex
}

func NewQueue[T resource.Object[T]](rateLimiter workqueue.TypedRateLimiter[resource.ObjectKey], mp workqueue.MetricsProvider) *Queue[T] {
	t := assertions.TypeOf[T]()
	rateLimitingConfig := workqueue.TypedRateLimitingQueueConfig[resource.ObjectKey]{}
	rateLimitingConfig.DelayingQueue = workqueue.NewTypedDelayingQueueWithConfig[resource.ObjectKey](
		workqueue.TypedDelayingQueueConfig[resource.ObjectKey]{
			Name:            t.Name(),
			MetricsProvider: mp,
		})
	queue := workqueue.NewTypedRateLimitingQueueWithConfig[resource.ObjectKey](rateLimiter, rateLimitingConfig)
	return &Queue[T]{queue: queue, existedItems: make(map[resource.ObjectKey]resource.Object[T]), m: &sync.RWMutex{}}
}

func (q *Queue[T]) getExistedItems() map[resource.ObjectKey]resource.Object[T] {
	q.m.RLock()
	defer q.m.RUnlock()
	mapCopy := make(map[resource.ObjectKey]resource.Object[T], len(q.existedItems))
	for k, v := range q.existedItems {
		mapCopy[k] = v
	}
	return mapCopy
}

func (q *Queue[T]) len() int {
	q.m.RLock()
	defer q.m.RUnlock()
	return len(q.existedItems)
}

func (q *Queue[T]) add(item resource.Object[T]) {
	q.m.Lock()
	defer q.m.Unlock()
	q.existedItems[item.GetName()] = item
	q.queue.Add(item.GetName())
}

func (q *Queue[T]) finalize(objectKey resource.ObjectKey) {
	q.m.Lock()
	defer q.m.Unlock()
	delete(q.existedItems, objectKey)
}

func (q *Queue[T]) done(item resource.Object[T]) {
	q.queue.Done(item.GetName())
}

func (q *Queue[T]) addAfter(item resource.Object[T], duration time.Duration) {
	q.queue.AddAfter(item.GetName(), duration)
}

func (q *Queue[T]) addRateLimited(item resource.Object[T]) {
	q.queue.AddRateLimited(item.GetName())
}

func (q *Queue[t]) get() (resource.ObjectKey, bool) {
	name, shutdown := q.queue.Get()
	if shutdown {
		return resource.ObjectKey{}, true
	}
	return name, false
}
