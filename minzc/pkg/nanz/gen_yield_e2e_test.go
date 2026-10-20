package nanz_test

// G1 scalar gen/yield acceptance (ADR 0048/0049/0050, TASK G1a/G1b).
//
// The same scenarios the G0 oracle pinned for hand-written cursors
// (generator_contract_e2e_test.go) are run here against `gen`/`yield`
// declarations lowered to concrete caller-owned frames:
//
//   - creation stores arguments and runs no body effects (source reads are
//     observed through the peek probe);
//   - each request runs to the next yield or to completion; DONE is sticky
//     and leaves the output unchanged;
//   - two frames of one generator are independent; partial consumption
//     resumes the same frame;
//   - the `for x in gen(args)` form equals the manual pull form, with break,
//     continue and consumer return semantics;
//   - RLE with nested loops, several distinct yield sites, bool and u16
//     element types;
//   - the ADR 0050 demand example as a generator: filter_pos consumed by a
//     manual take(2) reads exactly the four needed source bytes.
//
// Expected values are derived from the Go reference model in the G0 oracle
// file at test time; the literal ADR 0050 pins are asserted independently.

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

const g1GenSrc = `
global data: [u8; 256]
global rle_in: [u8; 32]
global wdata: [u16; 4]
global g_pulls: u16 = 0
global g_reads: u16 = 0

// Source reads go through a probe so demand order is observable, mirroring
// the hand-written ByteCursor/RleCursor of the G0 oracle.
fun peek(p: ^u8) -> u8 {
    g_reads = g_reads + 1
    return p^
}

fun load_data(n: u8) -> void {
    for i in 0..n {
        data[i] = i * 37 + 11
    }
}

// G1a: the byte cursor as a generator.
gen bytes(src: ^u8, n: u8) -> u8 {
    while n != 0 {
        yield peek(src)
        src = src + 1
        n = n - 1
    }
}

// G1b: RLE over [count,value] pairs — nested loops, yield inside the inner
// loop, zero-length runs yield nothing (design report example).
gen run_bytes(src: ^u8, pairs: u8) -> u8 {
    while pairs != 0 {
        var left: u8 = peek(src)
        let value: u8 = peek(src + 1)
        src = src + 2
        pairs = pairs - 1
        while left != 0 {
            yield value
            left = left - 1
        }
    }
}

// G1b: three distinct yield sites — header, payload loop, trailer.
gen framed(src: ^u8, n: u8) -> u8 {
    yield 170
    while n != 0 {
        yield peek(src)
        src = src + 1
        n = n - 1
    }
    yield 153
}

// ADR 0050 demand example as a generator: only positive values are yielded,
// so a consumer that stops early stops the source reads with them.
gen filter_pos(src: ^u8, n: u8) -> u8 {
    while n != 0 {
        var v: u8 = peek(src)
        src = src + 1
        n = n - 1
        if v > 0 {
            yield v
        }
    }
}

// u16 elements and a frame-resident index local.
gen words(src: ^u16, n: u8) -> u16 {
    var i: u8 = 0
    while i < n {
        yield src[i]
        i = i + 1
    }
}

// bool elements with two yield sites.
gen flags(n: u8) -> bool {
    while n != 0 {
        if n > 2 {
            yield true
        } else {
            yield false
        }
        n = n - 1
    }
}

// A yield inside a switch arm.
gen classify(n: u8) -> u8 {
    var i: u8 = 0
    while i < n {
        switch i {
            case 0:
                yield 100
            case 1:
                yield 101
            default:
                yield 200
        }
        i = i + 1
    }
}

// ── byte cursor scenarios ─────────────────────────────────────────────────

fun sc_seq_mix(n: u8) -> u16 {
    load_data(n)
    var it = bytes(&data, n)
    var v: u8 = 0
    var mix: u16 = 0
    while pull_next(&it, &v) {
        mix = mix * 31 + v
    }
    return mix
}

fun sc_seq_count(n: u8) -> u16 {
    load_data(n)
    var it = bytes(&data, n)
    var v: u8 = 0
    var cnt: u16 = 0
    while pull_next(&it, &v) {
        cnt = cnt + 1
    }
    return cnt
}

fun sc_empty() -> u16 {
    var it = bytes(&data, 0)
    var v: u8 = 170
    var got: u8 = 1
    if !pull_next(&it, &v) { got = 0 }
    return got * 256 + v
}

fun sc_one() -> u16 {
    data[0] = 42
    var it = bytes(&data, 1)
    var v: u8 = 170
    var cnt: u8 = 0
    if pull_next(&it, &v) { cnt = 1 } else { cnt = 99 }
    var after: u8 = 55
    if pull_next(&it, &v) { after = 1 }
    return v * 256 + cnt * 16 + after
}

fun sc_sticky(n: u8) -> u16 {
    load_data(n)
    var it = bytes(&data, n)
    var v: u8 = 0
    while pull_next(&it, &v) { v = v }
    // A readable source must not revive a DONE frame.
    data[0] = 99
    v = 170
    var extra: u8 = 0
    if pull_next(&it, &v) { extra = 1 }
    if pull_next(&it, &v) { extra = 1 }
    return extra * 256 + v
}

fun sc_interleave(n: u8) -> u16 {
    load_data(n)
    var it1 = bytes(&data, n)
    var it2 = bytes(&data, n)
    var v: u8 = 0
    var mix: u16 = 0
    var a1: u8 = 1
    var a2: u8 = 1
    while (a1 != 0) || (a2 != 0) {
        if a1 != 0 {
            if pull_next(&it1, &v) { mix = mix * 31 + v } else { a1 = 0 }
        }
        if a2 != 0 {
            if pull_next(&it2, &v) { mix = mix * 31 + v } else { a2 = 0 }
        }
    }
    return mix
}

fun sc_partial(n: u8) -> u16 {
    load_data(n)
    var it = bytes(&data, n)
    var v: u8 = 0
    var mix: u16 = 0
    var i: u8 = 0
    while i < 3 {
        if !pull_next(&it, &v) { break }
        mix = mix * 31 + v
        i = i + 1
    }
    while pull_next(&it, &v) {
        mix = mix * 31 + v
    }
    return mix
}

// Creation stores arguments only: no source reads, no pulls, and the first
// request observes source changes made after creation.
fun sc_make_no_effects(n: u8) -> u16 {
    load_data(n)
    g_reads = 13
    var it = bytes(&data, n)
    if g_reads != 13 { return 1 }
    if data[0] != 11 { return 2 }
    data[0] = 99
    var v: u8 = 170
    if !pull_next(&it, &v) { return 3 }
    if v != 99 { return 4 }
    if g_reads != 14 { return 5 }
    return 0
}

// ── for-form scenarios ────────────────────────────────────────────────────

fun sc_for_mix(n: u8) -> u16 {
    load_data(n)
    var mix: u16 = 0
    for v in bytes(&data, n) {
        mix = mix * 31 + v
    }
    return mix
}

fun sc_for_empty() -> u16 {
    var cnt: u8 = 0
    for v in bytes(&data, 0) {
        cnt = cnt + 1
    }
    return cnt
}

// break stops consumption without further source reads.
fun sc_for_break() -> u16 {
    load_data(8)
    g_reads = 0
    var cnt: u8 = 0
    for v in bytes(&data, 8) {
        cnt = cnt + 1
        if cnt == 3 { break }
    }
    return g_reads * 256 + cnt
}

// continue requests the next element.
fun sc_for_continue(n: u8) -> u16 {
    load_data(n)
    var mix: u16 = 0
    var skipped: u8 = 0
    for v in bytes(&data, n) {
        if (v & 1) == 0 {
            skipped = skipped + 1
            continue
        }
        mix = mix * 31 + v
    }
    return mix
}

// return in the consumer body returns from the consumer function.
fun sc_for_return(n: u8) -> u16 {
    load_data(n)
    for v in bytes(&data, n) {
        if v == 85 { return 1 }
    }
    return 0
}

// A temporary frame is re-created for every iteration of an outer loop.
fun sc_for_in_loop(k: u8) -> u16 {
    load_data(4)
    var mix: u16 = 0
    var round: u8 = 0
    while round < k {
        for v in bytes(&data, 4) {
            mix = mix * 31 + v
        }
        round = round + 1
    }
    return mix
}

// ── RLE scenarios (G1b) ───────────────────────────────────────────────────

fun load_rle_example() -> void {
    rle_in[0] = 3
    rle_in[1] = 65
    rle_in[2] = 0
    rle_in[3] = 90
    rle_in[4] = 2
    rle_in[5] = 66
}

fun sc_rle() -> u16 {
    load_rle_example()
    var it = run_bytes(&rle_in, 3)
    var v: u8 = 0
    var mix: u16 = 0
    while pull_next(&it, &v) {
        mix = mix * 31 + v
    }
    return mix
}

fun sc_rle_for() -> u16 {
    load_rle_example()
    var mix: u16 = 0
    for v in run_bytes(&rle_in, 3) {
        mix = mix * 31 + v
    }
    return mix
}

fun load_rle_255() -> void {
    var i: u8 = 0
    while i < 5 {
        rle_in[i * 2] = 51
        rle_in[i * 2 + 1] = 7
        i = i + 1
    }
}

fun sc_rle_255() -> u16 {
    load_rle_255()
    var it = run_bytes(&rle_in, 5)
    var v: u8 = 0
    var mix: u16 = 0
    while pull_next(&it, &v) {
        mix = mix * 31 + v
    }
    return mix
}

fun sc_rle_255_cnt() -> u16 {
    load_rle_255()
    var it = run_bytes(&rle_in, 5)
    var v: u8 = 0
    var cnt: u16 = 0
    while pull_next(&it, &v) {
        cnt = cnt + 1
    }
    return cnt
}

fun sc_rle_sticky() -> u16 {
    rle_in[0] = 2
    rle_in[1] = 9
    var it = run_bytes(&rle_in, 1)
    var v: u8 = 0
    while pull_next(&it, &v) { v = v }
    // Fresh source pairs must be ignored after DONE.
    rle_in[2] = 1
    rle_in[3] = 42
    v = 170
    var extra: u8 = 0
    if pull_next(&it, &v) { extra = 1 }
    if pull_next(&it, &v) { extra = 1 }
    return extra * 256 + v
}

// Two independent RLE frames over the same source, pulled alternately.
fun sc_rle_interleave() -> u16 {
    load_rle_example()
    var it1 = run_bytes(&rle_in, 3)
    var it2 = run_bytes(&rle_in, 3)
    var v: u8 = 0
    var mix: u16 = 0
    var a1: u8 = 1
    var a2: u8 = 1
    while (a1 != 0) || (a2 != 0) {
        if a1 != 0 {
            if pull_next(&it1, &v) { mix = mix * 31 + v } else { a1 = 0 }
        }
        if a2 != 0 {
            if pull_next(&it2, &v) { mix = mix * 31 + v } else { a2 = 0 }
        }
    }
    return mix
}

// ── multiple yield sites (G1b) ────────────────────────────────────────────

fun sc_framed(n: u8) -> u16 {
    load_data(n)
    var it = framed(&data, n)
    var v: u8 = 0
    var mix: u16 = 0
    while pull_next(&it, &v) {
        mix = mix * 31 + v
    }
    return mix
}

fun sc_framed_empty() -> u16 {
    var it = framed(&data, 0)
    var v: u8 = 0
    var mix: u16 = 0
    while pull_next(&it, &v) {
        mix = mix * 31 + v
    }
    return mix
}

// ── ADR 0050 demand example as a generator (G1b) ──────────────────────────

fun sc_filter_take2() -> u16 {
    data[0] = 0
    data[1] = 0
    data[2] = 7
    data[3] = 8
    data[4] = 9
    g_reads = 0
    var it = filter_pos(&data, 5)
    var v: u8 = 170
    var got: u8 = 0
    var acc: u16 = 0
    while got < 2 {
        if !pull_next(&it, &v) { break }
        if v > 0 {
            acc = acc * 256 + v
            got = got + 1
        }
    }
    return g_reads * 4096 + acc
}

fun sc_filter_take3() -> u16 {
    data[0] = 0
    data[1] = 0
    data[2] = 7
    data[3] = 8
    data[4] = 9
    g_reads = 0
    var it = filter_pos(&data, 5)
    var v: u8 = 170
    var got: u8 = 0
    var acc: u16 = 0
    while got < 3 {
        if !pull_next(&it, &v) { break }
        if v > 0 {
            acc = acc * 256 + v
            got = got + 1
        }
    }
    return g_reads * 4096 + acc
}

// ── element types (G1b) ───────────────────────────────────────────────────

fun sc_words() -> u16 {
    wdata[0] = 1000
    wdata[1] = 37
    wdata[2] = 65535
    wdata[3] = 7
    var it = words(&wdata, 4)
    var v: u16 = 0
    var mix: u16 = 0
    while pull_next(&it, &v) {
        mix = mix * 31 + v
    }
    return mix
}

fun sc_words_for() -> u16 {
    wdata[0] = 1000
    wdata[1] = 37
    wdata[2] = 65535
    wdata[3] = 7
    var mix: u16 = 0
    for v in words(&wdata, 4) {
        mix = mix + v
    }
    return mix
}

fun sc_flags() -> u16 {
    var it = flags(5)
    var b: bool = false
    var trues: u8 = 0
    var falses: u8 = 0
    while pull_next(&it, &b) {
        if b { trues = trues + 1 } else { falses = falses + 1 }
    }
    return trues * 256 + falses
}

fun sc_flags_for() -> u16 {
    var trues: u8 = 0
    var falses: u8 = 0
    for b in flags(5) {
        if b { trues = trues + 1 } else { falses = falses + 1 }
    }
    return trues * 256 + falses
}

fun sc_classify() -> u16 {
    var it = classify(4)
    var v: u8 = 0
    var mix: u16 = 0
    while pull_next(&it, &v) {
        mix = mix * 31 + v
    }
    return mix
}

// A frame created inside an if branch is scoped like any other local.
fun sc_conditional(c: u8) -> u16 {
    load_data(4)
    var mix: u16 = 0
    if c != 0 {
        var it = bytes(&data, 4)
        var v: u8 = 0
        while pull_next(&it, &v) {
            mix = mix * 31 + v
        }
    } else {
        mix = 7
    }
    return mix
}

// Nested consumption: one generator drained inside the loop of another.
fun sc_nested() -> u16 {
    load_data(4)
    var outer_mix: u16 = 0
    for o in bytes(&data, 4) {
        for i in bytes(&data, 2) {
            outer_mix = outer_mix + o * i
        }
    }
    return outer_mix
}
`

