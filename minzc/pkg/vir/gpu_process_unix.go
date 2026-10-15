//go:build unix

package vir

import (
	"os/exec"
	"syscall"
)

func configureGPUServerProcess(cmd *exec.Cmd) {
	// Keep the persistent CUDA worker outside the parent test signal group.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
