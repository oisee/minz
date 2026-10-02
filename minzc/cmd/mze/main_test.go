package main

import (
	"testing"

	"github.com/minz/minzc/pkg/emulator"
)

func TestTimeoutExecutionLimit(t *testing.T) {
	for _, tc := range []struct {
		timeout uint
		want    int
	}{{0, -1}, {1234, 1234}} {
		cpu := emulator.NewRemogattoZ80()
		setExecutionLimit(cpu, tc.timeout)
		if cpu.MaxCycles != tc.want {
			t.Errorf("timeout %d: MaxCycles = %d, want %d", tc.timeout, cpu.MaxCycles, tc.want)
		}
	}
}
