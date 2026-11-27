package hir_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// unrollProbe is one function of the admission table: the number of loops the
// lowering must unroll in it, and argument/result pairs checked on the raw MIR
// VM with and without unrolling.
type unrollProbe struct {
	name  string
	loops int
	calls [][2][]int64 // {args, want}
}

const unrollC89Source = `
unsigned char g, h;
volatile unsigned char port;
unsigned char sink(unsigned char x) { return x + 1; }

unsigned char trips1(unsigned char n, unsigned char x) {
    unsigned char r, b;
    for (r = 0; r < n; r++) for (b = 0; b < 1; b++) x = x * 3 + 1;
    return x;
}
unsigned char trips2(unsigned char n, unsigned char x) {
    unsigned char r, b;
    for (r = 0; r < n; r++) for (b = 0; b < 2; b++) x = x * 3 + 1;
    return x;
}
unsigned char trips8(unsigned char n, unsigned char x) {
    unsigned char r, b;
    for (r = 0; r < n; r++) for (b = 0; b < 8; b++) x = x * 3 + 1;
    return x;
}
unsigned char trips9(unsigned char n, unsigned char x) {
    unsigned char r, b;
    for (r = 0; r < n; r++) for (b = 0; b < 9; b++) x = x * 3 + 1;
    return x;
}
unsigned char trips0(unsigned char n, unsigned char x) {
    unsigned char r, b;
    for (r = 0; r < n; r++) for (b = 0; b < 0; b++) x = x * 3 + 1;
    return x;
}
unsigned char offset(unsigned char n, unsigned char x) {
    unsigned char r, b;
    for (r = 0; r < n; r++) for (b = 3; b < 6; b++) x = x + b;
    return x;
}
/* the body reads the induction variable: bit reverse */
unsigned char reverse(unsigned char n, unsigned char m) {
    unsigned char r, b, x = 0;
    for (r = 0; r < n; r++) {
        x = 0;
        for (b = 0; b < 8; b++) if ((m >> b) & 1) x |= 1 << (7 - b);
        m = x;
    }
    return x;
}
/* loop-carried values and the induction variable used after the loop */
unsigned char after(unsigned char n, unsigned char x) {
    unsigned char r, b = 0, y = 0;
    for (r = 0; r < n; r++) for (b = 0; b < 5; b++) { y = y + x; x = x ^ y; }
    return x + y + b;
}
unsigned char early(unsigned char n, unsigned char x) {
    unsigned char r, b;
    for (r = 0; r < n; r++) for (b = 0; b < 4; b++) { if (x == 7) break; x = x + 3; }
    return x;
}
unsigned char skip(unsigned char n, unsigned char x) {
    unsigned char r, b;
    for (r = 0; r < n; r++) for (b = 0; b < 4; b++) { if (x & 1) continue; x = x + 3; }
    return x;
}
unsigned char leave(unsigned char n, unsigned char x) {
    unsigned char r, b;
    for (r = 0; r < n; r++) for (b = 0; b < 4; b++) { if (x == 9) return 99; x = x + 3; }
    return x;
}
unsigned char jump(unsigned char n, unsigned char x) {
    unsigned char r, b;
    for (r = 0; r < n; r++) for (b = 0; b < 4; b++) { if (x == 9) goto out; x = x + 3; }
out:
    return x;
}
unsigned char call(unsigned char n, unsigned char x) {
    unsigned char r, b;
    for (r = 0; r < n; r++) for (b = 0; b < 4; b++) x = sink(x);
    return x;
}
unsigned char writes(unsigned char n, unsigned char x) {
    unsigned char r, b;
    for (r = 0; r < n; r++) for (b = 0; b < 4; b++) { x = x + 1; b = b + 1; }
    return x;
}
unsigned char outer(unsigned char x) {
    unsigned char b;
    for (b = 0; b < 4; b++) x = x * 3 + 1;
    return x;
}
unsigned char nested(unsigned char n, unsigned char x) {
    unsigned char r, b, c;
    for (r = 0; r < n; r++) for (b = 0; b < 4; b++) for (c = 0; c < 2; c++) x = x * 3 + 1;
    return x;
}
unsigned char reads(unsigned char n) {
    unsigned char r, b, s = 0;
    for (r = 0; r < n; r++) for (b = 0; b < 3; b++) s = s + port;
    return s;
}
`

