package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

// A switch join is reachable through unmatched dispatch and break as well as
// ordinary fallthrough. Fully terminating switches must stop enclosing lowering.
func TestSwitchTerminationVMZ80(t *testing.T) {
	cases := []struct {
		name, source string
		wants        []int64
	}{
		{"no_default", `fun choose(v:u8)->u8 {switch v {case 1:return 1 case 2:return 2} return 0}`, []int64{0, 1, 2, 0}},
		{"default_all_return", `fun choose(v:u8)->u8 {switch v {case 1:return 1 case 2:return 2 default:return 3} return 99}`, []int64{3, 1, 2, 3}},
		{"some_return", `fun choose(v:u8)->u8 {var x:u8=4 switch v {case 1:return 1 case 2:x=7} return x}`, []int64{4, 1, 7, 4}},
		{"nested", `fun choose(v:u8)->u8 {switch v {case 1:switch v {case 1:return 8} return 9 case 2:return 2} return 0}`, []int64{0, 8, 2, 0}},
		{"enum_exhaustive", `enum Kind {Zero,One,Two,Three} fun choose(v:Kind)->u8 {switch v {case Kind.Zero:return 4 case Kind.One:return 5 case Kind.Two:return 6 case Kind.Three:return 7} return 99}`, []int64{4, 5, 6, 7}},
		{"break_mutation", `fun choose(v:u8)->u8 {var x:u8=4 while true {switch v {case 1:x=7 break default:return 3} return x} return 99}`, []int64{3, 7, 3, 3}},
		{"loop_break_continue", `fun choose(v:u8)->u8 {var i:u8=0 var x:u8=0 while i<3 {i=i+1 switch v {case 1:x=x+2 break case 2:continue default:return 9} x=x+1} return x}`, []int64{9, 9, 0, 9}},
		{"for_continue", `fun choose(v:u8)->u8 {var x:u8=0 for i in 0..3 {switch v {case 1:continue default:x=x+1}} return x}`, []int64{3, 0, 3, 3}},
		{"if_else_returns", `fun choose(v:u8)->u8 {if v==1 {return 1} else if v==2 {return 2} else {return 3} return 99}`, []int64{3, 1, 2, 3}},
		{"empty", `fun choose(v:u8)->u8 {switch v {} return v}`, []int64{0, 1, 2, 3}},
		{"default_only", `fun choose(v:u8)->u8 {switch v {default:return 8} return 99}`, []int64{8, 8, 8, 8}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("compiler panic: %v", r)
				}
			}()
			parse := func() *hir.Module {
				h, e := nanz.Parse(tc.source, "switch-termination.nanz")
				if e != nil {
					t.Fatal(e)
				}
				return h
			}
			h := parse()
			if e := hir.Verify(h); e != nil {
				t.Fatal(e)
			}
			raw := hir.LowerModule(h)
			steps, e := CompileHIRSteps(parse(), Options{ContractOpt: true, AssertMode: "none"})
			if e != nil {
				t.Fatal(e)
			}
			for tier, m := range map[string]*mir2.Module{"raw": raw, "optimized": steps.MIR2Module} {
				t.Run(tier, func(t *testing.T) {
					for v, want := range tc.wants {
						got, e := mir2.NewVM(m).Call("choose", []mir2.Value{{I: int64(v)}})
						if e != nil || len(got) != 1 || got[0].I != want {
							t.Fatalf("input %d: got %v, error %v, want %d", v, got, e, want)
						}
					}
				})
			}
			t.Run("z80", func(t *testing.T) {
				for v, want := range tc.wants {
					measureGraceZ80Compiled(t, parse(), steps, "choose", []int64{int64(v)}, nil, want, 100000)
				}
			})
		})
	}
	// Unsupported control transfers must be diagnosed before MIR2 lowering.
	for _, statement := range []string{"break", "continue"} {
		t.Run("outside_loop_"+statement, func(t *testing.T) {
			h, err := nanz.Parse("fun choose(v:u8)->u8 {switch v {case 1:"+statement+"} return 0}", "invalid-switch.nanz")
			if err != nil {
				return
			}
			if _, err := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"}); err == nil {
				t.Fatal("expected a compile error for control transfer outside a loop")
			}
		})
	}

}
