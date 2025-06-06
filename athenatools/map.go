package athenatools

func Map[T any, Y any](input []T, x func(T) Y) []Y {
	result := make([]Y, len(input))
	for i, value := range input {
		result[i] = x(value)
	}

	return result
}
