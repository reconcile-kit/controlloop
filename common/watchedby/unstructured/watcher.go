package unstructured

import (
	"context"
	"errors"
	"fmt"

	"github.com/reconcile-kit/api/resource"

	"github.com/reconcile-kit/controlloop/common/listall"
	"github.com/reconcile-kit/controlloop/common/unstructured_resource"
	"github.com/reconcile-kit/controlloop/common/watchedby/selectors"
)

type Notifier struct {
	client unstructured.UnstructuredClient
}

func NewNotifier(client unstructured.UnstructuredClient) *Notifier {
	return &Notifier{
		client: client,
	}
}

func (w *Notifier) NotifyWatchedByResources(
	ctx context.Context,
	selector selectors.Selector[*unstructured.UnstructuredResource],
) error {
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

func (w *Notifier) getWatchedResources(
	ctx context.Context,
	selector selectors.Selector[*unstructured.UnstructuredResource],
) ([]*unstructured.UnstructuredResource, error) {
	listOpts := selector.GetListOpts()
	md := w.client.GetMetadataProvider()
	listOpts.ShardID = md.ShardID

	resourceMap, err := helpers.ListAll(ctx, w.client.List, listOpts)
	if err != nil {
		return nil, err
	}

	resources := make([]*unstructured.UnstructuredResource, 0, len(resourceMap))
	for _, obj := range resourceMap {
		resources = append(resources, obj)
	}

	return resources, nil
}
