package mir2_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

// TestE2E_Div8 verifies 8-bit unsigned division via shift-and-subtract.
func TestE2E_Div8(t *testing.T) {
	src := `
fun div8(a: u8, b: u8) -> u8 { return a / b }
`
	asm := compileNanz(t, src)
	t.Logf("div8 asm:\n%s", asm)

	cases := [][3]int{
		{10, 3, 3}, {255, 5, 51}, {7, 7, 1}, {0, 5, 0},
		{100, 10, 10}, {200, 7, 28}, {1, 1, 1}, {255, 1, 255},
	}
	for _, tc := range cases {
		a, b, want := tc[0], tc[1], tc[2]
		got := runDiv8(t, asm, a, b)
		if got != want {
			t.Errorf("div8(%d, %d) = %d, want %d", a, b, got, want)
		} else {
			t.Logf("div8(%d, %d) = %d ✓", a, b, got)
		}
	}
}

// TestE2E_Div16 verifies 16-bit unsigned division via shift-and-subtract.
func TestE2E_Div16(t *testing.T) {
	src := `
fun div16(a: u16, b: u16) -> u16 { return a / b }
`
	asm := compileNanz(t, src)
	t.Logf("div16 asm:\n%s", asm)

	cases := [][3]int{
		{1000, 10, 100}, {255, 3, 85}, {0, 5, 0},
		{65535, 256, 255}, {50000, 100, 500}, {1, 1, 1},
		{7, 2, 3}, {32768, 2, 16384},
	}
	for _, tc := range cases {
		a, b, want := tc[0], tc[1], tc[2]
		got := runDiv16(t, asm, a, b)
		if got != want {
			t.Errorf("div16(%d, %d) = %d, want %d", a, b, got, want)
		} else {
			t.Logf("div16(%d, %d) = %d ✓", a, b, got)
		}
	}
}

// TestE2E_Mod8 verifies 8-bit modulo (remainder from shift-and-subtract).
func TestE2E_Mod8(t *testing.T) {
	src := `
fun mod8(a: u8, b: u8) -> u8 { return a % b }
`
	asm := compileNanz(t, src)
	t.Logf("mod8 asm:\n%s", asm)

	cases := [][3]int{
		{10, 3, 1}, {7, 2, 1}, {255, 5, 0}, {0, 5, 0},
		{100, 10, 0}, {200, 7, 4}, {1, 1, 0}, {13, 5, 3},
	}
	for _, tc := range cases {
		a, b, want := tc[0], tc[1], tc[2]
		got := runMod8(t, asm, a, b)
		if got != want {
			t.Errorf("mod8(%d, %d) = %d, want %d", a, b, got, want)
		} else {
			t.Logf("mod8(%d, %d) = %d ✓", a, b, got)
		}
	}
}

