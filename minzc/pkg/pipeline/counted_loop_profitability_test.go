package pipeline

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

const countedLoopProbeSource = `
fun m8(n: u8, k: u8) -> u8 {
    var s: u8 = 0
    for i in 0..n { s = s + i * k }
    return s
}
fun rng(a: u8, b: u8) -> u16 {
    var s: u16 = 0
    for i in a..b { s = s + i as u16 }
    return s
}
fun cnt(n: u8) -> u8 {
    var s: u8 = 0
    for i in 0..n { s = s + 3 }
    return s
}
fun xr(n: u8) -> u8 {
    var s: u8 = 0
    for i in 0..n { s = s xor i }
    return s
}
fun fib(n: u8) -> u16 {
    var a: u16 = 0
    var b: u16 = 1
    for i in 0..n { var t: u16 = a + b
      a = b
      b = t }
    return a
}
fun tri(n: u8) -> u16 {
    var s: u16 = 0
    for i in 0..n { for j in 0..i { s = s + 1 } }
    return s
}
fun many(n: u8, p: u8, q: u8) -> u8 {
    var a: u8 = 0
    var b: u8 = 1
    var c: u8 = 2
    var d: u8 = 3
    for i in 0..n { a = a + p
      b = b xor a
      c = c + b + i
      d = d + q }
    return a + b + c + d
}
`

