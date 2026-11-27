package pipeline

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

const unrollNativeC = `
volatile unsigned char port;
unsigned char crc(unsigned char crc, unsigned char poly, unsigned char reps) {
    unsigned char r, bit;
    for (r = 0; r < reps; r++)
        for (bit = 0; bit < 8; bit++) {
            if (crc & 128) crc = (crc << 1) ^ poly;
            else crc = crc << 1;
        }
    return crc;
}
unsigned char pop(unsigned char v, unsigned char reps) {
    unsigned char r, bit, w;
    unsigned short c = 0;
    for (r = 0; r < reps; r++) {
        c = 0; w = v;
        for (bit = 0; bit < 8; bit++) { c += w & 1; w = w >> 1; }
    }
    return c;
}
unsigned char rev(unsigned char m, unsigned char reps) {
    unsigned char r, bit, x = 0;
    for (r = 0; r < reps; r++) {
        x = 0;
        for (bit = 0; bit < 8; bit++) if ((m >> bit) & 1) x |= 1 << (7 - bit);
    }
    return x;
}
unsigned short mul(unsigned char a, unsigned char b, unsigned char reps) {
    unsigned char r, bit, bb;
    unsigned short p = 0, aa;
    for (r = 0; r < reps; r++) {
        p = 0; aa = a; bb = b;
        for (bit = 0; bit < 8; bit++) { if (bb & 1) p += aa; aa = aa << 1; bb = bb >> 1; }
    }
    return p;
}
unsigned char reads(unsigned char n) {
    unsigned char r, b, s = 0;
    for (r = 0; r < n; r++) for (b = 0; b < 3; b++) s = s + port;
    return s;
}
`

const unrollNativeNanz = `
fun crc(c: u8, poly: u8, reps: u8) -> u8 {
    var x: u8 = c
    for r in 0..reps {
        for b in 0..8 { if (x & 128) != 0 { x = (x << 1) xor poly } else { x = x << 1 } }
    }
    return x
}
fun pop(v: u8, reps: u8) -> u8 {
    var c: u8 = 0
    var w: u8 = v
    for r in 0..reps {
        c = 0
        w = v
        for b in 0..8 { c = c + (w & 1)
          w = w >> 1 }
    }
    return c
}
fun rev(m: u8, reps: u8) -> u8 {
    var x: u8 = 0
    for r in 0..reps {
        x = 0
        for b in 0..8 { if ((m >> b) & 1) == 1 { x = x | (128 >> b) } }
    }
    return x
}
fun mul(a: u8, b: u8, reps: u8) -> u16 {
    var p: u16 = 0
    for r in 0..reps {
        p = 0
        var aa: u16 = a as u16
        var bb: u8 = b
        for i in 0..8 { if (bb & 1) == 1 { p = p + aa }
          aa = aa << 1
          bb = bb >> 1 }
    }
    return p
}
`

func unrollOracle(name string, a []int64) int64 {
	switch name {
	case "crc":
		c := byte(a[0])
		for r := int64(0); r < a[2]; r++ {
			for b := 0; b < 8; b++ {
				if c&128 != 0 {
					c = c<<1 ^ byte(a[1])
				} else {
					c <<= 1
				}
			}
		}
		return int64(c)
	case "pop", "rev":
		if a[1] == 0 {
			return 0
		}
		v, x := byte(a[0]), byte(0)
		for b := 0; b < 8; b++ {
			if v>>b&1 != 0 {
				if name == "pop" {
					x++
				} else {
					x |= 1 << (7 - b)
				}
			}
		}
		return int64(x)
	case "mul":
		if a[2] == 0 {
			return 0
		}
		return a[0] * a[1]
	}
	panic(name)
}

