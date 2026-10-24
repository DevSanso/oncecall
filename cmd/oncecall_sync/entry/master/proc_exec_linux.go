package master

import (
	"fmt"
	"os"
	"strings"

	"oncecall/errlist"
	"oncecall/errlist/prefix"
)

func (pm *implProcManager) readPPID(pid int) (int, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return -1, errlist.ErrG.NewError(
			err,
			"read proc status failed",
		)
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PPid:") {
			var ppid int
			if _, err := fmt.Sscanf(line, "PPid:\t%d", &ppid); err != nil {
				return -1, errlist.ErrG.NewError(
					err,
					"parse PPid failed",
				)
			}
			return ppid, nil
		}
	}

	return 0, errlist.ErrG.NewError(
		prefix.NotExistsError,
		"PPid not found for PID %d",
		pid,
	)
}

func (pm *implProcManager) existsProcPid(pid int) (bool, error) {
	_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))

	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, errlist.ErrG.NewError(
			err,
			"exists proc failed",
		)
	}

	return true, nil
}
