package controlloop

import (
	"github.com/reconcile-kit/controlloop/assertions"
	"github.com/reconcile-kit/controlloop/resource"
	"k8s.io/client-go/util/workqueue"
	"reflect"
	"sync"
	"time"
)

type StorageSet struct {
	mu    sync.RWMutex
	store map[reflect.Type]interface{}
}

func NewStorageSet() *StorageSet {
	return &StorageSet{
		store: make(map[reflect.Type]interface{}),
	}
}

func GetStorage[T resource.Object[T]](storages *StorageSet) (T, bool) {
	storages.mu.RLock()
	defer storages.mu.RUnlock()
	var zero T
	raw, ok := storages.store[assertions.TypeOf[T]()]
	if !ok {
		return zero, false
	}
	return assertions.As[T](raw)
}

func SetStorage[T resource.Object[T]](storages *StorageSet, store Storage[T]) {
	storages.mu.Lock()
	defer storages.mu.Unlock()
	storages.store[assertions.TypeOf[T]()] = store
}

type Storage[T resource.Object[T]] interface {
	Add(item T)
	Get(item T) T
	List() map[resource.ObjectKey]T
	Update(item T) error
	Delete(item T)
	getLast() (T, bool, error)
}

func NewMemoryStorage[T resource.Object[T]](opts ...StorageOption) *MemoryStorage[T] {
	currentsOptions := &storageOpts{}
	for _, o := range opts {
		o(currentsOptions)
	}
	rateLimiter := workqueue.DefaultTypedControllerRateLimiter[resource.ObjectKey]()
	if currentsOptions.rateLimits != nil {
		rateLimiter = workqueue.NewTypedItemExponentialFailureRateLimiter[resource.ObjectKey](currentsOptions.rateLimits.Min, currentsOptions.rateLimits.Max)
	}
	q := NewQueue[T](rateLimiter)
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

func (s *MemoryStorage[T]) Get(item T) T {
	s.m.RLock()
	defer s.m.RUnlock()
	var zero T
	val, exist := s.objects[item.GetName()]
	if !exist {
		return zero
	}
	return val.DeepCopy()
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
	s.Queue.done(item)
	return nil
}

func (s *MemoryStorage[T]) Delete(item T) {
	s.m.Lock()
	defer s.m.Unlock()
	s.Queue.finalize(item)
	delete(s.objects, item.GetName())
}

func (s *MemoryStorage[T]) getLast() (T, bool, error) {
	s.m.Lock()
	defer s.m.Unlock()
	var zero T
	name, shutdown := s.Queue.get()
	if shutdown {
		return zero, true, nil
	}
	// object already deleted
	if _, exist := s.objects[name]; !exist {
		return zero, false, KeyNotExist
	}
	return s.objects[name].DeepCopy(), false, nil
}

type storageOpts struct {
	rateLimits *storageRateLimits
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