func unrollC89Probes() []unrollProbe {
	mul := func(n, x, k int64) int64 {
		for i := int64(0); i < n*k; i++ {
			x = (x*3 + 1) & 255
		}
		return x
	}
	calls := func(k int64) [][2][]int64 {
		var out [][2][]int64
		for _, a := range [][2]int64{{0, 5}, {1, 5}, {3, 200}, {7, 0}} {
			out = append(out, [2][]int64{{a[0], a[1]}, {mul(a[0], a[1], k)}})
		}
		return out
	}
	rev := func(n, m int64) int64 {
		x := int64(0)
		for r := int64(0); r < n; r++ {
			x = 0
			for b := 0; b < 8; b++ {
				if (m>>b)&1 != 0 {
					x |= 1 << (7 - b)
				}
			}
			m = x
		}
		return x
	}
	after := func(n, x int64) int64 {
		b, y := int64(0), int64(0)
		for r := int64(0); r < n; r++ {
			for b = 0; b < 5; b++ {
				y = (y + x) & 255
				x ^= y
			}
		}
		return (x + y + b) & 255
	}
	return []unrollProbe{
		{"trips1", 1, calls(1)},
		{"trips2", 1, calls(2)},
		{"trips8", 1, calls(8)},
		{"trips9", 0, calls(9)},
		{"trips0", 0, calls(0)},
		{"offset", 1, [][2][]int64{{{0, 1}, {1}}, {{2, 1}, {25}}, {{20, 250}, {(250 + 20*12) & 255}}}},
		{"reverse", 1, [][2][]int64{{{1, 1}, {rev(1, 1)}}, {{1, 0xb4}, {rev(1, 0xb4)}}, {{2, 0x35}, {rev(2, 0x35)}}, {{0, 9}, {0}}}},
		{"after", 1, [][2][]int64{{{0, 3}, {after(0, 3)}}, {{1, 3}, {after(1, 3)}}, {{4, 77}, {after(4, 77)}}}},
		{"early", 0, [][2][]int64{{{1, 1}, {7}}, {{2, 0}, {24}}}},
		{"skip", 0, [][2][]int64{{{1, 0}, {3}}, {{2, 2}, {5}}}},
		{"leave", 0, [][2][]int64{{{1, 0}, {99}}, {{1, 1}, {13}}}},
		{"jump", 0, [][2][]int64{{{1, 0}, {9}}, {{1, 1}, {13}}}},
		{"call", 0, [][2][]int64{{{2, 1}, {9}}}},
		{"writes", 0, [][2][]int64{{{1, 0}, {2}}, {{3, 0}, {6}}}},
		{"outer", 0, [][2][]int64{{{5}, {mul(1, 5, 4)}}}},
		// The innermost loop (inside another loop) unrolls; the loop that
		// contains it does not.
		{"nested", 1, [][2][]int64{{{2, 5}, {mul(2, 5, 8)}}}},
		{"reads", 1, nil},
	}
}

func lowerUnroll(t *testing.T, h *hir.Module, on bool, deny map[string]bool) (*mir2.Module, map[string]int) {
	t.Helper()
	unrolled := map[string]int{}
	m := hir.LowerModuleWithOptions(h, hir.LowerOptions{UnrollLoops: on, UnrollDeny: deny, Unrolled: unrolled})
	if err := mir2.Verify(m); err != nil {
		t.Fatal(err)
	}
	return m, unrolled
}

func checkUnrollProbes(t *testing.T, h *hir.Module, probes []unrollProbe) {
	t.Helper()
	plain, none := lowerUnroll(t, h, false, nil)
	if len(none) != 0 {
		t.Fatalf("unrolled without the option: %v", none)
	}
	m, unrolled := lowerUnroll(t, h, true, nil)
	for _, p := range probes {
		if unrolled[p.name] != p.loops {
			t.Errorf("%s: unrolled %d loops, want %d", p.name, unrolled[p.name], p.loops)
		}
		f := m.FuncByName(p.name)
		if f == nil {
			t.Fatalf("%s missing", p.name)
		}
		// An unrolled loop leaves neither a counter nor loop metadata.
		if p.loops > 0 && p.name != "nested" {
			for _, b := range f.Blocks {
				if _, ok := b.Term.(*mir2.TermDJNZ); ok {
					t.Errorf("%s: unrolled inner loop kept DJNZ", p.name)
				}
			}
			if got, want := len(f.StaticLoops), len(plain.FuncByName(p.name).StaticLoops)-p.loops; got != want {
				t.Errorf("%s: %d static loops, want %d", p.name, got, want)
			}
		}
		for _, c := range p.calls {
			args := make([]mir2.Value, len(c[0]))
			for i, a := range c[0] {
				args[i] = mir2.Value{I: a}
			}
			for mode, mod := range []*mir2.Module{plain, m} {
				got, err := mir2.NewVM(mod).Call(p.name, args)
				if err != nil || len(got) != 1 || got[0].I != c[1][0] {
					t.Errorf("%s%v mode %d: got %v %v want %d", p.name, c[0], mode, got, err, c[1][0])
				}
			}
		}
	}
	// Per-function refusal is honoured.
	deny := map[string]bool{}
	for _, p := range probes {
		deny[p.name] = true
	}
	if _, denied := lowerUnroll(t, h, true, deny); len(denied) != 0 {
		t.Errorf("deny ignored: %v", denied)
	}
}

