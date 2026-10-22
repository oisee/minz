package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestVariantSearchGCDVMZ80(t *testing.T) {
	const source = `fun remainder(a: u8, b: u8) -> u8 {
  if a == 0 { return b }
  if b == 0 { return a }
  while a != b {
   if a > b { a = a - b } else { b = b - a }
  }
  return a
 }`
	h, e := nanz.Parse(source, "variant_gcd.nanz")
	if e != nil {
		t.Fatal(e)
	}
	original := h.Dump()
	s, e := CompileHIRSteps(h, DefaultOptions())
	if e != nil {
		t.Fatal(e)
	}
	if h.Dump() != original {
		t.Fatal("search mutated the source HIR")
	}
	if strings.Count(s.Assembly, "    CP B") != 1 {
		t.Fatalf("duplicate compare remained: %s", s.Assembly)
	}
	if !strings.Contains(s.Assembly, "mir-grace=true") {
		t.Fatalf("MIR Grace winner missing: %s", s.Assembly)
	}
	for _, args := range [][]int64{{12, 8}, {0, 9}, {9, 0}, {255, 153}, {17, 13}} {
		a, b := args[0], args[1]
		for b != 0 {
			a, b = b, a%b
		}
		got, e := mir2.NewVM(s.MIR2Module).Call("remainder", []mir2.Value{{I: args[0]}, {I: args[1]}})
		if e != nil || len(got) != 1 || got[0].I != a {
			t.Fatalf("MIR %v %v", got, e)
		}
		cyc, size := measureGraceZ80Compiled(t, h, s, "remainder", args, nil, a, 2000000)
		if args[0] == 12 && cyc >= 200 {
			t.Fatalf("GCD selection did not improve runtime: %dT", cyc)
		}
		t.Logf("args%v %dT %dB", args, cyc, size)
	}
}

func TestVariantSearchNestedVMZ80(t *testing.T) {
	// Fixed loop shapes exercise both counted and ordinary winners under
	// different register pressure. Names do not participate in selection.
	for _, tc := range []struct {
		name, source string
		args         []int64
		want         int64
		counted      int
		maxT         int
	}{
		{"winning", variantWinningNest, []int64{93, 9}, 237, 2, 2327},
		{"losing", variantLosingNest, []int64{108, 199}, 225, 1, 0},
		{"campaign", variantCampaignLoop, []int64{146, 182}, 198, 0, 65226},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, e := nanz.Parse(tc.source, "variant_nest.nanz")
			if e != nil {
				t.Fatal(e)
			}
			rawHIR, e := nanz.Parse(tc.source, "raw.nanz")
			if e != nil {
				t.Fatal(e)
			}
			raw := hir.LowerModuleWithOptions(rawHIR, hir.LowerOptions{DisableCountedLoops: true})
			args := []mir2.Value{{I: tc.args[0]}, {I: tc.args[1]}}
			expected, e := mir2.NewVM(raw).Call("probe", args)
			if e != nil || len(expected) != 1 || expected[0].I != tc.want {
				t.Fatalf("raw %v %v", expected, e)
			}
			s, e := CompileHIRSteps(h, Options{AssertMode: "none"})
			if e != nil {
				t.Fatal(e)
			}
			n := 0
			for _, b := range s.MIR2Module.FuncByName("probe").Blocks {
				if _, ok := b.Term.(*mir2.TermDJNZ); ok {
					n++
				}
			}
			if !strings.Contains(s.Assembly, "variant=") {
				t.Error("search provenance missing")
			}
			if n != tc.counted {
				t.Errorf("counted loops %d want %d", n, tc.counted)
			}
			got, e := mir2.NewVM(s.MIR2Module).Call("probe", args)
			if e != nil || len(got) != 1 || got[0].I != expected[0].I {
				t.Fatalf("MIR %v %v want %v", got, e, expected)
			}
			cyc, size := measureGraceZ80Compiled(t, h, s, "probe", tc.args, nil, expected[0].I, 20000000)
			if tc.maxT > 0 && cyc > tc.maxT {
				t.Fatalf("search result lost: %dT > %dT", cyc, tc.maxT)
			}
			t.Logf("%dT %dB", cyc, size)
		})
	}
}

