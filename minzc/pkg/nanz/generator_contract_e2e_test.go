package nanz_test

// G0 generator-contract scaffold (ADR 0048/0049/0050).
//
// This file is the executable oracle for the proposed Nanz pull protocol
// `next(frame, out) -> bool`. It contains:
//
//   - a Go reference model of the protocol (sticky DONE, out unchanged on
//     DONE, independent instances, creation without effects, partial
//     consumption, and the ADR 0050 filter(p).take(k) demand example on
//     input [0,0,7,8,9]);
//   - the same two cursors hand-written in Nanz today as explicit state
//     machines (a byte cursor and an RLE decoder over a byte buffer of
//     [count,value] pairs) — no `gen`/`yield` syntax exists yet;
//   - scenario functions whose observable results are compared against the
//     Go reference on three tiers: raw MIR2 VM, optimised MIR2 VM, and the
//     assembled Z80 program running on the emulator.
//
// When G1 lowers `gen`/`yield` to concrete frames, its generated cursors must
// reproduce these scenario results bit-for-bit; this file then becomes the
// differential gate between generated and manual implementations.

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

// ── Go reference model of the pull protocol (ADR 0049) ───────────────────────

// g0RefByteCursor is the reference byte cursor: next publishes one element and
// advances; false is permanent completion and leaves *out untouched.
type g0RefByteCursor struct {
	data []byte
	pos  int
	done bool
}

func (c *g0RefByteCursor) make(data []byte) {
	c.data, c.pos, c.done = data, 0, false
}

func (c *g0RefByteCursor) next(out *byte) bool {
	if c.done {
		return false
	}
	if c.pos >= len(c.data) {
		c.done = true
		return false
	}
	*out = c.data[c.pos]
	c.pos++
	return true
}

// g0RefRleCursor is the reference RLE decoder over [count,value] pairs with a
// nested pull loop (a zero-length run yields nothing).
type g0RefRleCursor struct {
	pairs []byte
	left  int
	val   byte
	done  bool
}

func (c *g0RefRleCursor) make(pairs []byte) {
	c.pairs, c.left, c.val, c.done = pairs, 0, 0, false
}

func (c *g0RefRleCursor) next(out *byte) bool {
	if c.done {
		return false
	}
	for c.left == 0 {
		if len(c.pairs) == 0 {
			c.done = true
			return false
		}
		c.left, c.val = int(c.pairs[0]), c.pairs[1]
		c.pairs = c.pairs[2:]
	}
	*out = c.val
	c.left--
	return true
}

// g0RefFilterTake is the ADR 0050 demand example: pull the source until k
// elements satisfy pred. It returns the accepted outputs and the number of
// source requests issued. For source [0,0,7,8,9], pred x>0, k=2 the outputs
// are [7,8] and exactly 4 requests are made — 9 is never requested.
func g0RefFilterTake(src []byte, pred func(byte) bool, k int) (outs []byte, pulls int) {
	var c g0RefByteCursor
	c.make(src)
	var v byte = 0xAA
	for len(outs) < k {
		pulls++
		if !c.next(&v) {
			break
		}
		if pred(v) {
			outs = append(outs, v)
		}
	}
	return outs, pulls
}

// g0MixSeq folds values in pull order exactly as the Nanz scenarios do:
// mix = mix*31 + v with u16 wrap-around.
func g0MixSeq(values []byte) uint16 {
	var mix uint16
	for _, v := range values {
		mix = mix*31 + uint16(v)
	}
	return mix
}

// g0RefData mirrors the Nanz load_data pattern: data[i] = (i*37 + 11) & 0xFF.
func g0RefData(n int) []byte {
	d := make([]byte, n)
	for i := range d {
		d[i] = byte((i*37 + 11) & 0xFF)
	}
	return d
}

// g0RefDecodeAll drains a reference cursor into a slice.
func g0RefDecodeAll(next func(*byte) bool) []byte {
	var out []byte
	var v byte = 0xAA
	for next(&v) {
		out = append(out, v)
	}
	return out
}

// ── Nanz manual cursors (the implementation under test) ─────────────────────

