//go:build windows

package syncd

import (
	"os"

	"golang.org/x/sys/windows"
)

func (p *ProcSync) IsParentRunning() bool {
	ppid := os.Getppid()
	if ppid <= 0 {
		return false
	}

	handle, err := windows.OpenProcess(
		windows.PROCESS_QUERY_LIMITED_INFORMATION,
		false,
		uint32(ppid),
	)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}

	return exitCode == 259 // STILL_ACTIVE
}
