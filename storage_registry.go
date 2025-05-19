package controlloop

import (
	"github.com/reconcile-kit/controlloop/assertions"
	"github.com/reconcile-kit/controlloop/resource"
	"reflect"
	"sync"
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

func GetStorage[T resource.Object[T]](storages *StorageSet) (Storage[T], bool) {
	storages.mu.RLock()
	defer storages.mu.RUnlock()
	var zero Storage[T]
	raw, ok := storages.store[assertions.TypeOf[T]()]
	if !ok {
		return zero, false
	}
	return assertions.As[Storage[T]](raw)
}

func SetStorage[T resource.Object[T]](storages *StorageSet, store Storage[T]) {
	storages.mu.Lock()
	defer storages.mu.Unlock()
	storages.store[assertions.TypeOf[T]()] = store
}
