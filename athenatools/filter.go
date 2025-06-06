package athenatools

func Filter[T any](input []T, x func(T) bool) []T {
	result := make([]T, len(input))
	resultIndex := 0
	for _, value := range input {
		if x(value) {
			result[resultIndex] = value
			resultIndex++
		}
	}

	return result[0:resultIndex]
}
