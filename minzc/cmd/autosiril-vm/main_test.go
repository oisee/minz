package main

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestMixedScalarBindingUsesWideMZVMemory(t *testing.T) {
	h, e := nanz.Parse(`@extern fun quant(tick:u32,ppqn:u32,beat:u32)->u32
 global pad:[u8;131072]
 global result:u32
 fun probe(tick:u32)->u32 { result = quant(tick,480,8) return result }`, "mixed.nanz")
	if e != nil {
		t.Fatal(e)
	}
	c, e := lanz.Compile(`(fun native_quant ((tick u32) (ppqn u32) (beat u32)) u32
  (return (+ (* (/ tick ppqn) beat) (/ (+ (* (% tick ppqn) beat) (/ ppqn (cast 2 u32))) ppqn))))`, "music")
	if e != nil {
		t.Fatal(e)
	}
	if e = linkMusic(h, c, manifest{Functions: []binding{{Source: "quant", Symbol: "native_quant"}}}); e != nil {
		t.Fatal(e)
	}
	h.Target = hir.TargetMZV
	for _, optimized := range []bool{false, true} {
		m := hir.LowerModule(hir.CloneModule(h))
		if optimized {
			optimize(m)
		}
		if e := mir2.Verify(m); e != nil {
			t.Fatal(e)
		}
		vm := mir2.NewVM(m)
		vm.MaxMemory = 1 << 20
		if vm.GlobalAddr("result") <= 65535 {
			t.Fatal("high address not exercised")
		}
		for _, tc := range []struct{ tick, want int64 }{{0, 0}, {30, 1}, {65535, 1092}, {228820, 3814}, {4294967295, 71582788}} {
			got, e := vm.Call("probe", []mir2.Value{{I: tc.tick}})
			if e != nil || len(got) != 1 || got[0].I != tc.want {
				t.Fatalf("opt%v tick%d got%v err%v want%d", optimized, tc.tick, got, e, tc.want)
			}
		}
	}
}
func TestMixedBindingsRejectIncompatibleABI(t *testing.T) {
	for _, tc := range []struct {
		name, proto, foreign, param string
		fallible                    bool
		want                        string
	}{
		{"return", "u32", "u16", "u32", false, "incompatible"},
		{"parameter", "u32", "u32", "u16", false, "parameter"},
		{"fallible", "u32", "u32", "u32", true, "incompatible"},
		{"unresolved", "u32", "u32", "u32", false, "unresolved"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, e := nanz.Parse("@extern fun expected(x:u32)->"+tc.proto, "prototype.nanz")
			if e != nil {
				t.Fatal(e)
			}
			c, e := lanz.Compile("(fun foreign ((x "+tc.param+")) "+tc.foreign+" (return (cast 1 "+tc.foreign+")))", "foreign")
			if e != nil {
				t.Fatal(e)
			}
			b := binding{Source: "expected", Symbol: "foreign", Fallible: tc.fallible}
			if tc.name == "unresolved" {
				b.Source = "absent"
			}
			e = linkMusic(h, c, manifest{Functions: []binding{b}})
			if e == nil || !strings.Contains(e.Error(), tc.want) {
				t.Fatalf("got%v want%s", e, tc.want)
			}
		})
	}
}
