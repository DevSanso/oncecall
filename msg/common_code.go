package msg

const COMMON_TAGET MessageTarget = 0
const (
	PROC_STARTUP MessageCode = 1
	PROC_CORE    MessageCode = 99999
	PROC_WARNING MessageCode = 2
	PROC_ERROR   MessageCode = 3
)
