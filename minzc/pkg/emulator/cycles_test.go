package emulator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestRemogattoNOPCycles(t *testing.T) {
	for _, n := range []int{10, 100} {
		for _, step := range []bool{false, true} {
			t.Run(fmt.Sprintf("n=%d/step=%t", n, step), func(t *testing.T) {
				z := NewRemogattoZ80()
				program := make([]byte, n+1)
				program[n] = 0x76
				if err := z.LoadMemory(0, program); err != nil {
					t.Fatal(err)
				}
				want := 4*n + 4
				if step {
					sum := 0
					for i := 0; i <= n; i++ {
						sum += z.Step()
					}
					if sum != want {
						t.Errorf("summed Step cycles = %d, want %d", sum, want)
					}
				} else if err := z.Run(); err != nil {
					t.Fatal(err)
				}
				if got := z.GetCycles(); got != want {
					t.Errorf("GetCycles = %d, want %d", got, want)
				}
			})
		}
	}
}

func TestRemogattoInstructionCycles(t *testing.T) {
	tests := []struct {
		name string
		code []byte
		bc   uint16
		want int
	}{
		{"LD A,n", []byte{0x3e, 0x42}, 0, 7},
		{"LD (HL),A", []byte{0x77}, 0, 7},
		{"LD A,(nn)", []byte{0x3a, 0x00, 0x90}, 0, 13},
		{"PUSH BC", []byte{0xc5}, 0, 11},
		{"POP BC", []byte{0xc1}, 0, 10},
		{"CALL nn", []byte{0xcd, 0x00, 0x90}, 0, 17},
		{"RET", []byte{0xc9}, 0, 10},
		{"JP nn", []byte{0xc3, 0x00, 0x90}, 0, 10},
		{"JR e", []byte{0x18, 0x02}, 0, 12},
		{"DJNZ taken", []byte{0x10, 0x02}, 0x0200, 13},
		{"DJNZ not taken", []byte{0x10, 0x02}, 0x0100, 8},
		{"ADD HL,BC", []byte{0x09}, 0, 11},
		{"LD (IX+d),A", []byte{0xdd, 0x77, 0x02}, 0, 19},
		{"OUT (n),A", []byte{0xd3, 0x01}, 0, 11},
		{"IN A,(n)", []byte{0xdb, 0x01}, 0, 11},
		{"EX (SP),HL", []byte{0xe3}, 0, 19},
		// A protected write still consumes a memory bus cycle.
		{"LD (ROM),A", []byte{0x32, 0x00, 0x10}, 0, 13},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			z := NewRemogattoZ80()
			z.SetMemory(0x1000, 0x55)
			z.SetRegisters(Registers{A: 0x42, PC: 0x8000, SP: 0xff00, HL: 0x9000, IX: 0x9000, BC: tt.bc})
			if err := z.LoadMemory(0x8000, tt.code); err != nil {
				t.Fatal(err)
			}
			if got := z.Step(); got != tt.want {
				t.Errorf("Step = %d, want %d", got, tt.want)
			}
			if got := z.GetMemory(0x1000); got != 0x55 {
				t.Errorf("ROM byte = %02x, want 55", got)
			}
			if got := z.GetCycles(); got != tt.want {
				t.Errorf("GetCycles = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRemogattoMaxCycles(t *testing.T) {
	// Isolate Run so a regression cannot leave an infinite loop in the test process.
	if os.Getenv("MINZ_TEST_MAX_CYCLES_CHILD") == "1" {
		z := NewRemogattoZ80()
		z.MaxCycles = 1000
		if err := z.LoadMemory(0, []byte{0x18, 0xfe}); err != nil {
			t.Fatal(err)
		}
		if err := z.Run(); err == nil || !strings.Contains(err.Error(), "execution limit exceeded") {
			t.Fatalf("Run error = %v", err)
		}
		if got := z.GetCycles(); got != 1008 {
			t.Fatalf("GetCycles = %d, want 1008", got)
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRemogattoMaxCycles$")
	cmd.Env = append(os.Environ(), "MINZ_TEST_MAX_CYCLES_CHILD=1")
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatal("Run did not return within 10 seconds")
	}
	if err != nil {
		t.Fatalf("cycle-limit subprocess: %v\n%s", err, output)
	}
}

func TestRemogattoResetCycles(t *testing.T) {
	z := NewRemogattoZ80()
	for _, n := range []int{10, 100} {
		z.Reset()
		if got := z.GetCycles(); got != 0 {
			t.Fatalf("cycles after Reset = %d", got)
		}
		program := make([]byte, n+1)
		program[n] = 0x76
		if err := z.LoadMemory(0, program); err != nil {
			t.Fatal(err)
		}
		if err := z.Run(); err != nil {
			t.Fatal(err)
		}
		if got, want := z.GetCycles(), 4*n+4; got != want {
			t.Errorf("cycles after %d NOPs and HALT = %d, want %d", n, got, want)
		}
	}
}

func TestRemogattoUndefinedOpcodeCycles(t *testing.T) {
	for _, step := range []bool{false, true} {
		t.Run(fmt.Sprintf("step=%t", step), func(t *testing.T) {
			z := NewRemogattoZ80()
			if err := z.LoadMemory(0, []byte{0xed, 0x00, 0x00, 0x76}); err != nil {
				t.Fatal(err)
			}
			if step {
				for i, want := range []int{8, 4, 4} {
					if got := z.Step(); got != want {
						t.Errorf("Step %d = %d, want %d", i, got, want)
					}
				}
			} else if err := z.Run(); err != nil {
				t.Fatal(err)
			}
			if got := z.GetCycles(); got != 16 {
				t.Errorf("GetCycles = %d, want 16", got)
			}
		})
	}
}

func TestRemogattoUnlimitedRun(t *testing.T) {
	z := NewRemogattoZ80()
	z.MaxCycles = -1
	// 20 outer iterations of 65,536 inner iterations: over 17M T-states.
	// DJNZ with B=0 gives 256 iterations; repeat with D=0 for 256 batches.
	program := []byte{0x0e, 20, 0x16, 0, 0x06, 0, 0x10, 0xfe, 0x15, 0x20, 0xf9, 0x0d, 0x20, 0xf4, 0x3e, 42, 0xf3, 0x76}
	if err := z.LoadMemory(0x8000, program); err != nil {
		t.Fatal(err)
	}
	z.SetPC(0x8000)
	if err := z.Run(); err != nil {
		t.Fatal(err)
	}
	if z.GetCycles() <= 17_000_000 || z.GetExitCode() != 42 {
		t.Fatalf("cycles = %d, exit = %d; want >17M and 42", z.GetCycles(), z.GetExitCode())
	}
}

func TestRemogattoRunBudgetPerCall(t *testing.T) {
	z := NewRemogattoZ80()
	z.MaxCycles = 44
	program := make([]byte, 11)
	program[10] = 0x76
	if err := z.LoadMemory(0x8000, program); err != nil {
		t.Fatal(err)
	}
	for run := 1; run <= 2; run++ {
		z.Unhalt()
		z.SetPC(0x8000)
		if err := z.Run(); err != nil {
			t.Fatalf("Run %d: %v", run, err)
		}
		if got := z.GetCycles(); got != 44*run {
			t.Fatalf("Run %d cycles = %d, want %d", run, got, 44*run)
		}
	}
}

func TestRemogattoHostMemoryUntimed(t *testing.T) {
	z := NewRemogattoZ80WithScreen()
	z.Step() // Start with nonzero CPU and wrapper clocks.
	cycles, tstates := z.GetCycles(), z.cpu.Tstates
	prof := NewProfiler()
	z.SetProfiler(prof)
	changes := 0
	z.SetSMCTracker(func(addr uint16, old, val byte) {
		if addr != 0x9000 || old != 0 || val != 42 {
			t.Errorf("unexpected SMC change: %04x %d -> %d", addr, old, val)
		}
		changes++
	})
	z.WriteMemory(0x9000, 42)
	z.WriteMemory(0x9000, 42) // An unchanged write must not trigger SMC.
	if got := z.ReadMemory(0x9000); got != 42 {
		t.Errorf("RAM = %d, want 42", got)
	}
	z.WriteMemory(0x1000, 42)
	if got := z.ReadMemory(0x1000); got != 0 {
		t.Errorf("ROM changed to %d", got)
	}
	if changes != 1 || prof.ReadCount[0x9000] != 1 || prof.WriteCount[0x9000] != 2 || prof.ReadCount[0x1000] != 1 || prof.WriteCount[0x1000] != 0 {
		t.Errorf("host accesses did not preserve SMC/profiler semantics")
	}
	if z.GetCycles() != cycles || z.cpu.Tstates != tstates {
		t.Fatalf("host access advanced clocks: cycles %d -> %d, Tstates %d -> %d", cycles, z.GetCycles(), tstates, z.cpu.Tstates)
	}
}