type g1Scenario struct {
	name string
	fn   string
	args []int64
	want int64
}

// g1GenScenarios derives expectations from the G0 Go reference model.
func g1GenScenarios() []g1Scenario {
	var sc []g1Scenario
	add := func(name, fn string, want int64, args ...int64) {
		sc = append(sc, g1Scenario{name: name, fn: fn, args: args, want: want})
	}

	// Full forward decode; the single-element and empty cases are pinned
	// separately below.
	for _, n := range []int{0, 1, 8, 255} {
		want := int64(g0MixSeq(g0RefData(n)))
		add("seq_mix", "sc_seq_mix", want, int64(n))
		add("seq_count", "sc_seq_count", int64(n), int64(n))
		add("for_mix", "sc_for_mix", want, int64(n))
	}

	// Empty: DONE on the first request, out keeps sentinel 0xAA.
	add("empty", "sc_empty", 0xAA)
	add("for_empty", "sc_for_empty", 0)

	// Single element then DONE, out preserved.
	add("one", "sc_one", 42*256+1*16+55)

	// Sticky DONE against a mutated readable source.
	for _, n := range []int{8, 255} {
		add("sticky", "sc_sticky", 0xAA, int64(n))
	}

	// Two independent frames pulled alternately.
	for _, n := range []int{8, 255} {
		d := g0RefData(n)
		var mix uint16
		var v byte
		half := make([]byte, 0, 2*n)
		for i := 0; i < n; i++ {
			half = append(half, d[i], d[i])
		}
		for _, b := range half {
			mix = mix*31 + uint16(b)
		}
		_ = v
		add("interleave", "sc_interleave", int64(mix), int64(n))
	}

	// Partial consumption then resume equals a full decode.
	for _, n := range []int{0, 1, 3, 8, 255} {
		add("partial", "sc_partial", int64(g0MixSeq(g0RefData(n))), int64(n))
	}

	// Creation issues no source reads; the first pull sees later mutations.
	add("make_no_effects", "sc_make_no_effects", 0, 8)

	// break stops consumption without further source reads: exactly three
	// reads for three accepted items.
	add("for_break", "sc_for_break", 3*256+3)

	// continue requests the next element: mix over the odd values only.
	for _, n := range []int{8, 255} {
		var odd []byte
		for _, b := range g0RefData(n) {
			if b%2 == 1 {
				odd = append(odd, b)
			}
		}
		add("for_continue", "sc_for_continue", int64(g0MixSeq(odd)), int64(n))
	}

	// Consumer return leaves the consumer: data[2] == 85 for load_data(8).
	add("for_return", "sc_for_return", 1, 8)

	// A temporary frame per outer-loop iteration: k full decodes folded.
	for _, k := range []int{1, 3} {
		d := g0RefData(4)
		var mix uint16
		for i := 0; i < k; i++ {
			mix = int642Mix(mix, d)
		}
		add("for_in_loop", "sc_for_in_loop", int64(mix), int64(k))
	}

	// RLE design-report example: [3,65, 0,90, 2,66] → 65,65,65,66,66.
	rleWant := int64(g0MixSeq([]byte{65, 65, 65, 66, 66}))
	add("rle", "sc_rle", rleWant)
	add("rle_for", "sc_rle_for", rleWant)

	// RLE with exactly 255 outputs.
	var c g0RefRleCursor
	pairs := []byte{51, 7, 51, 7, 51, 7, 51, 7, 51, 7}
	c.make(pairs)
	out := g0RefDecodeAll(c.next)
	add("rle_255_mix", "sc_rle_255", int64(g0MixSeq(out)))
	add("rle_255_cnt", "sc_rle_255_cnt", int64(len(out)))

	// RLE sticky DONE with fresh source pairs available.
	add("rle_sticky", "sc_rle_sticky", 0xAA)

	// Two independent RLE frames over the same source.
	rle2 := []byte{65, 65, 65, 66, 66}
	var mix2 uint16
	inter := make([]byte, 0, 2*len(rle2))
	for _, b := range rle2 {
		inter = append(inter, b, b)
	}
	for _, b := range inter {
		mix2 = mix2*31 + uint16(b)
	}
	add("rle_interleave", "sc_rle_interleave", int64(mix2))

	// Header/payload/trailer: [0xAA] + data + [0x99].
	for _, n := range []int{0, 1, 8} {
		seq := append([]byte{0xAA}, g0RefData(n)...)
		seq = append(seq, 0x99)
		add("framed", "sc_framed", int64(g0MixSeq(seq)), int64(n))
	}
	add("framed_empty", "sc_framed_empty", int64(g0MixSeq([]byte{0xAA, 0x99})))

	// ADR 0050 demand example as a generator: take(2) over [0,0,7,8,9]
	// accepts [7,8] having read exactly the four needed source bytes.
	take2 := uint16(7)*256 + 8
	take3 := take2*256 + 9
	add("filter_take2", "sc_filter_take2", 4*4096+int64(take2))
	add("filter_take3", "sc_filter_take3", 5*4096+int64(take3))

	// u16 elements.
	w := []uint16{1000, 37, 65535, 7}
	var wmix uint16
	for _, x := range w {
		wmix = wmix*31 + x
	}
	add("words", "sc_words", int64(wmix))
	wordSum := uint16(0)
	for _, x := range w {
		wordSum += x
	}
	add("words_for", "sc_words_for", int64(wordSum))

	// bool elements: flags(5) yields true,true,true,false,false.
	add("flags", "sc_flags", 3*256+2)
	add("flags_for", "sc_flags_for", 3*256+2)

	// A yield inside a switch arm: 100, 101, 200, 200.
	add("classify", "sc_classify", int64(g0MixSeq([]byte{100, 101, 200, 200})))

	// A frame created inside a branch, drained only on that branch.
	add("conditional_taken", "sc_conditional", int64(g0MixSeq(g0RefData(4))), 1)
	add("conditional_not_taken", "sc_conditional", 7, 0)

	// Nested for-loops over independent temporary frames: for each of the 4
	// outer elements, the inner 2-element decode. `o * i` is u8 arithmetic in
	// Nanz, so the reference wraps each product mod 256.
	var nested uint16
	inner := g0RefData(2)
	for _, o := range g0RefData(4) {
		for _, i := range inner {
			nested += uint16(byte(uint16(o) * uint16(i)))
		}
	}
	add("nested", "sc_nested", int64(nested))

	return sc
}

