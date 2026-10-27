//go:build windows

package master

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"oncecall/errlist"
)

func (pm *implProcManager) readPPID(pid int) (int, error) {
	script := fmt.Sprintf(
		"$p = Get-CimInstance Win32_Process -Filter 'ProcessId = %d' -ErrorAction Stop; "+
			"if ($null -eq $p) { exit 2 }; "+
			"[Console]::Write($p.ParentProcessId)",
		pid,
	)

	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		script,
	)

	output, err := cmd.Output()
	if err != nil {
		return -1, errlist.ErrG.NewError(
			err,
			"read process PPID failed",
		)
	}

	ppid, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return -1, errlist.ErrG.NewError(
			err,
			"parse PPID failed",
		)
	}

	return ppid, nil
}

func (pm *implProcManager) existsProcPid(pid int) (bool, error) {
	script := fmt.Sprintf(
		"$p = Get-CimInstance Win32_Process -Filter 'ProcessId = %d' -ErrorAction Stop; "+
			"if ($null -eq $p) { exit 2 }",
		pid,
	)

	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		script,
	)

	err := cmd.Run()
	if err == nil {
		return true, nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
		return false, nil
	}

	return false, errlist.ErrG.NewError(
		err,
		"exists process failed",
	)
}
