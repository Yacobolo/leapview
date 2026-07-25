package signals

const RouteAdmin RouteKind = "admin"

func Pointer[T any](value T) *T {
	return &value
}

func Optional[T comparable](value T) *T {
	var zero T
	if value == zero {
		return nil
	}
	return &value
}

func OptionalSlice[T any](value []T) *[]T {
	if len(value) == 0 {
		return nil
	}
	copyValue := append([]T(nil), value...)
	return &copyValue
}

func ValueOrZero[T any](value *T) T {
	if value == nil {
		var zero T
		return zero
	}
	return *value
}
