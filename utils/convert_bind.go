package utils

import "oncecall/errlist"

func ConvertMapDataBind(m map[string]any, bindKey []string) ([]any, error) {
	if bindKey == nil && len(bindKey) <= 0 {
		return nil, nil
	}

	buffer := make([]any, len(bindKey))
	for idx, k := range bindKey {
		if val, exists := m[k]; !exists {
			return nil, errlist.ErrG.NewError(nil, "not support key %s", bindKey)
		} else {
			buffer[idx] = val
		}
	}

	return buffer, nil
}
