package assertions

import "reflect"

func TypeOf[T any]() reflect.Type {
	var t T
	return reflect.TypeOf(t)
}

func As[T any](raw interface{}) (T, bool) {
	v, ok := raw.(T)
	return v, ok
}
