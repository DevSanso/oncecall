package msg

type MessageCode int

type MessageLevel string

var (
	C MessageLevel = "Critical"
	N MessageLevel = "Normal"
	W MessageLevel = "Warn"
	E MessageLevel = "Error"
)
