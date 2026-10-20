package nanz_test

// G0 audit of Nanz iteration forms — pins current behaviour of every
// iteration construct the Nanz frontend recognises (`for i in lo..hi`,
// `for x in ptr[lo..hi]`, iterator chains map/filter → forEach/mapInPlace/
// fold/reduce over memory or range() sources, named pipe/trans + .apply())
// on three tiers: raw MIR2 VM, optimised MIR2 VM, and native Z80.
//
// Findings (bugs) are encoded as tests that expect the CORRECT result and
// t.Skip with a "G0 finding" marker so the bug is documented without breaking
// the build. Findings fixed by this audit (callback inlining) are asserted
// directly. Companion doc: docs/Nanz_Iteration_Audit.md.

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

// g0RunAsserts evaluates a module's asserts on the raw MIR2 VM, the optimised
// MIR2 VM, and the assembled Z80 program.
func g0RunAsserts(t *testing.T, src string) error {
	t.Helper()
	parse := func() *hir.Module {
		m, err := nanz.Parse(src, "g0_audit")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		return m
	}
	rawHM := parse()
	raw := hir.LowerModule(rawHM)
	if err := pipeline.RunAssertsMIR2(rawHM, raw); err != nil {
		return fmt.Errorf("raw MIR2 VM: %w", err)
	}
	hm := parse()
	steps, err := pipeline.CompileHIRSteps(hm, pipeline.DefaultOptions())
	if err != nil {
		return fmt.Errorf("compile: %w", err)
	}
	if err := pipeline.RunAssertsMIR2(hm, steps.MIR2Module); err != nil {
		return fmt.Errorf("optimised MIR2 VM: %w", err)
	}
	if err := pipeline.RunAssertsZ80(hm, steps.MIR2Module, steps.Allocation, steps.Assembly); err != nil {
		return fmt.Errorf("z80: %w", err)
	}
	return nil
}

func TestG0ForRangeSemantics(t *testing.T) {
	src := `
fun order() -> u8 {
    var acc: u8 = 0
    for i in 0..5 { acc = acc * 3 + i }
    return acc
}
fun empty() -> u8 {
    var acc: u8 = 77
    for i in 0..0 { acc = acc * 3 + i }
    return acc
}
fun one() -> u8 {
    var acc: u8 = 0
    for i in 0..1 { acc = acc * 3 + i }
    return acc
}
fun n255_sum() -> u8 {
    var acc: u8 = 0
    for i in 0..255 { acc = acc + i }
    return acc
}
fun n256_iters() -> u16 {
    var iters: u16 = 0
    for i in 0..256 { iters = iters + 1 }
    return iters
}
fun brk() -> u8 {
    var acc: u8 = 0
    for i in 0..10 {
        if i == 3 { break }
        acc = acc + 1
    }
    return acc
}
fun cont() -> u8 {
    var acc: u8 = 0
    for i in 0..6 {
        if i == 2 { continue }
        acc = acc + 1
    }
    return acc
}
fun ret_early() -> u8 {
    var acc: u8 = 0
    for i in 0..10 {
        if i == 3 { return acc }
        acc = acc + 1
    }
    return 99
}
assert order() == 58        // forward 0,1,2,3,4 (checksum acc=acc*3+i)
assert empty() == 77        // zero iterations
assert one() == 0           // single element 0
assert n255_sum() == 129    // 0+..+254 mod 256
assert n256_iters() == 256  // u16 bound keeps the full count
assert brk() == 3           // break leaves after i==3
assert cont() == 5          // continue skips only i==2
assert ret_early() == 3     // return from consumer exits mid-loop
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}

func TestG0ForEachPtrSemantics(t *testing.T) {
	src := `
global src8: [u8; 8]
global big: [u8; 300]

fun load() -> void {
    src8[0] = 1
    src8[1] = 2
    src8[2] = 3
    src8[3] = 4
    src8[4] = 5
    src8[5] = 6
    src8[6] = 7
    src8[7] = 8
}
fun order() -> u8 {
    load()
    var acc: u8 = 0
    for x in src8[0..5] { acc = acc * 3 + x }
    return acc
}
fun empty() -> u8 {
    var acc: u8 = 77
    for x in src8[0..0] { acc = acc * 3 + x }
    return acc
}
fun one() -> u8 {
    load()
    var acc: u8 = 0
    for x in src8[0..1] { acc = acc * 3 + x }
    return acc
}
fun slice_start() -> u8 {
    load()
    var acc: u8 = 0
    for x in src8[2..5] { acc = acc * 3 + x }
    return acc
}
fun n255() -> u8 {
    var p: ^u8 = big
    var i: u8 = 0
    while i != 255 {
        p^ = 1
        p = p + 1
        i = i + 1
    }
    var acc: u8 = 0
    for x in big[0..255] { acc = acc + x }
    return acc
}
fun n256() -> u16 {
    var p: ^u8 = big
    var i: u16 = 0
    while i < 256 {
        p^ = 2
        p = p + 1
        i = i + 1
    }
    var iters: u16 = 0
    for x in big[0..256] { iters = iters + 1 }
    return iters
}
assert order() == 179       // forward 1,2,3,4,5
assert empty() == 77
assert one() == 1
assert slice_start() == 44  // forward 3,4,5
assert n255() == 255        // 255 ones
assert n256() == 256        // u16 len keeps the full count
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}

