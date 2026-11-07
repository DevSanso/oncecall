package msg

func init() {
	messageMap[COMMON_TAGET] = map[MessageCode]MessageDesc{
		PROC_STARTUP: {Level: N, Name: "PROC_STARTUP", Text: "start up process"},

		PROC_CORE: {Level: C, Name: "PROC_CORE", Text: "process is sigfault"},

		PROC_WARNING: {Level: W, Name: "PROC_WARNING", Text: "warning message"},

		PROC_ERROR: {Level: E, Name: "PROC_ERROR", Text: "error message"},
	}
}
