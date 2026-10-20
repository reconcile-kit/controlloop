package selectors

import (
	"github.com/reconcile-kit/api/resource"
)

type Selector[T resource.Object[T]] interface {
	GetListOpts() resource.ListOpts
	FilterResources(resources []T) []T

	WithFilters(filters ...Filter[T]) Selector[T]
}

type Filter[T resource.Object[T]] func(obj T) bool

type CreatorOpts[T resource.Object[T]] func(s Selector[T]) Selector[T]

func WithFiltersSelectorOpts[T resource.Object[T]](filters ...Filter[T]) CreatorOpts[T] {
	return func(s Selector[T]) Selector[T] {
		if len(filters) == 0 {
			return s
		}

		return s.WithFilters(filters...)
	}
}

func filterResources[T resource.Object[T]](
	resources []T,
	filters []Filter[T],
) []T {
	result := make([]T, 0, len(resources))

	for _, res := range resources {
		if !matchesFilters(res, filters) {
			continue
		}

		result = append(result, res)
	}

	return result
}

func matchesFilters[T resource.Object[T]](obj T, filters []Filter[T]) bool {
	for _, filter := range filters {
		if !filter(obj) {
			return false
		}
	}

	return true
}
