package pipeline

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

const graceSumSource = `fun sum_array(ptr: ^u8, n: u8) -> u8 {
 var s: u8 = 0
 var i: u8 = 0
 while i < n {
  s = s + ptr[i]
  i = i + 1
 }
 return s
}`

func graceSumMemory(size int) []byte {
	out := make([]byte, size)
	for i := range out {
		out[i] = byte((i*37 + 11) ^ (i >> 7))
	}
	return out
}
func graceSumError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if strings.HasPrefix(s, "block @") {
		if i := strings.Index(s, ": "); i >= 0 {
			return s[i+2:]
		}
	}
	return s
}
func graceSumCall(t *testing.T, m *mir2.Module, memory []byte, base, n int64) ([]mir2.Value, string, int64) {
	t.Helper()
	vm := mir2.NewVM(m)
	vm.EnsureHeap(int64(len(memory)))
	vm.WriteHeapSlice(0, memory)
	result, err := vm.Call("sum_array", []mir2.Value{{I: base}, {I: n}})
	if !bytes.Equal(vm.ReadHeap(0, len(memory)), memory) {
		t.Fatal("sum changed memory")
	}
	return result, graceSumError(err), vm.StepsExecuted()
}

func TestHIRGracePointerWalkRealSourceAndFacts(t *testing.T) {
	src, err := os.ReadFile("../../../examples/nanz/01_sum_array.nanz")
	if err != nil {
		t.Fatal(err)
	}
	h := graceMeasureParse(t, string(src))
	facts := hir.AnalyzeFacts(h)
	if len(facts.PointerWalks) != 1 {
		t.Fatalf("facts: %+v", facts.PointerWalks)
	}
	f := facts.PointerWalks[0]
	if f.Function != "sum_array" || f.IndexMinimum != 0 || f.IndexMaximum != 254 || f.Bound != "n" || f.Base != "ptr" || f.Stride != 1 || f.AddressMaximum != 65535 {
		t.Fatalf("range contract: %+v", f)
	}
	if len(facts.DB.Query("hir-index-range", f.Site, "i", "0", "n", "exclusive")) != 1 || facts.DB.Count("hir-address-nowrap-guard") != 1 {
		t.Fatal("missing range/guard predicates")
	}
	r, err := hir.RunGrace(h, 1)
	if err != nil || r.ByRule["hir-guarded-pointer-walk"] != 1 {
		t.Fatalf("rule: %+v %v", r, err)
	}
	fn := h.FuncByName("sum_array")
	branch, ok := fn.Body.Body[2].(*hir.IfStmt)
	if !ok {
		t.Fatal("missing versioned loop")
	}
	if _, ok := branch.Then.Body[0].(*hir.ForEachStmt); !ok {
		t.Fatal("missing pointer loop")
	}
	if _, ok := branch.Else.Body[0].(*hir.WhileStmt); !ok {
		t.Fatal("missing original fallback")
	}
	r, err = hir.RunGrace(h, 0)
	if err != nil || r.Applied != 0 || len(hir.AnalyzeFacts(h).PointerWalks) != 0 {
		t.Fatalf("fallback rewritten again: %+v %v", r, err)
	}
	vm := mir2.NewVM(hir.LowerModule(h))
	addr := vm.GlobalAddr("arr")
	vm.WriteHeapSlice(addr, []byte{1, 2, 3, 4, 5})
	got, err := vm.Call("sum_array", []mir2.Value{{I: addr}, {I: 5}})
	if err != nil || len(got) != 1 || got[0].I != 15 {
		t.Fatalf("original program oracle: %v %v", got, err)
	}
}