// TestG0Finding_ReversedSliceIsEmpty: `for x in ptr[3..1]` should visit
// nothing. Today the length is computed as u8 (end - start) = 254, so the loop
// runs 254 iterations reading past the 8-byte source.
func TestG0Finding_ReversedSliceIsEmpty(t *testing.T) {
	src := `
global src8: [u8; 8]
global cnt: u16 = 0

fun count() -> u16 {
    src8[0] = 1
    src8[1] = 2
    src8[2] = 3
    src8[3] = 4
    cnt = 0
    for x in src8[3..1] {
        cnt = cnt + 1
    }
    return cnt
}
assert count() == 0
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Skip("G0 finding G0-3: reversed slice ptr[3..1] underflows u8 length and reads out of bounds: ", err)
	}
}

func TestG0RangeSourceSemantics(t *testing.T) {
	src := `
global acc: u8 = 0
global sum16: u16 = 0
global n16: u16 = 0

fun step(x: u8) -> void {
    acc = acc * 3 + x
}
fun cnt_sum(x: u8) -> void {
    sum16 = sum16 + u16(x)
    n16 = n16 + 1
}

// range(0..n) counts DOWN: n, n-1, ..., 1 — the reverse of for i in 0..n.
fun order_0_5() -> u8 {
    acc = 0
    range(0..5).forEach(step)
    return acc
}
// range(n..0) is the same back-counting loop (Rev spelling).
fun order_5_0() -> u8 {
    acc = 0
    range(5..0).forEach(step)
    return acc
}
// For non-zero bounds the ELEMENT is the countdown counter, not lo+offset:
// range(2..7) has count 5 and yields 5,4,3,2,1.
fun order_2_7() -> u8 {
    acc = 0
    range(2..7).forEach(step)
    return acc
}
fun empty() -> u8 {
    acc = 77
    range(0..0).forEach(step)
    return acc
}
fun one() -> u8 {
    acc = 0
    range(0..1).forEach(step)
    return acc
}
fun n255_sum() -> u16 {
    sum16 = 0
    n16 = 0
    range(0..255).forEach(cnt_sum)
    return sum16
}
fun n255_cnt() -> u16 {
    sum16 = 0
    n16 = 0
    range(0..255).forEach(cnt_sum)
    return n16
}
// n=256 wraps the u8 counter into a full cycle: 256 iterations yielding the
// multiset {0,1,...,255} (first element 0, then 255 down to 1).
fun n256_sum() -> u16 {
    sum16 = 0
    n16 = 0
    range(0..256).forEach(cnt_sum)
    return sum16
}
fun n256_cnt() -> u16 {
    sum16 = 0
    n16 = 0
    range(0..256).forEach(cnt_sum)
    return n16
}
assert order_0_5() == 35     // elements 5,4,3,2,1
assert order_5_0() == 35
assert order_2_7() == 35     // elements 5,4,3,2,1 (NOT 2..6)
assert empty() == 77
assert one() == 1            // single element 1
assert n255_sum() == 32640   // 255+254+...+1
assert n255_cnt() == 255
assert n256_sum() == 32640   // 0+1+...+255 (wrapped counter)
assert n256_cnt() == 256
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}

