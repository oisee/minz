//go:build darwin && !mzx_headless

package main

import "syscall"

var savedStderr int = -1

// suppressStderr redirects fd 2 to /dev/null to suppress
// CAMetalLayer allocation warnings during Ebitengine startup on macOS.
func suppressStderr() {
	var err error
	savedStderr, err = syscall.Dup(2)
	if err != nil {
		savedStderr = -1
		return
	}
	devNull, err := syscall.Open("/dev/null", syscall.O_WRONLY, 0)
	if err != nil {
		syscall.Close(savedStderr)
		savedStderr = -1
		return
	}
	syscall.Dup2(devNull, 2)
	syscall.Close(devNull)
}

// restoreStderr brings back the original stderr.
func restoreStderr() {
	if savedStderr >= 0 {
		syscall.Dup2(savedStderr, 2)
		syscall.Close(savedStderr)
		savedStderr = -1
	}
}
