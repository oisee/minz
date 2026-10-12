package main

import (
	"syscall"
	"time"
)

// cpmTickInput is an opt-in emulator game clock, not a CP/M clock service.
// Ordinary reads consume at most one key per deadline; queued keys cannot make
// a game whose gravity advances on each read run faster. VT100 continuations
// belong to that same key and therefore do not consume additional game ticks.
// The caller must arrange raw terminal mode; pipes work without terminal setup.
type cpmTickInput struct {
	fd       int
	interval time.Duration
	next     time.Time
	escape   byte
}

func (in *cpmTickInput) read() byte {
	continuation := in.escape != 0
	if !continuation {
		if in.next.IsZero() {
			in.next = time.Now().Add(in.interval)
		}
		time.Sleep(time.Until(in.next))
		in.next = in.next.Add(in.interval)
		// Slow rendering must not cause a burst of catch-up game ticks.
		if !in.next.After(time.Now()) {
			in.next = time.Now().Add(in.interval)
		}
	}
	wait := time.Duration(0)
	if continuation {
		// Bound an incomplete sequence, including a standalone Escape key.
		wait = 40 * time.Millisecond
	}
	ch := cpmReadByteReady(in.fd, wait)
	if in.escape == 1 && ch == '[' {
		in.escape = 2
	} else if in.escape == 0 && ch == 27 {
		in.escape = 1
	} else {
		in.escape = 0
	}
	return ch
}

// Use select and the raw read syscall. os.File.Read on a Go-pollable fd can
// wait for data despite toggling O_NONBLOCK, which would stop idle game ticks.
func cpmReadByteReady(fd int, wait time.Duration) byte {
	deadline := time.Now().Add(wait)
	for {
		var ready syscall.FdSet
		if fd < 0 || fd >= len(ready.Bits)*64 {
			return 0
		}
		ready.Bits[fd/64] |= 1 << uint(fd%64)
		remaining := time.Until(deadline)
		if remaining < 0 {
			remaining = 0
		}
		tv := syscall.NsecToTimeval(remaining.Nanoseconds())
		n, err := syscall.Select(fd+1, &ready, nil, nil, &tv)
		if err == syscall.EINTR {
			continue
		}
		if err != nil || n == 0 {
			return 0
		}
		var buf [1]byte
		n, err = syscall.Read(fd, buf[:])
		if err == syscall.EINTR {
			continue
		}
		if err != nil || n == 0 { // EOF continues producing paced idle ticks.
			return 0
		}
		return buf[0]
	}
}
