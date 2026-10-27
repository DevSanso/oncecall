//go:build linux

package syncd

import (
	"os"
	"strconv"
	"strings"
)

func (p *ProcSync) IsParentRunning() bool {
	ppid := os.Getppid()
	if ppid <= 0 {
		return false
	}

	data, err := os.ReadFile(
		"/proc/" + strconv.Itoa(ppid) + "/stat",
	)
	if err != nil {
		return false
	}

	stat := string(data)
	idx := strings.LastIndex(stat, ")")
	if idx < 0 || idx+2 >= len(stat) {
		return false
	}

	state := stat[idx+2]
	return state != 'Z' && state != 'X'
}
