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

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
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
		{"return src.skip(2).count()", `needs the element count`},
		{"src.forEach(step)\n return 0", `unsupported iterator chain .forEach`},
		{"return src.fold(u16(0), add16)", `needs the element count`},
		{"src.zip(oth).mapInPlace(|x: u8| x, n)\n return 0", `.mapInPlace does not support .zip`},
		{"src.filter(f).skip(1).mapInPlace(|x: u8| x, n)\n return 0", `.mapInPlace supports .skip only directly on the source`},
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
			if !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "line 8") {
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
