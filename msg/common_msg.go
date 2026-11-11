package msg

func init() {
	messageMap[COMMON_TAGET] = map[MessageCode]MessageDesc{
		COMMON_CONTEXT_WRAP_ERROR_EMPTY_CODE: {Code: 8000, Level: C, Name: "COMMON_CONTEXT_WRAP_ERROR_EMPTY_CODE", Text: "never not exist stack data, check process code"},

		COMMON_CONTEXT_WRAP_ERROR_TOP_IS_DUMMY_CODE: {Code: 8001, Level: C, Name: "COMMON_CONTEXT_WRAP_ERROR_TOP_IS_DUMMY_CODE", Text: "never dummy stack top data, check process code"},

		COMMON___BROKEN_GO_SYSTEM_CALL_CODE: {Code: 8002, Level: C, Name: "COMMON___BROKEN_GO_SYSTEM_CALL_CODE", Text: "broken go runtime syscall"},

		COMMON___NIL_POINTER_CODE: {Code: 8003, Level: C, Name: "COMMON___NIL_POINTER_CODE", Text: "nil pointer code"},

		COMMON_DUMMY_C: {Code: 9997, Level: C, Name: "COMMON_DUMMY_C", Text: ""},

		COMMON_DUMMY_W: {Code: 9998, Level: W, Name: "COMMON_DUMMY_W", Text: ""},

		COMMON_DUMMY_E: {Code: 9999, Level: E, Name: "COMMON_DUMMY_E", Text: ""},
	}
}