func TestHIRGracePointerWalkDifferential(t *testing.T) {
	for _, tuple := range []bool{false, true} {
		src := graceSumSource
		if tuple {
			src = strings.Replace(src, "-> u8", "-> (u8, u8)", 1)
			src = strings.Replace(src, "return s", "return (s, i)", 1)
		}
		for _, stage := range []struct {
			name       string
			final, mir bool
		}{{"raw", false, false}, {"optimized", true, false}, {"optimized_mir_grace", true, true}} {
			t.Run(fmt.Sprintf("%s/tuple=%v", stage.name, tuple), func(t *testing.T) {
				before := graceMeasureModule(t, src, false, stage.final, stage.mir)
				after := graceMeasureModule(t, src, true, stage.final, stage.mir)
				for _, size := range []int{16, 65536} {
					memory := graceSumMemory(size)
					for n := int64(0); n < 256; n++ {
						for _, base := range []int64{0, 15, 65535 - n, 65536 - n, 65535} {
							a, ae, _ := graceSumCall(t, before, memory, base, n)
							b, be, _ := graceSumCall(t, after, memory, base, n)
							if ae != be || !reflect.DeepEqual(a, b) {
								t.Fatalf("heap=%d ptr=%d n=%d before=%v %q after=%v %q", size, base, n, a, ae, b, be)
							}
							// Independent byte sum and first invalid address, with function-parameter
							// truncation performed explicitly instead of relying on either VM run.
							p := int64(uint16(base))
							count := int64(uint8(n))
							var sum uint8
							bad := int64(-1)
							for i := int64(0); i < count; i++ {
								addr := int64(uint16(p + i))
								if addr >= int64(size) {
									bad = addr
									break
								}
								sum += memory[addr]
							}
							if bad >= 0 {
								want := fmt.Sprintf("load: out-of-bounds address %d (heap size %d)", bad, size)
								if be != want {
									t.Fatalf("trap oracle: %q want %q", be, want)
								}
							} else {
								want := []mir2.Value{{I: int64(sum)}}
								if tuple {
									want = append(want, mir2.Value{I: count})
								}
								if be != "" || !reflect.DeepEqual(b, want) {
									t.Fatalf("sum/index oracle heap=%d ptr=%d n=%d got=%v %q want=%v", size, base, n, b, be, want)
								}
							}
						}
					}
				}
			})
		}
	}
}

func TestHIRGracePointerWalkMeasurements(t *testing.T) {
	memory := graceSumMemory(65536)
	for _, stage := range []struct {
		name       string
		final, mir bool
	}{{"raw", false, false}, {"optimized", true, false}, {"optimized_mir_grace", true, true}} {
		before := graceMeasureModule(t, graceSumSource, false, stage.final, stage.mir)
		after := graceMeasureModule(t, graceSumSource, true, stage.final, stage.mir)
		bb, bi := graceMeasureShape(before)
		ab, ai := graceMeasureShape(after)
		t.Logf("%s MIR shape: blocks %d -> %d; instructions %d -> %d", stage.name, bb, ab, bi, ai)
		for _, n := range []int64{0, 1, 5, 16, 64, 255} {
			_, _, a := graceSumCall(t, before, memory, 1024, n)
			_, _, b := graceSumCall(t, after, memory, 1024, n)
			t.Logf("%s n=%d VM instructions %d -> %d", stage.name, n, a, b)
			if stage.final && n >= 16 && b >= a {
				t.Fatal("long-loop VM work did not improve")
			}
		}
	}
}

func TestHIRGracePointerWalkGuards(t *testing.T) {
	for name, src := range map[string]string{
		"stride":           strings.Replace(graceSumSource, "ptr[i]", "ptr[i*2]", 1),
		"nonzero_start":    strings.Replace(graceSumSource, "var i: u8 = 0", "var i: u8 = 1", 1),
		"changed_bound":    strings.Replace(graceSumSource, "s = s + ptr[i]", "n = n - 1\n s = s + ptr[i]", 1),
		"binding":          strings.Replace(graceSumSource, "s = s + ptr[i]", "let x:u8=ptr[i]\n s = s + x", 1),
		"global_collision": "global s:u8\n" + graceSumSource,
	} {
		t.Run(name, func(t *testing.T) {
			h := graceMeasureParse(t, src)
			r, err := hir.RunGrace(h, 0)
			if err != nil || r.ByRule["hir-guarded-pointer-walk"] != 0 {
				t.Fatalf("unsupported loop: %+v %v", r, err)
			}
		})
	}
	h := graceMeasureParse(t, "global __grace_element_0:u8\n"+graceSumSource)
	r, err := hir.RunGrace(h, 0)
	if err != nil || r.ByRule["hir-guarded-pointer-walk"] != 1 || !strings.Contains(h.Dump(), "__grace_element_1") {
		t.Fatalf("freshness: %+v %v", r, err)
	}
}

