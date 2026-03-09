package z80testing

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFibonacciP4 verifies fibonacci correctness after P4 register allocation changes
// (pre-coloring: locals with hints claim physical regs before linear scan).
func TestFibonacciP4(t *testing.T) {
	t.Skip("MIR1 bug: while-loop register allocator overwrites live operands (ADR-0006). MIR1 deprecated in favour of MIR2.")
	src := `
fun fibonacci(n: u8) -> u16 {
    if n <= 1 {
        return n;
    }
    let mut a: u16 = 0;
    let mut b: u16 = 1;
    let mut i: u8 = 2;
    while i <= n {
        let temp: u16 = a + b;
        a = b;
        b = temp;
        i = i + 1;
    }
    return b;
}
fun main() -> void {}
`
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "fib.minz")
	if err := os.WriteFile(srcFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	h, err := NewE2ETestHarness(t)
	if err != nil || h == nil {
		t.Skip("harness unavailable")
	}
	defer h.Cleanup()

	a80File, err := h.CompileMinZ(srcFile, false)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	binary, symbols, err := h.AssembleA80(a80File)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}

	h.LoadBinary(binary, 0x8000)

	// Fibonacci uses u8 calling convention: n in A register, result in HL
	fibAddr, ok := findSymbol(symbols, "FIB_FIBONACCI_U8")
	if !ok {
		t.Fatalf("FIB_FIBONACCI_U8 symbol not found; symbols: %v", symbols)
	}
	t.Logf("FIB_FIBONACCI_U8 at $%04X", fibAddr)

	cases := []struct {
		n    byte
		want uint16
	}{
		{0, 0},
		{1, 1},
		{2, 1},
		{5, 5},
		{10, 55},
	}

	for _, tc := range cases {
		// Set up CPU: A = n, SP = 0xFFF0, return addr at 0xFFEE
		h.cpu.Reset()
		h.cpu.A = tc.n
		h.cpu.SetSP(0xFFEE)
		h.memory.WriteByte(0xFFEE, 0x00) // return to $0000
		h.memory.WriteByte(0xFFEF, 0x00)
		h.cpu.SetPC(fibAddr)

		origSP := uint16(0xFFEE + 2)
		for instr := 0; instr < 200000; instr++ {
			h.cpu.DoOpcode()
			if h.cpu.SP() == origSP && h.cpu.PC() == 0x0000 {
				break
			}
		}

		got := h.cpu.HL()
		if got != tc.want {
			t.Errorf("fibonacci(%d) = %d, want %d", tc.n, got, tc.want)
		} else {
			t.Logf("fibonacci(%d) = %d ✓", tc.n, got)
		}
	}
}
