package msg

import "fmt"

var pkgMessages = map[MessageCode]struct {
	Level MessageLevel
	Name  string
	Text  string
}{
	PROC_STARTUP: {Level: N, Name: "PROC_STARTUP", Text: "start up process"},
	PROC_CORE:    {Level: C, Name: "PROC_CORE", Text: "process is sigfault"},
	PROC_WARNING: {Level: W, Name: "PROC_WARNING", Text: "warning message"},
	PROC_ERROR:   {Level: E, Name: "PROC_ERROR", Text: "error message"},
}

func PkgMessage(code MessageCode) (MessageLevel, string, string) {
	message, ok := pkgMessages[code]

	if !ok {
		panic("undefined message code: " + fmt.Sprint(code))
	}

	return message.Level, message.Name, message.Text
}
