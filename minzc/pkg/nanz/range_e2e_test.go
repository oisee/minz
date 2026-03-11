package nanz_test

// E2E emulator tests for range(lo..hi).fold() and .forEach() — verifies that
// the generated Z80 assembly produces correct results when executed.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

const rangeFoldLoadAddr = 0x8000

func assembleFold(t *testing.T, genAsm string) func(n int) (uint8, error) {
	t.Helper()
	return func(n int) (uint8, error) {
		boot := fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD A, %d
    CALL sum_range
    DI
    HALT
`, rangeFoldLoadAddr, n)
		src := boot + "\n" + genAsm
		as := z80asm.NewAssembler()
		res, err := as.AssembleString(src)
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
		if lerr := z80.LoadMemory(rangeFoldLoadAddr, res.Binary); lerr != nil {
			return 0, fmt.Errorf("load: %w", lerr)
		}
		z80.SetPC(rangeFoldLoadAddr)
		if rerr := z80.Run(); rerr != nil {
			return 0, fmt.Errorf("run: %w", rerr)
		}
		regs := z80.GetRegisters()
		return regs.A, nil
	}
}

// TestRangeFold_E2E_SumRange verifies range(0..n).fold(0, |acc,i| acc+i)
// returns the correct sum.  Because range(0..n) counts DOWN from n to 1, the
// result is n*(n+1)/2 (sum of 1..n), not sum of 0..n-1.
func TestRangeFold_E2E_SumRange(t *testing.T) {
	src := `
fun sum_range(n: u8) -> u8 {
    return range(0..n).fold(0, |acc: u8, i: u8| { return acc + i })
}
`
	m, err := nanz.Parse(src, "e2e_range_fold")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	genAsm, err := pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	t.Logf("generated Z80:\n%s", genAsm)

	run := assembleFold(t, genAsm)

	// range(0..n) counts DOWN: elements = n, n-1, ..., 1.  Sum = n*(n+1)/2.
	cases := []struct {
		n, want int
	}{
		{0, 0},
		{1, 1},   // just 1
		{4, 10},  // 4+3+2+1
		{5, 15},  // 5+4+3+2+1
		{10, 55}, // 10*(10+1)/2
	}
	for _, tc := range cases {
		got, err := run(tc.n)
		if err != nil {
			t.Errorf("sum_range(%d): %v", tc.n, err)
			continue
		}
		if int(got) != tc.want {
			t.Errorf("sum_range(%d) = %d, want %d", tc.n, got, tc.want)
		} else {
			t.Logf("sum_range(%d) = %d ✓", tc.n, got)
		}
	}
}

func TestRangeVariants_Asm(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string // asm substrings that must appear
		bad  []string // asm substrings that must NOT appear
	}{
		{
			name: "range(n..0) fold — direct DJNZ, no SUB",
			src:  `fun f(n: u8) -> u8 { return range(n..0).fold(0, |acc: u8, i: u8| { return acc + i }) }`,
			want: []string{"DJNZ", "ADD A"},
			bad:  []string{"SUB"},
		},
		{
			name: "range(0..n) fold — direct DJNZ, no SUB",
			src:  `fun f(n: u8) -> u8 { return range(0..n).fold(0, |acc: u8, i: u8| { return acc + i }) }`,
			want: []string{"DJNZ", "ADD A"},
			bad:  []string{"SUB"},
		},
		{
			name: "range(3..8) fold — DJNZ with count=5 via SUB or Const(5)",
			src:  `fun f() -> u8 { return range(3..8).fold(0, |acc: u8, i: u8| { return acc + i }) }`,
			want: []string{"DJNZ", "ADD A"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := nanz.Parse(tc.src, "range_variant_test")
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			asm, err := pipeline.CompileHIR(m)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			t.Logf("asm:\n%s", asm)
			for _, w := range tc.want {
				if !strings.Contains(asm, w) {
					t.Errorf("missing %q in output", w)
				}
			}
			for _, b := range tc.bad {
				if strings.Contains(asm, b) {
					t.Errorf("unexpected %q in output", b)
				}
			}
		})
	}
}