const g0ContractSrc = `
struct ByteCursor {
    pos: ^u8,
    left: u8,
    done: u8,
}

struct RleCursor {
    src: ^u8,
    pairs: u8,
    left: u8,
    val: u8,
    done: u8,
}

global data: [u8; 256]
global rle_in: [u8; 32]
global bc1: ByteCursor
global bc2: ByteCursor
global rc: RleCursor
global g_pulls: u16 = 0

// Creation only writes the frame: it reads no source bytes and issues no
// pulls (observed via g_pulls in sc_make_no_effects).
fun bc_make(cur: ^ByteCursor, src: ^u8, n: u8) -> void {
    cur.pos = src
    cur.left = n
    cur.done = 0
}

// next(frame, out) -> bool. false is sticky (done flag) and leaves out
// untouched. g_pulls counts every request for demand-order scenarios.
fun bc_next(cur: ^ByteCursor, out: ^u8) -> u8 {
    g_pulls = g_pulls + 1
    if cur.done != 0 { return 0 }
    if cur.left == 0 {
        cur.done = 1
        return 0
    }
    out^ = cur.pos^
    cur.pos = cur.pos + 1
    cur.left = cur.left - 1
    return 1
}

fun rle_make(cur: ^RleCursor, src: ^u8, pairs: u8) -> void {
    cur.src = src
    cur.pairs = pairs
    cur.left = 0
    cur.val = 0
    cur.done = 0
}

// Nested-loop pull: refill the current run from the next [count,value] pair.
// Zero-length runs yield nothing.
fun rle_next(cur: ^RleCursor, out: ^u8) -> u8 {
    if cur.done != 0 { return 0 }
    while cur.left == 0 {
        if cur.pairs == 0 {
            cur.done = 1
            return 0
        }
        cur.left = cur.src[0]
        cur.val = cur.src[1]
        cur.src = cur.src + 2
        cur.pairs = cur.pairs - 1
    }
    out^ = cur.val
    cur.left = cur.left - 1
    return 1
}

fun load_data(n: u8) -> void {
    for i in 0..n {
        data[i] = i * 37 + 11
    }
}

// S1: full forward decode of n elements (mix in pull order).
fun sc_seq_mix(n: u8) -> u16 {
    load_data(n)
    bc_make(&bc1, &data, n)
    var v: u8 = 0
    var mix: u16 = 0
    while bc_next(&bc1, &v) != 0 {
        mix = mix * 31 + v
    }
    return mix
}

// S2: element count of a full decode (n=255 crosses the u8 counter boundary).
fun sc_seq_count(n: u8) -> u16 {
    load_data(n)
    bc_make(&bc1, &data, n)
    var v: u8 = 0
    var cnt: u16 = 0
    while bc_next(&bc1, &v) != 0 {
        cnt = cnt + 1
    }
    return cnt
}

// S3: empty input — first request reports DONE and leaves out unchanged.
fun sc_empty() -> u16 {
    bc_make(&bc1, &data, 0)
    var v: u8 = 170
    var got: u8 = 1
    if bc_next(&bc1, &v) == 0 { got = 0 }
    return got * 256 + v
}

// S4: single element, then DONE.
fun sc_one() -> u16 {
    data[0] = 42
    bc_make(&bc1, &data, 1)
    var v: u8 = 170
    var cnt: u8 = 0
    if bc_next(&bc1, &v) != 0 {
        cnt = 1
    } else {
        cnt = 99
    }
    var after: u8 = 55
    if bc_next(&bc1, &v) != 0 {
        after = 1
    }
    return v * 256 + cnt * 16 + after
}

// S5: sticky DONE — after exhaustion two further requests both report DONE
// with no effects and out keeps its sentinel value.
fun sc_sticky(n: u8) -> u16 {
    load_data(n)
    bc_make(&bc1, &data, n)
    var v: u8 = 0
    while bc_next(&bc1, &v) != 0 { v = v }
    v = 170
    var extra: u8 = 0
    if bc_next(&bc1, &v) != 0 { extra = 1 }
    if bc_next(&bc1, &v) != 0 { extra = 1 }
    return extra * 256 + v
}

// S6: two independent instances over the same source, pulled alternately.
fun sc_interleave(n: u8) -> u16 {
    load_data(n)
    bc_make(&bc1, &data, n)
    bc_make(&bc2, &data, n)
    var v: u8 = 0
    var mix: u16 = 0
    var a1: u8 = 1
    var a2: u8 = 1
    while (a1 != 0) || (a2 != 0) {
        if a1 != 0 {
            if bc_next(&bc1, &v) != 0 { mix = mix * 31 + v } else { a1 = 0 }
        }
        if a2 != 0 {
            if bc_next(&bc2, &v) != 0 { mix = mix * 31 + v } else { a2 = 0 }
        }
    }
    return mix
}

// S7: partial consumption — pull 3, abandon the loop, resume the same frame.
fun sc_partial(n: u8) -> u16 {
    load_data(n)
    bc_make(&bc1, &data, n)
    var v: u8 = 0
    var mix: u16 = 0
    var i: u8 = 0
    while i < 3 {
        if bc_next(&bc1, &v) == 0 { break }
        mix = mix * 31 + v
        i = i + 1
    }
    while bc_next(&bc1, &v) != 0 {
        mix = mix * 31 + v
    }
    return mix
}

// S8: RLE decode — the design-report example [3,65, 0,90, 2,66] expands to
// 65,65,65,66,66 (the zero-length run contributes nothing).
fun sc_rle() -> u16 {
    rle_in[0] = 3
    rle_in[1] = 65
    rle_in[2] = 0
    rle_in[3] = 90
    rle_in[4] = 2
    rle_in[5] = 66
    rle_make(&rc, &rle_in, 3)
    var v: u8 = 0
    var mix: u16 = 0
    while rle_next(&rc, &v) != 0 {
        mix = mix * 31 + v
    }
    return mix
}

// S9: RLE total output of exactly 255 elements (u8 boundary).
fun sc_rle_255() -> u16 {
    var i: u8 = 0
    while i < 5 {
        rle_in[i * 2] = 51
        rle_in[i * 2 + 1] = 7
        i = i + 1
    }
    rle_make(&rc, &rle_in, 5)
    var v: u8 = 0
    var mix: u16 = 0
    var cnt: u16 = 0
    while rle_next(&rc, &v) != 0 {
        mix = mix * 31 + v
        cnt = cnt + 1
    }
    return mix
}

fun sc_rle_255_cnt() -> u16 {
    var i: u8 = 0
    while i < 5 {
        rle_in[i * 2] = 51
        rle_in[i * 2 + 1] = 7
        i = i + 1
    }
    rle_make(&rc, &rle_in, 5)
    var v: u8 = 0
    var cnt: u16 = 0
    while rle_next(&rc, &v) != 0 {
        cnt = cnt + 1
    }
    return cnt
}

// S10: RLE sticky DONE with out preserved.
fun sc_rle_sticky() -> u16 {
    rle_in[0] = 2
    rle_in[1] = 9
    rle_make(&rc, &rle_in, 1)
    var v: u8 = 0
    while rle_next(&rc, &v) != 0 { v = v }
    v = 170
    var extra: u8 = 0
    if rle_next(&rc, &v) != 0 { extra = 1 }
    if rle_next(&rc, &v) != 0 { extra = 1 }
    return extra * 256 + v
}

// S11: ADR 0050 demand example — filter(x>0).take(2) over [0,0,7,8,9]:
// outputs 7,8 and exactly 4 source requests; 9 is never requested.
fun sc_filter_take2() -> u16 {
    data[0] = 0
    data[1] = 0
    data[2] = 7
    data[3] = 8
    data[4] = 9
    g_pulls = 0
    bc_make(&bc1, &data, 5)
    var v: u8 = 170
    var got: u8 = 0
    var acc: u16 = 0
    while got < 2 {
        if bc_next(&bc1, &v) == 0 { break }
        if v > 0 {
            acc = acc * 256 + v
            got = got + 1
        }
    }
    return g_pulls * 4096 + acc
}

// S12: k=3 exhausts the source first: outputs stay [7,8], requests reach 5.
fun sc_filter_take3() -> u16 {
    data[0] = 0
    data[1] = 0
    data[2] = 7
    data[3] = 8
    data[4] = 9
    g_pulls = 0
    bc_make(&bc1, &data, 5)
    var v: u8 = 170
    var got: u8 = 0
    var acc: u16 = 0
    while got < 3 {
        if bc_next(&bc1, &v) == 0 { break }
        if v > 0 {
            acc = acc * 256 + v
            got = got + 1
        }
    }
    return g_pulls * 4096 + acc
}

// S13: creation issues no requests (no body effects at construction time).
fun sc_make_no_effects(n: u8) -> u16 {
    g_pulls = 0
    load_data(n)
    bc_make(&bc1, &data, n)
    return g_pulls
}
`