func TestVariantSearchPointerSizeVMZ80(t *testing.T) {
	data, e := os.ReadFile("../../../examples/nanz/01_sum_array.nanz")
	if e != nil {
		t.Fatal(e)
	}
	h, e := nanz.Parse(string(data), "sum_array.nanz")
	if e != nil {
		t.Fatal(e)
	}
	s, e := CompileHIRSteps(h, Options{ContractOpt: true, OptSize: true, AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	t.Log(s.Assembly)
	if strings.Contains(s.Assembly, "hir-grace=true") {
		t.Fatal("HIR Grace selected under size")
	}
	// Exercise zero length and wrapping byte sums through both backends.
	for _, input := range [][]byte{{}, {1, 2, 3, 4, 5}, {255, 255, 3}} {
		vm := mir2.NewVM(s.MIR2Module)
		ptr := vm.AllocHeap(input)
		var want byte
		memory := map[uint16]byte{}
		for i, v := range input {
			want += v
			memory[0xc100+uint16(i)] = v
		}
		got, e := vm.Call("sum_array", []mir2.Value{ptr, {I: int64(len(input))}})
		if e != nil || len(got) != 1 || got[0].I != int64(want) {
			t.Fatalf("pointer MIR %v %v want %d", got, e, want)
		}
		measureGraceZ80Compiled(t, h, s, "sum_array", []int64{0xc100, int64(len(input))}, memory, int64(want), 100000)
	}
	if !strings.Contains(s.Assembly, "variant=") {
		t.Fatal("missing search provenance")
	}
}

const variantWinningNest = `fun probe(x: u8, y: u8) -> u16 {
 var a: u8 = x
 var b: u8 = y
 for i in 0..13 { a = a + 3
  for j in 0..2 { var t: u8 = a
   a = b + 112
   b = t }
 }
 return (a + b) as u16
}`
const variantLosingNest = `fun probe(x: u8, y: u8) -> u16 {
 var a: u8 = 58
 var b: u8 = 56
 var c: u8 = 10
 var w: u16 = 0
 for i0 in 0..255 {
  var t0_0: u8 = a
  var t0_1: u8 = b
  var t0_2: u8 = c
  a = t0_0 | 103
  b = t0_2 - 43
  c = t0_1
  for i1 in 0..7 {
   var t1_0: u8 = a
   var t1_1: u8 = b
   var t1_2: u8 = c
   a = t1_2 & 50
   b = t1_0
   c = t1_1 & 117
  }
 }
 var h: u8 = x
 h = (h + a) xor 29
 h = h + h
 h = (h + b) xor 29
 h = h + h
 h = (h + c) xor 29
 h = h + h
 h = h xor y
 return w + h as u16
}
`

// Read the public accounting by reflection so this exact regression test also
// runs (and fails behaviorally) on main before variant-search accounting existed.
type variantBudgetDecision struct {
	Function   string
	Candidates int
}

func variantBudgetDecisions(t *testing.T, s Steps) []variantBudgetDecision {
	t.Helper()
	field := reflect.ValueOf(s).FieldByName("VariantDecisions")
	if !field.IsValid() {
		t.Fatal("variant search candidate accounting missing")
	}
	b, err := json.Marshal(field.Interface())
	if err != nil {
		t.Fatal(err)
	}
	var decisions []variantBudgetDecision
	if err := json.Unmarshal(b, &decisions); err != nil {
		t.Fatal(err)
	}
	if len(decisions) == 0 {
		t.Fatal("search decisions missing")
	}
	return decisions
}

func checkVariantBudgetAccounting(t *testing.T, s Steps, count int, exhausted bool) {
	t.Helper()
	field := reflect.ValueOf(s).FieldByName("VariantSearch")
	if !field.IsValid() {
		t.Fatal("module budget accounting missing")
	}
	b, err := json.Marshal(field.Interface())
	if err != nil {
		t.Fatal(err)
	}
	var stats struct {
		CandidateCompilations, Limit int
		Exhausted                    bool
	}
	if err := json.Unmarshal(b, &stats); err != nil {
		t.Fatal(err)
	}
	if stats.CandidateCompilations != count || stats.Limit < 4 || stats.Limit > 16 || stats.Exhausted != exhausted {
		t.Fatalf("inconsistent module accounting: %+v, decisions=%d exhausted=%t", stats, count, exhausted)
	}
}

func TestVariantSearchLoopFreeBudgetVMZ80(t *testing.T) {
	var source strings.Builder
	for i := 0; i < 40; i++ {
		// Distinct exported leaves survive pruning; every axis is an IR identity.
		fmt.Fprintf(&source, "fun leaf%d(x: u8) -> u8 { return x + %d }\n", i, i+1)
	}
	h, err := nanz.Parse(source.String(), "leaves.nanz")
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	decisions := variantBudgetDecisions(t, s)
	for _, d := range decisions {
		if d.Candidates != 0 {
			t.Fatalf("loop-free identity %s compiled %d candidates; want zero", d.Function, d.Candidates)
		}
	}
	checkVariantBudgetAccounting(t, s, 0, false)
	if !strings.Contains(s.Assembly, "leaf39:") {
		t.Fatal("leaf disappeared")
	}
	got, err := mir2.NewVM(s.MIR2Module).Call("leaf39", []mir2.Value{{I: 255}})
	if err != nil || len(got) != 1 || got[0].I != 39 {
		t.Fatalf("MIR wraparound: %v %v", got, err)
	}
	measureGraceZ80Compiled(t, h, s, "leaf39", []int64{255}, nil, 39, 10000)
}

func TestVariantSearchDeterministicBudgetVMZ80(t *testing.T) {
	var source strings.Builder
	for i := 0; i < 24; i++ {
		fmt.Fprintf(&source, `fun work%02d(a:u8,b:u8)->u8 {
   while a!=b { if a>b { a=a-b } else { b=b-a } }
   return a
  }
`, i)
	}
	h, err := nanz.Parse(source.String(), "budget.nanz")
	if err != nil {
		t.Fatal(err)
	}
	var previous Steps
	for run := 0; run < 2; run++ {
		s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
		if err != nil {
			t.Fatal(err)
		}
		decisions := variantBudgetDecisions(t, s)
		count := 0
		for _, d := range decisions {
			count += d.Candidates
		}
		if count == 0 || count > 16 {
			t.Fatalf("candidate compilations = %d, want 1..16", count)
		}
		checkVariantBudgetAccounting(t, s, count, true)
		if run > 0 && (!reflect.DeepEqual(variantBudgetDecisions(t, previous), decisions) || previous.Assembly != s.Assembly) {
			t.Fatal("candidate budget or output depends on earlier compilation")
		}
		t.Logf("run %d: candidate compilations %d (cap 16)", run, count)
		got, err := mir2.NewVM(s.MIR2Module).Call("work23", []mir2.Value{{I: 255}, {I: 153}})
		if err != nil || len(got) != 1 || got[0].I != 51 {
			t.Fatalf("MIR: %v %v", got, err)
		}
		lateT, _ := measureGraceZ80Compiled(t, h, s, "work23", []int64{255, 153}, nil, 51, 100000)
		earlyT, _ := measureGraceZ80Compiled(t, h, s, "work00", []int64{255, 153}, nil, 51, 100000)
		if earlyT >= lateT {
			t.Fatalf("best trial before exhaustion lost: early %dT, fallback %dT", earlyT, lateT)
		}
		previous = s
	}
}

// Frozen winning shape from the previous campaign (seed 1009, function 11).
const variantCampaignLoop = `fun probe(x: u8, y: u8) -> u16 {
 var a: u8 = 203
 var b: u8 = 131
 var c: u8 = 110
 var d: u8 = 141
 var e: u8 = 207
 var w: u16 = 0
 for i0 in 0..255 {
  var t0_0: u8 = a
  var t0_1: u8 = b
  var t0_2: u8 = c
  var t0_3: u8 = d
  var t0_4: u8 = e
  a = t0_3 & 192
  b = t0_1 & 111
  c = t0_0 & 189
  d = t0_4 xor 220
  e = t0_2 xor 17
 }
 var h: u8 = x
 h = (h + a) xor 29
 h = h + h
 h = (h + b) xor 29
 h = h + h
 h = (h + c) xor 29
 h = h + h
 h = (h + d) xor 29
 h = h + h
 h = (h + e) xor 29
 h = h + h
 h = h xor y
 return w + h as u16
}
`
