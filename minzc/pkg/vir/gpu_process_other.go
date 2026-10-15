//go:build !unix && !windows

package vir

import "os/exec"

func configureGPUServerProcess(cmd *exec.Cmd) {}
