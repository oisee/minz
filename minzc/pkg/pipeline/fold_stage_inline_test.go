package pipeline

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// F0-REPORT.md §2.2 / §5 "F1": a map/filter stage feeding a fold terminal is
// lowered by foldStageBody, which — unlike prependStageStmts for forEach —
// never inlined the stage callback. The predicate therefore stayed a CallExpr,
// its flag was materialised into a bool vreg, parked in a $F0xx memory home
// and reloaded with AND A before the branch (160-290 T per element).
const foldFilterStageInlineSource = `global a: [u8; 4]
global n: u16
global r: u16
fun execute() {
    let s = a.filter(|x: u8| x >= 128).fold(u16(0), |acc: u16, x: u8| acc + u16(x), n)
    r = s
}
`

// Same program with a named fold callback. The frontend infers the fold
// accumulator type only from a *named* callback (nanz/parse.go, let/fold type
// inference), so this is the form whose u16 accumulator survives the `let`.
const foldFilterStageNamedSource = `global a: [u8; 4]
global n: u16
global r: u16
fun add16(acc: u16, x: u8) -> u16 { return acc + u16(x) }
fun execute() {
    let s = a.filter(|x: u8| x >= 128).fold(u16(0), add16, n)
    r = s
}
`

// fusedLoopBody returns the assembly lines from a fused iterator loop's head
// label through the back-edge that targets it, i.e. exactly the code that runs
// once per element.
func fusedLoopBody(t *testing.T, asm string) string {
	t.Helper()
	head := -1
	label := ""
	lines := strings.Split(asm, "\n")
	for i, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if !strings.HasSuffix(trimmed, ":") {
			continue
		}
		name := strings.TrimSuffix(trimmed, ":")
		if !strings.HasPrefix(name, ".") || !strings.Contains(name, "_fe_head") {
			continue
		}
		head, label = i, name
		break
	}
	if head < 0 {
		t.Fatalf("no fused loop head label (_fe_head) in:\n%s", asm)
	}
	for i := head + 1; i < len(lines); i++ {
		if strings.Contains(lines[i], label) {
			return strings.Join(lines[head:i+1], "\n")
		}
	}
	t.Fatalf("no back-edge to %s in:\n%s", label, asm)
	return ""
}

// runExecuteGlobalsZ80 assembles the compiled module behind a CALL execute
// bootstrap, seeds the a/n globals and returns the u16 written to r.
func runExecuteGlobalsZ80(t *testing.T, steps Steps, data []byte, n int) uint16 {
	t.Helper()
	const load = 0x8000
	boot := "    ORG 0x8000\n    LD SP, 0xFF00\n    CALL execute\n    DI\n    HALT\n"
	res, err := z80asm.NewAssembler().AssembleString(boot + "\n" + steps.Assembly)
	if err != nil || len(res.Errors) > 0 {
		t.Fatalf("assemble: %v %v", err, res.Errors)
	}
	global := func(base string) uint16 {
		for _, cand := range []string{base, "v_" + base} {
			if addr, ok := res.Symbols[cand]; ok {
				return uint16(addr)
			}
		}
		t.Fatalf("global symbol %q missing: %v", base, res.Symbols)
		return 0
	}
	platform := emulator.NewRemogattoZ80WithScreen()
	platform.SetROMEnd(0) // all-RAM fixture: no screen/RST interception
	z := platform.RemogattoZ80
	z.SetSMCTracker(nil)
	if err := z.LoadMemory(load, res.Binary); err != nil {
		t.Fatal(err)
	}
	aAddr, nAddr, rAddr := global("a"), global("n"), global("r")
	for i, v := range data {
		z.SetMemory(aAddr+uint16(i), v)
	}
	z.SetMemory(nAddr, byte(n))
	z.SetMemory(nAddr+1, byte(n>>8))
	z.SetMemory(rAddr, 0xAA)
	z.SetMemory(rAddr+1, 0xAA)
	z.SetPC(load)
	for i := 0; !z.IsHalted(); i++ {
		if i > 100000 {
			t.Fatalf("execute did not halt; PC=%04x\n%s", z.GetPC(), steps.Assembly)
		}
		z.Step()
	}
	return uint16(z.GetMemory(rAddr)) | uint16(z.GetMemory(rAddr+1))<<8
}

func TestFoldFilterStageInlinesPredicateVMZ80(t *testing.T) {
	allPass := []byte{128, 200, 255, 129}
	nonePass := []byte{0, 127, 5, 100}
	cases := []struct {
		name string
		data []byte
		n    int
		sum  uint16
	}{
		{"all_pass_n4", allPass, 4, 128 + 200 + 255 + 129},
		{"none_pass_n4", nonePass, 4, 0},
		{"all_pass_n0", allPass, 0, 0},
		{"all_pass_n2", allPass, 2, 128 + 200},
		{"all_pass_n3", allPass, 3, 128 + 200 + 255},
	}
	for _, src := range []struct {
		name string
		text string
		// The lambda-callback form is typed u8 by the frontend's fold type
		// inference (named callbacks only), so its u16 accumulator is truncated
		// at the `let` — pre-existing on origin/main (LD D,0 before the store)
		// and unchanged by the stage inlining under test.
		truncated bool
	}{
		{"lambda_callback", foldFilterStageInlineSource, true},
		{"named_callback", foldFilterStageNamedSource, false},
	} {
		t.Run(src.name, func(t *testing.T) {
			steps, err := CompileHIRSteps(graceMeasureParse(t, src.text), Options{AssertMode: "none"})
			if err != nil {
				t.Fatal(err)
			}
			// Real names from the unpatched compiler's output:
			// .bool_false_0/.bool_true_0/.bool_done_0, AND A,
			// _spill_execute_r13/_spill_execute_r14.
			loop := fusedLoopBody(t, steps.Assembly)
			for _, bad := range []string{".bool_false_", ".bool_true_", ".bool_done_", "AND A", "_spill_"} {
				if strings.Contains(loop, bad) {
					t.Fatalf("fused loop still materialises the filter predicate: %q present\n%s", bad, loop)
				}
			}
			if !strings.Contains(loop, "CP 128") {
				t.Fatalf("inlined predicate compare missing from fused loop:\n%s", loop)
			}
			for _, c := range cases {
				want := c.sum
				if src.truncated {
					want &= 0xFF
				}
				if got := runExecuteGlobalsZ80(t, steps, c.data, c.n); got != want {
					t.Fatalf("%s: r = %d, want %d (data=%v n=%d)\n%s", c.name, got, want, c.data, c.n, steps.Assembly)
				}
			}
		})
	}
}
