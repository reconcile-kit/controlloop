package controlloop

import (
	"context"
	"github.com/reconcile-kit/api/resource"
	"k8s.io/client-go/util/workqueue"
	"sync"
	"time"
)

func NewMemoryStorage[T resource.Object[T]](opts ...StorageOption) *MemoryStorage[T] {
	currentsOptions := &storageOpts{}
	for _, o := range opts {
		o(currentsOptions)
	}
	rateLimiter := workqueue.DefaultTypedControllerRateLimiter[resource.ObjectKey]()
	if currentsOptions.rateLimits != nil {
		rateLimiter = workqueue.NewTypedItemExponentialFailureRateLimiter[resource.ObjectKey](currentsOptions.rateLimits.Min, currentsOptions.rateLimits.Max)
	}
	q := NewQueue[T](rateLimiter, currentsOptions.metricsProvider)
	return &MemoryStorage[T]{
		m:       &sync.RWMutex{},
		Queue:   q,
		objects: make(map[resource.ObjectKey]T),
	}
}

type MemoryStorage[T resource.Object[T]] struct {
	m       *sync.RWMutex
	Queue   *Queue[T]
	objects map[resource.ObjectKey]T
}

func (s *MemoryStorage[T]) Add(item T) {
	s.m.Lock()
	defer s.m.Unlock()
	s.objects[item.GetName()] = item
	s.Queue.add(item)
}

func (s *MemoryStorage[T]) AddWithoutRequeue(item T) {
	s.m.Lock()
	defer s.m.Unlock()
	s.objects[item.GetName()] = item
}

func (s *MemoryStorage[T]) Get(key resource.ObjectKey) (T, bool) {
	s.m.RLock()
	defer s.m.RUnlock()
	var zero T
	val, exist := s.objects[key]
	if !exist {
		return zero, false
	}
	return val.DeepCopy(), true
}

func (s *MemoryStorage[T]) List() map[resource.ObjectKey]T {
	s.m.RLock()
	defer s.m.RUnlock()
	res := make(map[resource.ObjectKey]T, len(s.objects))
	for key, v := range s.objects {
		res[key] = v.DeepCopy()
	}
	return res
}

func (s *MemoryStorage[T]) Update(item T) error {
	s.m.Lock()
	defer s.m.Unlock()
	curr, exist := s.objects[item.GetName()]
	if !exist {
		return KeyNotExist
	}
	if curr.GetGeneration() > item.GetGeneration() {
		return AlreadyUpdated
	}
	curr.IncGeneration()
	s.objects[item.GetName()] = item
	s.Queue.add(item)
	return nil
}

func (s *MemoryStorage[T]) UpdateWithoutRequeue(item T) error {
	s.m.Lock()
	defer s.m.Unlock()
	curr, exist := s.objects[item.GetName()]
	if !exist {
		return KeyNotExist
	}
	if curr.GetGeneration() > item.GetGeneration() {
		return AlreadyUpdated
	}
	item.IncGeneration()
	s.objects[item.GetName()] = item
	return nil
}

func (s *MemoryStorage[T]) Delete(objectKey resource.ObjectKey) {
	s.m.Lock()
	defer s.m.Unlock()
	s.Queue.finalize(objectKey)
	delete(s.objects, objectKey)
}

func (s *MemoryStorage[T]) getLast() (resource.ObjectKey, T, bool, error) {
	var zero T
	name, shutdown := s.Queue.get()
	if shutdown {
		return name, zero, true, nil
	}
	s.m.Lock()
	defer s.m.Unlock()
	if _, exist := s.objects[name]; !exist {
		return name, zero, false, KeyNotExist
	}
	objectCopy := s.objects[name].DeepCopy()
	return name, objectCopy, false, nil
}

type MemoryStorageWrapped[T resource.Object[T]] struct {
	memoryStorage *MemoryStorage[T]
}

func NewMemoryStorageWrapped[T resource.Object[T]](opts ...StorageOption) *MemoryStorageWrapped[T] {
	return &MemoryStorageWrapped[T]{
		memoryStorage: NewMemoryStorage[T](opts...),
	}
}

func (m MemoryStorageWrapped[T]) Create(ctx context.Context, item T) error {
	m.memoryStorage.Add(item)
	return nil
}

func (m MemoryStorageWrapped[T]) Get(ctx context.Context, key resource.ObjectKey) (T, bool, error) {
	item, ok := m.memoryStorage.Get(key)
	if !ok {
		return item, false, nil
	}
	return item, true, nil
}

func (m MemoryStorageWrapped[T]) List(ctx context.Context, listOpts resource.ListOpts) (map[resource.ObjectKey]T, error) {
	list := m.memoryStorage.List()
	return list, nil
}

func (m MemoryStorageWrapped[T]) Update(ctx context.Context, item T) error {
	return m.memoryStorage.Update(item)
}

func (m MemoryStorageWrapped[T]) UpdateStatus(ctx context.Context, item T) error {
	return m.memoryStorage.UpdateWithoutRequeue(item)
}

func (m MemoryStorageWrapped[T]) Delete(ctx context.Context, objectKey resource.ObjectKey) error {
	m.memoryStorage.Delete(objectKey)
	return nil
}

func (m MemoryStorageWrapped[T]) GetMemoryStorage() *MemoryStorage[T] {
	return m.memoryStorage
}

type storageOpts struct {
	rateLimits      *storageRateLimits
	metricsProvider workqueue.MetricsProvider
}

type storageRateLimits struct {
	Min time.Duration
	Max time.Duration
}

type StorageOption func(*storageOpts)

func WithCustomRateLimits(min, max time.Duration) StorageOption {
	return func(o *storageOpts) {
		o.rateLimits = &storageRateLimits{
			Min: min,
			Max: max,
		}
	}
}

func WithMetricsWorkqueueProvider(mp workqueue.MetricsProvider) StorageOption {
	return func(o *storageOpts) {
		o.metricsProvider = mp
	}
}
