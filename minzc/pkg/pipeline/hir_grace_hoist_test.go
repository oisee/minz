package pipeline

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// Extracted unchanged from irc_client.nanz; full-source inventory below keeps
// this fixture connected to the real program rather than only a toy pattern.
const graceRegionSource = `fun region_match(buf: ^u8, offset: u16, target: ^u8, len: u8) -> u8 {
 var i: u8 = 0
 while i < len {
  let p: ^u8 = buf + offset + i
  if p^ != target[i] { return 0 }
  i = i + 1
 }
 return 1
}`

func TestHIRGraceRealCorpusCandidates(t *testing.T) {
	src, err := os.ReadFile("../../../examples/nanz/irc_client.nanz")
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.ParseWithOpts(string(src), "irc_client.nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	f := hir.AnalyzeFacts(h)
	loops := 0
	for _, ff := range f.Functions {
		loops += ff.Loops
	}
	r, err := hir.RunGrace(h, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("irc_client: functions=%d calls=%d loops=%d invariant-base rewrites=%d", len(f.Functions), f.DB.Count("hir-call"), loops, r.ByRule["hir-invariant-base"])
	if r.ByRule["hir-invariant-base"] != 1 {
		t.Fatalf("real patterns missing: %+v", r)
	}
}

func TestHIRGraceInvariantBaseDifferential(t *testing.T) {
	for _, final := range []bool{false, true} {
		for _, mirGrace := range []bool{false, true} {
			t.Run(fmt.Sprintf("final=%v/mirGrace=%v", final, mirGrace), func(t *testing.T) {
				h := graceMeasureParse(t, graceRegionSource)
				r, err := hir.RunGrace(h, 0)
				if err != nil || r.ByRule["hir-invariant-base"] != 1 {
					t.Fatalf("rule: %+v %v", r, err)
				}
				before := graceMeasureModule(t, graceRegionSource, false, final, mirGrace)
				after := graceMeasureModule(t, graceRegionSource, true, final, mirGrace)
				var steps [2]int64
				cases := [][4]int64{{0, 0, 512, 0}, {65535, 1, 0, 2}, {1, 65535, 0, 2}, {65535, 65535, 0, 3}, {65535, 0, 0, 2}, {65535, 0, 65535, 2}, {1024, 32, 4096, 200}}
				for _, heapSize := range []int{16, 65536} {
					for _, args := range cases {
						for _, mismatch := range []int{-1, 0, int(args[3]) - 1} {
							memory := make([]byte, heapSize)
							if mismatch >= 0 {
								addr := args[2] + int64(mismatch)
								if addr < int64(heapSize) {
									memory[addr] = 1
								}
							}
							var results [2][]mir2.Value
							var errs [2]string
							var heaps [2][]byte
							for side, m := range []*mir2.Module{before, after} {
								vm := mir2.NewVM(m)
								vm.EnsureHeap(int64(heapSize))
								vm.WriteHeapSlice(0, memory)
								vals := []mir2.Value{{I: args[0]}, {I: args[1]}, {I: args[2]}, {I: args[3]}}
								results[side], err = vm.Call("region_match", vals)
								if err != nil {
									errs[side] = err.Error()
								}
								heaps[side] = append([]byte(nil), vm.ReadHeap(0, heapSize)...)
								if heapSize == 65536 && args[0] == 1024 && mismatch == -1 {
									steps[side] = vm.StepsExecuted()
								}
							}
							if !reflect.DeepEqual(results[0], results[1]) || errs[0] != errs[1] || !reflect.DeepEqual(heaps[0], heaps[1]) {
								t.Fatalf("heap=%d args=%v mismatch=%d before=%v %q after=%v %q", heapSize, args, mismatch, results[0], errs[0], results[1], errs[1])
							}
							// Independent oracle for successful non-mismatch executions.
							if mismatch == -1 && heapSize == 65536 && args != [4]int64{65535, 0, 65535, 2} && (errs[1] != "" || len(results[1]) != 1 || results[1][0].I != 1) {
								t.Fatalf("success oracle args=%v: %v %q", args, results[1], errs[1])
							}
						}
					}
				}
				t.Logf("200-byte matching region VM instructions: %d -> %d", steps[0], steps[1])
				if final && mirGrace {
					// MIR's generalized invariant-address pass now performs the same
					// hoist. HIR must preserve that benefit without claiming a second
					// reduction for the same operation.
					plain := graceMeasureModule(t, graceRegionSource, false, true, false)
					vm := mir2.NewVM(plain)
					vm.EnsureHeap(65536)
					got, err := vm.Call("region_match", []mir2.Value{{I: 1024}, {I: 32}, {I: 4096}, {I: 200}})
					if err != nil || len(got) != 1 || got[0].I != 1 {
						t.Fatalf("unhoisted oracle: %v %v", got, err)
					}
					t.Logf("unhoisted final MIR: %d VM instructions", vm.StepsExecuted())
					if steps[0] != steps[1] || steps[1] >= vm.StepsExecuted() {
						t.Fatal("overlapping HIR/MIR hoists must retain the measured baseline improvement")
					}
				} else if steps[1] >= steps[0] {
					t.Fatal("no VM work improvement")
				}
			})
		}
	}
}

func TestHIRGraceInvariantBaseMutationGuard(t *testing.T) {
	src := `fun f(buf: ^u8, offset:u16, len:u8)->u16 { var i:u8=0
 while i<len {
 let p:^u8=buf+offset+i
 offset=offset+1
 i=i+1
 }
 return offset }`
	h := graceMeasureParse(t, src)
	r, err := hir.RunGrace(h, 0)
	if err != nil || r.Applied != 0 {
		t.Fatalf("mutated parameter hoisted: %+v %v", r, err)
	}
}

func BenchmarkHIRGraceInvariantBaseVM(b *testing.B) {
	for _, useHIR := range []bool{false, true} {
		b.Run(fmt.Sprintf("hir=%v", useHIR), func(b *testing.B) {
			m := graceMeasureModule(b, graceRegionSource, useHIR, true)
			vm := mir2.NewVM(m)
			vm.EnsureHeap(65536)
			args := []mir2.Value{{I: 1024}, {I: 32}, {I: 4096}, {I: 200}}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ret, err := vm.Call("region_match", args)
				if err != nil || ret[0].I != 1 {
					b.Fatalf("%v %v", ret, err)
				}
			}
		})
	}
}

func TestHIRGraceCapturedIteratorGuard(t *testing.T) {
	src := `fun f(buf:^u8,offset:u16,len:u8)->u8 {
 var i:u8=0
 var s:u8=0
 while i<len {
  let p:^u8=buf+offset+i
  s=s+p^
  buf.forEach(|x:u8| {offset=offset+1},1)
  i=i+1
 }
 return s
}`
	h := graceMeasureParse(t, src)
	before := hir.LowerModule(h).Dump()
	r, err := hir.RunGrace(h, 0)
	if err != nil || r.Applied != 0 || hir.LowerModule(h).Dump() != before {
		t.Fatalf("captured binding hoisted: %+v %v", r, err)
	}
	vm := mir2.NewVM(hir.LowerModule(h))
	vm.EnsureHeap(4)
	vm.WriteHeapSlice(0, []byte{1, 2, 3, 4})
	got, err := vm.Call("f", []mir2.Value{{I: 0}, {I: 0}, {I: 2}})
	if err != nil || len(got) != 1 || got[0].I != 4 {
		t.Fatalf("capture oracle: %v %v", got, err)
	}
}

func TestHIRGraceConstantIfCapturedIteratorGuard(t *testing.T) {
	src := `fun f(buf:^u8,offset:u16)->u16 {
 if true { buf.forEach(|x:u8| {offset=offset+1},1) }
 return offset
}`
	h := graceMeasureParse(t, src)
	before := hir.LowerModule(h).Dump()
	r, err := hir.RunGrace(h, 0)
	if err != nil || r.Applied != 0 || hir.LowerModule(h).Dump() != before {
		t.Fatalf("branch capture leaked: %+v %v", r, err)
	}
}

func TestHIRGraceInvariantBaseFreshnessAndBudget(t *testing.T) {
	src := "global __grace_base_0:u16\n" + graceRegionSource
	h := graceMeasureParse(t, src)
	facts := hir.AnalyzeFacts(h)
	if facts.DB.Count("hir-invariant-base") != 1 {
		t.Fatal("missing candidate fact")
	}
	r, err := hir.RunGrace(h, 1)
	if err != nil || r.Applied != 1 {
		t.Fatalf("budget: %+v %v", r, err)
	}
	if facts.DB.Count("hir-invariant-base") != 1 || hir.AnalyzeFacts(h).DB.Count("hir-invariant-base") != 0 {
		t.Fatal("snapshot invalidation")
	}
	if !strings.Contains(h.Dump(), "__grace_base_1") {
		t.Fatal("global freshness collision")
	}
	r, err = hir.RunGrace(h, 0)
	if err != nil || r.Applied != 0 {
		t.Fatalf("idempotence: %+v %v", r, err)
	}
}
