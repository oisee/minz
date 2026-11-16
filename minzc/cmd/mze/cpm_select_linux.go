//go:build linux

package main

import "syscall"

func cpmSelectRead(fd int, tv *syscall.Timeval) (int, error) {
	var ready syscall.FdSet
	if fd < 0 || fd >= len(ready.Bits)*64 {
		return 0, nil
	}
	ready.Bits[fd/64] |= 1 << uint(fd%64)
	return syscall.Select(fd+1, &ready, nil, nil, tv)
}
