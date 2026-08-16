package conn

import (
	"slices"
	"strings"
)

func isTypeText(t string) bool {
	li := []string{
		"VARCHAR",
		"TEXT",
		"STRING",
		"VARCHAR",
		"BPCHAR",
	}
	uppr := strings.ToUpper(t)

	for _, chkT := range li {
		chk := strings.Contains(uppr, chkT)
		if chk {
			return true
		}
	}
	return false
}

func isTypeBigInt(t string) bool {
	li := []string{
		"INT8",
		"BIGINT",
	}

	return slices.Index(li, strings.ToUpper(t)) != -1
}

func isTypeSInt(t string) bool {
	li := []string{
		"INT",
		"INT2",
		"INT4",
		"INTEGER",
	}

	return slices.Index(li, strings.ToUpper(t)) != -1
}

func isTypeDouble(t string) bool {
	li := []string{
		"DOUBLE",
		"FLOAT",
		"FLOAT4",
		"FLOAT8",
	}

	return slices.Index(li, strings.ToUpper(t)) != -1
}

func isTypeBytes(t string) bool {
		li := []string{
		"BLOB",
	}

	return slices.Index(li, strings.ToUpper(t)) != -1
}
