package pipeline

import (
	"strings"
	"testing"

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
