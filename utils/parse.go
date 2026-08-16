package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseMemorySize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))

	if len(s) == 0 {
		return 0, fmt.Errorf("empty size")
	}

	multipliers := map[byte]int64{
		'K': 1024,
		'M': 1024 * 1024,
		'G': 1024 * 1024 * 1024,
		'T': 1024 * 1024 * 1024 * 1024,
	}

	last := s[len(s)-1]

	if last >= '0' && last <= '9' {
		return strconv.ParseInt(s, 10, 64)
	}

	multiplier, ok := multipliers[last]
	if !ok {
		return 0, fmt.Errorf("unknown unit: %c", last)
	}

	value, err := strconv.ParseFloat(s[:len(s)-1], 64)
	if err != nil {
		return 0, err
	}

	return int64(value * float64(multiplier)), nil
}