func TestUnrollConstantTripAdmissionC89(t *testing.T) {
	h, err := c89.CompileWithOpts(unrollC89Source, "unroll.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	checkUnrollProbes(t, h, unrollC89Probes())
}

// Cloning lowers the volatile read once per iteration, in order: three
// iterations of one read are three volatile loads in one straight line.
func TestUnrollKeepsVolatileAccessCount(t *testing.T) {
	h, err := c89.CompileWithOpts(unrollC89Source, "unroll.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	count := func(m *mir2.Module) int {
		n := 0
		for _, b := range m.FuncByName("reads").Blocks {
			for _, i := range b.Insts {
				if i.Op == mir2.OpLoad && i.Volatile {
					n++
				}
			}
		}
		return n
	}
	plain, _ := lowerUnroll(t, h, false, nil)
	m, _ := lowerUnroll(t, h, true, nil)
	if count(plain) != 1 || count(m) != 3 {
		t.Fatalf("volatile loads: loop %d, unrolled %d (want 1 and 3)", count(plain), count(m))
	}
}

const unrollNanzSource = `
fun pop(n: u8, v: u8) -> u8 {
    var c: u8 = 0
    var w: u8 = v
    for r in 0..n {
        c = 0
        w = v
        for b in 0..8 { c = c + (w & 1)
          w = w >> 1 }
    }
    return c
}
fun idx(n: u8, m: u8) -> u8 {
    var x: u8 = 0
    for r in 0..n {
        x = 0
        for b in 0..8 { if ((m >> b) & 1) == 1 { x = x | (1 << b) } }
    }
    return x
}
fun big(n: u8, m: u8) -> u8 {
    var x: u8 = m
    for r in 0..n { for b in 0..12 { x = x + b } }
    return x
}
fun top(m: u8) -> u8 {
    var x: u8 = m
    for b in 0..4 { x = x + b }
    return x
}
fun withasm(n: u8, m: u8) -> u8 {
    var x: u8 = m
    for r in 0..n { for b in 0..4 { x = x + b } }
    asm (in x) (clob all) { NOP }
    return x
}
fun fall?(n: u8, m: u8) -> u8 {
    var x: u8 = m
    for r in 0..n { for b in 0..4 { x = x + b } }
    if x == 0 { @error(1) }
    return x
}
`

func TestUnrollConstantTripAdmissionNanz(t *testing.T) {
	h, err := nanz.Parse(unrollNanzSource, "unroll.nanz")
	if err != nil {
		t.Fatal(err)
	}
	pop := func(v int64) int64 {
		c := int64(0)
		for ; v != 0; v >>= 1 {
			c += v & 1
		}
		return c
	}
	var popCalls, idxCalls [][2][]int64
	for _, v := range []int64{0, 1, 0x80, 0xff, 0x5a} {
		popCalls = append(popCalls, [2][]int64{{2, v}, {pop(v)}})
		idxCalls = append(idxCalls, [2][]int64{{1, v}, {v}})
	}
	checkUnrollProbes(t, h, []unrollProbe{
		{"pop", 1, popCalls},
		{"idx", 1, idxCalls},
		{"big", 0, [][2][]int64{{{1, 0}, {66}}}},
		{"top", 0, [][2][]int64{{{1}, {7}}}},
		{"fall?", 0, nil},
		{"withasm", 0, nil},
	})
}

// The induction value of every copy is its constant: a body reading b sees
// 0..7 in order, and the loop's register-free form names no counter class.
func TestUnrollSubstitutesInductionConstants(t *testing.T) {
	h, err := nanz.Parse(unrollNanzSource, "unroll.nanz")
	if err != nil {
		t.Fatal(err)
	}
	m, _ := lowerUnroll(t, h, true, nil)
	dump := m.Dump()
	dump = dump[strings.Index(dump, "fun @idx"):]
	dump = dump[:strings.Index(dump, "fun @big")]
	if strings.Contains(dump, "[counter]") || strings.Contains(dump, "djnz") {
		t.Fatalf("unrolled idx still has a counter:\n%s", dump)
	}
}

// A loop that carries measured lineage keeps its identity (and its
// LoopOrigins entry) even when lowering is asked to unroll.
func TestUnrollRefusesLineageLoop(t *testing.T) {
	h, err := nanz.Parse(unrollNanzSource, "unroll.nanz")
	if err != nil {
		t.Fatal(err)
	}
	h.LoopLineage = map[hir.Stmt]hir.LoopLineage{}
	var mark func(*hir.Block, int)
	mark = func(b *hir.Block, depth int) {
		for _, s := range b.Body {
			switch x := s.(type) {
			case *hir.ForRangeStmt:
				if depth == 1 {
					h.LoopLineage[x] = hir.LoopLineage{SiteID: "inner", Role: "loop"}
				}
				mark(x.Body, depth+1)
			case *hir.Block:
				mark(x, depth)
			}
		}
	}
	mark(h.FuncByName("pop").Body, 0)
	if len(h.LoopLineage) != 1 {
		t.Fatalf("inner loop not found: %d", len(h.LoopLineage))
	}
	_, unrolled := lowerUnroll(t, h, true, nil)
	if unrolled["pop"] != 0 {
		t.Fatalf("lineage loop unrolled")
	}
}
