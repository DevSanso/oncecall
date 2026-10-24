package master

import (
	"fmt"
	"maps"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"os"
	"os/exec"
	"slices"
	"sync"
)

type managedProcess struct {
	Cmd *exec.Cmd
	PID int
	Err error
}

type ProcManager interface {
	Start(syncIdent string) (int, error)
	Alive(syncIdent string) bool
	Kill(syncIdent string) error
	List() []string
}
type implProcManager struct {
	mu          sync.RWMutex
	rootFlagStr []string
	rootPid     int
	procs       map[string]managedProcess
}

func NewProcManager() ProcManager {

	return &implProcManager{
		procs:       make(map[string]managedProcess, 10),
		rootFlagStr: append(make([]string, 0, 10), os.Args...),
		rootPid:     os.Getpid(),
		mu:          sync.RWMutex{},
	}
}

func (pm *implProcManager) List() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return slices.Collect(maps.Keys(pm.procs))
}

func (pm *implProcManager) Start(syncIdent string) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, err
	}

	forkParam := append(
		[]string{fmt.Sprintf("--syncIdent=%s", syncIdent)},
		pm.rootFlagStr...,
	)

	cmd := exec.Command(exe, forkParam...)

	if err := cmd.Start(); err != nil {
		return 0, err
	}

	p := managedProcess{
		Cmd: cmd,
		PID: cmd.Process.Pid,
	}

	pm.mu.Lock()
	if pm.procs == nil {
		pm.procs = make(map[string]managedProcess)
	}
	pm.procs[syncIdent] = p
	pm.mu.Unlock()

	return p.PID, nil
}

func (pm *implProcManager) Alive(syncIdent string) bool {
	pm.mu.RLock()
	p, ok := pm.procs[syncIdent]
	rootPid := pm.rootPid
	pm.mu.RUnlock()

	if !ok {
		return false
	}

	exists, err := pm.existsProcPid(p.PID)
	if err != nil || !exists {
		return false
	}

	ppid, err := pm.readPPID(p.PID)
	if err != nil {
		return false
	}

	return ppid == rootPid
}

func (pm *implProcManager) Kill(syncIdent string) error {
	pm.mu.RLock()
	p, ok := pm.procs[syncIdent]
	rootPid := pm.rootPid
	pm.mu.RUnlock()

	if !ok {
		return errlist.ErrG.NewError(
			prefix.NotExistsError,
			"process not found: %s",
			syncIdent,
		)
	}

	exists, err := pm.existsProcPid(p.PID)
	if err != nil {
		return errlist.ErrG.NewError(err, "check process failed")
	}
	if !exists {
		return errlist.ErrG.NewError(
			prefix.NotExistsError,
			"process not found: %s",
			syncIdent,
		)
	}

	ppid, err := pm.readPPID(p.PID)
	if err != nil {
		return errlist.ErrG.NewError(err, "read process PPID failed")
	}

	if ppid != rootPid {
		return errlist.ErrG.NewError(
			prefix.NotExistsError,
			"process parent mismatch: %s",
			syncIdent,
		)
	}

	if err := p.Cmd.Process.Kill(); err != nil {
		return errlist.ErrG.NewError(err, "kill process failed")
	}

	pm.mu.Lock()
	delete(pm.procs, syncIdent)
	pm.mu.Unlock()

	return nil
}
