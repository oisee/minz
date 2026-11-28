package pipeline

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestVariantSearchScorerVMZ80(t *testing.T) {
	for _, policy := range []hir.CountedLoopPolicy{hir.CountedDefault, hir.CountedOrdinary, hir.CountedPermit} {
		t.Run(fmt.Sprint(policy), func(t *testing.T) {
			h, e := nanz.Parse(variantWinningNest, "static_cost.nanz")
			if e != nil {
				t.Fatal(e)
			}
			s, e := CompileHIRSteps(h, Options{DisableVariantSearch: true, AssertMode: "none", FunctionVariants: map[string]FunctionVariant{"probe": {Counted: policy}}})
			if e != nil {
				t.Fatal(e)
			}
			image, e := assembleVariant(s)
			if e != nil {
				t.Fatal(e)
			}
			cost, ok := image.score(s.MIR2Module.FuncByName("probe"))
			if !ok {
				t.Fatal("fixed nest not scored")
			}
			for _, args := range [][]int64{{0, 0}, {108, 199}, {255, 255}} {
				values, e := mir2.NewVM(s.MIR2Module).Call("probe", []mir2.Value{{I: args[0]}, {I: args[1]}})
				if e != nil {
					t.Fatal(e)
				}
				cycles, _ := measureGraceZ80Compiled(t, h, s, "probe", args, nil, values[0].I, 2000000)
				if cost != int64(cycles) {
					t.Fatalf("static %dT native %dT", cost, cycles)
				}
			}
		})
	}
	// The supplied losing nest is a complete-module fallback case: its
	// permitted recipe is legal and smaller, but substantially slower. Check
	// that candidate independently even though the refusing scorer cannot
	// account for its split helper/boundary change.
	t.Run("losing-candidate", func(t *testing.T) {
		var cycles [2]int
		for i, policy := range []hir.CountedLoopPolicy{hir.CountedDefault, hir.CountedPermit} {
			h, e := nanz.Parse(variantLosingNest, "losing_cost.nanz")
			if e != nil {
				t.Fatal(e)
			}
			s, e := CompileHIRSteps(h, Options{DisableVariantSearch: true, AssertMode: "none", FunctionVariants: map[string]FunctionVariant{"probe": {Counted: policy}}})
			if e != nil {
				t.Fatal(e)
			}
			got, e := mir2.NewVM(s.MIR2Module).Call("probe", []mir2.Value{{I: 108}, {I: 199}})
			if e != nil || len(got) != 1 || got[0].I != 225 {
				t.Fatalf("losing recipe %d: %v %v", i, got, e)
			}
			cycles[i], _ = measureGraceZ80Compiled(t, h, s, "probe", []int64{108, 199}, nil, 225, 20000000)
		}
		if cycles[1] <= cycles[0] {
			t.Fatalf("fixture must expose a losing permitted nest: %v", cycles)
		}
		t.Logf("losing default %dT permitted %dT", cycles[0], cycles[1])
	})

}

func TestVariantSearchRefusesCrossingPaths(t *testing.T) {
	makeImage := func(body string) *variantImage {
		t.Helper()
		asm := "; fun probe(a: u8 = A) -> u8 = A ; clobbers: A, F\nprobe:\n    AND A\n    JR Z, .zero\n" + body
		im, e := assembleVariant(Steps{Assembly: asm})
		if e != nil {
			t.Fatal(e)
		}
		if _, ok := im.score(&mir2.Func{Name: "probe"}); !ok {
			t.Fatal("acyclic score")
		}
		return im
	}
	baseline := makeImage("    NOP\n    NOP\n    NOP\n    RET\n.zero:\n    RET\n")
	crossing := makeImage("    RET\n.zero:\n    NOP\n    RET\n")
	if baseline.nonIncreasingPaths(crossing, "probe") {
		t.Fatal("a lower maximum hid a slower zero arm")
	}
	faster := makeImage("    NOP\n    RET\n.zero:\n    RET\n")
	if !baseline.nonIncreasingPaths(faster, "probe") {
		t.Fatal("pointwise faster paths refused")
	}
	// Unknown flag predicates must refuse the whole proof, including when a
	// different, supported arm still reaches a return.
	unsupported := makeImage("    JP PE, .zero\n    RET\n.zero:\n    RET\n")
	if baseline.nonIncreasingPaths(unsupported, "probe") {
		t.Fatal("unsupported parity predicate accepted")
	}
	changed := makeImage("    NOP\n    RET\n.zero:\n    RET\n")
	changed.context[""] = append(changed.context[""], "changed caller")
	if baseline.sameContext(changed, "probe") {
		t.Fatal("changed external context accepted")
	}
}

func TestVariantSearchEntryABIRefusals(t *testing.T) {
	// A complete product can publish a new ABI for an independent entry. A
	// direct caller, address constant, hard convention or opaque ASM forbids it.
	image := func(reg, suffix string) *variantImage {
		t.Helper()
		im, err := assembleVariant(Steps{Assembly: "; fun probe(k: u8 = " + reg + ") -> u8 = A ; clobbers: A, F\nprobe:\n    RET\n" + suffix})
		if err != nil {
			t.Fatal(err)
		}
		return im
	}
	for _, tc := range []struct {
		name, suffix string
		allowed      bool
	}{
		{"independent", "", true},
		{"direct-call", "; fun caller() -> u8 = A ; clobbers: A, F\ncaller:\n    CALL probe\n    RET\n", false},
		{"address-data", "; globals\nentry_address: DW probe\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := image("A", tc.suffix), image("B", tc.suffix)
			if a.sameContext(b, "probe") {
				t.Fatal("strict ABI gate changed")
			}
			if got := a.sameContext(b, "probe", true); got != tc.allowed {
				t.Fatalf("entry admission %v want %v", got, tc.allowed)
			}
		})
	}
	h, err := nanz.Parse(`fun probe(k:u8)->u8 { return k+1 }`, "entry_abi.nanz")
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none", DisableVariantSearch: true})
	if err != nil {
		t.Fatal(err)
	}
	m := s.MIR2Module
	if !sameVariantEntryABI(m, m, "probe") {
		t.Fatal("independent scalar entry refused")
	}
	param := &m.FuncByName("probe").Contract.Params[0]
	param.ClassHard = true
	if sameVariantEntryABI(m, m, "probe") {
		t.Fatal("hard entry convention accepted")
	}
	param.ClassHard = false
	for _, op := range []mir2.Op{mir2.OpCall, mir2.OpAddrOf, mir2.OpAsm} {
		peer := &mir2.Func{Name: "peer", Blocks: []*mir2.Block{{Label: "entry", Insts: []*mir2.Inst{{Op: op, Sym: "probe"}}}}}
		if op == mir2.OpAsm {
			peer.Blocks[0].Insts[0].Sym = "alias EQU probe\nCALL alias"
		}
		m.Funcs = append(m.Funcs, peer)
		if sameVariantEntryABI(m, m, "probe") {
			t.Errorf("incoming %v admitted", op)
		}
		m.Funcs = m.Funcs[:len(m.Funcs)-1]
	}
}
