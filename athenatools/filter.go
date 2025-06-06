package athenatools

func Filter[T any](input []T, x func(T) bool) []T {
	result := []T{}
	for _, i := range input {
		if x(i) {
			result = append(result, i)
		}
	}

	return result
}
