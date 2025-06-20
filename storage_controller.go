package controlloop

import (
	"context"
	"fmt"
	"github.com/reconcile-kit/api/resource"
	"github.com/reconcile-kit/controlloop/assertions"
)

type Storage[T resource.Object[T]] interface {
	Create(ctx context.Context, item T) error
	Get(ctx context.Context, objectKey resource.ObjectKey) (T, bool, error)
	List(ctx context.Context, listOpts resource.ListOpts) (map[resource.ObjectKey]T, error)
	Update(ctx context.Context, item T) error
	UpdateStatus(ctx context.Context, item T) error
	Delete(ctx context.Context, objectKey resource.ObjectKey) error
}

type Receiver interface {
	Receive(ctx context.Context, objectKey resource.ObjectKey) error
	Remove(ctx context.Context, objectKey resource.ObjectKey)
	Init(ctx context.Context) error
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
	shardID string,
	externalStorage resource.ExternalStorage[T],
	memoryStorage *MemoryStorage[T],
) (*StorageController[T], error) {
	gk := assertions.GetGroupKindFromType[T]()
	if gk.Kind == "" || gk.Group == "" {
		return nil, fmt.Errorf("group and kind must be set in resource")
	}
	if shardID == "" {
		return nil, fmt.Errorf("shardID is empty for %s %s ", gk.Kind, gk.Group)
	}
	sc := &StorageController[T]{
		shardID:         shardID,
		groupKind:       gk,
		memoryStorage:   memoryStorage,
		externalStorage: externalStorage,
	}

	return sc, nil
}

func (s *StorageController[T]) Init(ctx context.Context) error {
	listPendingResources, err := s.externalStorage.ListPending(ctx, s.shardID, s.groupKind)
	if err != nil {
		return fmt.Errorf("cannot list pending resources: %w", err)
	}
	for _, pendingResource := range listPendingResources {
		s.memoryStorage.Add(pendingResource)
	}

	return nil
}

func (s *StorageController[T]) Receive(ctx context.Context, objectKey resource.ObjectKey) error {
	res, exist, err := s.externalStorage.Get(ctx, s.groupKind, objectKey)
	if err != nil {
		return fmt.Errorf("cannot get received resource: %w", err)
	}
	if !exist {
		return nil
	}

	s.memoryStorage.Add(res)
	return nil
}

func (s *StorageController[T]) Remove(ctx context.Context, objectKey resource.ObjectKey) {
	s.memoryStorage.Delete(objectKey)
}

func (s *StorageController[T]) GetGroupKind() resource.GroupKind {
	return s.groupKind
}

func (s *StorageController[T]) Create(ctx context.Context, item T) error {
	item.SetResourceGroup(s.groupKind.Group)
	item.SetKind(s.groupKind.Kind)
	err := s.externalStorage.Create(ctx, item)
	if err != nil {
		return fmt.Errorf("cannot create resource: %w", err)
	}
	s.memoryStorage.Add(item)

	return nil
}

func (s *StorageController[T]) Get(ctx context.Context, objectKey resource.ObjectKey) (T, bool, error) {
	var zero T
	res, ok := s.memoryStorage.Get(objectKey)
	if ok {
		return res, true, nil
	}
	res, exist, err := s.externalStorage.Get(ctx, s.groupKind, objectKey)
	if err != nil {
		return zero, false, err
	}
	if !exist {
		return zero, false, nil
	}
	s.memoryStorage.AddWithoutRequeue(res)
	return res, true, nil
}

func (s *StorageController[T]) List(ctx context.Context, listOpts resource.ListOpts) (map[resource.ObjectKey]T, error) {
	result := map[resource.ObjectKey]T{}
	items, err := s.externalStorage.List(ctx, s.groupKind, listOpts)
	if err != nil {
		return nil, fmt.Errorf("cannot list resources: %w", err)
	}
	for _, item := range items {
		result[item.GetName()] = item
	}
	return result, nil
}

func (s *StorageController[T]) Update(ctx context.Context, item T) error {
	item.SetResourceGroup(s.groupKind.Group)
	item.SetKind(s.groupKind.Kind)
	err := s.externalStorage.Update(ctx, item)
	if err != nil {
		return fmt.Errorf("cannot update resource: %w", err)
	}
	s.memoryStorage.AddWithoutRequeue(item)

	return nil
}

func (s *StorageController[T]) UpdateStatus(ctx context.Context, item T) error {
	item.SetResourceGroup(s.groupKind.Group)
	item.SetKind(s.groupKind.Kind)
	err := s.externalStorage.UpdateStatus(ctx, item)
	if err != nil {
		return fmt.Errorf("cannot update resource: %w", err)
	}
	s.memoryStorage.AddWithoutRequeue(item)
	return nil
}

func (s *StorageController[T]) Delete(ctx context.Context, item resource.ObjectKey) error {
	err := s.externalStorage.Delete(ctx, s.groupKind, item)
	if err != nil {
		return fmt.Errorf("cannot delete resource: %w", err)
	}

	return nil
}

func (s *StorageController[T]) GetMemoryStorage() *MemoryStorage[T] {
	return s.memoryStorage
}
