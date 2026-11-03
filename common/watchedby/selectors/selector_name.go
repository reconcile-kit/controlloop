package selectors

import (
	"github.com/reconcile-kit/api/resource"
)

type NameSelector[T resource.Object[T]] struct {
	Name      string
	Namespace string
	Filters   []Filter[T]
}

func NewNameSelector[T resource.Object[T]](
	name, namespace string,
	opts ...CreatorOpts[T],
) Selector[T] {
	var selector Selector[T]
	selector = &NameSelector[T]{
		Name:      name,
		Namespace: namespace,
	}

	for _, opt := range opts {
		selector = opt(selector)
	}

	return selector
}

func (n *NameSelector[T]) GetListOpts() resource.ListOpts {
	opts := resource.ListOpts{
		Namespace: n.Namespace,
		Name:      n.Name,
	}

	return opts
}

func (n *NameSelector[T]) FilterResources(resources []T) []T {
	return filterResources(resources, n.Filters)
}

func (n *NameSelector[T]) WithFilters(filters ...Filter[T]) Selector[T] {
	n.Filters = append(n.Filters, filters...)
	return n
}
