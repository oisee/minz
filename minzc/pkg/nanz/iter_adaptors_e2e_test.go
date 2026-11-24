package nanz_test

// Iterator chain methods skip, take, zip, count and sum (see
// docs/Nanz_Iteration_Audit.md and ADR 0050). Every expected value comes from
// the Go reference model below, which applies the stages in source order to
// the same data the Nanz init() writes. Results are order sensitive: folds and
// forEach traces mix with h = h*31 + v (u16 wrap), so a dropped, duplicated or
// reordered element changes the value. Each module runs on the raw MIR2 VM,
// the optimised MIR2 VM and the assembled Z80 program (g0RunAsserts).

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/lizp"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

const iterN = 260 // array length; n up to 256 is exercised

func iterSrc() []int {
	s := make([]int, iterN)
	for i := range s {
		s[i] = (i*37 + 11) & 0xFF
	}
	return s
}

func iterOth() []int {
	s := make([]int, iterN)
	for i := range s {
		s[i] = (i*13 + 5) & 0xFF
	}
	return s
}

const iterPrelude = `
global src: [u8; 260]
global oth: [u8; 260]
global h: u16 = 0
global calls: u16 = 0
fun init() {
    var i: u16 = 0
    while i < 260 {
        src[i] = u8(i * 37 + 11)
        oth[i] = u8(i * 13 + 5)
        i = i + 1
    }
    h = 0
    calls = 0
}
fun mix(a: u16, x: u8) -> u16 { return a * 31 + u16(x) }
fun odd(x: u8) -> bool { return (x & 1) == 1 }
`

// iterOp is one stage of the reference model.
type iterOp struct {
	kind string // filter, map, skip, take
	f    func(int) int
	p    func(int) bool
	k    int
}

// iterRef applies ops to xs in order with lazy pull semantics and returns the
// elements reaching the terminal. take stops pulling after k accepted elements.
func iterRef(xs []int, ops []iterOp) []int {
	cnt := make([]int, len(ops))
	var out []int
	done := func() bool {
		for i, op := range ops {
			if op.kind == "take" && cnt[i] >= op.k {
				return true
			}
		}
		return false
	}
outer:
	for _, x := range xs {
		if done() {
			break
		}
		for i, op := range ops {
			switch op.kind {
			case "filter":
				if !op.p(x) {
					continue outer
				}
			case "map":
				x = op.f(x)
			case "skip":
				if cnt[i] < op.k {
					cnt[i]++
					continue outer
				}
			case "take":
				cnt[i]++
			}
		}
		out = append(out, x)
	}
	return out
}

func iterMix(xs []int) int {
	var h uint16
	for _, x := range xs {
		h = h*31 + uint16(x)
	}
	return int(h)
}

func iterSum(xs []int) int {
	var s uint16
	for _, x := range xs {
		s += uint16(x)
	}
	return int(s)
}

func iterOdd(x int) bool { return x&1 == 1 }

func iterKs(n int) []int {
	seen := map[int]bool{}
	var ks []int
	for _, k := range []int{0, 1, n - 1, n, n + 1} {
		if k >= 0 && !seen[k] {
			seen[k] = true
			ks = append(ks, k)
		}
	}
	return ks
}

var iterNs = []int{0, 1, 255, 256}

type iterAsserts struct{ b strings.Builder }

func (a *iterAsserts) add(call string, want int) {
	fmt.Fprintf(&a.b, "assert %s == %d\n", call, want)
}

func runIterModule(t *testing.T, body string, a *iterAsserts) {
	t.Helper()
	if err := g0RunAsserts(t, iterPrelude+body+"\n"+a.b.String()); err != nil {
		t.Fatal(err)
	}
}

func TestIterSkipTakeMemory(t *testing.T) {
	body := `
fun f_skip(n: u16, k: u16) -> u16 { init()
    return src.skip(k).fold(u16(0), mix, n) }
fun f_take(n: u16, k: u16) -> u16 { init()
    return src.take(k).fold(u16(0), mix, n) }
fun f_fskip(n: u16, k: u16) -> u16 { init()
    return src.filter(odd).skip(k).fold(u16(0), mix, n) }
fun f_ftake(n: u16, k: u16) -> u16 { init()
    return src.filter(odd).take(k).fold(u16(0), mix, n) }
fun f_mtake(n: u16, k: u16) -> u16 { init()
    return src.map(|x: u8| x xor 0x5A).take(k).fold(u16(0), mix, n) }
fun e_skip(n: u16, k: u16) -> u16 { init()
    src.skip(k).forEach(|x: u8| { h = h * 31 + u16(x) }, n)
    return h }
fun e_take(n: u16, k: u16) -> u16 { init()
    src.take(k).forEach(|x: u8| { h = h * 31 + u16(x) }, n)
    return h }
fun e_fskip(n: u16, k: u16) -> u16 { init()
    src.filter(odd).skip(k).forEach(|x: u8| { h = h * 31 + u16(x) }, n)
    return h }
fun e_ftake(n: u16, k: u16) -> u16 { init()
    src.filter(odd).take(k).forEach(|x: u8| { h = h * 31 + u16(x) }, n)
    return h }
`
	xs := iterSrc()
	var a iterAsserts
	filt := iterOp{kind: "filter", p: iterOdd}
	mp := iterOp{kind: "map", f: func(x int) int { return x ^ 0x5A }}
	for _, n := range iterNs {
		for _, k := range iterKs(n) {
			skip, take := iterOp{kind: "skip", k: k}, iterOp{kind: "take", k: k}
			args := fmt.Sprintf("(%d, %d)", n, k)
			a.add("f_skip"+args, iterMix(iterRef(xs[:n], []iterOp{skip})))
			a.add("f_take"+args, iterMix(iterRef(xs[:n], []iterOp{take})))
			a.add("f_fskip"+args, iterMix(iterRef(xs[:n], []iterOp{filt, skip})))
			a.add("f_ftake"+args, iterMix(iterRef(xs[:n], []iterOp{filt, take})))
			a.add("f_mtake"+args, iterMix(iterRef(xs[:n], []iterOp{mp, take})))
			a.add("e_skip"+args, iterMix(iterRef(xs[:n], []iterOp{skip})))
			a.add("e_take"+args, iterMix(iterRef(xs[:n], []iterOp{take})))
			a.add("e_fskip"+args, iterMix(iterRef(xs[:n], []iterOp{filt, skip})))
			a.add("e_ftake"+args, iterMix(iterRef(xs[:n], []iterOp{filt, take})))
		}
	}
	runIterModule(t, body, &a)
}

func TestIterSkipTakeCombined(t *testing.T) {
	body := `
fun st(n: u16, a: u16, b: u16) -> u16 { init()
    return src.skip(a).take(b).fold(u16(0), mix, n) }
fun ts(n: u16, a: u16, b: u16) -> u16 { init()
    return src.take(b).skip(a).fold(u16(0), mix, n) }
fun fst(n: u16, a: u16, b: u16) -> u16 { init()
    return src.filter(odd).skip(a).take(b).fold(u16(0), mix, n) }
fun fts(n: u16, a: u16, b: u16) -> u16 { init()
    return src.filter(odd).take(b).skip(a).fold(u16(0), mix, n) }
fun tft(n: u16, a: u16, b: u16) -> u16 { init()
    return src.take(b).filter(odd).take(a).fold(u16(0), mix, n) }
fun est(n: u16, a: u16, b: u16) -> u16 { init()
    src.skip(a).take(b).forEach(|x: u8| { h = h * 31 + u16(x) }, n)
    return h }
fun ets(n: u16, a: u16, b: u16) -> u16 { init()
    src.filter(odd).take(b).skip(a).forEach(|x: u8| { h = h * 31 + u16(x) }, n)
    return h }
`
	xs := iterSrc()
	filt := iterOp{kind: "filter", p: iterOdd}
	var a iterAsserts
	for _, n := range []int{0, 1, 7, 256} {
		for _, sa := range []int{0, 1, 3, 300} {
			for _, sb := range []int{0, 2, 5, 256} {
				skip, take := iterOp{kind: "skip", k: sa}, iterOp{kind: "take", k: sb}
				args := fmt.Sprintf("(%d, %d, %d)", n, sa, sb)
				a.add("st"+args, iterMix(iterRef(xs[:n], []iterOp{skip, take})))
				a.add("ts"+args, iterMix(iterRef(xs[:n], []iterOp{take, skip})))
				a.add("fst"+args, iterMix(iterRef(xs[:n], []iterOp{filt, skip, take})))
				a.add("fts"+args, iterMix(iterRef(xs[:n], []iterOp{filt, take, skip})))
				a.add("tft"+args, iterMix(iterRef(xs[:n], []iterOp{take, filt, {kind: "take", k: sa}})))
				a.add("est"+args, iterMix(iterRef(xs[:n], []iterOp{skip, take})))
				a.add("ets"+args, iterMix(iterRef(xs[:n], []iterOp{filt, take, skip})))
			}
		}
	}
	runIterModule(t, body, &a)
}