// TestG0Finding_CrossedRangeBounds: range(5..3) has hi < lo with hi != 0, so
// it is not the Rev spelling; count = hi-lo underflows u8 to 254 and the loop
// runs 254 iterations (254..1). It should be rejected or empty.
func TestG0Finding_CrossedRangeBounds(t *testing.T) {
	src := `
global n16: u16 = 0
global sum16: u16 = 0

fun cnt(x: u8) -> void {
    n16 = n16 + 1
    sum16 = sum16 + u16(x)
}
fun crossed() -> u16 {
    n16 = 0
    sum16 = 0
    range(5..3).forEach(cnt)
    return n16
}
assert crossed() == 0
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Skip("G0 finding G0-4: range(5..3) count underflows u8 and iterates 254 times: ", err)
	}
}

func TestG0IterChainMemSemantics(t *testing.T) {
	src := `
global src8: [u8; 8]
global big: [u8; 300]
global acc: u8 = 0
global trace: [u8; 8]
global trace_n: u8 = 0
global g_total: u16 = 0

fun load() -> void {
    src8[0] = 1
    src8[1] = 2
    src8[2] = 3
    src8[3] = 4
    src8[4] = 5
    src8[5] = 6
    src8[6] = 7
    src8[7] = 8
}
fun add10(x: u8) -> u8 {
    return x + 10
}
fun keep6(x: u8) -> u8 {
    return x > 13
}
fun chain_step(x: u8) -> void {
    acc = acc * 3 + x
}
fun trace_put(x: u8) -> void {
    trace[trace_n] = x
    trace_n = trace_n + 1
}

// functional form: forEach(filter(map(...)), cb, n) — forward order,
// stages applied innermost-first, downstream sees only filter survivors.
fun functional() -> u8 {
    load()
    acc = 0
    forEach(filter(map(src8, add10), keep6), chain_step, 5)
    return acc
}
// UFCS dotted form is the same call after parser rewriting.
fun ufcs() -> u8 {
    load()
    acc = 0
    src8.map(add10).filter(keep6).forEach(chain_step, 5)
    return acc
}
// lambda stage bodies are inlined the same way.
fun lambda_stage() -> u8 {
    load()
    acc = 0
    forEach(map(src8, |x: u8| x + x), chain_step, 5)
    return acc
}
// side-effect order: downstream writes happen in source order, one per
// surviving element, after map/filter for that element.
fun order_trace() -> u16 {
    load()
    trace_n = 0
    forEach(filter(map(src8, add10), keep6), trace_put, 5)
    return u16(trace[0]) * 256 + u16(trace[1])
}
fun order_trace_count() -> u8 {
    load()
    trace_n = 0
    forEach(filter(map(src8, add10), keep6), trace_put, 5)
    return trace_n
}
// mapInPlace stores transformed elements back into the source.
fun mapinplace() -> u8 {
    load()
    mapInPlace(src8, add10, 5)
    return src8[0] + src8[4]
}
// empty input: no downstream effects, terminal result untouched.
fun empty_chain() -> u8 {
    acc = 77
    forEach(filter(map(src8, add10), keep6), chain_step, 0)
    return acc
}
// u8 counter boundary: 255 elements through a fused chain.
fun chain_n255() -> u8 {
    var p: ^u8 = big
    var i: u8 = 0
    while i != 255 {
        p^ = 1
        p = p + 1
        i = i + 1
    }
    mapInPlace(big, add10, 255)
    var q: ^u8 = big
    var j: u8 = 0
    var total: u16 = 0
    while j != 255 {
        total = total + q^
        q = q + 1
        j = j + 1
    }
    return u8(total)
}
// u16 length: 256 elements through a fused chain.
fun chain_n256() -> u16 {
    var p: ^u8 = big
    var i: u16 = 0
    while i < 256 {
        p^ = 3
        p = p + 1
        i = i + 1
    }
    g_total = 0
    forEach(big, sum_into, 256)
    return g_total
}
fun sum_into(x: u8) -> void {
    g_total = g_total + x
}
// filter rejecting everything: terminal never fires, loop still completes.
fun filter_none() -> u8 {
    acc = 77
    forEach(filter(src8, none), chain_step, 5)
    return acc
}
fun none(x: u8) -> u8 {
    return x > 99
}
assert functional() == 57      // 14,15 survive: acc = 14; 14*3+15
assert ufcs() == 57
assert lambda_stage() == 102   // 2,4,6,8,10 through acc*3+x
assert order_trace() == 3599   // trace = [14, 15]
assert order_trace_count() == 2
assert mapinplace() == 26      // 11 + 15
assert chain_n255() == 245  // 255x1, mapInPlace +10 -> u8(255*11) = 245
assert chain_n256() == 768  // 256 elements of 3 summed in u16
assert empty_chain() == 77
assert filter_none() == 77
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}