func unrollNativeCalls() map[string][][]int64 {
	calls := map[string][][]int64{}
	for v := int64(0); v < 256; v++ {
		for _, poly := range []int64{0x07, 0x31, 0x1d, 0x9b} {
			calls["crc"] = append(calls["crc"], []int64{v, poly, 1})
		}
		calls["pop"] = append(calls["pop"], []int64{v, 1})
		calls["rev"] = append(calls["rev"], []int64{v, 1})
		calls["mul"] = append(calls["mul"], []int64{v, (v * 37) & 255, 1})
	}
	calls["crc"] = append(calls["crc"], []int64{0x5a, 7, 0}, []int64{0x5a, 7, 3})
	calls["pop"] = append(calls["pop"], []int64{0xff, 0}, []int64{0xa5, 4})
	calls["rev"] = append(calls["rev"], []int64{0x01, 0}, []int64{0x35, 2})
	calls["mul"] = append(calls["mul"], []int64{255, 255, 2}, []int64{9, 9, 0})
	return calls
}

// Unrolled short loops against an oracle on the raw VM (both lowerings), the
// optimised VM and the Z80, in the default, grace and size profiles. Default
// and grace must actually unroll the inner loops; size must not.
func TestUnrollConstantTripNative(t *testing.T) {
	grace := DefaultOptions()
	grace.UseHIRGrace = true
	size := DefaultOptions()
	size.OptSize = true
	forced := DefaultOptions()
	forced.unrollUngated = true
	modes := []struct {
		name   string
		opt    Options
		unroll bool
	}{{"default", DefaultOptions(), true}, {"grace", grace, true}, {"size", size, false}, {"ungated", forced, true}}
	calls := unrollNativeCalls()
	for _, front := range []string{"c89", "nanz"} {
		parse := func() *hir.Module {
			var h *hir.Module
			var err error
			if front == "c89" {
				h, err = c89.CompileWithOpts(unrollNativeC, "unroll.c", c89.CompileOpts{NoLibc: true})
			} else {
				h, err = nanz.Parse(unrollNativeNanz, "unroll.nanz")
			}
			if err != nil {
				t.Fatal(err)
			}
			return h
		}
		var raws []*mir2.Module
		for _, on := range []bool{false, true} {
			raws = append(raws, hir.LowerModuleWithOptions(parse(), hir.LowerOptions{UnrollLoops: on}))
		}
		for _, mode := range modes {
			h := parse()
			opt := mode.opt
			opt.AssertMode = "none"
			s, err := CompileHIRSteps(h, opt)
			if err != nil {
				t.Fatalf("%s %s: %v", front, mode.name, err)
			}
			for _, name := range []string{"crc", "pop", "rev", "mul"} {
				f := s.MIR2Module.FuncByName(name)
				if f == nil {
					t.Fatalf("%s %s: %s missing", front, mode.name, name)
				}
				// Unrolling is a measured choice (bytes and static T); the CRC
				// step must win it, the size profile must never unroll.
				unrolled := len(f.StaticLoops) == 1
				t.Logf("%s %s %s: unrolled=%v", front, mode.name, name, unrolled)
				if unrolled && !mode.unroll || (name == "crc" || mode.name == "ungated") && unrolled != mode.unroll {
					t.Errorf("%s %s %s: %d static loops, unroll expected %v", front, mode.name, name, len(f.StaticLoops), mode.unroll)
				}
				for i, a := range calls[name] {
					want := unrollOracle(name, a)
					args := make([]mir2.Value, len(a))
					for j, v := range a {
						args[j] = mir2.Value{I: v}
					}
					vms := []*mir2.Module{s.MIR2Module}
					if mode.name == "default" {
						vms = append(vms, raws...)
					}
					for k, m := range vms {
						got, err := mir2.NewVM(m).Call(name, args)
						if err != nil || len(got) != 1 || got[0].I != want {
							t.Fatalf("%s %s %s%v VM %d: %v %v want %d", front, mode.name, name, a, k, got, err, want)
						}
					}
					// Every input on the Z80 for crc (the 256 x 4 table);
					// a stride of the others keeps the test fast.
					if name == "crc" || i%5 == 0 || i >= 256 {
						measureGraceZ80Compiled(t, h, s, name, a, nil, want, 200000)
					}
				}
			}
		}
	}
}

