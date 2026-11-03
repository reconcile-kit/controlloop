package helpers

import (
	"context"
	"fmt"
	"maps"

	"github.com/reconcile-kit/api/resource"
)

const pageSize = 500

// ListAll collects all resources matching opts, starting at the first page.
// It overrides Limit and Offset and returns no partial result on error.
func ListAll[T any](
	ctx context.Context,
	list func(context.Context, resource.ListOpts) (map[resource.ObjectKey]T, error),
	opts resource.ListOpts,
) (map[resource.ObjectKey]T, error) {
	opts.Limit = pageSize
	opts.Offset = 0
	result := make(map[resource.ObjectKey]T)

	for true {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		page, err := list(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("list resources at offset %d: %w", opts.Offset, err)
		}

		maps.Copy(result, page)
		if len(page) < opts.Limit {
			break
		}
		opts.Offset += opts.Limit
	}

	return result, nil
}
