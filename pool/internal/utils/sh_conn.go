package utils

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/pool/types"
	"slices"
	"strconv"
	"strings"
)

type ShUtils struct{}

func (ShUtils) splitLine(line string, sep string) []string {
	var result []string
	var field strings.Builder
	inQuote := false

	for i := 0; i < len(line); i++ {
		if line[i] == '"' {
			inQuote = !inQuote
			continue
		}

		if !inQuote && strings.HasPrefix(line[i:], sep) {
			result = append(result, field.String())
			field.Reset()
			i += len(sep) - 1
			continue
		}

		field.WriteByte(line[i])
	}

	result = append(result, field.String())

	return result
}

func (ShUtils) splitLineKeepQuote(line string, sep string) []string {
	var result []string
	var field strings.Builder
	inQuote := false

	for i := 0; i < len(line); i++ {
		if line[i] == '"' {
			inQuote = !inQuote
			field.WriteByte(line[i])
			continue
		}

		if !inQuote && strings.HasPrefix(line[i:], sep) {
			result = append(result, field.String())
			field.Reset()
			i += len(sep) - 1
			continue
		}

		field.WriteByte(line[i])
	}

	result = append(result, field.String())

	return result
}

func (ShUtils) countFields(line string, sep string) int {
	count := 1
	inQuote := false

	for i := 0; i < len(line); i++ {
		if line[i] == '"' {
			inQuote = !inQuote
			continue
		}

		if !inQuote && strings.HasPrefix(line[i:], sep) {
			count++
			i += len(sep) - 1
		}
	}

	return count
}
func (s ShUtils) ParseResponseFromReader(reader io.Reader, newlineChar, splitChar, endChar string) (rows [][]any, name []string, err error) {
	scanner := bufio.NewScanner(reader)

	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if idx := bytes.Index(data, []byte(newlineChar)); idx >= 0 {
			return idx + len(newlineChar), data[:idx], nil
		}

		if slices.Compare(data, []byte(endChar)) == 0 {
			return 0, nil, bufio.ErrFinalToken
		}

		if atEOF && len(data) > 0 {
			return len(data), data[:len(data)-1], nil
		}

		return 0, nil, nil
	})
	lines := make([]string, 0, 100)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	var res = make([][]any, len(lines)-1)
	var dataLen = 0

	if splitChar != "" {
		for _, line := range lines {
			if cnt := s.countFields(line, splitChar); dataLen < cnt {
				if dataLen == 0 {
					dataLen = cnt
				} else if dataLen != cnt {
					return nil, nil, errlist.ErrG.NewError(prefix.SentinelCatchError, "not eq, prev[%d] != current[%d]", dataLen, cnt)
				}
			}
		}

		if dataLen <= 0 {
			dataLen = 1
		}

		name = make([]string, dataLen)
		for idx, rowName := range strings.Split(lines[0], splitChar) {
			name[idx] = rowName
		}

		for idx, line := range lines[1:] {
			res[idx] = make([]any, dataLen)
			for dIdx, data := range s.splitLine(line, splitChar) {
				res[idx][dIdx] = data
			}
		}
	} else {
		name = make([]string, 1)
		name[0] = lines[0]

		for idx, line := range lines[1:] {
			res[idx] = make([]any, 1)
			res[idx][0] = line
		}
	}

	rows = res
	return
}

func (s ShUtils) ParseResponse(data, newlineChar, splitChar string) (rows [][]any, name []string) {
	lines := s.splitLineKeepQuote(data, newlineChar)
	var res = make([][]any, len(lines))
	var dataMax = 0

	if splitChar != "" {
		for _, line := range lines {
			if cnt := s.countFields(line, splitChar); dataMax < cnt {
				dataMax = cnt
			}
		}

		if dataMax <= 0 {
			dataMax = 1
		}

		for idx, line := range lines {
			res[idx] = make([]any, dataMax)
			for dIdx, data := range s.splitLine(line, splitChar) {
				res[idx][dIdx] = data
			}
		}
	} else {
		for idx, line := range lines {
			res[idx] = make([]any, 1)
			res[idx][0] = line
		}
	}

	name = make([]string, len(res))
	for idx := range res {
		name[idx] = strconv.Itoa(idx + 1)
	}

	rows = res
	return
}

func (ShUtils) MakeParam(inputNextLineChar string, inputDivisionChar string, endString string, arg *types.Args) []byte {
	var buf bytes.Buffer

	for rowIdx := range arg.Args {
		for dataIdx := range arg.Args[rowIdx] {

			switch arg.Args[rowIdx][dataIdx].(type) {
			case []byte:
				buf.WriteString(hex.EncodeToString(arg.Args[rowIdx][dataIdx].([]byte)))
			case string:
				buf.WriteString(arg.Args[rowIdx][dataIdx].(string))
			case int:
				buf.WriteString(strconv.Itoa(arg.Args[rowIdx][dataIdx].(int)))
			case int64:
				buf.WriteString(strconv.FormatInt(arg.Args[rowIdx][dataIdx].(int64), 10))
			case float32:
				buf.WriteString(strconv.FormatFloat(float64(arg.Args[rowIdx][dataIdx].(float32)), 'f', -1, 64))
			case float64:
				buf.WriteString(strconv.FormatFloat(arg.Args[rowIdx][dataIdx].(float64), 'f', -1, 64))
			case bool:
				buf.WriteString(fmt.Sprintf("%t", arg.Args[rowIdx][dataIdx]))
			default:
			}
			buf.WriteString(inputDivisionChar)
		}
		buf.WriteString(inputNextLineChar)
	}

	buf.WriteString(endString)
	return buf.Bytes()
}
