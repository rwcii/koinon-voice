//go:build unix

package koinon

import (
	"os/exec"
	"syscall"
)

// setGroup starts the process in its own process group, so killGroup also ends anything it
// started, such as ssh.
func setGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	// The group's ID is the leader's PID. An error means the group is already gone.
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
