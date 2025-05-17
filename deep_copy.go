package controlloop

import "reflect"

// DeepCopyStruct DeepCopyStructPtr принимает указатель на структуру (вход через interface{}),
// проверяет тип и возвращает указатель на глубокую копию этой структуры.
func DeepCopyStruct(src interface{}) interface{} {
	srcVal := reflect.ValueOf(src)
	if srcVal.Kind() != reflect.Ptr || srcVal.Elem().Kind() != reflect.Struct {
		panic("DeepCopyStructPtr: input must be a pointer to a struct")
	}
	// Вызов deepCopyValue для обработки указателя (case assertions.Ptr вернёт новый указатель)
	return deepCopyValue(srcVal).Interface()
}

// deepCopyValue рекурсивно копирует значение с учётом его вида (kind).
func deepCopyValue(val reflect.Value) reflect.Value {
	switch val.Kind() {
	case reflect.Ptr:
		if val.IsNil() {
			return reflect.Zero(val.Type())
		}
		// Создаём новый указатель того же типа, копируя значение, на которое он указывает.
		copyPtr := reflect.New(val.Elem().Type())
		copyPtr.Elem().Set(deepCopyValue(val.Elem()))
		return copyPtr

	case reflect.Interface:
		if val.IsNil() {
			return reflect.Zero(val.Type())
		}
		return deepCopyValue(val.Elem())

	case reflect.Struct:
		cp := reflect.New(val.Type()).Elem()
		for i := 0; i < val.NumField(); i++ {
			field := val.Field(i)
			// Если поле может быть установлено, делаем глубокую копию, иначе просто копируем.
			if cp.Field(i).CanSet() {
				cp.Field(i).Set(deepCopyValue(field))
			} else {
				cp.Field(i).Set(field)
			}
		}
		return cp

	case reflect.Slice:
		if val.IsNil() {
			return reflect.Zero(val.Type())
		}
		cp := reflect.MakeSlice(val.Type(), val.Len(), val.Cap())
		for i := 0; i < val.Len(); i++ {
			cp.Index(i).Set(deepCopyValue(val.Index(i)))
		}
		return cp

	case reflect.Array:
		cp := reflect.New(val.Type()).Elem()
		for i := 0; i < val.Len(); i++ {
			cp.Index(i).Set(deepCopyValue(val.Index(i)))
		}
		return cp

	case reflect.Map:
		if val.IsNil() {
			return reflect.Zero(val.Type())
		}
		cp := reflect.MakeMapWithSize(val.Type(), val.Len())
		for _, key := range val.MapKeys() {
			cp.SetMapIndex(deepCopyValue(key), deepCopyValue(val.MapIndex(key)))
		}
		return cp

	// Для примитивных типов (чисел, строк, bool и т.д.) достаточно вернуть значение, так как оно уже копируется по значению.
	default:
		return val
	}
}
