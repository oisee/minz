//go:build darwin

package main

import "syscall"

func cpmSelectRead(fd int, tv *syscall.Timeval) (int, error) {
	var ready syscall.FdSet
	if fd < 0 || fd >= len(ready.Bits)*32 {
		return 0, nil
	}
	ready.Bits[fd/32] |= 1 << uint(fd%32)
	if err := syscall.Select(fd+1, &ready, nil, nil, tv); err != nil {
		return 0, err
	}
	if ready.Bits[fd/32]&(1<<uint(fd%32)) == 0 {
		return 0, nil
	}
	return 1, nil
}
