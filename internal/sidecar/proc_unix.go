//go:build unix

package sidecar

import (
	"os/exec"
	"syscall"
)

// setGroup starts the helper in its own process group, so killGroup also ends anything the
// helper started.
func setGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	// The group's ID is the helper's PID. An error means the group is already gone.
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
