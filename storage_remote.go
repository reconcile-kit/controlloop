package controlloop

import (
	"fmt"
	"github.com/reconcile-kit/api/resource"
)

type RemoteClient[T resource.Object[T]] struct {
	shardID         string
	groupKind       resource.GroupKind
	externalStorage resource.ExternalStorage[T]
}

func NewRemoteClient[T resource.Object[T]](
	ShardID string,
	groupKind resource.GroupKind,
	externalStorage resource.ExternalStorage[T],
) (*RemoteClient[T], error) {
	sc := &RemoteClient[T]{
		shardID:         ShardID,
		groupKind:       groupKind,
		externalStorage: externalStorage,
	}
	return sc, nil
}

func (s *RemoteClient[T]) Delete(objectKey resource.ObjectKey) error {
	return s.externalStorage.Delete(s.shardID, s.groupKind, objectKey)
}

func (s *RemoteClient[T]) Create(item T) error {
	item, err := s.externalStorage.Create(item)
	if err != nil {
		return fmt.Errorf("cannot create resource: %w", err)
	}
	return nil
}

func (s *RemoteClient[T]) Get(objectKey resource.ObjectKey) (T, bool, error) {
	var zero T
	res, exist, err := s.externalStorage.Get(s.shardID, s.groupKind, objectKey)
	if err != nil {
		return zero, false, err
	}
	if !exist {
		return zero, false, nil
	}
	return res, true, nil
}

func (s *RemoteClient[T]) List(listOpts resource.ListOpts) (map[resource.ObjectKey]T, error) {
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

func (s *RemoteClient[T]) Update(item T) error {
	_, err := s.externalStorage.Update(item)
	if err != nil {
		return fmt.Errorf("cannot update resource: %w", err)
	}
	return nil
}

func (s *RemoteClient[T]) UpdateStatus(item T) error {
	_, err := s.externalStorage.UpdateStatus(item)
	if err != nil {
		return fmt.Errorf("cannot update resource: %w", err)
	}
	return nil
}