// The critic's seven-function module must not pay for a second live counter.
// Compare complete modules, so size includes entry paths, edge copies and spills.
func TestCountedLoopProfitabilityVMZ80(t *testing.T) {
	type probe struct {
		name string
		args []int64
	}
	cases := []probe{{"m8", []int64{100, 3}}, {"rng", []int64{5, 200}}, {"cnt", []int64{200}}, {"xr", []int64{200}}, {"fib", []int64{20}}, {"tri", []int64{60}}, {"many", []int64{100, 3, 5}},
		{"m8", []int64{0, 255}}, {"m8", []int64{255, 255}}, {"rng", []int64{200, 5}}, {"rng", []int64{0, 255}}, {"cnt", []int64{0}}, {"cnt", []int64{255}}, {"xr", []int64{255}}, {"fib", []int64{0}}, {"fib", []int64{255}}, {"tri", []int64{0}}, {"tri", []int64{255}}, {"many", []int64{255, 255, 255}}}
	var modules [2]*hir.Module
	var steps [2]Steps
	var raw [2]*mir2.Module
	for i := range steps {
		h, err := nanz.Parse(countedLoopProbeSource, "counted_probes.nanz")
		if err != nil {
			t.Fatal(err)
		}
		modules[i] = h
		raw[i] = hir.LowerModuleWithOptions(h, hir.LowerOptions{DisableCountedLoops: i == 0})
		if err := mir2.Verify(raw[i]); err != nil {
			t.Fatal(err)
		}
		steps[i], err = CompileHIRSteps(h, Options{AssertMode: "none", DisableCountedLoops: i == 0})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range cases {
		want := countedLoopProbeReference(tc.name, tc.args)
		var cycles, sizes [2]int
		for i, s := range steps {
			args := make([]mir2.Value, len(tc.args))
			for j, a := range tc.args {
				args[j] = mir2.Value{I: a}
			}
			got, err := mir2.NewVM(raw[i]).Call(tc.name, args)
			if err != nil || len(got) != 1 || got[0].I != want {
				t.Fatalf("%s %v MIR mode %d: %v %v want %d", tc.name, tc.args, i, got, err, want)
			}
			cycles[i], sizes[i] = measureGraceZ80Compiled(t, modules[i], s, tc.name, tc.args, nil, want, 2000000)
		}
		t.Logf("%s %v: T %d -> %d; module bytes %d -> %d", tc.name, tc.args, cycles[0], cycles[1], sizes[0], sizes[1])
		if cycles[1] > cycles[0] || sizes[1] > sizes[0] {
			t.Errorf("%s regressed: T %v bytes %v", tc.name, cycles, sizes)
		}
	}
}

// This independent shape check also runs on origin/main without option plumbing.
func TestCountedLoopFixedProfitability(t *testing.T) {
	const src = `fun probe() -> u8 { var s: u8 = 0
 for i in 0..8 { s = s + 3 }
 return s }`
	h, err := nanz.Parse(src, "fixed_count.nanz")
	if err != nil {
		t.Fatal(err)
	}
	m := hir.LowerModule(h)
	found := false
	for _, b := range m.FuncByName("probe").Blocks {
		if _, ok := b.Term.(*mir2.TermDJNZ); ok {
			found = true
		}
	}
	if !found {
		t.Fatal("fixed-count loop must use TermDJNZ")
	}
	got, err := mir2.NewVM(m).Call("probe", nil)
	if err != nil || len(got) != 1 || got[0].I != 24 {
		t.Fatalf("MIR %v %v", got, err)
	}
	steps, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(steps.Assembly, "DJNZ") {
		t.Fatal("missing emitted DJNZ")
	}
	measureGraceZ80Compiled(t, h, steps, "probe", nil, nil, 24, 10000)
}

func countedLoopProbeReference(f string, a []int64) int64 {
	switch f {
	case "m8":
		s := byte(0)
		for i := int64(0); i < a[0]; i++ {
			s += byte(i) * byte(a[1])
		}
		return int64(s)
	case "rng":
		s := uint16(0)
		for i := a[0]; i < a[1]; i++ {
			s += uint16(i)
		}
		return int64(s)
	case "cnt":
		return int64(byte(a[0] * 3))
	case "xr":
		s := byte(0)
		for i := int64(0); i < a[0]; i++ {
			s ^= byte(i)
		}
		return int64(s)
	case "fib":
		x, y := uint16(0), uint16(1)
		for i := int64(0); i < a[0]; i++ {
			x, y = y, x+y
		}
		return int64(x)
	case "tri":
		return a[0] * (a[0] - 1) / 2
	case "many":
		A, B, C, D := byte(0), byte(1), byte(2), byte(3)
		for i := int64(0); i < a[0]; i++ {
			A += byte(a[1])
			B ^= A
			C = C + B + byte(i)
			D += byte(a[2])
		}
		return int64(A + B + C + D)
	}
	return 0
}

const countedConstantNanz = `
fun c1() -> u8 { var s: u8 = 0
 for i in 0..8 { s = s + 3 }
 return s }
fun c2(k: u8) -> u16 { var s: u16 = 0
 for i in 1..200 { s = s + k as u16 }
 return s }
fun c3(k: u8) -> u8 { var s: u8 = 1
 for i in 0..255 { s = s * k + 1 }
 return s }
fun c4(x: u8, k: u8) -> u16 { var s: u16 = 0
 var y: u8 = x
 for i in 0..100 { y = y / k + x
  s = s + y as u16 }
 return s }
fun c5() -> u16 { var s: u16 = 0
 for i in 0..16 { for j in 0..16 { s = s + 1 } }
 return s }
fun c6(k: u16) -> u16 { var s: u16 = 0
 for i in 0..256 { s = s + k }
 return s }
fun c6b(k: u16) -> u16 { var s: u16 = 0
 for i in 300..555 { s = s + k }
 return s }
fun g(x: u8) -> u8 { return x + 1 }
fun c7() -> u8 { var s: u8 = 0
 for i in 0..10 { s = g(s) }
 return s }
fun c8(c: u8) -> u8 { var x: u8 = c
 for i in 0..8 { if (x & 0x80) != 0 { x = (x << 1) xor 0x07 } else { x = x << 1 } }
 return x }
fun c9(k: u16) -> u16 { var s: u16 = 0
 for i in 0..50 { s = s * 3 + k }
 return s }
fun c10(p: u8, q: u8) -> u8 { var a: u8 = 0
 var b: u8 = 1
 var c: u8 = 2
 var d: u8 = 3
 for i in 0..100 { a = a + p
  b = b xor a
  c = c + b
  d = d + q }
 return a + b + c + d }
fun c11() -> u16 { var a: u16 = 0
 var b: u16 = 1
 for i in 0..20 { var t: u16 = a + b
  a = b
  b = t }
 return a }
fun c12(k: u8) -> u8 { var s: u8 = 0
 for i in 0..1 { s = s + k }
 for j in 5..5 { s = s + 100 }
 return s }
fun c13() -> u8 { var s: u8 = 0
 var i: u8 = 0
 while i < 10 { s = s + 2
  i = i + 1 }
 return s + i }
fun c14() -> u16 { var s: u16 = 0
 var i: u8 = 250
 while i != 4 { s = s + 1
  i = i + 1 }
 return s * 256 + i as u16 }
fun c15(k: u16) -> u16 { var s: u16 = 0
 for i in 0..255 { s = s + k * 5 }
 return s }
fun c16(x: u8) -> u8 { var s: u8 = x
 for i in 0..200 { s = s % 7 + s }
 return s }
fun c17() -> u16 { var s: u16 = 0
 for i in 0..10 { for j in 0..i { s = s + 1 } }
 return s }
fun c18() -> u16 { var s: u16 = 0
 var i: u8 = 0
 while i < 10 { var j: u8 = 0
  while j < 4 { s = s + i as u16
   j = j + 1 }
  i = i + 1 }
 return s + i as u16 }
`

const countedConstantC = `
unsigned char f1(void){unsigned char i,s=0; for(i=0;i<10;i++){s+=3;} return s+i;}
unsigned short f2(unsigned char k){unsigned char i; unsigned short s=0; for(i=0;i!=200;i++){s+=k;} return s+i;}
unsigned char f3(unsigned char c){unsigned char i; for(i=0;i<8;i++){ if(c&0x80) c=(c<<1)^7; else c<<=1;} return c;}
unsigned char f4(void){unsigned char i=10,s=0; for(;i!=5;i++){s++;} return s+i;}
unsigned char f5(void){unsigned char i,s=0; for(i=0;i<10;i++){s += (s>3) ? i : 1;} return s;}
unsigned short f6(unsigned short k){unsigned int i; unsigned short s=0; for(i=1000;i<1200;i++){s+=k;} return s+i;}
unsigned char f7(void){unsigned char i,j,s=0; for(i=0;i<5;i++){for(j=0;j<i;j++){s++;}} return s+i+j;}
unsigned char f8(void){unsigned char i,s=0; unsigned char a[4]={1,2,3,4}; for(i=0;i<4;i++){s+=a[i];} return s;}
unsigned char f9(void){unsigned char i,s=0,b; for(i=0;i<4;i++){for(b=0;b<3;b++){s+=2;}} return s+b+i;}
`

func countedConstantReference(name string, a []int64) int64 {
	switch name {
	case "c1":
		return 24
	case "c2":
		return 199 * a[0]
	case "c3":
		s := byte(1)
		for i := 0; i < 255; i++ {
			s = s*byte(a[0]) + 1
		}
		return int64(s)
	case "c4":
		s, y := uint16(0), byte(a[0])
		for i := 0; i < 100; i++ {
			y = y/byte(a[1]) + byte(a[0])
			s += uint16(y)
		}
		return int64(s)
	case "c5":
		return 256
	case "c6":
		return int64(uint16(256 * a[0]))
	case "c6b":
		return int64(uint16(255 * a[0]))
	case "c7":
		return 10
	case "c8", "f3":
		x := byte(a[0])
		for i := 0; i < 8; i++ {
			if x&0x80 != 0 {
				x = (x << 1) ^ 7
			} else {
				x <<= 1
			}
		}
		return int64(x)
	case "c9":
		s := uint16(0)
		for i := 0; i < 50; i++ {
			s = s*3 + uint16(a[0])
		}
		return int64(s)
	case "c10":
		A, B, C, D := byte(0), byte(1), byte(2), byte(3)
		for i := 0; i < 100; i++ {
			A += byte(a[0])
			B ^= A
			C = C + B
			D += byte(a[1])
		}
		return int64(A + B + C + D)
	case "c11":
		x, y := uint16(0), uint16(1)
		for i := 0; i < 20; i++ {
			x, y = y, x+y
		}
		return int64(x)
	case "c12":
		return a[0] & 255
	case "c13":
		return 30
	case "c14":
		return 10*256 + 4
	case "c15":
		return int64(uint16(255 * 5 * a[0]))
	case "c16":
		s := byte(a[0])
		for i := 0; i < 200; i++ {
			s = s%7 + s
		}
		return int64(s)
	case "c17":
		return 45
	case "c18":
		return 4*45 + 10
	case "f1":
		return 40
	case "f2":
		return int64(uint16(200*a[0] + 200))
	case "f4":
		return (251 + 5) & 255
	case "f5":
		s := byte(0)
		for i := 0; i < 10; i++ {
			if s > 3 {
				s += byte(i)
			} else {
				s++
			}
		}
		return int64(s)
	case "f6":
		return int64(uint16(200*a[0] + 1200))
	case "f7":
		return 10 + 5 + 4
	case "f8":
		return 10
	case "f9":
		return 24 + 3 + 4
	}
	panic(name)
}

func TestCountedLoopConstantProfitabilityVMZ80(t *testing.T) {
	type pc struct {
		f    string
		args []int64
	}
	nz := []pc{{"c1", nil}, {"c2", []int64{7}}, {"c3", []int64{3}}, {"c4", []int64{200, 3}}, {"c5", nil}, {"c6", []int64{3}}, {"c6b", []int64{3}}, {"c7", nil}, {"c8", []int64{0xA5}}, {"c9", []int64{7}}, {"c10", []int64{3, 5}}, {"c11", nil}, {"c12", []int64{9}}, {"c13", nil}, {"c14", nil}, {"c15", []int64{11}}, {"c16", []int64{5}}, {"c17", nil}, {"c18", nil}}
	cc := []pc{{"f1", nil}, {"f2", []int64{3}}, {"f3", []int64{0xA5}}, {"f4", nil}, {"f5", nil}, {"f6", []int64{3}}, {"f7", nil}, {"f8", nil}, {"f9", nil}}
	run := func(lang string, list []pc) {
		var h *hir.Module
		var err error
		parse := func() {
			if lang == "nanz" {
				h, err = nanz.Parse(countedConstantNanz, "zz.nanz")
			} else {
				h, err = c89.CompileWithOpts(countedConstantC, "zz.c", c89.CompileOpts{NoLibc: true})
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		parse()
		s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
		if err != nil {
			t.Fatal(err)
		}
		raw := hir.LowerModule(h)
		if err := mir2.Verify(raw); err != nil {
			t.Fatal(err)
		}
		old, err := CompileHIRSteps(h, Options{AssertMode: "none", DisableCountedLoops: true})
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range list {
			want := countedConstantReference(tc.f, tc.args)
			args := make([]mir2.Value, len(tc.args))
			for i, a := range tc.args {
				args[i] = mir2.Value{I: a}
			}
			got, err := mir2.NewVM(raw).Call(tc.f, args)
			if err != nil || len(got) != 1 || got[0].I != want {
				t.Fatalf("%s MIR: %v %v want %d", tc.f, got, err, want)
			}
			beforeT, beforeB := measureGraceZ80Compiled(t, h, old, tc.f, tc.args, nil, want, 2000000)
			afterT, afterB := measureGraceZ80Compiled(t, h, s, tc.f, tc.args, nil, want, 2000000)
			t.Logf("%s %s %v: T %d -> %d; module bytes %d -> %d", lang, tc.f, tc.args, beforeT, afterT, beforeB, afterB)
			if afterT > beforeT || afterB > beforeB {
				t.Errorf("%s %s regressed", lang, tc.f)
			}
		}

	}
	run("nanz", nz)
	run("c", cc)
}

// A constant-bound IV used only in a byte recurrence must keep ordinary
// lowering. This shape isolates the unused-IV gate from the endpoint and
// pair-budget gates; removing that gate must change the MIR shape.
func TestCountedLoopUsedIVFallback(t *testing.T) {
	const src = `fun used(k: u8) -> u8 { var s: u8 = 0
 for i in 0..100 { s = s + i * k }
 return s }`
	h, err := nanz.Parse(src, "used_iv.nanz")
	if err != nil {
		t.Fatal(err)
	}
	m := hir.LowerModule(h)
	for _, b := range m.FuncByName("used").Blocks {
		if _, ok := b.Term.(*mir2.TermDJNZ); ok {
			t.Fatal("body uses IV: must retain ordinary lowering")
		}
	}
	want := int64(byte(4950 * 3 & 255))
	got, err := mir2.NewVM(m).Call("used", []mir2.Value{{I: 3}})
	if err != nil || len(got) != 1 || got[0].I != want {
		t.Fatalf("MIR: %v %v", got, err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	measureGraceZ80Compiled(t, h, s, "used", []int64{3}, nil, want, 2000000)
}

func TestCountedLoopBitLoopProfitabilityVMZ80(t *testing.T) {
	const src = `unsigned char popcount8(unsigned char x) {
 unsigned char i, s=0;
 for(i=0;i<8;i++){s += x & 1; x >>= 1;}
 return s;
 }`
	h, err := c89.CompileWithOpts(src, "bit_loop.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	raw := hir.LowerModule(h)
	found := false
	for _, b := range raw.FuncByName("popcount8").Blocks {
		if _, ok := b.Term.(*mir2.TermDJNZ); ok {
			found = true
		}
	}
	if !found {
		t.Fatal("fixed-count byte bit loop must retain DJNZ")
	}
	var steps [2]Steps
	for i := range steps {
		steps[i], err = CompileHIRSteps(h, Options{AssertMode: "none", DisableCountedLoops: i == 0})
		if err != nil {
			t.Fatal(err)
		}
	}
	vm := mir2.NewVM(raw)
	for x := 0; x < 256; x++ {
		want := int64(0)
		for v := byte(x); v != 0; v >>= 1 {
			want += int64(v & 1)
		}
		got, err := vm.Call("popcount8", []mir2.Value{{I: int64(x)}})
		if err != nil || len(got) != 1 || got[0].I != want {
			t.Fatalf("x=%d MIR %v %v want %d", x, got, err, want)
		}
		if x != 0 && x != 1 && x != 128 && x != 165 && x != 254 && x != 255 {
			continue
		}
		beforeT, beforeB := measureGraceZ80Compiled(t, h, steps[0], "popcount8", []int64{int64(x)}, nil, want, 10000)
		afterT, afterB := measureGraceZ80Compiled(t, h, steps[1], "popcount8", []int64{int64(x)}, nil, want, 10000)
		t.Logf("popcount8(%d): T %d -> %d; bytes %d -> %d", x, beforeT, afterT, beforeB, afterB)
		if afterT >= beforeT || afterB > beforeB {
			t.Errorf("bit loop lost profitability")
		}
	}
}

// A back-edge shuffle hoisted before DJNZ must not leave an empty trampoline
// or grow the exit copies (hoist), and an outer loop must not take B from an
// inner one (nest). Both shapes came from a random campaign.
const countedHoistNestSource = `
fun hoist(x: u8, y: u8) -> u8 { var a: u8 = 209
 var b: u8 = 93
 var c: u8 = 31
 var d: u8 = 21
 var e: u8 = 111
 for i in 0..1 { var t0: u8 = a
  var t1: u8 = b
  var t2: u8 = c
  var t3: u8 = d
  var t4: u8 = e
  a = t2
  b = t3 + 241
  c = t0 - 213
  d = t4 - 66
  e = t1 }
 return (a + x) xor (b + y) xor (c + d) xor e }
fun swap1(x: u8, y: u8) -> u8 { var a: u8 = x
 var b: u8 = y
 for i in 0..1 { var t: u8 = a
  a = b
  b = t }
 return a xor (b + b) }
fun nest(x: u8, y: u8) -> u8 { var a: u8 = 58
 var b: u8 = 56
 var c: u8 = 10
 for i0 in 0..255 { var t0: u8 = a
  var t1: u8 = b
  var t2: u8 = c
  a = t0 | 103
  b = t2 - 43
  c = t1
  for i1 in 0..7 { var u0: u8 = a
   var u1: u8 = b
   var u2: u8 = c
   a = u2 & 50
   b = u0
   c = u1 & 117 } }
 return ((x + a) xor 29) + ((y + b) xor c) }
`

func TestCountedLoopHoistAndNestProfitabilityVMZ80(t *testing.T) {
	h, err := nanz.Parse(countedHoistNestSource, "hoist.nanz")
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	old, err := CompileHIRSteps(h, Options{AssertMode: "none", DisableCountedLoops: true})
	if err != nil {
		t.Fatal(err)
	}
	raw := hir.LowerModule(h)
	for _, f := range []string{"hoist", "swap1", "nest"} {
		args := []int64{17, 99}
		got, err := mir2.NewVM(raw).Call(f, []mir2.Value{{I: 17}, {I: 99}})
		if err != nil || len(got) != 1 {
			t.Fatalf("%s MIR: %v %v", f, got, err)
		}
		want := got[0].I
		beforeT, beforeB := measureGraceZ80Compiled(t, h, old, f, args, nil, want, 2000000)
		afterT, afterB := measureGraceZ80Compiled(t, h, s, f, args, nil, want, 2000000)
		t.Logf("%s: T %d -> %d; module bytes %d -> %d", f, beforeT, afterT, beforeB, afterB)
		if afterT > beforeT || afterB > beforeB {
			t.Errorf("%s regressed", f)
		}
	}
}
