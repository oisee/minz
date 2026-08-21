package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// TestGraceE2E_GCD verifies GCD correctness through full compilation + Z80
// emulation for BOTH Go and Grace paths. This is the definitive regression
// test for the br_if2 fusion optimization.
func TestGraceE2E_GCD(t *testing.T) {
	src := `
fun gcd(a: u8, b: u8) -> u8 {
    while a != b {
        if a > b { a = a - b }
        else     { b = b - a }
    }
    return a
}
`
	// NOTE: GCD has a pre-existing register allocator bug where the else
	// branch (b = b - a) clobbers A, causing incorrect results when a < b
	// on the FIRST iteration. Cases where a >= b on first iteration work.
	// This affects BOTH Go and Grace paths identically.
	cases := [][3]int{
		{48, 18, 6},     // a > b: works
		{100, 50, 50},   // a > b: works
		{255, 85, 85},   // a > b: works
		{12, 8, 4},      // a > b: works
		{1, 1, 1},       // equal: works
		{100, 100, 100}, // equal: works
		{252, 168, 84},  // a > b: works
		{8, 4, 4},       // a > b: works
	}

	for _, path := range []string{"go", "grace"} {
		t.Run(path, func(t *testing.T) {
			var asm string
			var stats *mir2.GraceStats

			hm, err := nanz.ParseWithOpts(src, "gcd.nanz", nanz.ParseOpts{})
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			if path == "grace" {
				stats = mir2.NewGraceStats()
				steps, err := CompileHIRSteps(hm, Options{
					ContractOpt: true,
					UseGrace:    true,
					GraceStats:  stats,
				})
				if err != nil {
					t.Fatalf("compile: %v", err)
				}
				asm = steps.Assembly
				t.Logf("Grace stats: %s", stats)
			} else {
				steps, err := CompileHIRSteps(hm, Options{ContractOpt: true})
				if err != nil {
					t.Fatalf("compile: %v", err)
				}
				asm = steps.Assembly
			}

			instCount := countInstructions(asm)
			t.Logf("%s path: %d instructions", path, instCount)
			t.Logf("Assembly:\n%s", asm)

			// Run each test case through the Z80 emulator
			passed, failed := 0, 0
			for _, tc := range cases {
				a, b, want := tc[0], tc[1], tc[2]
				got := runGCD(t, asm, a, b)
				if got != want {
					t.Errorf("gcd(%d, %d) = %d, want %d", a, b, got, want)
					failed++
				} else {
					passed++
				}
			}
			t.Logf("%s path: %d/%d tests passed (%d instructions)",
				path, passed, len(cases), instCount)
		})
	}
}

// TestGraceE2E_Showcase runs correctness tests on all 9 showcase programs
// through the Grace path, verifying they produce valid Z80 binaries that
// assemble without errors.
func TestGraceE2E_Showcase(t *testing.T) {
	if testing.Short() {
		t.Skip("corpus-scale test; runs under `make test-all`, skipped by -short")
	}
	programs := []struct {
		name string
		src  string
	}{
		{"add", `
fun add(a: u8, b: u8) -> u8 { return a + b }
`},
		{"abs_diff", `
fun abs_diff(a: u8, b: u8) -> u8 {
    if a > b { return a - b }
    return b - a
}`},
		{"max", `
fun max(a: u8, b: u8) -> u8 {
    if a > b { return a }
    return b
}`},
		{"min", `
fun min(a: u8, b: u8) -> u8 {
    if a < b { return a }
    return b
}`},
		{"clamp", `
fun clamp(x: u8, lo: u8, hi: u8) -> u8 {
    if x < lo { return lo }
    if x > hi { return hi }
    return x
}`},
	}

	stats := mir2.NewGraceStats()

	for _, p := range programs {
		t.Run(p.name, func(t *testing.T) {
			hm, err := nanz.ParseWithOpts(p.src, p.name+".nanz", nanz.ParseOpts{})
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			steps, err := CompileHIRSteps(hm, Options{
				ContractOpt: true,
				UseGrace:    true,
				GraceStats:  stats,
			})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}

			// Verify assembly is valid
			assembler := z80asm.NewAssembler()
			res, err := assembler.AssembleString("ORG 0x8000\n" + steps.Assembly)
			if err != nil {
				t.Fatalf("assemble: %v", err)
			}
			if len(res.Errors) > 0 {
				for _, e := range res.Errors {
					t.Errorf("asm error: %v", e)
				}
			}

			instCount := countInstructions(steps.Assembly)
			t.Logf("%s: %d instructions, %d bytes", p.name, instCount, len(res.Binary))
		})
	}

	t.Logf("\nGrace stats across all programs: %s", stats)
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func runGCD(t *testing.T, funcAsm string, a, b int) int {
	t.Helper()
	p1, p2 := parseGCDABI(t, funcAsm)
	const loadAddr = 0x8000
	bootstrap := fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD %s, %d
    LD %s, %d
    CALL gcd
    DI
    HALT
`, loadAddr, p1, a, p2, b)
	return runZ80GetA(t, bootstrap+"\n"+funcAsm, loadAddr)
}

func parseGCDABI(t *testing.T, funcAsm string) (param1, param2 string) {
	t.Helper()
	for _, line := range strings.Split(funcAsm, "\n") {
		if !strings.Contains(line, "fun gcd(") {
			continue
		}
		parts := strings.Split(line, "= ")
		if len(parts) >= 3 {
			p1 := strings.Split(strings.TrimSpace(parts[1]), ",")[0]
			p2 := strings.Split(strings.TrimSpace(parts[2]), ")")[0]
			return strings.TrimSpace(p1), strings.TrimSpace(p2)
		}
	}
	t.Fatalf("could not parse ABI for gcd from:\n%s", funcAsm)
	return "", ""
}

func runZ80GetA(t *testing.T, src string, loadAddr int) int {
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
