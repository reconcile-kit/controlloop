package controlloop

import (
	"context"
	"fmt"
	"github.com/reconcile-kit/api/resource"
)

type RemoteClient[T resource.Object[T]] struct {
	shardID         string
	groupKind       resource.GroupKind
	externalStorage resource.ExternalStorage[T]
}

func NewRemoteClient[T resource.Object[T]](
	groupKind resource.GroupKind,
	externalStorage resource.ExternalStorage[T],
) (*RemoteClient[T], error) {
	sc := &RemoteClient[T]{
		groupKind:       groupKind,
		externalStorage: externalStorage,
	}
	return sc, nil
}

func (s *RemoteClient[T]) Delete(ctx context.Context, objectKey resource.ObjectKey) error {
	return s.externalStorage.Delete(ctx, s.groupKind, objectKey)
}

func (s *RemoteClient[T]) Create(ctx context.Context, item T) error {
	err := s.externalStorage.Create(ctx, item)
	if err != nil {
		return fmt.Errorf("cannot create resource: %w", err)
	}
	return nil
}

func (s *RemoteClient[T]) Get(ctx context.Context, objectKey resource.ObjectKey) (T, bool, error) {
	var zero T
	res, exist, err := s.externalStorage.Get(ctx, s.groupKind, objectKey)
	if err != nil {
		return zero, false, err
	}
	if !exist {
		return zero, false, nil
	}
	return res, true, nil
}

func (s *RemoteClient[T]) List(ctx context.Context, listOpts resource.ListOpts) (map[resource.ObjectKey]T, error) {
	result := map[resource.ObjectKey]T{}
	items, err := s.externalStorage.List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("cannot list resources: %w", err)
	}
	for _, item := range items {
		result[item.GetName()] = item
	}
	return result, nil
}

func (s *RemoteClient[T]) Update(ctx context.Context, item T) error {
	err := s.externalStorage.Update(ctx, item)
	if err != nil {
		return fmt.Errorf("cannot update resource: %w", err)
	}
	return nil
}

func (s *RemoteClient[T]) UpdateStatus(ctx context.Context, item T) error {
	err := s.externalStorage.UpdateStatus(ctx, item)
	if err != nil {
		return fmt.Errorf("cannot update resource: %w", err)
	}
	return nil
}
