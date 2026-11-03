package unstructured

import (
	"context"
	"fmt"

	"github.com/reconcile-kit/api/resource"
	cl "github.com/reconcile-kit/controlloop"
)

type UnstructuredClient interface {
	Delete(ctx context.Context, objectKey resource.ObjectKey) error
	Create(ctx context.Context, item *UnstructuredResource) error
	Get(ctx context.Context, objectKey resource.ObjectKey) (*UnstructuredResource, bool, error)
	List(ctx context.Context, listOpts resource.ListOpts) (map[resource.ObjectKey]*UnstructuredResource, error)
	Update(ctx context.Context, item *UnstructuredResource) error
	UpdateStatus(ctx context.Context, item *UnstructuredResource) error
	GetMetadataProvider() MetadataProvider
}

type unstructuredClient struct {
	client *cl.RemoteClient[*UnstructuredResource]
	md     MetadataProvider
}

func NewUnstructuredClient(
	client *cl.RemoteClient[*UnstructuredResource],
	md MetadataProvider,
) UnstructuredClient {
	return &unstructuredClient{
		client: client,
		md:     md,
	}
}

func (s *unstructuredClient) Delete(ctx context.Context, objectKey resource.ObjectKey) error {
	return s.client.Delete(ctx, objectKey)
}

func (s *unstructuredClient) Create(ctx context.Context, item *UnstructuredResource) error {
	item.SetMetadataProvider(&s.md)

	if err := s.client.Create(ctx, item); err != nil {
		return fmt.Errorf("cannot create resource: %w", err)
	}

	return nil
}

func (s *unstructuredClient) Get(ctx context.Context, objectKey resource.ObjectKey) (*UnstructuredResource, bool, error) {
	object, exists, err := s.client.Get(ctx, objectKey)
	if err != nil {
		return object, false, err
	}
	if !exists {
		return nil, false, nil
	}

	object.SetMetadataProvider(&s.md)
	return object, true, nil
}

func (s *unstructuredClient) List(
	ctx context.Context,
	listOpts resource.ListOpts,
) (map[resource.ObjectKey]*UnstructuredResource, error) {
	items, err := s.client.List(ctx, listOpts)
	if err != nil {
		return nil, err
	}

	for k := range items {
		item := items[k]
		item.SetMetadataProvider(&s.md)
		items[k] = item
	}

	return items, nil
}

func (s *unstructuredClient) Update(ctx context.Context, item *UnstructuredResource) error {
	item.SetMetadataProvider(&s.md)

	return s.client.Update(ctx, item)
}

func (s *unstructuredClient) UpdateStatus(ctx context.Context, item *UnstructuredResource) error {
	item.SetMetadataProvider(&s.md)

	return s.client.UpdateStatus(ctx, item)
}

func (s *unstructuredClient) GetMetadataProvider() MetadataProvider {
	return s.md
}