// int642Mix folds one more pass of d into mix (used by for_in_loop).
func int642Mix(mix uint16, d []byte) uint16 {
	for _, b := range d {
		mix = mix*31 + uint16(b)
	}
	return mix
}

// TestG1GenYield runs every scenario on the raw MIR2 VM, the optimised MIR2
// VM and the assembled Z80 program.
func TestG1GenYield(t *testing.T) {
	scenarios := g1GenScenarios()

	parseWithAsserts := func() *hir.Module {
		m, err := nanz.Parse(g1GenSrc, "g1_gen_yield")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		for _, s := range scenarios {
			m.Asserts = append(m.Asserts, hir.Assert{
				FuncName: s.fn,
				Args:     s.args,
				Expected: s.want,
				Source:   "G1 scenario " + s.name,
			})
		}
		return m
	}

	rawHM := parseWithAsserts()
	raw := hir.LowerModule(rawHM)
	if err := pipeline.RunAssertsMIR2(rawHM, raw); err != nil {
		t.Errorf("raw MIR2 VM: %v", err)
	}

	hm := parseWithAsserts()
	steps, err := pipeline.CompileHIRSteps(hm, pipeline.DefaultOptions())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if err := pipeline.RunAssertsMIR2(hm, steps.MIR2Module); err != nil {
		t.Errorf("optimised MIR2 VM: %v", err)
	}
	if err := pipeline.RunAssertsZ80(hm, steps.MIR2Module, steps.Allocation, steps.Assembly); err != nil {
		t.Errorf("z80: %v", err)
	}
}

