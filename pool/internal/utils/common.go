package utils

type CommonUtils struct{}

func (CommonUtils) SplitRespectQuotesStr(s string) []string {
	var result []string
	var current []rune

	inDouble := false
	inSingle := false

	for _, r := range s {
		switch r {
		case '"':
			if !inSingle {
				inDouble = !inDouble
				continue
			}
		case '\'':
			if !inDouble {
				inSingle = !inSingle
				continue
			}
		case ' ':
			if !inDouble && !inSingle {
				if len(current) > 0 {
					result = append(result, string(current))
					current = nil
				}
				continue
			}
		}
		current = append(current, r)
	}

	if len(current) > 0 {
		result = append(result, string(current))
	}

	return result
}
func (CommonUtils) SplitRespectQuotesAny(s string) []any {
	var result []any
	var current []rune

	inDouble := false
	inSingle := false

	for _, r := range s {
		switch r {
		case '"':
			if !inSingle {
				inDouble = !inDouble
				continue
			}
		case '\'':
			if !inDouble {
				inSingle = !inSingle
				continue
			}
		case ' ':
			if !inDouble && !inSingle {
				if len(current) > 0 {
					result = append(result, string(current))
					current = nil
				}
				continue
			}
		}
		current = append(current, r)
	}

	if len(current) > 0 {
		result = append(result, string(current))
	}

	return result
}
