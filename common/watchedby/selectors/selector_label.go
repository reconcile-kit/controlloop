package selectors

import (
	"github.com/reconcile-kit/api/resource"
)

type LabelSelector[T resource.Object[T]] struct {
	LabelSelectors []resource.LabelSelector
	Namespace      string
	Filters        []Filter[T]
}

func NewLabelSelector[T resource.Object[T]](
	labelSelectors []resource.LabelSelector,
	namespace string,
	opts ...CreatorOpts[T],
) Selector[T] {
	var selector Selector[T]
	selector = &LabelSelector[T]{
		LabelSelectors: labelSelectors,
		Namespace:      namespace,
	}

	for _, opt := range opts {
		selector = opt(selector)
	}

	return selector
}

func (l *LabelSelector[T]) GetListOpts() resource.ListOpts {
	opts := resource.ListOpts{
		Namespace:      l.Namespace,
		LabelSelectors: l.LabelSelectors,
	}

	return opts
}

func (l *LabelSelector[T]) FilterResources(resources []T) []T {
	return filterResources(resources, l.Filters)
}

func (l *LabelSelector[T]) WithFilters(filters ...Filter[T]) Selector[T] {
	l.Filters = append(l.Filters, filters...)
	return l
}