// Volatile reads keep their count through the whole optimiser: three
// unrolled iterations of one read are three volatile loads.
func TestUnrollVolatileCountNative(t *testing.T) {
	h, err := c89.CompileWithOpts(unrollNativeC, "unroll.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	opt := DefaultOptions()
	opt.AssertMode = "none"
	s, err := CompileHIRSteps(h, opt)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, b := range s.MIR2Module.FuncByName("reads").Blocks {
		for _, i := range b.Insts {
			if i.Op == mir2.OpLoad && i.Volatile {
				n++
			}
		}
	}
	if n != 1 && n != 3 {
		t.Fatalf("volatile loads %d", n)
	}
	if len(s.MIR2Module.FuncByName("reads").StaticLoops) == 1 && n != 3 {
		t.Fatalf("unrolled reads has %d volatile loads, want 3", n)
	}
}

// The size profile never unrolls: its product is byte-identical to one
// compiled with unrolling disabled.
func TestUnrollSizeProfileUnchanged(t *testing.T) {
	compile := func(disable bool) string {
		h, err := c89.CompileWithOpts(unrollNativeC, "unroll.c", c89.CompileOpts{NoLibc: true})
		if err != nil {
			t.Fatal(err)
		}
		opt := DefaultOptions()
		opt.OptSize, opt.AssertMode, opt.DisableLoopUnroll = true, "none", disable
		s, err := CompileHIRSteps(h, opt)
		if err != nil {
			t.Fatal(err)
		}
		return s.Assembly
	}
	if compile(false) != compile(true) {
		t.Fatal("size profile changed by unrolling")
	}
}

// A body whose copies exceed the measured byte budget keeps its loop.
func TestUnrollByteBudgetRefuses(t *testing.T) {
	const src = `
unsigned char big(unsigned char n, unsigned char x) {
    unsigned char r, i, y = 1, z = 2, w = 3;
    for (r = 0; r < n; r++)
        for (i = 0; i < 8; i++) {
            y = (y ^ x) + (z >> 2); z = (z + y) ^ (w << 1); w = (w - z) ^ (x >> 3);
            x = (x + w) ^ (y << 2); y = (y + z) ^ (w >> 1); z = (z ^ x) + (y << 3);
            w = (w + y) ^ (z >> 1); x = (x ^ z) + (w << 1);
        }
    return x + y + z + w;
}
`
	h, err := c89.CompileWithOpts(src, "big.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, unrolled := lowerUnrollProbe(h); unrolled["big"] != 1 {
		t.Fatalf("lowering must admit the loop before the byte gate: %v", unrolled)
	}
	opt := DefaultOptions()
	opt.AssertMode = "none"
	s, err := CompileHIRSteps(h, opt)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(s.MIR2Module.FuncByName("big").StaticLoops); got != 2 {
		t.Fatalf("over-budget loop unrolled (%d static loops)", got)
	}
}

func lowerUnrollProbe(h *hir.Module) (*mir2.Module, map[string]int) {
	u := map[string]int{}
	return hir.LowerModuleWithOptions(hir.CloneModule(h), hir.LowerOptions{UnrollLoops: true, Unrolled: u}), u
}

func unrollFunctionAsm(t *testing.T, asm, name string) string {
	t.Helper()
	start := strings.Index(asm, "\n"+name+":\n")
	if start < 0 {
		t.Fatalf("%s not emitted", name)
	}
	body := asm[start+1:]
	if end := strings.Index(body, "\n; fun "); end >= 0 {
		body = body[:end]
	}
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if l := strings.TrimSpace(line); l != "" && !strings.HasPrefix(l, ";") {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// The copies must be as good as the loop body was: eight ADD A,A / JR NC /
// XOR for the CRC step (no SLA r, no edge copy, no counter), and eight fused
// SRL / JR NC / INC steps for the population count (no AND 1 fallback).
func TestUnrollCopiesKeepLoopBodyQuality(t *testing.T) {
	h, err := c89.CompileWithOpts(unrollNativeC, "unroll.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	opt := DefaultOptions()
	opt.AssertMode = "none"
	s, err := CompileHIRSteps(h, opt)
	if err != nil {
		t.Fatal(err)
	}
	crc := unrollFunctionAsm(t, s.Assembly, "crc")
	if n := strings.Count(crc, "ADD A, A"); n != 8 || strings.Contains(crc, "SLA") || strings.Count(crc, "DJNZ") > 1 {
		t.Errorf("crc copies degraded (%d ADD A,A):\n%s", n, crc)
	}
	// The crc8 benchmark's shape: a constant polynomial over a byte table.
	h, err = c89.CompileWithOpts(`unsigned char values[256], n;
unsigned short result;
void execute(void) {
    unsigned char crc = 0;
    unsigned char i, bit;
    for (i = 0; i < n; i++) {
        crc ^= values[i];
        for (bit = 0; bit < 8; bit++) {
            if (crc & 128) crc = (crc << 1) ^ 7;
            else crc = crc << 1;
        }
    }
    result = crc;
}`, "crc8.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	if s, err = CompileHIRSteps(h, opt); err != nil {
		t.Fatal(err)
	}
	crc = unrollFunctionAsm(t, s.Assembly, "execute")
	if n := strings.Count(crc, "ADD A, A"); n != 8 || strings.Contains(crc, "SLA") || strings.Contains(crc, "DJNZ") {
		t.Errorf("crc8 copies degraded (%d ADD A,A):\n%s", n, crc)
	}
	// The popcount8 benchmark's shape (a word count over a byte table).
	h, err = c89.CompileWithOpts(`unsigned char values[256], n;
unsigned short result;
void execute(void) {
    unsigned char i, bit, value;
    unsigned short count = 0;
    for (i = 0; i < n; i++) {
        value = values[i];
        for (bit = 0; bit < 8; bit++) { count += value & 1; value = value >> 1; }
    }
    result = count;
}`, "pop.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	if s, err = CompileHIRSteps(h, opt); err != nil {
		t.Fatal(err)
	}
	pop := unrollFunctionAsm(t, s.Assembly, "execute")
	if n := strings.Count(pop, "SRL"); n != 8 || strings.Contains(pop, "AND 1") || strings.Contains(pop, "PUSH") {
		t.Errorf("pop copies degraded (%d SRL):\n%s", n, pop)
	}
}

// Measured or replayed loop configurations keep their loops.
func TestUnrollRefusedForLoopSiteCollection(t *testing.T) {
	h, err := c89.CompileWithOpts(unrollNativeC, "unroll.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	opt := DefaultOptions()
	opt.AssertMode, opt.CollectHIRSites = "none", true
	s, err := CompileHIRSteps(h, opt)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(s.MIR2Module.FuncByName("crc").StaticLoops); got != 2 {
		t.Fatalf("loop-site collection unrolled crc (%d static loops)", got)
	}
}

// The gate charges a function its own code only: unrolling here turns
// x * i into constant multiplies and drops the shared square table, which
// once made a 94-byte growth of big's own code look like a saving.
func TestUnrollGateChargesOwnCode(t *testing.T) {
	const src = `
unsigned char big(unsigned char x, unsigned char n) {
    unsigned char r, i, y = 1, z = 2, w = 3;
    for (r = 0; r < n; r++)
        for (i = 0; i < 8; i++) {
            y = (y ^ x) + (z >> 2); z = (z + y) ^ (w << 1); w = w - (y >> 1);
            y = y + (unsigned char)(x * i);
        }
    return w + y + z;
}
`
	h, err := c89.CompileWithOpts(src, "qsq.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	o := DefaultOptions()
	o.AssertMode = "none"
	if admitted, err := unrollAdmissions(h, o); err != nil || admitted["big"] != 1 {
		t.Fatalf("lowering must admit the loop: %v %v", admitted, err)
	}
	decided, err := decideUnroll(h, o)
	if err != nil {
		t.Fatal(err)
	}
	if !decided.DisableLoopUnroll && !decided.unrollDeny["big"] {
		t.Fatal("over-budget unroll admitted")
	}
	u := DefaultOptions()
	u.AssertMode, u.unrollUngated = "none", true
	s, err := CompileHIRSteps(h, u)
	if err != nil {
		t.Fatal(err)
	}
	b := DefaultOptions()
	b.AssertMode, b.DisableLoopUnroll = "none", true
	base, err := CompileHIRSteps(h, b)
	if err != nil {
		t.Fatal(err)
	}
	afterImage, err := assembleVariant(s)
	if err != nil {
		t.Fatal(err)
	}
	beforeImage, err := assembleVariant(base)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := ownCodeBytes(afterImage)
	before, _ := ownCodeBytes(beforeImage)
	t.Logf("big own code %d -> %d bytes", before["big"], after["big"])
	if after["big"]-before["big"] <= 48 {
		t.Fatalf("fixture no longer exceeds the budget: %d -> %d", before["big"], after["big"])
	}
}

// Unrolling this inner loop yields a block-scoped declaration after a
// diamond, which the MIR pipeline of origin/main cannot compile (the same
// code written by hand fails with "MIR2 verify: ... use of undefined"). The
// gate keeps the loop instead of turning a compiling module into an error.
func TestUnrollCandidateErrorKeepsLoops(t *testing.T) {
	const src = `
unsigned short f1(unsigned char a, unsigned char b) {
    unsigned char x = a, y = b, z = a, r, i;
    unsigned short h = 1;
    for (r = 0; r < 1; r++) {
        if (x & 128) x = (x << 1) ^ y; else x = x << 1;
        for (i = 0; i != 5; i++) { { unsigned char t = ((121 & 64) ? x : y); y ^= t; } h = h + (x | (i + z)); x = (y >> 3); x += i; }
    }
    return h + x + y + z;
}
`
	h, err := c89.CompileWithOpts(src, "verify.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	raw := hir.LowerModule(hir.CloneModule(h))
	u := DefaultOptions()
	u.AssertMode, u.unrollUngated = "none", true
	if _, err := CompileHIRSteps(hir.CloneModule(h), u); err == nil {
		t.Log("the unrolled candidate compiles now; the fixture no longer exercises the fallback")
	}
	o := DefaultOptions()
	o.AssertMode = "none"
	s, err := CompileHIRSteps(h, o)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range [][]int64{{0, 0}, {200, 7}, {129, 255}} {
		want, _ := mir2.NewVM(raw).Call("f1", []mir2.Value{{I: a[0]}, {I: a[1]}})
		_, got, err := RunZ80Cycles(h, s.MIR2Module, s.Allocation, s.Assembly, "f1", a)
		if err != nil || got != want[0].I&0xffff {
			t.Errorf("f1%v: Z80 %d %v want %d", a, got, err, want[0].I)
		}
	}
}

// The static model scores neither product of this module (T 0 -> 0); the
// gate refuses it. (Whether the "unscored" or the "slower" clause refuses it
// is not observable here; TestUnrollVerdict checks each clause.)
func TestUnrollGateRefusesUnscorableModule(t *testing.T) {
	const src = `
unsigned char d(unsigned char a, unsigned char n) {
    unsigned char r, b, x = a, y = 1;
    for (r = 0; r < n; r++) for (b = 0; b < 4; b++) { y = y + x; x = x ^ y; }
    return x + y;
}
`
	h, err := c89.CompileWithOpts(src, "d.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	o := DefaultOptions()
	o.AssertMode = "none"
	if admitted, err := unrollAdmissions(h, o); err != nil || admitted["d"] != 1 {
		t.Fatalf("lowering must admit the loop: %v %v", admitted, err)
	}
	decided, err := decideUnroll(h, o)
	if err != nil {
		t.Fatal(err)
	}
	if !decided.DisableLoopUnroll && !decided.unrollDeny["d"] {
		t.Fatal("unscorable unroll admitted")
	}
}

// The gate's trial compiles print nothing: a module with an admitted loop
// and a high-pressure split prints exactly what the same compilation prints
// with unrolling disabled (review: [HIR-SPLIT] lines were repeated).
func TestUnrollGateTrialsAreQuiet(t *testing.T) {
	const src = `// assert f0(1, 221) == 62704 via z80
// assert f0(146, 241) == 1702 via z80
// assert f0(159, 73) == 24228 via z80
// assert f0(84, 244) == 3026 via z80
// assert f0(254, 169) == 49038 via z80
// assert f0(0, 0) == 6042 via z80
// assert f0(255, 255) == 5348 via z80
// assert f1(1, 221) == 206 via z80
// assert f1(146, 241) == 47 via z80
// assert f1(159, 73) == 16 via z80
// assert f1(84, 244) == 25 via z80
// assert f1(254, 169) == 91 via z80
// assert f1(0, 0) == 55 via z80
// assert f1(255, 255) == 136 via z80
// assert f2(1, 221) == 60752 via z80
// assert f2(146, 241) == 11218 via z80
// assert f2(159, 73) == 34500 via z80
// assert f2(84, 244) == 6336 via z80
// assert f2(254, 169) == 29802 via z80
// assert f2(0, 0) == 29680 via z80
// assert f2(255, 255) == 65288 via z80
#define c8(e) ((unsigned char)(e))
unsigned char ga[256];
volatile unsigned char gv;
unsigned short f0(unsigned char a, unsigned char b) {
  unsigned char x = a, y = b, z = (unsigned char)(a ^ 183), o, i, j, ki, kj;
  unsigned short w = 3801, wi, wj;
  unsigned char *p = &ga[238];
  gv = 127;
  { unsigned short q; for (q = 0; q < 256; q++) ga[q] = (unsigned char)(q * 167 + 26); }
  for (o = 0; o < (unsigned char)((a & 3) + 1); o++) {
    ga[y] = (unsigned char)((y) > (y));
    for (i = 4; i < 12; i++) {
      x = i;
    }
    x = (unsigned char)(x + i);
  }
  return (unsigned short)(w + x + (y << 8) + z * 3 + gv + ga[2] + ga[4] * 7 + ga[6] + ga[z] + ga[253]);
}
unsigned char f1(unsigned char a, unsigned char b) {
  unsigned char x = a, y = b, z = (unsigned char)(a ^ 104), o, i, j, ki, kj;
  unsigned short w = 23811, wi, wj;
  unsigned char *p = &ga[100];
  gv = 196;
  { unsigned short q; for (q = 0; q < 256; q++) ga[q] = (unsigned char)(q * 77 + 11); }
  o = 0; while (o < 3) {
    o++;
    for (i = 0; i <= 5; i++) {
      if ((y) < ((unsigned char)((unsigned char)(i * x) | z))) {
        if (((unsigned char)((unsigned char)((y) > (i)) >> 1)) > ((unsigned char)((unsigned char)(y * x) << 5))) {
          x = ga[z];
          z = (unsigned char)((unsigned char)(i | (unsigned char)(x | x)) | (((unsigned char)(i + y)) > ((unsigned char)(i ^ y)) ? x : c8(w)));
        }
        y = (unsigned char)((unsigned char)(z | ((ga[i]) < (ga[i]) ? 91 : c8(w))) + (unsigned char)(ga[i] * (unsigned char)(i + 143)));
      }
      z = (unsigned char)((unsigned char)(((unsigned char)(ga[i] << 7)) != (ga[i])) * (unsigned char)((unsigned char)(186 ^ z) ^ x));
    }
  }
  return (unsigned char)(x + y * 3 + z * 7 + c8(w) + (w >> 8) + gv + ga[0] + ga[1] * 2 + ga[5] + ga[7] * 5 + ga[3] + ga[x] + ga[(unsigned char)(y + 1)] * 3 + ga[254]);
}
unsigned short f2(unsigned char a, unsigned char b) {
  unsigned char x = a, y = b, z = (unsigned char)(a ^ 144), o, i, j, ki, kj;
  unsigned short w = 10333, wi, wj;
  unsigned char *p = &ga[213];
  gv = 87;
  { unsigned short q; for (q = 0; q < 256; q++) ga[q] = (unsigned char)(q * 159 + 215); }
  for (o = 0; o < 3; o++) {
    if ((gv) && ((unsigned char)((unsigned char)(c8(w) ^ 241) & (unsigned char)(68 ^ 92)))) {
      x = (unsigned char)((unsigned char)((191) != (142)) * (unsigned char)((unsigned char)(z * c8(w)) ^ ga[z]));
      y = (unsigned char)(y + x);
    } else {
      ga[z] = (unsigned char)((unsigned char)((unsigned char)(z * 55) ^ (unsigned char)((ga[z]) != (c8(w)))) << 6);
      w = (unsigned short)((unsigned short)w << 8);
    }
    for (i = 2; i < 7; i++) {
      x = (unsigned char)(x + ga[i]);
      x = (unsigned char)(ga[i] >> 0);
    }
    x = (unsigned char)(x + i);
    for (j = 5; j < 10; j++) {
      ga[j] = j;
      ga[j] = (unsigned char)((unsigned char)((j) != ((unsigned char)(z - j))) * (unsigned char)((unsigned char)(c8(w) + y) ^ (unsigned char)(x + y)));
    }
    x = (unsigned char)(x + j);
  }
  return (unsigned short)(w + x + (y << 8) + z * 3 + gv + ga[2] + ga[4] * 7 + ga[6] + ga[z] + ga[253]);
}
`
	capture := func(disable bool) string {
		h, err := c89.CompileWithOpts(src, "u5.c", c89.CompileOpts{NoLibc: true})
		if err != nil {
			t.Fatal(err)
		}
		if !disable {
			o := DefaultOptions()
			o.AssertMode = "none"
			if admitted, err := unrollAdmissions(h, o); err != nil || len(admitted) == 0 {
				t.Fatalf("fixture must admit a loop: %v %v", admitted, err)
			}
		}
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		stdout, stderr := os.Stdout, os.Stderr
		os.Stdout, os.Stderr = w, w
		done := make(chan string)
		go func() {
			b, _ := io.ReadAll(r)
			done <- string(b)
		}()
		o := DefaultOptions()
		o.AssertMode, o.DisableLoopUnroll = "none", disable
		_, cerr := CompileHIRSteps(h, o)
		os.Stdout, os.Stderr = stdout, stderr
		w.Close()
		out := <-done
		if cerr != nil {
			t.Fatal(cerr)
		}
		return out
	}
	without, with := capture(true), capture(false)
	if !strings.Contains(without, "[HIR-SPLIT]") {
		t.Fatalf("fixture no longer splits:\n%s", without)
	}
	if with != without {
		t.Fatalf("output differs with unrolling enabled:\n--- disabled\n%s--- enabled\n%s", without, with)
	}
}

// Each clause of the per-function verdict and the module image rule.
func TestUnrollVerdict(t *testing.T) {
	for _, c := range []struct {
		measured       bool
		growth, budget int
		slow           int64
		scored         bool
		fast           int64
		scoredAfter    bool
		want           string
	}{
		{true, 20, 48, 100, true, 60, true, ""},
		{false, 20, 48, 100, true, 60, true, "unmeasured"},
		{true, 49, 48, 100, true, 60, true, "bytes"},
		// A base the model scores and a candidate it cannot: never admitted,
		// although the unscored candidate's 0 is below the base's T.
		{true, 20, 48, 100, true, 0, false, "unscored"},
		{true, 20, 48, 0, false, 60, true, "unscored"},
		{true, 20, 48, 100, true, 100, true, "slower"},
	} {
		if got := unrollVerdict(c.measured, c.growth, c.budget, c.slow, c.scored, c.fast, c.scoredAfter); got != c.want {
			t.Errorf("%+v: %q, want %q", c, got, c.want)
		}
	}
	// The image may grow by the admitted functions' own growth, not by
	// their unused budgets: 5 bytes of own growth admit 5, not 48.
	if !unrollImageAdmits(5, 5) || unrollImageAdmits(6, 5) || unrollImageAdmits(48, 5) {
		t.Error("image rule")
	}
}
