package structured

import (
	"context"
	"errors"
	"fmt"

	"github.com/reconcile-kit/api/resource"

	"github.com/reconcile-kit/controlloop"
	"github.com/reconcile-kit/controlloop/common/listall"
	"github.com/reconcile-kit/controlloop/common/watchedby/selectors"
)

type Notifier[T resource.Object[T]] struct {
	client *controlloop.RemoteClient[T]

	shardID string
}

func NewNotifier[T resource.Object[T]](client *controlloop.RemoteClient[T], shardID string) *Notifier[T] {
	return &Notifier[T]{
		client:  client,
		shardID: shardID,
	}
}

func (w *Notifier[T]) NotifyWatchedByResources(ctx context.Context, selector selectors.Selector[T]) error {
	resources, err := w.getWatchedResources(ctx, selector)
	if err != nil {
		return err
	}

	filteredResources := selector.FilterResources(resources)
	if len(filteredResources) == 0 {
		return nil
	}

	for _, watchedResource := range filteredResources {
		if err := w.client.Update(ctx, watchedResource); err != nil {
			if errors.Is(err, resource.NotFoundError) {
				continue
			}

			objectKey := watchedResource.GetName()
			return fmt.Errorf("update watched resource %q/%q: %w", objectKey.Namespace, objectKey.Name, err)
		}
	}

	return nil
}

func (w *Notifier[T]) getWatchedResources(ctx context.Context, selector selectors.Selector[T]) ([]T, error) {
	listOpts := selector.GetListOpts()
	listOpts.ShardID = w.shardID

	resourceMap, err := helpers.ListAll(ctx, w.client.List, listOpts)
	if err != nil {
		return nil, err
	}

	resources := make([]T, 0, len(resourceMap))
	for _, obj := range resourceMap {
		resources = append(resources, obj)
	}

	return resources, nil
}