// Byte-width counts and mixed widths (audit G0-3/4/9: no u8 underflow or
// truncation): n: u8 takes the DJNZ path; k: u16 above 255 must not wrap.
func TestIterSkipTakeWidths(t *testing.T) {
	body := `
fun b_take(n: u8, k: u8) -> u16 { init()
    return src.take(k).fold(u16(0), mix, n) }
fun b_skip(n: u8, k: u8) -> u16 { init()
    return src.skip(k).fold(u16(0), mix, n) }
fun b_skip16(n: u8, k: u16) -> u16 { init()
    return src.skip(k).sum(n) }
fun b_take16(n: u8, k: u16) -> u16 { init()
    return src.take(k).sum(n) }
fun w_take8(n: u16, k: u8) -> u16 { init()
    return src.take(k).count(n) }
fun w_ftake8(n: u16, k: u8) -> u16 { init()
    return src.filter(odd).take(k).count(n) }
fun lit_skip(n: u16) -> u16 { init()
    return src.skip(300).count(n) }
fun lit_take(n: u8) -> u16 { init()
    return src.take(300).count(n) }
`
	xs := iterSrc()
	filt := iterOp{kind: "filter", p: iterOdd}
	var a iterAsserts
	for _, n := range []int{0, 1, 4, 254, 255} {
		for _, k := range []int{0, 1, 3, 254, 255} {
			args := fmt.Sprintf("(%d, %d)", n, k)
			a.add("b_take"+args, iterMix(iterRef(xs[:n], []iterOp{{kind: "take", k: k}})))
			a.add("b_skip"+args, iterMix(iterRef(xs[:n], []iterOp{{kind: "skip", k: k}})))
		}
		for _, k := range []int{0, 2, 255, 256, 300} {
			args := fmt.Sprintf("(%d, %d)", n, k)
			a.add("b_skip16"+args, iterSum(iterRef(xs[:n], []iterOp{{kind: "skip", k: k}})))
			a.add("b_take16"+args, iterSum(iterRef(xs[:n], []iterOp{{kind: "take", k: k}})))
		}
		a.add(fmt.Sprintf("lit_take(%d)", n), len(iterRef(xs[:n], []iterOp{{kind: "take", k: 300}})))
	}
	for _, n := range []int{0, 1, 255, 256, 260} {
		for _, k := range []int{0, 1, 200, 255} {
			args := fmt.Sprintf("(%d, %d)", n, k)
			a.add("w_take8"+args, len(iterRef(xs[:n], []iterOp{{kind: "take", k: k}})))
			a.add("w_ftake8"+args, len(iterRef(xs[:n], []iterOp{filt, {kind: "take", k: k}})))
		}
		a.add(fmt.Sprintf("lit_skip(%d)", n), len(iterRef(xs[:n], []iterOp{{kind: "skip", k: 300}})))
	}
	runIterModule(t, body, &a)
}

func TestIterSkipTakeRange(t *testing.T) {
	body := `
fun r_skip(m: u8, k: u8) -> u16 { return range(0..m).skip(k).fold(u16(0), mix) }
fun r_take(m: u8, k: u8) -> u16 { return range(0..m).take(k).fold(u16(0), mix) }
fun r_fskip(m: u8, k: u8) -> u16 { return range(0..m).filter(odd).skip(k).fold(u16(0), mix) }
fun r_ftake(m: u8, k: u8) -> u16 { return range(0..m).filter(odd).take(k).fold(u16(0), mix) }
fun r_eskip(m: u8, k: u8) -> u16 {
    h = 0
    range(0..m).skip(k).forEach(|x: u8| { h = h * 31 + u16(x) })
    return h }
fun r_etake(m: u8, k: u8) -> u16 {
    h = 0
    range(0..m).filter(odd).take(k).forEach(|x: u8| { h = h * 31 + u16(x) })
    return h }
fun r_count(m: u8, k: u8) -> u16 { return range(0..m).take(k).count() }
fun r_sum(m: u8, k: u8) -> u16 { return range(0..m).skip(k).sum() }
`
	var a iterAsserts
	for _, m := range []int{0, 1, 5, 255} {
		xs := make([]int, m)
		for i := range xs {
			xs[i] = i
		}
		filt := iterOp{kind: "filter", p: iterOdd}
		for _, k := range iterKs(m) {
			if k > 255 {
				continue
			}
			skip, take := iterOp{kind: "skip", k: k}, iterOp{kind: "take", k: k}
			args := fmt.Sprintf("(%d, %d)", m, k)
			a.add("r_skip"+args, iterMix(iterRef(xs, []iterOp{skip})))
			a.add("r_take"+args, iterMix(iterRef(xs, []iterOp{take})))
			a.add("r_fskip"+args, iterMix(iterRef(xs, []iterOp{filt, skip})))
			a.add("r_ftake"+args, iterMix(iterRef(xs, []iterOp{filt, take})))
			a.add("r_eskip"+args, iterMix(iterRef(xs, []iterOp{skip})))
			a.add("r_etake"+args, iterMix(iterRef(xs, []iterOp{filt, take})))
			a.add("r_count"+args, len(iterRef(xs, []iterOp{take})))
			a.add("r_sum"+args, iterSum(iterRef(xs, []iterOp{skip})))
		}
	}
	runIterModule(t, body, &a)
}

// ADR 0050: for source [0,0,7,8,9], predicate x>0 and take(2), the outputs
// are [7,8] and the predicate (a side-effecting stage) runs for 0,0,7,8 —
// never for 9. Also proves the callback does not run after the k-th output.
func TestIterTakeADRDemand(t *testing.T) {
	src := `
global adr: [u8; 5]
global calls: u16 = 0
global h: u16 = 0
global seen: u16 = 0
fun pos(x: u8) -> bool {
    calls = calls + 1
    return x > 0
}
fun setup() {
    adr[0] = 0
    adr[1] = 0
    adr[2] = 7
    adr[3] = 8
    adr[4] = 9
    calls = 0
    h = 0
    seen = 0
}
fun demand(k: u8) -> u16 {
    setup()
    adr.filter(pos).take(k).forEach(|x: u8| { h = h * 256 + u16(x) }, 5)
    return calls * 4096 + h
}
fun demand_fold(k: u8) -> u16 {
    setup()
    let v: u16 = adr.filter(pos).take(k).fold(u16(0), |a: u16, x: u8| a * 256 + u16(x), 5)
    return calls * 4096 + v
}
fun mapped(k: u16) -> u16 {
    setup()
    adr.map(|x: u8| { seen = seen + 1
        return x }).take(k).forEach(|x: u8| { h = h * 31 + u16(x) }, 5)
    return seen * 4096 + h
}
`
	// Reference: g0RefFilterTake counts source requests until k outputs.
	data := []byte{0, 0, 7, 8, 9}
	var asserts strings.Builder
	for _, k := range []int{0, 1, 2, 3, 4} {
		outs, pulls := g0RefFilterTake(data, func(b byte) bool { return b > 0 }, k)
		// pulls counts a request that finds the source exhausted; the predicate
		// runs only for delivered items.
		predRuns := pulls
		if len(outs) < k && pulls > len(data) {
			predRuns = len(data)
		}
		var acc uint16
		for _, o := range outs {
			acc = acc*256 + uint16(o)
		}
		want := uint16(predRuns)*4096 + acc
		fmt.Fprintf(&asserts, "assert demand(%d) == %d\nassert demand_fold(%d) == %d\n", k, want, k, want)
	}
	fmt.Fprintf(&asserts, "assert demand(2) == %d\n", 4*4096+7*256+8) // literal ADR pin
	for _, k := range []int{0, 1, 3, 5, 6} {
		xs := []int{0, 0, 7, 8, 9}
		got := iterRef(xs, []iterOp{{kind: "take", k: k}})
		fmt.Fprintf(&asserts, "assert mapped(%d) == %d\n", k, uint16(len(got))*4096+uint16(iterMix(got)))
	}
	if err := g0RunAsserts(t, src+asserts.String()); err != nil {
		t.Fatal(err)
	}
}

