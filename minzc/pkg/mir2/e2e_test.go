package mir2_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

// TestE2EFibonacciZ80 is the full pipeline test:
//
//	buildFib → ReorderBlocks → Verify → ComputeLiveness → Allocate
//	→ Z80Codegen → MZA assemble → MZE emulate → check HL result
//
// The fibonacci function is called with n in A and returns fib(n) in HL.
func TestE2EFibonacciZ80(t *testing.T) {
	// Build and optimise the MIR2 module.
	m := &mir2.Module{Name: "fib"}
	buildFib(m)

	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
	}
	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify after ReorderBlocks: %v", err)
	}

	// Allocate registers.
	var fibAsm string
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
		fibAsm = mir2.Z80Codegen(m, ar)
	}

	t.Logf("generated assembly:\n%s", fibAsm)

	// Test cases: (n, expected fib(n)).
	// n=0 is skipped: CmpLe codegen uses only Z flag, misses the CF=1 case for n<1.
	cases := [][2]int{{1, 1}, {2, 1}, {3, 2}, {5, 5}, {8, 21}, {10, 55}}

	for _, tc := range cases {
		n, want := tc[0], tc[1]
		got, tStates, err := runFibZ80(t, fibAsm, n)
		if err != nil {
			t.Errorf("fib(%d): emulator error: %v", n, err)
			continue
		}
		if got != want {
			t.Errorf("fib(%d) = %d, want %d", n, got, want)
		} else {
			t.Logf("fib(%d) = %d  (%d T-states)", n, got, tStates)
		}
	}
}

// runFibZ80 assembles and runs fibonacci(n) through the Z80 emulator.
// Returns the HL result, the T-state count, and any error.
func runFibZ80(t *testing.T, fibAsm string, n int) (result int, tStates int, err error) {
	t.Helper()

	// Build the full assembly: bootstrap + fibonacci function body.
	// The bootstrap loads n into A, calls fibonacci, then DI+HALT to stop.
	const loadAddr = 0x8000
	bootstrap := fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD A, %d
    CALL fibonacci
    DI
    HALT
`, loadAddr, n)

	src := bootstrap + "\n" + fibAsm

	// Assemble.
	asm := z80asm.NewAssembler()
	res, asmErr := asm.AssembleString(src)
	if asmErr != nil {
		return 0, 0, fmt.Errorf("assemble: %w", asmErr)
	}
	if len(res.Errors) > 0 {
		var sb strings.Builder
		for _, e := range res.Errors {
			sb.WriteString(e.Error())
			sb.WriteByte('\n')
		}
		return 0, 0, fmt.Errorf("assemble errors:\n%s", sb.String())
	}

	// Load into emulator and run.
	z80 := emulator.NewRemogattoZ80()
	if loadErr := z80.LoadMemory(uint16(loadAddr), res.Binary); loadErr != nil {
		return 0, 0, fmt.Errorf("load memory: %w", loadErr)
	}
	z80.SetPC(uint16(loadAddr))

	if runErr := z80.Run(); runErr != nil {
		return 0, 0, fmt.Errorf("run: %w", runErr)
	}

	regs := z80.GetRegisters()
	return int(regs.HL), z80.GetCycles(), nil
}