// TestE2E_DivMod_Combined verifies div and mod together in one program.
func TestE2E_DivMod_Combined(t *testing.T) {
	// Verify: for all test cases, (a/b)*b + (a%b) == a
	m := compileNanzToMIR2(t, `
fun div8(a: u8, b: u8) -> u8 { return a / b }
fun mod8(a: u8, b: u8) -> u8 { return a % b }
`)

	vm := mir2.NewVM(m)
	vm.MaxSteps = 100_000

	cases := [][2]int{{10, 3}, {255, 5}, {0, 7}, {200, 13}, {1, 1}, {128, 100}}
	for _, tc := range cases {
		a, b := tc[0], tc[1]
		divResult, err := vm.Call("div8", []mir2.Value{{I: int64(a)}, {I: int64(b)}})
		if err != nil {
			t.Fatalf("div8(%d, %d) VM error: %v", a, b, err)
		}
		modResult, err := vm.Call("mod8", []mir2.Value{{I: int64(a)}, {I: int64(b)}})
		if err != nil {
			t.Fatalf("mod8(%d, %d) VM error: %v", a, b, err)
		}
		q := int(divResult[0].I)
		r := int(modResult[0].I)
		reconstructed := q*b + r
		if reconstructed != a {
			t.Errorf("div8(%d,%d)=%d, mod8(%d,%d)=%d → %d*%d+%d=%d ≠ %d",
				a, b, q, a, b, r, q, b, r, reconstructed, a)
		} else {
			t.Logf("%d / %d = %d rem %d  (check: %d*%d+%d = %d) ✓", a, b, q, r, q, b, r, reconstructed)
		}
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func compileNanz(t *testing.T, src string) string {
	t.Helper()
	hm, err := nanz.Parse(src, "test.nanz")
	if err != nil {
		t.Fatalf("nanz parse: %v", err)
	}
	steps, err := pipeline.CompileHIRSteps(hm, pipeline.DefaultOptions())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return steps.Assembly
}

// compileNanzToMIR2 compiles nanz source and returns the optimized MIR2 module.
func compileNanzToMIR2(t *testing.T, src string) *mir2.Module {
	t.Helper()
	hm, err := nanz.Parse(src, "test.nanz")
	if err != nil {
		t.Fatalf("nanz parse: %v", err)
	}
	m := hir.LowerModule(hm)
	for _, f := range m.Funcs {
		mir2.EliminateDeadBlocks(f)
		mir2.ReorderBlocks(f)
		for {
			p := mir2.PropagateConstants(f)
			c := mir2.FoldConstants(f)
			s := mir2.SimplifyIdentities(f)
			e := mir2.ConstantCallElim(m, f)
			if !p && !c && !s && !e {
				break
			}
		}
		mir2.DeadStoreElim(f)
	}
	return m
}

// parseU8ABI extracts param register names from the assembly ABI annotation.
// E.g. "; fun div8(a: u8 = D, b: u8 = C) -> u8 = A" → ["D", "C"]
func parseU8ABI(t *testing.T, funcAsm, funcName string) (param1, param2 string) {
	t.Helper()
	for _, line := range strings.Split(funcAsm, "\n") {
		if !strings.Contains(line, "fun "+funcName+"(") {
			continue
		}
		// Extract "X = REG" patterns
		parts := strings.Split(line, "= ")
		if len(parts) >= 3 {
			// parts[1] = "D, b: u8 ", parts[2] = "C) -> ..."
			p1 := strings.TrimRight(strings.TrimSpace(parts[1]), ", b:u8")
			p1 = strings.Split(strings.TrimSpace(parts[1]), ",")[0]
			p2 := strings.Split(strings.TrimSpace(parts[2]), ")")[0]
			return strings.TrimSpace(p1), strings.TrimSpace(p2)
		}
	}
	t.Fatalf("could not parse ABI for %s", funcName)
	return "", ""
}

func runMod8(t *testing.T, funcAsm string, a, b int) int {
	t.Helper()
	p1, p2 := parseU8ABI(t, funcAsm, "mod8")
	const loadAddr = 0x8000
	bootstrap := fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD %s, %d
    LD %s, %d
    CALL mod8
    DI
    HALT
`, loadAddr, p1, a, p2, b)
	return runAndGetA(t, bootstrap+"\n"+funcAsm, loadAddr)
}

func runDiv8(t *testing.T, funcAsm string, a, b int) int {
	t.Helper()
	p1, p2 := parseU8ABI(t, funcAsm, "div8")
	const loadAddr = 0x8000
	bootstrap := fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD %s, %d
    LD %s, %d
    CALL div8
    DI
    HALT
`, loadAddr, p1, a, p2, b)
	return runAndGetA(t, bootstrap+"\n"+funcAsm, loadAddr)
}

func runDiv16(t *testing.T, funcAsm string, a, b int) int {
	t.Helper()
	const loadAddr = 0x8000
	bootstrap := fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD HL, %d
    LD DE, %d
    CALL div16
    DI
    HALT
`, loadAddr, a, b)
	return runAndGetHL(t, bootstrap+"\n"+funcAsm, loadAddr)
}

func runAndGetA(t *testing.T, src string, loadAddr int) int {
	t.Helper()
	asm := z80asm.NewAssembler()
	res, err := asm.AssembleString(src)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if len(res.Errors) > 0 {
		var sb strings.Builder
		for _, e := range res.Errors {
			sb.WriteString(e.Error())
			sb.WriteByte('\n')
		}
		t.Fatalf("assemble errors:\n%s\nSource:\n%s", sb.String(), src)
	}
	z80 := emulator.NewRemogattoZ80()
	z80.LoadMemory(uint16(loadAddr), res.Binary)
	z80.SetPC(uint16(loadAddr))
	z80.Run()
	return int(z80.GetRegisters().A)
}

func runAndGetHL(t *testing.T, src string, loadAddr int) int {
	t.Helper()
	asm := z80asm.NewAssembler()
	res, err := asm.AssembleString(src)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if len(res.Errors) > 0 {
		var sb strings.Builder
		for _, e := range res.Errors {
			sb.WriteString(e.Error())
			sb.WriteByte('\n')
		}
		t.Fatalf("assemble errors:\n%s\nSource:\n%s", sb.String(), src)
	}
	z80 := emulator.NewRemogattoZ80()
	z80.LoadMemory(uint16(loadAddr), res.Binary)
	z80.SetPC(uint16(loadAddr))
	z80.Run()
	return int(z80.GetRegisters().HL)
}