func TestIterZip(t *testing.T) {
	body := `
fun dotf(a: u16, x: u8, y: u8) -> u16 { return a * 31 + u16(x xor y) }
fun gt(x: u8, y: u8) -> bool { return x > y }
fun zstep(x: u8, y: u8) { h = h * 31 + u16(x) * 2 + u16(y) }
fun z_fold(n: u16) -> u16 { init()
    return src.zip(oth).fold(u16(0), dotf, n) }
fun z_fold_l(n: u16) -> u16 { init()
    return src.zip(oth).fold(u16(0), |a: u16, x: u8, y: u8| a * 31 + u16(x) * u16(y), n) }
fun z_each(n: u16) -> u16 { init()
    src.zip(oth).forEach(|x: u8, y: u8| { h = h * 31 + u16(x) * 2 + u16(y) }, n)
    return h }
fun z_each_named(n: u16) -> u16 { init()
    src.zip(oth).forEach(zstep, n)
    return h }
fun z_mapfold(n: u16) -> u16 { init()
    return src.zip(oth).map(|x: u8, y: u8| x + y).fold(u16(0), mix, n) }
fun z_map16(n: u16) -> u16 { init()
    return src.zip(oth).map(|x: u8, y: u8| u16(x) * u16(y)).sum(n) }
fun z_filter(n: u16) -> u16 { init()
    return src.zip(oth).filter(|x: u8, y: u8| x > y).fold(u16(0), dotf, n) }
fun z_filter_named(n: u16) -> u16 { init()
    return src.zip(oth).filter(gt).fold(u16(0), dotf, n) }
fun z_skip(n: u16, k: u16) -> u16 { init()
    return src.zip(oth).skip(k).fold(u16(0), dotf, n) }
fun z_take(n: u16, k: u16) -> u16 { init()
    return src.zip(oth).take(k).fold(u16(0), dotf, n) }
fun z_fskip(n: u16, k: u16) -> u16 { init()
    return src.zip(oth).filter(gt).skip(k).fold(u16(0), dotf, n) }
fun z_ftake(n: u16, k: u16) -> u16 { init()
    src.zip(oth).filter(gt).take(k).forEach(zstep, n)
    return h }
fun z_count(n: u16) -> u16 { init()
    return src.zip(oth).filter(gt).count(n) }
fun z_ptr(n: u8) -> u16 { init()
    let p: ^u8 = &oth + 3
    return src.zip(p).fold(u16(0), dotf, n) }
`
	xs, ys := iterSrc(), iterOth()
	type pair struct{ x, y int }
	// pairRef: positions surviving the ops, applied to the pair stream.
	pairRef := func(n int, pred func(x, y int) bool, ops ...iterOp) []pair {
		idx := make([]int, n)
		for i := range idx {
			idx[i] = i
		}
		var all []iterOp
		if pred != nil {
			all = append(all, iterOp{kind: "filter", p: func(i int) bool { return pred(xs[i], ys[i]) }})
		}
		all = append(all, ops...)
		var out []pair
		for _, i := range iterRef(idx, all) {
			out = append(out, pair{xs[i], ys[i]})
		}
		return out
	}
	dot := func(ps []pair) int {
		var a uint16
		for _, p := range ps {
			a = a*31 + uint16(p.x^p.y)
		}
		return int(a)
	}
	step := func(ps []pair) int {
		var a uint16
		for _, p := range ps {
			a = a*31 + uint16(p.x)*2 + uint16(p.y)
		}
		return int(a)
	}
	gt := func(x, y int) bool { return x > y }
	var a iterAsserts
	for _, n := range []int{0, 1, 5, 256} {
		all := pairRef(n, nil)
		var prod uint16
		var ml uint16
		var sums []int
		for _, p := range all {
			prod = prod*31 + uint16(p.x)*uint16(p.y)
			ml += uint16(p.x) * uint16(p.y)
			sums = append(sums, (p.x+p.y)&0xFF)
		}
		a.add(fmt.Sprintf("z_fold(%d)", n), dot(all))
		a.add(fmt.Sprintf("z_fold_l(%d)", n), int(prod))
		a.add(fmt.Sprintf("z_each(%d)", n), step(all))
		a.add(fmt.Sprintf("z_each_named(%d)", n), step(all))
		a.add(fmt.Sprintf("z_mapfold(%d)", n), iterMix(sums))
		a.add(fmt.Sprintf("z_map16(%d)", n), int(ml))
		a.add(fmt.Sprintf("z_filter(%d)", n), dot(pairRef(n, gt)))
		a.add(fmt.Sprintf("z_filter_named(%d)", n), dot(pairRef(n, gt)))
		a.add(fmt.Sprintf("z_count(%d)", n), len(pairRef(n, gt)))
		for _, k := range iterKs(n) {
			args := fmt.Sprintf("(%d, %d)", n, k)
			a.add("z_skip"+args, dot(pairRef(n, nil, iterOp{kind: "skip", k: k})))
			a.add("z_take"+args, dot(pairRef(n, nil, iterOp{kind: "take", k: k})))
			a.add("z_fskip"+args, dot(pairRef(n, gt, iterOp{kind: "skip", k: k})))
			a.add("z_ftake"+args, step(pairRef(n, gt, iterOp{kind: "take", k: k})))
		}
	}
	for _, n := range []int{0, 1, 200} {
		var acc uint16
		for i := 0; i < n; i++ {
			acc = acc*31 + uint16(xs[i]^ys[i+3])
		}
		a.add(fmt.Sprintf("z_ptr(%d)", n), int(acc))
	}
	runIterModule(t, body, &a)
}

func TestIterCountSum(t *testing.T) {
	body := `
global ff: [u8; 260]
global w16: [u16; 6]
fun c_all(n: u16) -> u16 { init()
    return src.count(n) }
fun c_f(n: u16) -> u16 { init()
    return src.filter(odd).count(n) }
fun c_m(n: u16) -> u16 { init()
    return src.map(|x: u8| x & 3).filter(|x: u8| x == 1).count(n) }
fun s_all(n: u16) -> u16 { init()
    return src.sum(n) }
fun s_f(n: u16) -> u16 { init()
    return src.filter(odd).sum(n) }
fun s_m(n: u16) -> u16 { init()
    return src.map(|x: u8| x xor 0xFF).sum(n) }
fun s_wrap(n: u16) -> u16 {
    var i: u16 = 0
    while i < 260 {
        ff[i] = 255
        i = i + 1
    }
    return ff.sum(n) }
fun s16() -> u16 {
    w16[0] = 1000
    w16[1] = 30000
    w16[2] = 40000
    w16[3] = 7
    w16[4] = 65535
    w16[5] = 2
    return w16.sum(6) }
fun s16_skip() -> u16 {
    w16[0] = 1000
    w16[1] = 30000
    w16[2] = 40000
    w16[3] = 7
    w16[4] = 65535
    w16[5] = 2
    return w16.skip(1).take(3).sum(6) }
fun c_r(m: u8) -> u16 { return range(0..m).filter(odd).count() }
fun s_r(m: u8) -> u16 { return range(0..m).sum() }
fun s_rm(m: u8) -> u16 { return range(0..m).map(|x: u8| x * 3).sum() }
global out: u16
fun to_global(n: u16) { init()
    out = src.filter(odd).count(n) }
fun via_global(n: u16) -> u16 {
    out = 77
    to_global(n)
    return out }
fun let_bound(n: u16) -> u16 { init()
    let c = src.count(n)
    let s: u16 = src.sum(n)
    return c xor s }
`
	xs := iterSrc()
	filt := iterOp{kind: "filter", p: iterOdd}
	var a iterAsserts
	for _, n := range []int{0, 1, 100, 256, 260} {
		args := fmt.Sprintf("(%d)", n)
		a.add("c_all"+args, n)
		a.add("c_f"+args, len(iterRef(xs[:n], []iterOp{filt})))
		a.add("c_m"+args, len(iterRef(xs[:n], []iterOp{{kind: "map", f: func(x int) int { return x & 3 }}, {kind: "filter", p: func(x int) bool { return x == 1 }}})))
		a.add("s_all"+args, iterSum(xs[:n]))
		a.add("s_f"+args, iterSum(iterRef(xs[:n], []iterOp{filt})))
		a.add("s_m"+args, iterSum(iterRef(xs[:n], []iterOp{{kind: "map", f: func(x int) int { return x ^ 0xFF }}})))
		a.add("s_wrap"+args, int(uint16(255*n)))
		a.add("via_global"+args, len(iterRef(xs[:n], []iterOp{filt})))
		a.add("let_bound"+args, n^iterSum(xs[:n]))
	}
	a.add("s16()", (1000+30000+40000+7+65535+2)&0xFFFF)
	a.add("s16_skip()", (30000+40000+7)&0xFFFF)
	for _, m := range []int{0, 1, 10, 255} {
		var s, s3 uint16
		odd := 0
		for i := 0; i < m; i++ {
			s += uint16(i)
			s3 += uint16(byte(i * 3))
			if i&1 == 1 {
				odd++
			}
		}
		a.add(fmt.Sprintf("c_r(%d)", m), odd)
		a.add(fmt.Sprintf("s_r(%d)", m), int(s))
		a.add(fmt.Sprintf("s_rm(%d)", m), int(s3))
	}
	runIterModule(t, body, &a)
}

