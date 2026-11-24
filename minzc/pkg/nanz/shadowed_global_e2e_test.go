package nanz_test

import (
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
)

// A local or parameter named like a global shadows it for writes as it does
// for reads. Assignment used to look the name up among the globals first, so
// the store went to the global and the local kept its old value.
func TestLocalShadowsGlobalOnAssignment(t *testing.T) {
	for name, src := range map[string]string{"scalars": `
global tot: u16 = 0
global k: u8 = 3
fun gt() -> u16 { return tot }
fun gk() -> u8 { return k }
fun s1() -> u16 { let tot: u16 = 7
  tot = tot + 1
  return tot }
fun s2() -> u16 { let tot: u16 = 7
  tot = tot + 1
  return gt() }
fun s3() -> u8 { var k: u8 = 10
  k = k + 1
  return k }
fun s4() -> u8 { var k: u8 = 10
  k = k + 1
  return gk() }
fun s5(k: u8) -> u8 { k = k + 1
  return k }
fun s6(k: u8) -> u8 { k = k + 1
  return gk() }
assert s1() == 8
assert s2() == 0
assert s3() == 11
assert s4() == 3
assert s5(10) == 11
assert s6(10) == 3
`, "aggregates and loops": `
struct P { x: u8, y: u8 }
global p: P
global arr: [u8; 4] = [1, 2, 3, 4]
global k: u8 = 3
global w: u16 = 1000
global q: ^u8 = 0
fun gpx() -> u8 { return p.x }
fun ga0() -> u8 { return arr[0] }
fun gk() -> u8 { return k }
fun gw() -> u16 { return w }
fun init() -> void { p.x = 1
  p.y = 2 }
fun t1() -> u8 { init()
  var p: P
  p.x = 9
  return p.x + gpx() }
fun t2() -> u8 { init()
  var p: P = P{x: 7, y: 7}
  p = P{x: 9, y: 8}
  return p.x + gpx() }
fun t3() -> u8 { var arr: [u8; 4]
  arr[0] = 7
  return arr[0] + ga0() }
fun t4(arr: ^u8) -> u8 { arr[0] = 9
  return ga0() }
fun t5() -> u8 { var b: [u8; 2]
  b[0] = 0
  return t4(&b[0]) + b[0] }
fun t6(p: ^P) -> u8 { p.x = 5
  return gpx() }
fun t7() -> u8 { init()
  var z: P
  z.x = 0
  return t6(&z) + z.x }
fun t8() -> u8 { var k: u8 = 0
  for i in 0..5 { k = k + i }
  return k + gk() }
fun t9(w: u16) -> u16 { while w < 2000 { w = w + 300 }
  return w + gw() }
fun t10() -> u8 { var k: u8 = 1
  if gk() == 3 { k = 20 } else { k = 30 }
  return k + gk() }
fun t11() -> u8 { k = 4
  var r: u8 = k
  k = 3
  return r }
assert t1() == 10
assert t2() == 10
assert t3() == 8
assert t5() == 10
assert t7() == 6
assert t8() == 13
assert t9(1000) == 3200
assert t10() == 23
assert t11() == 4
`} {
		if err := g0RunAsserts(t, src); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// The same rule through the C frontend, where a parameter named like a file
// scope variable is ordinary code.
func TestLocalShadowsGlobalOnAssignmentC(t *testing.T) {
	src := `
unsigned char k = 3;
unsigned int w = 1000;
struct P { unsigned char x; unsigned char y; };
struct P p = {1, 2};
unsigned char arr[4] = {1, 2, 3, 4};
unsigned char gk(void) { return k; }
unsigned int gw(void) { return w; }
unsigned char gpx(void) { return p.x; }
unsigned char ga0(void) { return arr[0]; }
unsigned char s5(unsigned char k) { k = k + 1; return k; }
unsigned char s6(unsigned char k) { k = k + 1; return gk(); }
unsigned char s3(void) { unsigned char k = 10; k = k + 1; return k; }
unsigned char s7(void) { unsigned char k = 10; k++; return k; }
unsigned char s8(void) { unsigned char k = 10; k += 2; return gk(); }
unsigned char s9(void) { unsigned char k = 0; unsigned char i; for (i = 0; i < 5; i++) k += i; return k; }
unsigned char s10(void) { unsigned char k = 0; unsigned char i; for (i = 0; i < 5; i++) k += i; return gk(); }
unsigned int s11(unsigned int w) { while (w < 2000) w += 300; return w; }
unsigned int s12(unsigned int w) { while (w < 2000) w += 300; return gw(); }
unsigned char s13(void) { struct P p; p.x = 9; p.y = 8; return p.x + gpx(); }
unsigned char s14(void) { unsigned char arr[4]; arr[0] = 7; return arr[0] + ga0(); }
unsigned char s15(void) { unsigned char k = 10; unsigned char *q = &k; *q = 20; return k + gk(); }
unsigned char s16(struct P *p) { p->x = 5; return gpx(); }
unsigned char s17(void) { struct P q; q.x = 0; q.y = 0; return s16(&q) + q.x; }
unsigned char s18(unsigned char *arr) { arr[0] = 9; return ga0(); }
unsigned char s19(void) { unsigned char b[2]; b[0] = 0; return s18(b) + b[0]; }
unsigned char s20(void) { k = 4; { unsigned char k = 10; k = k + 1; } return k; }
// assert s5(10) == 11 via mir2
// assert s5(10) == 11 via z80
// assert s6(10) == 3 via mir2
// assert s6(10) == 3 via z80
// assert s3() == 11 via mir2
// assert s3() == 11 via z80
// assert s7() == 11 via mir2
// assert s7() == 11 via z80
// assert s8() == 3 via mir2
// assert s8() == 3 via z80
// assert s9() == 10 via mir2
// assert s9() == 10 via z80
// assert s10() == 3 via mir2
// assert s10() == 3 via z80
// assert s11(1000) == 2200 via mir2
// assert s11(1000) == 2200 via z80
// assert s12(1000) == 1000 via mir2
// assert s12(1000) == 1000 via z80
// assert s13() == 10 via mir2
// assert s13() == 10 via z80
// assert s14() == 8 via mir2
// assert s14() == 8 via z80
// assert s15() == 23 via mir2
// assert s15() == 23 via z80
// assert s17() == 6 via mir2
// assert s17() == 6 via z80
// assert s19() == 10 via mir2
// assert s19() == 10 via z80
// assert s20() == 4 via mir2
// assert s20() == 4 via z80
`
	m, err := c89.CompileWithOpts(src, "shadow.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := pipeline.RunAssertsMIR2(m, hir.LowerModule(m)); err != nil {
		t.Fatalf("raw MIR2 VM: %v", err)
	}
	m2, _ := c89.CompileWithOpts(src, "shadow.c", c89.CompileOpts{NoLibc: true})
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

// A named function used as a chain callback reads and writes module globals,
// also when the enclosing function has a local of the same name. Inlining it
// resolved such a name to the caller's local (m10 keeps the lambda capture).
func TestNamedChainCallbackSeesGlobals(t *testing.T) {
	src := `
global a: [u8; 6] = [1, 2, 3, 4, 5, 6]
global b: [u8; 6] = [6, 5, 4, 3, 2, 1]
global g: u8 = 5
global tot: u16 = 0
fun addg(x: u8) -> u8 { return x + g }
fun gtg(x: u8) -> bool { return x > g }
fun acc(s: u16, x: u8) -> u16 { return s + u16(x) }
fun accg(s: u16, x: u8) -> u16 { return s + u16(x) + u16(g) }
fun note(x: u8) -> void { tot = tot + u16(x) + u16(g) }
fun mulg(x: u8, y: u8) -> u8 { return x * y + g }
fun rng(s: u16, i: u8) -> u16 { return s + u16(i) + u16(g) }
fun m1() -> u16 { let g: u8 = 100
  return a.map(addg).fold(u16(0), acc, 6) }
fun m2() -> u16 { let g: u8 = 0
  return a.filter(gtg).fold(u16(0), acc, 6) }
fun m3() -> u16 { let g: u8 = 100
  return a.fold(u16(0), accg, 6) }
fun m4() -> u16 { let g: u8 = 100
  tot = 0
  a.forEach(note, 6)
  return tot + u16(g) }
fun m5() -> u16 { let tot: u16 = 7
  a.forEach(note, 6)
  return tot }
fun m6() -> u16 { let g: u8 = 100
  return range(0..4).fold(u16(0), rng) }
fun m7() -> u16 { let g: u8 = 100
  return a.map(addg).sum() }
fun m8() -> u16 { let g: u8 = 0
  return a.filter(gtg).count() }
fun m9() -> u16 { let g: u8 = 100
  return a.zip(b).map(mulg).sum() }
fun m10() -> u16 { let g: u8 = 100
  return a.map(|x: u8| x + g).fold(u16(0), acc, 6) }
fun m11(g: u8) -> u16 { return a.map(addg).fold(u16(0), acc, 6) }
fun m12() -> u16 { let t: u8 = 100
  return a.map(addg).fold(u16(0), acc, 6) }
assert m1() == 51
assert m2() == 6
assert m3() == 51
assert m4() == 151
assert m5() == 7
assert m6() == 26
assert m7() == 51
assert m8() == 1
assert m9() == 86
assert m10() == 621
assert m11(100) == 51
assert m12() == 51
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}

// A range source whose element is not read counts in B only when the count is
// a byte that cannot underflow; wider and crossed ranges follow the for loop
// (audit findings G0-4 and G0-9).
func TestRangeChainCountWidthAndBounds(t *testing.T) {
	src := `
fun a1() -> u16 { return range(0..300).fold(u16(0), |s: u16, i: u16| s + 1) }
fun a2(lo: u8, hi: u8) -> u16 { return range(lo..hi).fold(u16(0), |s: u16, i: u8| s + 1) }
fun a3(n: u16) -> u16 { return range(n..0).fold(u16(0), |s: u16, i: u16| s + 1) }
fun a4(n: u16) -> u16 { return range(n..0).fold(u16(0), |s: u16, i: u16| s + i) }
fun a5(n: u16) -> u16 { return range(0..n).fold(u16(0), |s: u16, i: u16| s + 1) }
fun a6() -> u16 { return range(0..256).fold(u16(0), |s: u16, i: u16| s + 1) }
fun a7(n: u8) -> u16 { return range(0..n).fold(u16(0), |s: u16, i: u8| s + 1) }
fun a8(n: u8) -> u16 { return range(n..0).fold(u16(0), |s: u16, i: u8| s + u16(i)) }
assert a1() == 300
assert a2(5, 3) == 0
assert a2(3, 5) == 2
assert a3(300) == 300
assert a3(0) == 0
assert a4(300) == 44850
assert a5(300) == 300
assert a5(0) == 0
assert a5(256) == 256
assert a6() == 256
assert a7(0) == 0
assert a7(255) == 255
assert a8(10) == 45
assert a8(0) == 0
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}
