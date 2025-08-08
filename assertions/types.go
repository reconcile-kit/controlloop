package assertions

import (
	"github.com/reconcile-kit/api/resource"
	"reflect"
)

func TypeOf[T any]() reflect.Type {
	var t T
	return reflect.TypeOf(t)
}

func As[T any](raw interface{}) (T, bool) {
	v, ok := raw.(T)
	return v, ok
}

func createNonNilInstance[T resource.Object[T]](zero T) T {
	if reflect.TypeOf(zero).Kind() == reflect.Ptr && reflect.ValueOf(zero).IsNil() {
		return reflect.New(reflect.TypeOf(zero).Elem()).Interface().(T)
	}
	return zero
}

func GetGroupKindFromType[T resource.Object[T]]() resource.GroupKind {
	var zero T
	instance := createNonNilInstance(zero)
	gk := instance.GetGK()
	return gk
}
