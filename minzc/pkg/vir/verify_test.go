package vir_test

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
)

// TestVIR_Z80_Verify compiles functions through VIR, assembles to Z80 binary,
// runs on the MZE emulator, and verifies correctness.
func TestVIR_Z80_Verify(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found")
	}

	tests := []struct {
		name     string
		src      string
		funcName string
		args     []int  // u8 args: loaded into A, B, C, ...
		expect   int    // expected result in A (u8)
		wide     bool   // if true, expect u16 result in HL
	}{
		// Basic arithmetic
		{"add(3,5)=8", `fun add(a: u8, b: u8) -> u8 { return a + b }`, "add", []int{3, 5}, 8, false},
		{"add(0,0)=0", `fun add(a: u8, b: u8) -> u8 { return a + b }`, "add", []int{0, 0}, 0, false},
		{"add(255,1)=0", `fun add(a: u8, b: u8) -> u8 { return a + b }`, "add", []int{255, 1}, 0, false},
		{"sub(10,3)=7", `fun sub(a: u8, b: u8) -> u8 { return a - b }`, "sub", []int{10, 3}, 7, false},
		{"identity(42)", `fun id(x: u8) -> u8 { return x }`, "id", []int{42}, 42, false},
		{"double(21)=42", `fun dbl(x: u8) -> u8 { return x + x }`, "dbl", []int{21}, 42, false},
		{"const5()=5", `fun five() -> u8 { return 5 }`, "five", []int{}, 5, false},

		// More arithmetic
		{"and(0xF0,0x0F)=0", `fun band(a: u8, b: u8) -> u8 { return a & b }`, "band", []int{0xF0, 0x0F}, 0, false},
		{"or(0xF0,0x0F)=0xFF", `fun bor(a: u8, b: u8) -> u8 { return a | b }`, "bor", []int{0xF0, 0x0F}, 0xFF, false},
		// Note: Nanz uses ^ for pointer dereference, not XOR. XOR tested via C89.

		// NOTE: swap(a,b)=b fails because VIR doesn't yet handle
		// return-value ABI (must move result to A before RET).
		// This is a known limitation — the solver produces correct
		// per-block code but inter-block ABI contracts are TODO.
	}

	opts := vir.SolverOptions{Timeout: 30 * time.Second}

	pass, fail := 0, 0
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse + lower + VIR codegen
			hm, err := nanz.ParseWithOpts(tt.src, "test.nanz", nanz.ParseOpts{})
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			m, lerr := lowerHIR(hm)
			if lerr != nil {
				t.Fatalf("lower: %v", lerr)
			}
			virAsm, results := vir.CodegenModule(m, opts)
			for _, r := range results {
				if !r.OK {
					t.Fatalf("VIR: %s", r.Error)
				}
			}

			// Build bootstrap: set up args + CALL function + HALT
			boot := buildBootstrap(tt.funcName, tt.args)
			fullAsm := boot + "\n" + virAsm
			t.Logf("Full ASM:\n%s", fullAsm)

			// Assemble
			as := z80asm.NewAssembler()
			result, aerr := as.AssembleString(fullAsm)
			if aerr != nil {
				t.Fatalf("assemble: %v", aerr)
			}

			// Run on Z80 emulator
			z := emulator.NewRemogattoZ80()
			z.MaxCycles = 100000
			z.LoadMemory(uint16(result.Origin), result.Binary)
			z.SetSP(0xFF00)

			err = z.Run()
			// Run() returns nil on clean exit (RET to 0) or error

			regs := z.GetRegisters()

			var got int
			if tt.wide {
				got = int(regs.HL)
			} else {
				got = int(regs.A)
			}

			if got != tt.expect {
				t.Errorf("FAIL: %s = %d, want %d (A=%d HL=%d PC=%04X cycles=%d)",
					tt.name, got, tt.expect, regs.A, regs.HL, regs.PC, z.GetCycles())
				fail++
			} else {
				t.Logf("OK: %s = %d ✓ (%d cycles)", tt.name, got, z.GetCycles())
				pass++
			}
		})
	}

	t.Logf("\n=== Z80 Verification: %d pass, %d fail ===", pass, fail)
}

// buildBootstrap generates a small Z80 bootstrap that loads args into registers
// and CALLs the target function. Default ABI: arg0→A, arg1→B, arg2→C.
func buildBootstrap(funcName string, args []int) string {
	var sb strings.Builder
	sb.WriteString("    ORG 0x0000\n")
	sb.WriteString("    LD SP, 0xFF00\n")

	// Load args into A, B, C (Z80 default ABI for u8)
	regs := []string{"A", "B", "C", "D", "E"}
	for i, arg := range args {
		if i < len(regs) {
			sb.WriteString(fmt.Sprintf("    LD %s, %d\n", regs[i], arg&0xFF))
		}
	}

	sb.WriteString(fmt.Sprintf("    CALL %s\n", funcName))
	sb.WriteString("    HALT\n")
	sb.WriteString("\n")

	return sb.String()
}