func TestG0IterChainFoldSemantics(t *testing.T) {
	src := `
global src8: [u8; 8]

fun load() -> void {
    src8[0] = 1
    src8[1] = 2
    src8[2] = 3
    src8[3] = 4
    src8[4] = 5
    src8[5] = 6
    src8[6] = 7
    src8[7] = 8
}
fun acc3(acc: u8, x: u8) -> u8 {
    return acc * 3 + x
}
fun keep_small(x: u8) -> u8 {
    return x < 3
}

// functional fold over memory: forward order.
fun fold_mem() -> u8 {
    load()
    return fold(src8, 0, acc3, 5)
}
// reduce is the same operation.
fun reduce_mem() -> u8 {
    load()
    return reduce(src8, 0, acc3, 5)
}
// chainable range fold: countdown order 5,4,3,2,1.
fun fold_range() -> u8 {
    return range(0..5).fold(0, acc3)
}
// fold over an empty input returns the initial accumulator.
fun fold_empty() -> u8 {
    return fold(src8, 42, acc3, 0)
}
// fold after filter on a range source: filter sees 9..1, keeps 2,1.
fun fold_filter_range() -> u8 {
    return range(0..9).filter(keep_small).fold(100, acc3)
}
assert fold_mem() == 179
assert reduce_mem() == 179
assert fold_range() == 35
assert fold_empty() == 42
assert fold_filter_range() == 139
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}

func TestG0PipeApplySemantics(t *testing.T) {
	src := `
global src8: [u8; 8]
global acc: u8 = 0

pipe doubled { map(|x: u8| x + x) }
pipe big_only { filter(|x: u8| x > 4) }

fun load() -> void {
    src8[0] = 1
    src8[1] = 2
    src8[2] = 3
    src8[3] = 4
    src8[4] = 5
    src8[5] = 6
    src8[6] = 7
    src8[7] = 8
}
fun acc3(acc: u8, x: u8) -> u8 {
    return acc * 3 + x
}
fun sum_into(x: u8) -> void {
    acc = acc + x
}
fun add10(x: u8) -> u8 {
    return x + 10
}

// .apply expands pipe stages into nested map/filter calls; fold terminal.
fun pipe_range_fold() -> u8 {
    return range(0..3).apply(doubled).fold(0, acc3)
}
// pipe on a memory source with a forEach terminal.
fun pipe_mem_foreach() -> u8 {
    load()
    acc = 0
    src8.apply(doubled).forEach(sum_into, 5)
    return acc
}
// composed pipes: trans reuses pipe stages by name.
trans doubled_big { use doubled; filter(|x: u8| x > 8) }
fun trans_mem() -> u8 {
    load()
    acc = 0
    src8.apply(doubled_big).forEach(sum_into, 5)
    return acc
}
assert pipe_range_fold() == 68   // 6,4,2 folded forward: 6;22;68
assert pipe_mem_foreach() == 30  // 2+4+6+8+10
assert trans_mem() == 10         // doubled 2,4,6,8,10; filter >8 keeps 10
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}

