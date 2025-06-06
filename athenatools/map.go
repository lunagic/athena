package athenatools

func Map[T any, Y any](input []T, x func(T) Y) []Y {
	result := []Y{}
	for _, i := range input {
		result = append(result, x(i))
	}

	return result
}
