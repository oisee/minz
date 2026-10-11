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

const weightedPrototypeSource = `fun weighted(ptr:^u8,n:u8,a:u16,b:u16,seed:u16)->u16 {
 var sum:u16=seed
 var i:u8=0
 while i<n {
 sum=sum+(a+b)
 sum=sum+u16(ptr[i])
 i=i+1
 }
 return sum
}`

func weightedPrototypeTransform(t *testing.T, source string) *hir.Module {
	t.Helper()
	m := graceMeasureParse(t, source)
	// Composition with the higher-priority scalar hoist is deliberately outside
	// this prototype. Preserve the exact weighted source proof shape.
	m.GraceRulePolicy = map[string]bool{"hir-guarded-pointer-walk": true}
	r, err := hir.RunGrace(m, 0)
	if err != nil || r.ByRule["hir-guarded-pointer-walk"] != 1 {
		t.Fatalf("weighted rewrite: %+v %v", r, err)
	}
	if r, err := hir.RunGrace(m, 0); err != nil || r.Applied != 0 {
		t.Fatalf("re-versioned fallback: %+v %v", r, err)
	}
	return m
}

func TestWeightedPointerPrototypeAddressDomain(t *testing.T) {
	count := 0
	for _, tuple := range []bool{false, true} {
		source := weightedPrototypeSource
		if tuple {
			source = strings.Replace(source, "->u16", "->(u16,u8)", 1)
			source = strings.Replace(source, "return sum", "return (sum,i)", 1)
		}
		before := hir.LowerModule(graceMeasureParse(t, source))
		modules := []*mir2.Module{before}
		for _, policy := range []string{"scalar-only", "pointer-only", "both"} {
			m := graceMeasureParse(t, source)
			m.GraceRulePolicy = map[string]bool{"hir-invariant-base": policy != "pointer-only", "hir-guarded-pointer-walk": policy != "scalar-only"}
			if policy == "scalar-only" {
				m.PointerWalkPolicy = map[string]bool{}
			}
			r, err := hir.RunGrace(m, 0)
			if err != nil {
				t.Fatal(err)
			}
			if policy != "scalar-only" && r.ByRule["hir-guarded-pointer-walk"] != 1 {
				t.Fatal("weighted pointer request not realized", policy, r)
			}
			if policy != "pointer-only" && r.ByRule["hir-invariant-base"] != 1 {
				t.Fatal("weighted scalar request not realized", policy, r)
			}
			if r, err := hir.RunGrace(m, 0); err != nil || r.Applied != 0 {
				t.Fatal("nested fallback rewritten", policy, r, err)
			}
			modules = append(modules, hir.LowerModule(m))
		}
		if err := mir2.Verify(before); err != nil {
			t.Fatal(err)
		}
		for _, module := range modules {
			if err := mir2.Verify(module); err != nil {
				t.Fatal(err)
			}
		}
		for _, heapSize := range []int{16, 65536} {
			memory := graceSumMemory(heapSize)
			for n := int64(0); n < 256; n++ {
				for _, base := range []int64{0, 15, 65535 - n, 65536 - n, 65535} {
					for _, weight := range [][3]int64{{0, 0, 0}, {65535, 2, 65535}, {32768, 32768, 1}} {
						var results [4][]mir2.Value
						var errors [4]string
						for j, module := range modules {
							vm := mir2.NewVM(module)
							vm.EnsureHeap(int64(heapSize))
							vm.WriteHeapSlice(0, memory)
							result, err := vm.Call("weighted", []mir2.Value{{I: base}, {I: n}, {I: weight[0]}, {I: weight[1]}, {I: weight[2]}})
							results[j] = result
							errors[j] = graceSumError(err)
							if !bytes.Equal(vm.ReadHeap(0, heapSize), memory) {
								t.Fatal("changed input memory")
							}
						}
						for j := 1; j < len(modules); j++ {
							if errors[0] != errors[j] || !reflect.DeepEqual(results[0], results[j]) {
								t.Fatalf("heap%d ptr%d n%d weights%v variant%d results%v errors%v", heapSize, base, n, weight, j, results, errors)
							}
						}
						sum := uint16(weight[2])
						bad := int64(-1)
						for i := int64(0); i < n; i++ {
							sum += uint16(weight[0]) + uint16(weight[1])
							addr := int64(uint16(base + i))
							if addr >= int64(heapSize) {
								bad = addr
								break
							}
							sum += uint16(memory[addr])
						}
						if bad >= 0 {
							want := fmt.Sprintf("load: out-of-bounds address %d (heap size %d)", bad, heapSize)
							if errors[1] != want {
								t.Fatalf("first invalid-address oracle: %s want%s", errors[1], want)
							}
						} else {
							want := []mir2.Value{{I: int64(sum)}}
							if tuple {
								want = append(want, mir2.Value{I: n})
							}
							if errors[1] != "" || !reflect.DeepEqual(results[1], want) {
								t.Fatalf("independent weighted modulo oracle: %v/%s want%v", results[1], errors[1], want)
							}
						}
						count++
					}
				}
			}
		}
	}
	t.Logf("Verified%d weighted address/length/seed scenarios before/after raw VM plus independent oracle", count)
}