func BenchmarkHIRGracePointerWalkVM(b *testing.B) {
	memory := graceSumMemory(65536)
	for _, n := range []int64{0, 1, 5, 16, 64, 255} {
		for _, useHIR := range []bool{false, true} {
			b.Run(fmt.Sprintf("n=%d/hir=%v", n, useHIR), func(b *testing.B) {
				m := graceMeasureModule(b, graceSumSource, useHIR, true)
				vm := mir2.NewVM(m)
				vm.MaxSteps = 1 << 60 // a cumulative budget across all benchmark calls
				vm.EnsureHeap(65536)
				vm.WriteHeapSlice(0, memory)
				args := []mir2.Value{{I: 1024}, {I: n}}
				var want uint8
				for i := int64(0); i < n; i++ {
					want += memory[1024+i]
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					ret, err := vm.Call("sum_array", args)
					if err != nil || len(ret) != 1 || ret[0].I != int64(want) {
						b.Fatalf("%v %v want%d", ret, err, want)
					}
				}
			})
		}
	}
}

func BenchmarkHIRGracePointerWalkCompile(b *testing.B) {
	for _, useHIR := range []bool{false, true} {
		b.Run(fmt.Sprintf("hir=%v", useHIR), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				h := graceMeasureParse(b, graceSumSource)
				b.StartTimer()
				if _, err := CompileHIRSteps(h, Options{UseHIRGrace: useHIR, SkipZ80Emission: true, AssertMode: "mir2"}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestHIRGracePointerWalkAssertions(t *testing.T) {
	src := graceSumSource + `
 global a:[u8;5]
 fun fixture(n:u8)->u8 {
 a[0]=1
 a[1]=2
 a[2]=3
 a[3]=4
 a[4]=5
 return sum_array(&a,n)
 }
 assert fixture(0)==0 via mir2
 assert fixture(1)==1 via mir2
 assert fixture(5)==15 via mir2
 `
	for _, backend := range []string{"mir2", "z80"} {
		for _, useHIR := range []bool{false, true} {
			for _, useMIR := range []bool{false, true} {
				h := graceMeasureParse(t, strings.ReplaceAll(src, "via mir2", "via "+backend))
				h.AssertStats = &hir.AssertStats{}
				_, err := CompileHIRSteps(h, Options{UseHIRGrace: useHIR, UseGrace: useMIR, AssertMode: backend, SkipZ80Emission: backend == "mir2"})
				if err != nil || h.AssertStats.Executed != 3 || h.AssertStats.Passed != 3 {
					t.Fatalf("HIR=%v MIR=%v asserts=%+v %v", useHIR, useMIR, h.AssertStats, err)
				}
			}
		}
	}
}

func TestForEachPreheaderConstantsAndContinue(t *testing.T) {
	for _, hasContinue := range []bool{false, true} {
		for _, useMIR := range []bool{false, true} {
			body := hir.Blk()
			if hasContinue {
				body.Body = append(body.Body, hir.If(&hir.BinExpr{Op: "==", L: hir.Var("x", mir2.TyU8), R: hir.U8(2), Ty: mir2.TyBool}, hir.Blk(hir.Continue()), nil))
			}
			body.Body = append(body.Body, hir.Assign(hir.Var("s", mir2.TyU8), hir.Add(hir.Var("s", mir2.TyU8), hir.Var("x", mir2.TyU8), mir2.TyU8)))
			h := &hir.Module{Funcs: []*hir.Func{{Name: "foreach_probe", Params: []hir.Param{{Name: "n", Ty: mir2.TyU8}}, RetTy: mir2.TyU8, Body: hir.Blk(hir.Decl("s", mir2.TyU8, hir.U8(0)), &hir.ForEachStmt{Var: "x", ElemTy: mir2.TyU8, Ptr: hir.Addr("data"), Start: hir.U8(0), Len: hir.Var("n", mir2.TyU8), Body: body}, hir.Ret(hir.Var("s", mir2.TyU8)))}}}
			h.Globals = []mir2.Global{{Name: "data", Ty: &mir2.ArrayTy{Elem: mir2.TyU8, Len: 4}, Init: []byte{1, 2, 3, 4}}}
			steps, err := CompileHIRSteps(h, Options{UseGrace: useMIR, SkipZ80Emission: true})
			if err != nil {
				t.Fatal(err)
			}
			for _, n := range []int64{0, 1, 4} {
				vm := mir2.NewVM(steps.MIR2Module)
				vm.EnsureHeap(4)
				vm.WriteHeapSlice(0, []byte{1, 2, 3, 4})
				want := int64(0)
				for i := int64(1); i <= n; i++ {
					if !hasContinue || i != 2 {
						want += i
					}
				}
				got, err := vm.Call("foreach_probe", []mir2.Value{{I: n}})
				if err != nil || len(got) != 1 || got[0].I != want {
					t.Fatalf("continue=%v MIRGrace=%v n%d got%v %v want%d", hasContinue, useMIR, n, got, err, want)
				}
			}
		}
	}
}
