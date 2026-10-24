package generic

import "slices"

func Difference[T comparable](existing, incoming []T) []T {
	result := make([]T, 0)

	for _, v := range incoming {
		found := slices.Contains(existing, v)

		if !found {
			result = append(result, v)
		}
	}

	return result
}