type g0Scenario struct {
	name string
	fn   string
	args []int64
	want int64
}

// g0ContractScenarios computes every expected value from the Go reference so
// the Nanz numbers are never hand-baked.
func g0ContractScenarios() []g0Scenario {
	var sc []g0Scenario
	add := func(name, fn string, want int64, args ...int64) {
		sc = append(sc, g0Scenario{name: name, fn: fn, args: args, want: want})
	}

	// S1/S2 full decode (the single-element case is S4 below).
	for _, n := range []int{8, 255} {
		want := int64(g0MixSeq(g0RefData(n)))
		add("seq_mix", "sc_seq_mix", want, int64(n))
		add("seq_count", "sc_seq_count", int64(n), int64(n))
	}

	// S3 empty: DONE on first request, out keeps sentinel 0xAA.
	add("empty", "sc_empty", 0xAA)

	// S4 single: element 42, then DONE leaves out at 42 and the after-flag
	// at its sentinel 55 (no rewrite on DONE).
	add("one", "sc_one", 42*256+1*16+55)

	// S5 sticky DONE.
	for _, n := range []int{8, 255} {
		add("sticky", "sc_sticky", 0xAA, int64(n))
	}

	// S6 interleaved independent instances.
	for _, n := range []int{8, 255} {
		d := g0RefData(n)
		var c1, c2 g0RefByteCursor
		c1.make(d)
		c2.make(d)
		var mix uint16
		var v byte
		a1, a2 := true, true
		for a1 || a2 {
			if a1 {
				if c1.next(&v) {
					mix = mix*31 + uint16(v)
				} else {
					a1 = false
				}
			}
			if a2 {
				if c2.next(&v) {
					mix = mix*31 + uint16(v)
				} else {
					a2 = false
				}
			}
		}
		add("interleave", "sc_interleave", int64(mix), int64(n))
	}

	// S7 partial consumption equals a full decode.
	for _, n := range []int{8, 255} {
		add("partial", "sc_partial", int64(g0MixSeq(g0RefData(n))), int64(n))
	}

	// S8 RLE design-report example.
	add("rle", "sc_rle", int64(g0MixSeq([]byte{65, 65, 65, 66, 66})))

	// S9 RLE with exactly 255 outputs.
	var c g0RefRleCursor
	pairs := []byte{51, 7, 51, 7, 51, 7, 51, 7, 51, 7}
	c.make(pairs)
	out := g0RefDecodeAll(c.next)
	add("rle_255_mix", "sc_rle_255", int64(g0MixSeq(out)))
	add("rle_255_cnt", "sc_rle_255_cnt", int64(len(out)))

	// S10 RLE sticky DONE.
	add("rle_sticky", "sc_rle_sticky", 0xAA)

	// S11/S12 ADR 0050 demand order on [0,0,7,8,9].
	src := []byte{0, 0, 7, 8, 9}
	pred := func(b byte) bool { return b > 0 }
	outs, pulls := g0RefFilterTake(src, pred, 2)
	var acc uint16
	for _, o := range outs {
		acc = acc*256 + uint16(o)
	}
	add("filter_take2", "sc_filter_take2", int64(uint16(pulls))*4096+int64(acc))
	outs, pulls = g0RefFilterTake(src, pred, 3)
	acc = 0
	for _, o := range outs {
		acc = acc*256 + uint16(o)
	}
	add("filter_take3", "sc_filter_take3", int64(uint16(pulls))*4096+int64(acc))

	// S13 creation issues no pulls.
	add("make_no_effects", "sc_make_no_effects", 0, 8)

	return sc
}

// TestG0GeneratorContract runs every scenario against the Go reference on the
// raw MIR2 VM, the optimised MIR2 VM, and the native Z80 program.
func TestG0GeneratorContract(t *testing.T) {
	scenarios := g0ContractScenarios()

	parseWithAsserts := func() *hir.Module {
		m, err := nanz.Parse(g0ContractSrc, "g0_contract")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		for _, s := range scenarios {
			m.Asserts = append(m.Asserts, hir.Assert{
				FuncName: s.fn,
				Args:     s.args,
				Expected: s.want,
				Source:   "G0 contract scenario " + s.name,
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
