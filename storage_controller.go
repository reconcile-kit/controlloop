package controlloop

import (
	"fmt"
	"github.com/reconcile-kit/api/resource"
)

type Storage[T resource.Object[T]] interface {
	Create(item T) error
	Get(objectKey resource.ObjectKey) (T, bool, error)
	List(listOpts resource.ListOpts) (map[resource.ObjectKey]T, error)
	Update(item T) error
	UpdateStatus(item T) error
	Delete(objectKey resource.ObjectKey) error
}

type Receiver interface {
	Receive(objectKey resource.ObjectKey) error
	Remove(objectKey resource.ObjectKey)
	Init() error
	GetGroupKind() resource.GroupKind
}

type ReconcileStorage[T resource.Object[T]] interface {
	Storage[T]
	GetMemoryStorage() *MemoryStorage[T]
}

type StorageController[T resource.Object[T]] struct {
	shardID         string
	groupKind       resource.GroupKind
	memoryStorage   *MemoryStorage[T]
	externalStorage resource.ExternalStorage[T]
}

func NewStorageController[T resource.Object[T]](
	ShardID string,
	groupKind resource.GroupKind,
	externalStorage resource.ExternalStorage[T],
	memoryStorage *MemoryStorage[T],
) (*StorageController[T], error) {

	sc := &StorageController[T]{
		shardID:         ShardID,
		groupKind:       groupKind,
		memoryStorage:   memoryStorage,
		externalStorage: externalStorage,
	}

	return sc, nil
}

func (s *StorageController[T]) Init() error {
	listPendingResources, err := s.externalStorage.ListPending(s.shardID, s.groupKind)
	if err != nil {
		return fmt.Errorf("cannot list pending resources: %w", err)
	}
	for _, pendingResource := range listPendingResources {
		s.memoryStorage.Add(pendingResource)
	}

	return nil
}

func (s *StorageController[T]) Receive(objectKey resource.ObjectKey) error {
	res, exist, err := s.externalStorage.Get(s.shardID, s.groupKind, objectKey)
	if err != nil {
		return fmt.Errorf("cannot get received resource: %w", err)
	}
	if !exist {
		return nil
	}

	s.memoryStorage.Add(res)
	return nil
}

func (s *StorageController[T]) Remove(objectKey resource.ObjectKey) {
	s.memoryStorage.Delete(objectKey)
}

func (s *StorageController[T]) GetGroupKind() resource.GroupKind {
	return s.groupKind
}

func (s *StorageController[T]) Create(item T) error {
	item, err := s.externalStorage.Create(item)
	if err != nil {
		return fmt.Errorf("cannot create resource: %w", err)
	}
	s.memoryStorage.Add(item)

	return nil
}

func (s *StorageController[T]) Get(objectKey resource.ObjectKey) (T, bool, error) {
	var zero T
	res, ok := s.memoryStorage.Get(objectKey)
	if ok {
		return res, true, nil
	}
	res, exist, err := s.externalStorage.Get(s.shardID, s.groupKind, objectKey)
	if err != nil {
		return zero, false, err
	}
	if !exist {
		return zero, false, nil
	}
	s.memoryStorage.AddWithoutRequeue(res)
	return res, true, nil
}

func (s *StorageController[T]) List(listOpts resource.ListOpts) (map[resource.ObjectKey]T, error) {
	result := map[resource.ObjectKey]T{}
	items, err := s.externalStorage.List(listOpts)
	if err != nil {
		return nil, fmt.Errorf("cannot list resources: %w", err)
	}
	for _, item := range items {
		result[item.GetName()] = item
	}
	return result, nil
}

func (s *StorageController[T]) Update(item T) error {
	res, err := s.externalStorage.Update(item)
	if err != nil {
		return fmt.Errorf("cannot update resource: %w", err)
	}
	s.memoryStorage.AddWithoutRequeue(res)

	return nil
}

func (s *StorageController[T]) UpdateStatus(item T) error {
	res, err := s.externalStorage.UpdateStatus(item)
	if err != nil {
		return fmt.Errorf("cannot update resource: %w", err)
	}
	s.memoryStorage.AddWithoutRequeue(res)
	return nil
}

func (s *StorageController[T]) Delete(item resource.ObjectKey) error {
	err := s.externalStorage.Delete(s.shardID, s.groupKind, item)
	if err != nil {
		return fmt.Errorf("cannot delete resource: %w", err)
	}

	return nil
}

func (s *StorageController[T]) GetMemoryStorage() *MemoryStorage[T] {
	return s.memoryStorage
}