// g1NegativeCases are out-of-scope constructs that must be rejected with a
// compile error (not a panic and not invalid assembly).
func g1NegativeCases() map[string]string {
	return map[string]string{
		"yield outside gen": `
fun f(n: u8) -> u8 {
    yield n
    return n
}`,
		"yield pointer type": `
gen g(p: ^u8) -> ^u8 {
    yield p
}`,
		"yield struct type": `
struct S { a: u8 }
gen g() -> S {
    yield S { a: 1 }
}`,
		"yield type mismatch": `
gen g() -> u8 {
    yield 300
}`,
		"copy frame": `
gen g() -> u8 {
    yield 1
}
fun f() -> u8 {
    var it = g()
    var jt = it
    return 0
}`,
		"return frame": `
gen g() -> u8 {
    yield 1
}
fun f() -> u8 {
    var it = g()
    return it
}`,
		"frame address escape": `
gen g() -> u8 {
    yield 1
}
fun f() -> u8 {
    var it = g()
    var p: ptr = &it
    return 0
}`,
		"frame as value": `
gen g() -> u8 {
    yield 1
}
fun f() -> u8 {
    var it = g()
    return it.tag
}`,
		"yield in lambda": `
gen g(n: u8) -> u8 {
    var f = |x: u8| { yield x }
    return 0
}`,
		"lambda in gen": `
fun dbl(x: u8) -> u8 { return x * 2 }
gen g(n: u8) -> u8 {
    var f = |x: u8| { return dbl(x) }
    yield 1
}`,
		"gen called as function": `
gen g() -> u8 {
    yield 1
}
fun f() -> u8 {
    return g()
}`,
		"for over frame variable": `
gen g() -> u8 {
    yield 1
}
fun f() -> u8 {
    var it = g()
    var c: u8 = 0
    for v in it {
        c = c + 1
    }
    return c
}`,
		"pull_next on non-frame": `
fun f() -> u8 {
    var v: u8 = 3
    var w: u8 = 0
    while pull_next(&v, &w) { }
    return w
}`,
		"pull_next output type mismatch": `
gen g() -> u16 {
    yield 1
}
fun f() -> u8 {
    var it = g()
    var w: u8 = 0
    while pull_next(&it, &w) { }
    return w
}`,
		"asm in gen body": `
gen g() -> u8 {
    yield 1
    asm z80 { NOP }
}`,
		"valued return in gen": `
gen g(n: u8) -> u8 {
    if n == 0 {
        return 5
    }
    yield 1
}`,
		"yield inside for": `
gen g(n: u8) -> u8 {
    for i in 0..n {
        yield i
    }
}`,
		"struct local in gen": `
struct S { a: u8 }
gen g(n: u8) -> u8 {
    var s: S
    yield n
}`,
		"address of gen local": `
gen g(n: u8, out: ^u8) -> u8 {
    var t: u8 = n
    out^ = t
    yield &n
}`,
		"duplicate gen local": `
gen g(n: u8) -> u8 {
    var t: u8 = 1
    yield t
    var t: u8 = 2
    yield t
}`,
		"fallible call in gen": `
fun fetch?(p: ^u8) -> u8 {
    return p^
}
gen g(p: ^u8) -> u8 {
    yield fetch(p)
}`,
		"use before declaration": `
fun f() -> u8 {
    var it = later()
    return 0
}
gen later() -> u8 {
    yield 1
}`,
		"gen consumed inside gen": `
gen inner() -> u8 {
    yield 1
}
gen outer() -> u8 {
    for v in inner() { }
    yield 2
}`,
		"for-var shadows parameter": `
gen g(n: u8) -> u8 {
    var s: u8 = 0
    for n in 0..3 { s = s + n }
    yield s
}`,
		"for-var collides with local": `
gen g() -> u8 {
    var t: u8 = 5
    yield t
    for t in 0..2 { }
    yield t
}`,
		"local collides with later for-var": `
gen g() -> u8 {
    for i in 0..2 { }
    var i: u8 = 1
    yield i
}`,
		"gen call in condition expression": `
gen g() -> u8 {
    yield 1
}
fun f(c: bool) -> u8 {
    var x: u8 = if c { 2 } else { g() }
    return x
}`,
		"frame redeclared then pulled": `
gen g() -> u8 {
    yield 1
}
fun f() -> u8 {
    var it = g()
    var it: u8 = 7
    var v: u8 = 0
    while pull_next(&it, &v) { }
    return v
}`,
	}
}

func TestG1GenYieldNegative(t *testing.T) {
	cases := g1NegativeCases()
	// Continuation tags share one u8 with the sticky DONE marker: a body with
	// more than 254 yield sites must be rejected, not silently miscompiled.
	cases["too many yield sites"] = "fun f() -> u8 { return 1 }\ngen g() -> u8 {\n" +
		strings.Repeat("    yield 1\n", 255) + "}\n"
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := nanz.Parse(src, "g1_negative")
			if err == nil {
				t.Fatalf("expected a compile error, parsed cleanly")
			}
			if strings.Contains(err.Error(), "panic") {
				t.Fatalf("error mentions panic: %v", err)
			}
			for _, frag := range []string{"goroutine", "invalid instruction"} {
				if strings.Contains(err.Error(), frag) {
					t.Fatalf("error looks like a crash: %v", err)
				}
			}
		})
	}
}