func TestWeightedPointerPrototypeLegality(t *testing.T) {
	for name, source := range map[string]string{
		"weight_call":        strings.Replace(weightedPrototypeSource, "a+b", "get_weight()+b", 1) + "\nfun get_weight()->u16{return 1}",
		"weight_global":      "global weight:u16=3\n" + strings.Replace(weightedPrototypeSource, "a+b", "weight+b", 1),
		"weight_nested_expr": strings.Replace(weightedPrototypeSource, "a+b", "(a+1)+b", 1),
		"weight_mutated":     strings.Replace(weightedPrototypeSource, "i=i+1", "a=a+1\n i=i+1", 1),
		"reordered":          strings.Replace(weightedPrototypeSource, "sum=sum+(a+b)\n sum=sum+u16(ptr[i])", "sum=sum+u16(ptr[i])\n sum=sum+(a+b)", 1),
		"nonzero_index":      strings.Replace(weightedPrototypeSource, "i:u8=0", "i:u8=1", 1),
		"stride":             strings.Replace(weightedPrototypeSource, "i=i+1", "i=i+2", 1),
		"unextended_byte":    strings.Replace(weightedPrototypeSource, "u16(ptr[i])", "u16(ptr[i]+1)", 1),
		"seed_expr":          strings.Replace(weightedPrototypeSource, "sum:u16=seed", "sum:u16=seed+1", 1),
		"stored_effect":      "global effect:u16=0\n" + strings.Replace(weightedPrototypeSource, "i=i+1", "effect=sum\n i=i+1", 1),
	} {
		t.Run(name, func(t *testing.T) {
			m := graceMeasureParse(t, source)
			if len(hir.AnalyzeFacts(m).PointerWalks) != 0 {
				t.Fatal("unsafe weighted source admitted")
			}
		})
	}
	for _, weight := range []string{"a+b", "a+65535", "32768+32768", "a+a"} {
		t.Run("allowed_"+weight, func(t *testing.T) {
			weightedPrototypeTransform(t, strings.Replace(weightedPrototypeSource, "a+b", weight, 1))
		})
	}
}

func TestWeightedPointerPrototypeRealSourceGuardAndOrdering(t *testing.T) {
	source, err := os.ReadFile("../../../examples/nanz/grace_weighted_checksum.nanz")
	if err != nil {
		t.Fatal(err)
	}
	m := graceMeasureParse(t, string(source))
	if len(hir.AnalyzeFacts(m).PointerWalks) != 1 {
		t.Fatal("existing weighted_checksum source not admitted")
	}
	p, err := hir.NewLoopProfiler(m)
	if err != nil {
		t.Fatal(err)
	}
	p.VM.EnsureHeap(65536)
	for _, n := range []int64{0, 1, 255} {
		if _, err := p.Call("weighted_checksum", []mir2.Value{{I: 0x4000}, {I: n}, {I: 65535}, {I: 2}, {I: 65535}}); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := p.Snapshot()
	if len(snapshot.Guards) != 1 || snapshot.Guards[0].FastEntries != 3 || snapshot.Guards[0].FastBodyEntries != 256 {
		t.Fatalf("weighted guard collector:%+v", snapshot.Guards)
	}
	transformed := weightedPrototypeTransform(t, weightedPrototypeSource)
	guard := transformed.FuncByName("weighted").Body.Body[2].(*hir.IfStmt)
	each := guard.Then.Body[0].(*hir.ForEachStmt)
	if !each.PtrIter || len(each.Body.Body) != 2 {
		t.Fatal("read must be deferred to original body position")
	}
	first := each.Body.Body[0].(*hir.AssignStmt).Val.(*hir.BinExpr)
	if _, ok := first.R.(*hir.BinExpr); !ok {
		t.Fatal("weight calculation lost")
	}
	second := each.Body.Body[1].(*hir.AssignStmt).Val.(*hir.BinExpr)
	extended := second.R.(*hir.CastExpr)
	if _, ok := extended.X.(*hir.LoadExpr); !ok {
		t.Fatal("byte load must remain after weight calculation")
	}
}

const weightedHoistedSource = `fun weighted(ptr:^u8,n:u8,a:u16,b:u16,seed:u16)->u16 {
 var sum:u16=seed
 var i:u8=0
 var weight:u16=a+b
 while i<n {
 sum=sum+weight
 sum=sum+u16(ptr[i])
 i=i+1
 }
 return sum
}`

func TestWeightedPointerPrototypeHoistedPrefixRefusals(t *testing.T) {
	weightedPrototypeTransform(t, weightedHoistedSource)
	for name, source := range map[string]string{
		"temp_global":        "global weight:u16=0\n" + weightedHoistedSource,
		"temp_parameter":     strings.Replace(weightedHoistedSource, "seed:u16)", "seed:u16,weight:u16)", 1),
		"temp_array":         strings.Replace(weightedHoistedSource, "weight:u16=a+b", "weight:[u16;1]=[a+b]", 1),
		"initializer_call":   strings.Replace(weightedHoistedSource, "weight:u16=a+b", "weight:u16=get_weight()+b", 1) + "\nfun get_weight()->u16{return 7}",
		"initializer_global": "global initial:u16=3\n" + strings.Replace(weightedHoistedSource, "weight:u16=a+b", "weight:u16=initial+b", 1),
		"initializer_nested": strings.Replace(weightedHoistedSource, "weight:u16=a+b", "weight:u16=(a+1)+b", 1),
		"extra_temp_use":     strings.Replace(weightedHoistedSource, "return sum", "return sum+weight", 1),
		"mutated_temp":       strings.Replace(weightedHoistedSource, "i=i+1", "weight=weight+1\n i=i+1", 1),
		"nonweighted_loop":   strings.Replace(weightedHoistedSource, "sum=sum+weight\n", "", 1),
	} {
		t.Run(name, func(t *testing.T) {
			m := graceMeasureParse(t, source)
			if len(hir.AnalyzeFacts(m).PointerWalks) != 0 {
				t.Fatal("unsafe hoisted prefix admitted")
			}
		})
	}
}