// Unsupported chain methods and shapes are compile errors with a source line,
// never a CALL to an undefined label; UFCS to user functions named like chain
// methods keeps working.
func TestIterChainDiagnostics(t *testing.T) {
	head := `
global src: [u8; 8]
global oth: [u8; 8]
fun f(x: u8) -> bool { return x > 1 }
fun step(x: u8) { }
fun add16(a: u16, x: u8) -> u16 { return a + u16(x) }
global ptr: ^u8
global w16: [u16; 8]
struct P { x: u8, y: u8 }
global ps: [P; 3]
global mm: [[u8; 3]; 2]
fun h(n: u8) -> u16 {
`
	cases := []struct{ body, want string }{
		{"return src.min(n)", `unknown method .min(...)`},
		{"return src.max(n)", `unknown method .max(...)`},
		{"return src.enumerate().count(n)", `unknown method .enumerate(...)`},
		{"return src.iter().count(n)", `unknown method .iter(...)`},
		{"return src.collect(n)", `unknown method .collect(...)`},
		{"return src.takeWhile(f).count(n)", `unknown method .takeWhile(...)`},
		{"return src.filter(f).zip(oth).count(n)", `applied directly to the source`},
		{"return src.zip(oth).zip(oth).count(n)", `one .zip(other)`},
		{"return range(0..n).zip(oth).count()", `memory source`},
		{"return 1 + src.count(n)", `must be the whole value`},
		{"let m = src.map(f)\n return 0", `iterator chain stage`},
		{"return src.skip().count(n)", `.skip(k) takes exactly one argument`},
		{"return src.take(1, 2).count(n)", `.take(k) takes exactly one argument`},
		{"return src.count(n, n)", `.count(n) takes one argument`},
		{"return src.zip(oth).sum(n)", `.sum after .zip`},
		{"return src.zip(oth).fold(u16(0), add16, n)", `two element parameters`},
		{"src.zip(oth).forEach(step, n)\n return 0", `two element parameters`},
		{"return ptr.skip(2).count()", `needs the element count`},
		{"ptr.forEach(step)\n return 0", `unsupported iterator chain .forEach`},
		{"return ptr.fold(u16(0), add16)", `needs the element count`},
		{"src.zip(oth).mapInPlace(|x: u8| x, n)\n return 0", `.mapInPlace does not support .zip`},
		{"src.filter(f).skip(1).mapInPlace(|x: u8| x, n)\n return 0", `.mapInPlace supports .skip only directly on the source`},
		{"let k: i8 = 2\n return src.skip(k).count(n)", `k must be a u8 or u16 value`},
		{"let k: i8 = 2\n return src.filter(f).take(k).count(n)", `k must be a u8 or u16 value`},
		{"return src.skip(true).count(n)", `k must be a u8 or u16 value`},
		{"let x: i8 = 0\n return src.sum(x)", `element count must be a u8 or u16 value`},
		{"return src.sum(true)", `element count must be a u8 or u16 value`},
		{"let x: i16 = 4\n return src.count(x)", `element count must be a u8 or u16`},
		{"return w16.zip(src).map(|x: u8, y: u8| y).sum()", `the source's elements are u16 but the first callback takes u8`},
		{"return src.zip(w16).map(|x: u8, y: u8| y).sum()", `other's elements are u16`},
		{"return w16.map(|x: u8| x).sum()", `the source's elements are u16`},
		{"return ps.count()", `element type P is not supported`},
		{"return mm.count()", `element type [3]u8 is not supported`},
		{"return mm.count(2)", `element type [3]u8 is not supported`},
	}
	for _, tc := range cases {
		t.Run(strings.SplitN(tc.body, "\n", 2)[0], func(t *testing.T) {
			src := head + " " + tc.body + "\n}\n"
			m, err := nanz.Parse(src, "iter_diag")
			if err == nil {
				_, err = pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
			}
			if err == nil {
				t.Fatalf("compiled; want error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), fmt.Sprintf("line %d", iterDiagLine(tc.body))) {
				t.Fatalf("error %q; want it to contain %q and the source line", err, tc.want)
			}
		})
	}
}

func TestIterChainNamesStayUFCS(t *testing.T) {
	src := `
global buf: [u8; 4]
fun sum(p: ^u8, k: u8) -> u8 { return p^ + k }
fun take(p: ^u8, k: u8) -> u8 { return p^ * k }
fun count(p: ^u8) -> u16 { return 1000 }
fun main() -> u16 {
    buf[0] = 5
    let a: u8 = buf.sum(3)
    let b: u8 = buf.take(4)
    let c: u16 = buf.count()
    return u16(a) * 100 + u16(b) + c
}
assert main() == 1820
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}

// A memory terminal without its count over a sized global array uses the
// declared length.
func TestIterSizedArrayLength(t *testing.T) {
	src := `
global a: [u8; 8]
global acc: u16 = 0
fun setup() {
    for i in 0..8 { a[i] = i + 1 }
    acc = 0
}
fun e_all() -> u16 { setup()
    a.forEach(|x: u8| { acc = acc * 10 + u16(x) })
    return acc }
fun e_take() -> u16 { setup()
    a.take(3).forEach(|x: u8| { acc = acc * 10 + u16(x) })
    return acc }
fun e_skip() -> u16 { setup()
    a.filter(|x: u8| x > 2).skip(2).forEach(|x: u8| { acc = acc * 10 + u16(x) })
    return acc }
fun f_all() -> u16 { setup()
    return a.fold(u16(0), |s: u16, x: u8| s * 3 + u16(x)) }
fun c_all() -> u16 { setup()
    return a.filter(|x: u8| x > 5).count() }
fun s_all() -> u16 { setup()
    return a.skip(6).sum() }
fun m_all() -> u16 { setup()
    a.mapInPlace(|x: u8| x * 2)
    return a.fold(u16(0), |s: u16, x: u8| s * 3 + u16(x)) }
`
	var asserts strings.Builder
	mix3 := func(xs ...int) int {
		v := 0
		for _, x := range xs {
			v = (v*3 + x) & 0xFFFF
		}
		return v
	}
	fmt.Fprintf(&asserts, "assert e_all() == %d\n", 12345678&0xFFFF)
	fmt.Fprintf(&asserts, "assert e_take() == 123\n")
	fmt.Fprintf(&asserts, "assert e_skip() == 5678\n")
	fmt.Fprintf(&asserts, "assert f_all() == %d\n", mix3(1, 2, 3, 4, 5, 6, 7, 8))
	fmt.Fprintf(&asserts, "assert c_all() == 3\n")
	fmt.Fprintf(&asserts, "assert s_all() == 15\n")
	fmt.Fprintf(&asserts, "assert m_all() == %d\n", mix3(2, 4, 6, 8, 10, 12, 14, 16))
	if err := g0RunAsserts(t, src+asserts.String()); err != nil {
		t.Fatal(err)
	}
}

// Review round 1, item 1: head skip/take before a type-changing map must not
// hand the chain to the original single-variable builders.
func TestIterHeadStageTypeChangingMap(t *testing.T) {
	body := `
fun f_take(n: u16, k: u16) -> u16 { init()
    return src.take(k).map(|x: u8| u16(x) * 100).fold(u16(0), |s: u16, x: u16| s * 3 + x, n) }
fun f_skip(n: u16, k: u16) -> u16 { init()
    return src.skip(k).map(|x: u8| u16(x) * 3 + 191).fold(u16(0), |s: u16, x: u16| s * 3 + x, n) }
fun f_skipfilt(n: u16, k: u16) -> u16 { init()
    return src.skip(k).filter(odd).map(|x: u8| u16(x) * 3 + 191).fold(u16(0), |s: u16, x: u16| s * 3 + x, n) }
fun e_skip(n: u16, k: u16) -> u16 { init()
    src.skip(k).map(|x: u8| u16(x) * 100).forEach(|x: u16| { h = h * 3 + x }, n)
    return h }
fun e_take8(n: u8, k: u8) -> u16 { init()
    src.take(k).map(|x: u8| u16(x) * 100).forEach(|x: u16| { h = h * 3 + x }, n)
    return h }
`
	xs := iterSrc()
	mix3 := func(ys []int) int {
		var v uint16
		for _, y := range ys {
			v = v*3 + uint16(y)
		}
		return int(v)
	}
	m100 := iterOp{kind: "map", f: func(x int) int { return (x * 100) & 0xFFFF }}
	m3 := iterOp{kind: "map", f: func(x int) int { return (x*3 + 191) & 0xFFFF }}
	var a iterAsserts
	for _, n := range []int{0, 1, 4, 255, 256} {
		for _, k := range []int{0, 1, 3, 256} {
			args := fmt.Sprintf("(%d, %d)", n, k)
			take, skip := iterOp{kind: "take", k: k}, iterOp{kind: "skip", k: k}
			a.add("f_take"+args, mix3(iterRef(xs[:n], []iterOp{take, m100})))
			a.add("f_skip"+args, mix3(iterRef(xs[:n], []iterOp{skip, m3})))
			a.add("f_skipfilt"+args, mix3(iterRef(xs[:n], []iterOp{skip, {kind: "filter", p: iterOdd}, m3})))
			a.add("e_skip"+args, mix3(iterRef(xs[:n], []iterOp{skip, m100})))
			if n <= 255 && k <= 255 {
				a.add("e_take8"+args, mix3(iterRef(xs[:n], []iterOp{take, m100})))
			}
		}
	}
	runIterModule(t, body, &a)
}

// Item 2: untyped lambda parameters take the source's element type (array
// element, pointer pointee; after zip, other's element type), on array and
// pointer sources, new and original methods.
func TestIterUntypedLambdas(t *testing.T) {
	src := `
global a: [u8; 4] = [1, 2, 3, 4]
global w: [u16; 4] = [1000, 2000, 3000, 4000]
global h: u16 = 0
fun arr_count() -> u16 { return a.filter(|x| x > 1).count() }
fun arr_take() -> u16 { return a.filter(|x| x > 1).take(9).fold(u16(0), |s: u16, x: u8| s * 3 + u16(x), 4) }
fun ptr_count() -> u16 {
    let p: ^u8 = &a[0]
    return p.filter(|x| x > 1).count(4) }
fun ptr16_sum() -> u16 {
    let p: ^u16 = &w[0]
    return p.filter(|x| x > 1500).sum(4) }
fun zip_map() -> u16 { return a.zip(w).map(|x, y| y + u16(x)).sum() }
fun zip_filter() -> u16 { return a.zip(w).filter(|x, y| y > 2500).count() }
fun zip_fold() -> u16 { return a.zip(w).fold(u16(0), |s, x, y| s + y + u16(x)) }
fun zip_each() -> u16 {
    h = 0
    a.zip(w).forEach(|x, y| { h = h + y / 1000 * u16(x) })
    return h }
fun old_fold() -> u16 { return a.filter(|x| x > 1).fold(u16(0), |s: u16, x: u8| s + u16(x), 4) }
assert arr_count() == 3
assert arr_take() == 31
assert ptr_count() == 3
assert ptr16_sum() == 9000
assert zip_map() == 10010
assert zip_filter() == 2
assert zip_fold() == 10010
assert zip_each() == 30
assert old_fold() == 9
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}

// Item 3: with an implicit count, zip walks the shorter of the two declared
// lengths (partner shorter or wider than the source); an explicit count is
// the caller's.
func TestIterZipImplicitLength(t *testing.T) {
	src := `
global a: [u8; 4] = [1, 2, 3, 4]
global w: [u16; 3] = [1000, 2000, 3000]
global after: u16 = 7777
global b: [u8; 6] = [10, 20, 30, 40, 50, 60]
fun short_wide() -> u16 { return a.zip(w).map(|x: u8, y: u16| y).sum() }
fun wide_short() -> u16 { return w.zip(a).map(|x: u16, y: u8| x + u16(y)).sum() }
fun longer() -> u16 { return a.zip(b).map(|x: u8, y: u8| x + y).sum() }
fun explicit2() -> u16 { return a.zip(w).map(|x: u8, y: u16| y).sum(2) }
assert short_wide() == 6000
assert wide_short() == 6006
assert longer() == 110
assert explicit2() == 3000
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}

// Item 4: take stops right after the k-th accepted element; the next source
// element is never loaded. The explicit count exceeds the 5-byte object and
// the second match is its last byte, so a further load is out of bounds on
// the MIR2 VM.
func TestIterTakeDoesNotReadPastDemand(t *testing.T) {
	src := `
global a: [u8; 5] = [0, 9, 0, 0, 7]
fun f() -> u16 { return a.filter(|x: u8| x > 0).take(2).sum(1000) }
fun fz() -> u16 { return a.zip(a).filter(|x: u8, y: u8| x > 0).take(2).count(1000) }
fun fk(k: u8) -> u16 { return a.filter(|x: u8| x > 0).take(k).sum(1000) }
fun e() -> u16 {
    var t: u16 = 0
    a.filter(|x: u8| x > 0).take(2).forEach(|x: u8| { t = t * 10 + u16(x) }, 1000)
    return t }
fun z0() -> u16 { return a.filter(|x: u8| x > 0).take(0).count(1000) }
assert f() == 16
assert fz() == 2
assert fk(2) == 16
assert fk(1) == 9
assert fk(0) == 0
assert e() == 97
assert z0() == 0
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
	// The VM heap places function-name tags after the globals, so a read past
	// a[4] is not out of bounds there: watch the loads instead, on the raw and
	// the optimised MIR2.
	parse := func() *hir.Module {
		m, err := nanz.Parse(src, "take_demand")
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	steps, err := pipeline.CompileHIRSteps(parse(), pipeline.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	for tier, m := range map[string]*mir2.Module{"raw": hir.LowerModule(parse()), "optimised": steps.MIR2Module} {
		for _, fn := range []string{"f", "fz", "e"} {
			vm := mir2.NewVM(m)
			base := vm.GlobalAddr("a")
			vm.CheckDataAccess = func(inst *mir2.Inst, addr int64) error {
				if inst.Op == mir2.OpLoad && addr >= base+5 && addr < base+64 {
					return fmt.Errorf("load of a[%d] after take(2) was satisfied", addr-base)
				}
				return nil
			}
			if _, err := vm.Call(fn, nil); err != nil {
				t.Errorf("%s MIR2 %s(): %v", tier, fn, err)
			}
		}
	}
}

// Item 6: chain arguments are evaluated once, in source order: source,
// zip's argument, stage arguments, then the terminal's count.
func TestIterEvaluationOrder(t *testing.T) {
	src := `
global a: [u8; 5] = [1, 2, 3, 4, 5]
global b: [u8; 5] = [10, 20, 30, 40, 50]
global h: u16 = 0
global ss: u16 = 0
fun gs() -> ^u8 { h = h * 10 + 1
    return &a[0] }
fun go() -> ^u8 { h = h * 10 + 2
    return &b[0] }
fun gk() -> u8 { h = h * 10 + 3
    return 1 }
fun gn() -> u8 { h = h * 10 + 4
    return 4 }
fun head_skip() -> u16 { h = 0
    ss = gs().skip(gk()).sum(gn())
    return h * 100 + ss }
fun mid_take() -> u16 { h = 0
    ss = gs().filter(|x: u8| x > 1).take(gk()).sum(gn())
    return h * 100 + ss }
fun zip_head_skip() -> u16 { h = 0
    ss = gs().zip(go()).skip(gk()).map(|x: u8, y: u8| x + y).sum(gn())
    return h * 10 + ss / 11 }
fun zip_mid_take() -> u16 { h = 0
    ss = gs().zip(go()).filter(|x: u8, y: u8| x > 1).take(gk()).map(|x: u8, y: u8| x + y).sum(gn())
    return h * 10 + ss / 11 }
assert head_skip() == 13409
assert mid_take() == 13402
assert zip_head_skip() == 12349
assert zip_mid_take() == 12342
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}

// Item 7 and 9: a value chain stored through a pointer, into an array element
// or a global — new methods and the original fold (whose global store used to
// be dropped).
func TestIterValueChainTargets(t *testing.T) {
	src := `
global a: [u8; 5] = [0, 9, 0, 0, 7]
global out: [u16; 2]
global r: u16 = 0
fun add16(s: u16, x: u8) -> u16 { return s + u16(x) }
fun store_new() -> u16 {
    let p: ^u16 = &out[1]
    p^ = a.filter(|x: u8| x > 0).count(5)
    return out[1] }
fun store_old() -> u16 {
    let p: ^u16 = &out[0]
    p^ = a.fold(u16(0), add16, 5)
    return out[0] }
fun index_old() -> u16 {
    out[1] = a.fold(u16(0), add16, 5)
    return out[1] }
fun set_global() { r = a.fold(u16(0), add16, 5) }
fun global_old() -> u16 {
    r = 1
    set_global()
    return r }
fun set_global_new() { r = a.skip(1).sum(5) }
fun global_new() -> u16 {
    r = 1
    set_global_new()
    return r }
fun local_old() -> u16 {
    var v: u16 = 3
    v = a.fold(u16(0), add16, 5)
    return v }
assert store_new() == 2
assert store_old() == 16
assert index_old() == 16
assert global_old() == 16
assert global_new() == 16
assert local_old() == 16
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}

// A u8 count with a non-head take and a body large enough that the DJNZ back
// edge leaves its 8-bit range; the backend routes it through a trampoline.
func TestIterByteCountLargeBody(t *testing.T) {
	body := `
fun big(n: u8, k: u8) -> u16 { init()
    src.filter(odd).take(k).forEach(|x: u8| {
        h = h * 31 + u16(x)
        h = h xor (u16(x) * 7)
        h = h + u16(x) * 3 + 1
        h = h * 5 + u16(x xor 0x55)
        h = h - u16(x) * 9
        h = h xor (h / 3)
    }, n)
    return h }
fun zbig(n: u8) -> u16 { init()
    src.zip(oth).filter(|x: u8, y: u8| (x + y) % 3 != 0).take(7).forEach(|x: u8, y: u8| { h = h * 7 + u16(x) * 3 + u16(y) }, n)
    return h }
fun bigs(n: u8) -> u16 { init()
    src.skip(2).forEach(|x: u8| {
        h = h * 31 + u16(x)
        h = h xor (u16(x) * 7)
        h = h + u16(x) * 3 + 1
        h = h * 5 + u16(x xor 0x55)
        h = h - u16(x) * 9
        h = h xor (h / 3)
    }, n)
    return h }
`
	step := func(h uint16, x int) uint16 {
		h = h*31 + uint16(x)
		h ^= uint16(x) * 7
		h = h + uint16(x)*3 + 1
		h = h*5 + uint16(x^0x55)
		h -= uint16(x) * 9
		h ^= h / 3
		return h
	}
	run := func(ys []int) int {
		var h uint16
		for _, y := range ys {
			h = step(h, y)
		}
		return int(h)
	}
	xs := iterSrc()
	var a iterAsserts
	for _, n := range []int{0, 1, 17, 255} {
		for _, k := range []int{0, 2, 255} {
			a.add(fmt.Sprintf("big(%d, %d)", n, k), run(iterRef(xs[:n], []iterOp{{kind: "filter", p: iterOdd}, {kind: "take", k: k}})))
		}
		a.add(fmt.Sprintf("bigs(%d)", n), run(iterRef(xs[:n], []iterOp{{kind: "skip", k: 2}})))
		ys := iterOth()
		var zh uint16
		taken := 0
		for i := 0; i < n && taken < 7; i++ {
			if (xs[i]+ys[i])&0xFF%3 != 0 {
				zh = zh*7 + uint16(xs[i])*3 + uint16(ys[i])
				taken++
			}
		}
		a.add(fmt.Sprintf("zbig(%d)", n), int(zh))
	}
	if err := g0RunAsserts(t, iterPrelude+body+"\n"+a.b.String()); err != nil {
		t.Fatal(err)
	}
}

// iterDiagLine is the source line of the chain in a TestIterChainDiagnostics
// body: the body starts on line 13; a trailing "return 0" follows the chain.
func iterDiagLine(body string) int {
	if strings.HasSuffix(body, "return 0") {
		return 13
	}
	return 13 + strings.Count(body, "\n")
}

// ── Review round 2 ─────────────────────────────────────────────────────────

// F1: an untyped fold accumulator takes the init's type; with an untyped
// literal init it keeps origin/main's hint (a pointer source's pointer type,
// a map's result type), u16 where main had none. Element parameters take the
// element type. Z80 is the tier that truncated.
func TestIterUntypedFoldAccumulator(t *testing.T) {
	src := `
global a: [u8; 4]
global b: [u8; 4]
fun init() {
    a[0] = 200
    a[1] = 100
    a[2] = 3
    a[3] = 4
    b[0] = 1
    b[1] = 1
    b[2] = 1
    b[3] = 1
}
fun arr_typed() -> u16 { init()
    let base: u16 = 1000
    return a.fold(base, |acc, x| acc + u16(x)) }
fun zip_typed() -> u16 { init()
    let base: u16 = 1000
    return a.zip(b).fold(base, |acc, x, y| acc + u16(x) + u16(y)) }
fun skip_typed() -> u16 { init()
    let base: u16 = 1000
    return a.skip(0).fold(base, |acc, x| acc + u16(x)) }
fun ptr_typed() -> u16 { init()
    let p: ^u8 = &a[0]
    let base: u16 = 1000
    return p.fold(base, |acc, x: u8| acc + u16(x), 4) }
fun ptr_literal() -> u16 { init()
    let p: ^u8 = &a[0]
    return p.fold(0, |acc, x: u8| acc + u16(x), 4) }
fun arr_literal() -> u16 { init()
    return a.fold(0, |acc, x| acc + u16(x)) }
fun map_literal() -> u8 { init()
    return a.map(|x: u8| x + 1).fold(0, |acc, x| acc + x, 4) }
fun expr_init() -> u16 { init()
    return a.fold(u16(a[2]) * 100, |acc, x| acc + u16(x), 4) }
assert arr_typed() == 1307
assert zip_typed() == 1311
assert skip_typed() == 1307
assert ptr_typed() == 1307
assert ptr_literal() == 307
assert arr_literal() == 307
assert map_literal() == 55
assert expr_init() == 607
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}

// F3: the new names are chain methods only in method syntax. A plain call to
// an assembly label named sum/count/take stays a call (main's behaviour).
func TestIterPlainCallsToChainNames(t *testing.T) {
	src := `
global buf: [u8; 4]
fun helpers() -> void {
    asm {
        RET
    sum:
        LD HL, 1234
        RET
    count:
        LD HL, 77
        RET
    take:
        LD HL, 5
        RET
    }
}
global res: u16 = 0
global h: u16 = 0
fun run() {
    buf[0] = 1
    buf[1] = 2
    let r: u16 = sum(buf, 4)
    let c: u16 = count(5)
    let k: u16 = take(7, 9)
    res = r
    h = c + k
}
`
	m, err := nanz.Parse(src, "plain_names")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
	if err != nil {
		t.Fatalf("plain calls to labels named like chain methods must compile: %v", err)
	}
	for _, name := range []string{"sum", "count", "take"} {
		if !strings.Contains(steps.Assembly, "CALL "+name) {
			t.Errorf("want CALL %s in the assembly (a plain call, not a fused loop)", name)
		}
	}
	got := iterRunZ80Word(t, steps.Assembly, "run", "res")
	if got != 1234 {
		t.Fatalf("Z80 sum(buf, 4) = %d, want 1234 from the assembly label", got)
	}
}

// F3, other frontends: a C program with functions named like chain methods.
func TestIterChainNamesInC(t *testing.T) {
	src := `
unsigned short sum(unsigned char *p, unsigned char n) { unsigned short s = 0; while (n) { s += *p++; n--; } return s; }
unsigned char count(unsigned char x) { return x + 1; }
unsigned char take(unsigned char a, unsigned char b) { return a * b; }
unsigned char skip(unsigned char a) { return a - 1; }
unsigned char zip(unsigned char a, unsigned char b) { return a ^ b; }
unsigned char map(unsigned char a, unsigned char b) { return a + b; }
unsigned char filter(unsigned char a) { return a > 3; }
unsigned short fold(unsigned short a, unsigned char b, unsigned char c, unsigned char d) { return a + b + c + d; }
unsigned char buf[4] = {1, 2, 3, 4};
unsigned short f(void) { return sum(buf, 4) + count(5) + take(2, 3) + skip(9) + zip(6, 3) + map(1, 2) + filter(7) + fold(100, 1, 2, 3); }
// assert f() == 145 via mir2
// assert f() == 145 via z80
`
	m, err := c89.CompileWithOpts(src, "names.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	raw := hir.LowerModule(m)
	if err := pipeline.RunAssertsMIR2(m, raw); err != nil {
		t.Fatalf("raw MIR2 VM: %v", err)
	}
	m2, _ := c89.CompileWithOpts(src, "names.c", c89.CompileOpts{NoLibc: true})
	steps, err := pipeline.CompileHIRSteps(m2, pipeline.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if err := pipeline.RunAssertsMIR2(m2, steps.MIR2Module); err != nil {
		t.Fatalf("optimised MIR2 VM: %v", err)
	}
	if err := pipeline.RunAssertsZ80(m2, steps.MIR2Module, steps.Allocation, steps.Assembly); err != nil {
		t.Fatalf("z80: %v", err)
	}
}

// F4: the nested-capture rule applies only to an inlined outer callback; an
// outer callback with an early return is called and keeps its parameter.
// F9: a new chain and an original fold of different widths in one inlined
// callback body each keep their own accumulator type.
func TestIterNestedCallbacks(t *testing.T) {
	src := `
global a: [u8; 4] = [10, 20, 30, 40]
global b: [u8; 4] = [1, 25, 35, 2]
global c3: [u8; 3]
global h: u16 = 0
global r: u16 = 0
fun called_outer() -> u16 {
    c3[0] = 1
    c3[1] = 2
    c3[2] = 3
    h = 0
    c3.forEach(|x: u8| {
        if x == 3 { return }
        c3.filter(|y: u8| y > x).forEach(|y: u8| { h = h + u16(y) }, 3)
    }, 3)
    return h }
fun mixed1() -> u16 {
    r = 0
    a.forEach(|x: u8| {
        let v: u8 = x
        let c: u16 = b.filter(|y: u8| y > v).count(4)
        let s: u8 = a.fold(u8(0), |acc: u8, y: u8| acc + y, 4)
        r = r + c * 1000 + u16(s)
    }, 4)
    return r }
fun mixed2() -> u16 {
    r = 0
    a.forEach(|x: u8| {
        let v: u8 = x
        let s: u8 = a.fold(u8(200), |acc: u8, y: u8| acc + y, 4)
        let c: u16 = b.filter(|y: u8| y > v).sum(4)
        r = r + c * 100 + u16(s)
    }, 4)
    return r }
assert called_outer() == 8
assert mixed1() == 5400
assert mixed2() == 15676
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}

// F5: mapInPlace gets the element-type hint like the other methods.
func TestIterMapInPlaceHint(t *testing.T) {
	src := `
global gi: [i8; 2]
global gu: [u16; 3]
fun neg() -> i8 {
    gi[0] = -4
    gi[1] = 6
    gi.mapInPlace(|x| x / 2)
    return gi[0] }
fun wide_explicit() -> u16 {
    gu[0] = 1000
    gu[1] = 2000
    gu[2] = 3000
    gu.mapInPlace(|x| x + 5, 3)
    return gu[0] + gu[1] + gu[2] }
fun wide_implicit() -> u16 {
    gu[0] = 1000
    gu[1] = 2000
    gu[2] = 3000
    gu.mapInPlace(|x| x + 5)
    return gu[0] + gu[1] + gu[2] }
assert neg() == -2
assert wide_explicit() == 6015
assert wide_implicit() == 6015
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}

// F2, F6, F7, F8: shapes that must be compile errors, never a panic or wrong
// code; and the struct/array shapes that worked on origin/main keep working.
func TestIterRound2Diagnostics(t *testing.T) {
	head := `
struct P { a: u8, b: u8 }
global ps: [P; 3]
global mm: [[u8; 2]; 3]
global a: [u8; 6]
global h: u16 = 0
fun f() -> u16 {
`
	cases := []struct{ body, want string }{
		{"a.filter(|x: i8| x > 2).mapInPlace(|x: u8| x * 10)\n return 0", ".mapInPlace stores back"},
		{"a.skip(1).map(|x: u8| u16(x)).mapInPlace(|x: u16| x, 6)\n return 0", ".mapInPlace stores back"},
		{"return ps.map(|x: u16| x).sum()", "the source's elements are P"},
		{"return mm.filter(|x: u16| x > 0).count()", "the source's elements are [2]u8"},
		{"return a.zip(ps).map(|x: u8, y: u16| y).sum(3)", "other's elements are P"},
		{"ps.forEach(|p: P| { h = h + u16(p.a) }, 3)\n return 0", "struct elements (P)"},
		{"let q: ^P = &ps[0]\n q.forEach(|p| { h = h + u16(p.a) }, 3)\n return 0", "struct elements (P)"},
		{"ps.forEach(|p: ^P| { h = h + u16(p.a) }, 3)\n return 0", "the source's elements are P"},
		{"ps.mapInPlace(|p: P| p, 3)\n return 0", "struct elements (P)"},
		{"let v: u8 = 7\n return a.zip(v).count()", "other must be an array or a pointer"},
		{"let v: u8 = 7\n return v.sum(3)", "the source must be an array or a pointer"},
		{"return true.sum(3)", "the source must be an array or a pointer"},
	}
	for _, tc := range cases {
		t.Run(strings.SplitN(tc.body, "\n", 2)[0], func(t *testing.T) {
			m, err := nanz.Parse(head+" "+tc.body+"\n}\n", "round2")
			if err == nil {
				_, err = pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v; want an error containing %q", err, tc.want)
			}
		})
	}
	keep := `
struct P { a: u8, b: u8 }
global ps: [P; 3]
global mm: [[u8; 2]; 3]
global h: u16 = 0
fun init() {
    ps[0].a = 1
    ps[0].b = 2
    ps[1].a = 3
    ps[1].b = 4
    ps[2].a = 5
    ps[2].b = 6
    mm[0][0] = 1
    mm[0][1] = 2
    mm[1][0] = 3
    mm[1][1] = 4
    mm[2][0] = 5
    mm[2][1] = 6
    h = 0
}
fun bytes() -> u16 { init()
    ps.forEach(|x: u8| { h = h + u16(x) }, 6)
    return h }
fun words() -> u16 { init()
    ps.forEach(|x: u16| { h = h + x }, 3)
    return h }
fun rows() -> u16 { init()
    mm.forEach(|r: [u8; 2]| { h = h + 1 }, 3)
    return h }
fun nested_bytes() -> u16 { init()
    mm.forEach(|x: u8| { h = h + u16(x) }, 6)
    return h }
assert bytes() == 21
assert words() == 3081
assert rows() == 3
assert nested_bytes() == 21
`
	if err := g0RunAsserts(t, keep); err != nil {
		t.Fatal(err)
	}
}

// A range count wider than a byte (audit G0-9, resolved) under a new terminal.
func TestIterRangeCountWide(t *testing.T) {
	src := `
fun c() -> u16 { return range(0..300).count() }
assert c() == 300
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}

// iterRunZ80Word assembles asm, calls fn on the emulator and returns the u16
// global named sym afterwards.
func iterRunZ80Word(t *testing.T, asm, fn, sym string) int {
	t.Helper()
	boot := fmt.Sprintf(`
    ORG 0x8000
    LD SP, 0xFF00
    CALL %s
    LD HL, (%s)
    LD (0xFE00), HL
    DI
    HALT
`, fn, sym)
	as := z80asm.NewAssembler()
	res, err := as.AssembleString(boot + "\n" + asm)
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
	return int(z80.GetMemory(0xFE00)) | int(z80.GetMemory(0xFE01))<<8
}

// ── Review round 3 ─────────────────────────────────────────────────────────

// The method mark has a spelling in every text form of HIR: the HIR dump
// (call @.sum), Lanz/Lizp ((.sum ...)) and Nanz printed from HIR
// (src.sum(n)). A new-method chain survives the round trip with the same
// asserts on all tiers and the same final assembly; a plain call to a label
// named sum stays a plain call.
func TestIterChainTextRoundTrip(t *testing.T) {
	src := `
global a: [u8; 8]
global h: u16 = 0
fun init() -> void { for i in 0..8 { a[i] = i + 1 } }
fun step(x: u8) -> void { h = h + u16(x) }
fun newc() -> u16 {
    init()
    return a.filter(|x: u8| x > 2).take(3).sum(8)
}
fun each() -> u16 {
    init()
    h = 0
    a.filter(|x: u8| x > 4).take(2).forEach(step, 8)
    return h
}
fun sized() -> u16 {
    init()
    h = 0
    a.forEach(step)
    return h
}
assert newc() == 12
assert each() == 11
assert sized() == 36
`
	orig, err := nanz.Parse(src, "rt")
	if err != nil {
		t.Fatal(err)
	}
	direct, _ := nanz.Parse(src, "rt")
	steps, err := pipeline.CompileHIRSteps(direct, pipeline.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"call @.sum(call @.take(", "call @.take(", "call @.forEach((a:[8]u8), addr @step:ptr)"} {
		if !strings.Contains(steps.HIR, want) {
			t.Fatalf("HIR dump lacks the method mark %q:\n%s", want, steps.HIR)
		}
	}
	reread := map[string]func() (*hir.Module, error){
		"hir": func() (*hir.Module, error) { return hir.ParseHIR(steps.HIR, "rt") },
		"nanz": func() (*hir.Module, error) {
			m, err := hir.ParseHIR(steps.HIR, "rt")
			if err != nil {
				return nil, err
			}
			return nanz.Parse(nanz.Print(m), "rt")
		},
	}
	// Nanz printed from HIR spells a function reference as &step, which the
	// Nanz parser does not read back (also on origin/main): that format is
	// checked on the lambda-only part below.
	lambdaSrc := `
global a: [u8; 8]
fun init() -> void { for i in 0..8 { a[i] = i + 1 } }
fun newc() -> u16 {
    init()
    return a.filter(|x: u8| x > 2).take(3).sum(8)
}
assert newc() == 12
`
	lorig, _ := nanz.Parse(lambdaSrc, "rtl")
	ldirect, _ := nanz.Parse(lambdaSrc, "rtl")
	lsteps, err := pipeline.CompileHIRSteps(ldirect, pipeline.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	reread["nanz"] = func() (*hir.Module, error) {
		m, err := hir.ParseHIR(lsteps.HIR, "rtl")
		if err != nil {
			return nil, err
		}
		return nanz.Parse(nanz.Print(m), "rtl")
	}
	for format, read := range reread {
		orig, steps := orig, steps
		if format == "nanz" {
			orig, steps = lorig, lsteps
		}
		t.Run(format, func(t *testing.T) {
			m, err := read()
			if err != nil {
				t.Fatal(err)
			}
			m.Asserts = orig.Asserts
			if err := pipeline.RunAssertsMIR2(m, hir.LowerModule(m)); err != nil {
				t.Fatalf("raw MIR2 VM: %v", err)
			}
			m2, _ := read()
			m2.Asserts = orig.Asserts
			s2, err := pipeline.CompileHIRSteps(m2, pipeline.DefaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			if err := pipeline.RunAssertsMIR2(m2, s2.MIR2Module); err != nil {
				t.Fatalf("optimised MIR2 VM: %v", err)
			}
			if err := pipeline.RunAssertsZ80(m2, s2.MIR2Module, s2.Allocation, s2.Assembly); err != nil {
				t.Fatalf("z80: %v", err)
			}
			if s2.Assembly != steps.Assembly {
				t.Fatalf("%s round trip changed the assembly", format)
			}
		})
	}
	// Lanz (and Lizp, which shares its reader): the source spelling.
	for name, compile := range map[string]func(string, string) (*hir.Module, error){"lanz": lanz.Compile, "lizp": lizp.Compile} {
		t.Run(name+"-source", func(t *testing.T) {
			lsrc := `
(global a (u8 8) (array 1 2 3 4 5 6 7 8))
(fun gt2 ((x u8)) bool (return (> x 2)))
(fun newc () u16 (return (.sum (.take (filter a gt2) 3) 8)))
(fun plain ((p u8) (q u8)) u16 (return (cast (+ p q) u16)))
(fun calls () u16 (return (plain 3 4)))
(assert newc == 12)
(assert calls == 7)
`
			m, err := compile(lsrc, "l")
			if err != nil {
				t.Fatal(err)
			}
			if err := pipeline.RunAssertsMIR2(m, hir.LowerModule(m)); err != nil {
				t.Fatalf("raw MIR2 VM: %v", err)
			}
			m2, _ := compile(lsrc, "l")
			s2, err := pipeline.CompileHIRSteps(m2, pipeline.DefaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			if err := pipeline.RunAssertsZ80(m2, s2.MIR2Module, s2.Allocation, s2.Assembly); err != nil {
				t.Fatalf("z80: %v", err)
			}
			if dumped := lanz.Dump(m2); !strings.Contains(dumped, "(.sum (.take") {
				t.Fatalf("lanz dump lacks the method mark:\n%s", dumped)
			}
		})
	}
	// A plain call to a function or label named sum is spelled without the
	// mark in every format and reads back as a plain call.
	plain := &hir.Module{Funcs: []*hir.Func{{Name: "f", RetTy: mir2.TyU16, Body: hir.Blk(&hir.ReturnStmt{Val: &hir.CallExpr{Fn: "sum", Args: []hir.Expr{&hir.IntLitExpr{Val: 4, Ty: mir2.TyU8}}, Ty: mir2.TyU16}})}}}
	if d := plain.Dump(); !strings.Contains(d, "call @sum(") || strings.Contains(d, "@.sum") {
		t.Fatalf("plain call dump: %s", d)
	}
	back, err := hir.ParseHIR(plain.Dump(), "plain")
	if err != nil {
		t.Fatal(err)
	}
	if c := back.Funcs[0].Body.Body[0].(*hir.ReturnStmt).Val.(*hir.CallExpr); c.Method {
		t.Fatal("a plain call read back as a method call")
	}
	if d := lanz.Dump(plain); !strings.Contains(d, "(sum 4)") {
		t.Fatalf("plain call lanz dump: %s", d)
	}
}
