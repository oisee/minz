package nanz_test

// E2E emulator tests for ptr[i] in while loop — BUG-003 regression test.
// Verifies that pointer indexing inside a while loop produces correct results.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

const ptrWhileLoadAddr = 0x8000

// TestPtrWhile_E2E_SumArray verifies ptr[i] inside a while loop (BUG-003).
// Compiles sum_array(ptr: ^u8, n: u8) -> u8 and runs it via Z80 emulator.
func TestPtrWhile_E2E_SumArray(t *testing.T) {
	src := `
fun sum_array(ptr: ^u8, n: u8) -> u8 {
    let acc: u8 = 0
    let i: u8 = 0
    while i < n {
        acc = acc + ptr[i]
        i = i + 1
    }
    return acc
}
`
	m, err := nanz.Parse(src, "e2e_ptr_while")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	genAsm, err := pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	t.Logf("generated Z80:\n%s", genAsm)

	// The function signature is: sum_array(ptr: ^u8 = HL, n: u8 = C) -> u8 = A.
	// Boot: initialise an array at 0x9000, load HL=0x9000, C=n, CALL sum_array, HALT.
	// Array values: [10, 20, 30, 40, 50, 100, 1, 2] so sums are easy to compute.
	runWith := func(n int) (uint8, error) {
		boot := fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD HL, 0x9000
    LD (HL), 10
    INC HL
    LD (HL), 20
    INC HL
    LD (HL), 30
    INC HL
    LD (HL), 40
    INC HL
    LD (HL), 50
    INC HL
    LD (HL), 100
    INC HL
    LD (HL), 1
    INC HL
    LD (HL), 2
    LD HL, 0x9000
    LD C, %d
    CALL sum_array
    DI
    HALT
`, ptrWhileLoadAddr, n)
		fullSrc := boot + "\n" + genAsm
		as := z80asm.NewAssembler()
		res, err := as.AssembleString(fullSrc)
		if err != nil {
			return 0, fmt.Errorf("assemble: %w", err)
		}
		if len(res.Errors) > 0 {
			var sb strings.Builder
			for _, e := range res.Errors {
				sb.WriteString(e.Error())
				sb.WriteByte('\n')
			}
			return 0, fmt.Errorf("assemble errors:\n%s", sb.String())
		}
		z80 := emulator.NewRemogattoZ80()
		if lerr := z80.LoadMemory(ptrWhileLoadAddr, res.Binary); lerr != nil {
			return 0, fmt.Errorf("load: %w", lerr)
		}
		z80.SetPC(ptrWhileLoadAddr)
		if rerr := z80.Run(); rerr != nil {
			return 0, fmt.Errorf("run: %w", rerr)
		}
		regs := z80.GetRegisters()
		return regs.A, nil
	}

	// Array = [10, 20, 30, 40, 50, 100, 1, 2]
	cases := []struct {
		n    int
		want int
	}{
		{0, 0},           // empty sum
		{1, 10},          // [10]
		{2, 30},          // 10+20
		{4, 100},         // 10+20+30+40
		{5, 150},         // 10+20+30+40+50
	}
	for _, tc := range cases {
		got, err := runWith(tc.n)
		if err != nil {
			t.Errorf("sum_array(ptr, %d): %v", tc.n, err)
			continue
		}
		if int(got) != tc.want {
			t.Errorf("sum_array(ptr, %d) = %d, want %d", tc.n, got, tc.want)
		} else {
			t.Logf("sum_array(ptr, %d) = %d ✓", tc.n, got)
		}
	}
}