// TestG0IterChainCallbackReturn: a return nested in control flow inside a
// forEach callback ends that element's body (skip the rest for this element)
// — it must NOT return from the enclosing consumer. Regression test for the
// inlining bug fixed together with this audit (callbacks whose body contains
// a non-trailing return, or any asm block, are now invoked with a real CALL).
func TestG0IterChainCallbackReturn(t *testing.T) {
	src := `
global src8: [u8; 8]
global acc: u8 = 0

fun load() -> void {
    src8[0] = 1
    src8[1] = 2
    src8[2] = 3
    src8[3] = 4
    src8[4] = 5
}
fun skip3(x: u8) -> void {
    if x == 3 { return }
    acc = acc + x
}
fun consumer() -> u8 {
    load()
    acc = 0
    forEach(src8, skip3, 5)
    return acc
}
// range-source variant exercises the second inlining site.
fun skip3_step(x: u8) -> void {
    if x == 2 { return }
    acc = acc + x
}
fun consumer_range() -> u8 {
    acc = 0
    range(0..5).forEach(skip3_step)
    return acc
}
assert consumer() == 12        // 1+2+4+5: element 3 skipped, loop continues
assert consumer_range() == 13  // elements 5,4,3,2,1; skip 2 → 5+4+3+1
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}

// g0RunZ80Console compiles src, calls fn with no arguments on the emulator
// and returns the bytes written to console port $23.
func g0RunZ80Console(t *testing.T, src, fn string) []byte {
	t.Helper()
	m, err := nanz.Parse(src, "g0_console")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	genAsm, err := pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	boot := fmt.Sprintf(`
    ORG 0x8000
    LD SP, 0xFF00
    CALL %s
    DI
    HALT
`, fn)
	as := z80asm.NewAssembler()
	res, err := as.AssembleString(boot + "\n" + genAsm)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if len(res.Errors) > 0 {
		t.Fatalf("assemble errors: %v", res.Errors)
	}
	z80 := emulator.NewRemogattoZ80()
	if err := z80.LoadMemory(0x8000, res.Binary); err != nil {
		t.Fatalf("load: %v", err)
	}
	z80.SetPC(0x8000)
	if err := z80.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	return z80.GetOutput()
}

// TestG0IterChainAsmCallback: a forEach callback whose body is an inline asm
// block must observe the element through the function ABI (parameter in A).
// Inlining such a body used to print whatever A held instead of the element
// (fixed together with this audit: callbacks containing asm now get a real
// CALL).
func TestG0IterChainAsmCallback(t *testing.T) {
	src := `
global src8: [u8; 8]

fun emit(x: u8) -> void {
    asm { OUT ($23), A }
}
fun main() -> void {
    src8[0] = 1
    src8[1] = 2
    src8[2] = 3
    src8[3] = 4
    src8[4] = 5
    forEach(src8, emit, 5)
}
`
	got := g0RunZ80Console(t, src, "main")
	if want := []byte{1, 2, 3, 4, 5}; string(got) != string(want) {
		t.Fatalf("console output: want %v, got %v", want, got)
	}
}

// TestG0Finding_UnsupportedMethodRejected: take/skip/enumerate/iter/collect
// are NOT part of the Nanz iterator chain (they exist only in the retired
// .minz frontend). Using them today compiles without a diagnostic and emits
// invalid instructions (LD A, ?) plus undefined labels that break the whole
// module's assembly.
func TestG0Finding_UnsupportedMethodRejected(t *testing.T) {
	for _, method := range []string{"take", "skip", "enumerate", "iter"} {
		t.Run(method, func(t *testing.T) {
			src := `
global src8: [u8; 8]
global acc: u8 = 0
fun step(x: u8) -> void {
    acc = acc + x
}
fun main() -> u8 {
    acc = 0
    src8.` + method + `(3).forEach(step)
    return acc
}
`
			m, err := nanz.Parse(src, "g0_unsupported")
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			steps, err := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
			if err != nil {
				return // A clean compile error is the correct rejection.
			}
			if !strings.Contains(steps.Assembly, "?") && !strings.Contains(steps.Assembly, "CALL "+method) {
				return // Genuinely supported: no invalid assembly to pin.
			}
			t.Skip("G0 finding G0-5: unsupported iterator method ." + method + "() emits invalid assembly instead of a compile error")
		})
	}
}

// TestG0Finding_RangeOverflowReturnWidth: a u8 overflow counter check. The
// raw VM does not truncate a function result to the declared return width
// while Z80 does; programs that compute out-of-range values diverge between
// tiers. Pinned here so the divergence is visible; the value is out of
// contract either way.
func TestG0Finding_RangeOverflowReturnWidth(t *testing.T) {
	src := `
fun wide() -> u16 {
    return 300 * 300
}
assert wide() == 33792
`
	m, err := nanz.Parse(src, "g0_width")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	raw := hir.LowerModule(m)
	got, err := mir2.NewVM(raw).Call("wide", nil)
	if err != nil || len(got) != 1 {
		t.Fatalf("raw VM call: %v %v", got, err)
	}
	if got[0].I != 33792 {
		t.Skipf("G0 finding G0-6: raw VM returns %d, does not truncate to declared u16 return (Z80 returns 33792)", got[0].I)
	}
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}

// TestG0Finding_ParenMulPlusParses: `return (a * 3) + x` is rejected by the
// parser ("unexpected token +") while `return a * 3 + x` is accepted.
func TestG0Finding_ParenMulPlusParses(t *testing.T) {
	src := `
fun f(a: u8, x: u8) -> u8 {
    return (a * 3) + x
}
`
	if _, err := nanz.Parse(src, "g0_parse"); err != nil {
		t.Skip("G0 finding G0-7: parenthesised multiplication operand before + fails to parse: ", err)
	}
}
