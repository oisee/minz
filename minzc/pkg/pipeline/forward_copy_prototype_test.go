package pipeline

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
	"strings"
	"testing"
)

const forwardCopySource = `fun copy_forward(dst:^u8,src:^u8,n:u8)->u8 {
 var i:u8=0
 while i<n {dst[i]=src[i]
 i=i+1}
 return i
}`

func forwardCopyModules(t *testing.T) (*mir2.Module, *mir2.Module) {
	t.Helper()
	original := graceMeasureParse(t, forwardCopySource)
	actual := graceMeasureParse(t, forwardCopySource)
	if err := hir.SeedLoopLineage(actual); err != nil {
		t.Fatal(err)
	}
	actual.GraceRulePolicy = map[string]bool{hir.ForwardCopyRule: true}
	result, err := hir.RunGrace(actual, 0)
	if err != nil || result.ByRule[hir.ForwardCopyRule] != 1 {
		t.Fatal("copy transformation", result, err)
	}
	lineage := hir.CurrentLoopLineage(actual)
	if len(lineage) != 2 || lineage[0].Lineage.SiteID != lineage[1].Lineage.SiteID {
		t.Fatal("copy lineage", lineage)
	}
	if result, err := hir.RunGrace(actual, 0); err != nil || result.Applied != 0 {
		t.Fatal("fallback reversioned", result, err)
	}
	before, after := hir.LowerModule(original), hir.LowerModule(actual)
	if err := mir2.Verify(before); err != nil {
		t.Fatal(err)
	}
	if err := mir2.Verify(after); err != nil {
		t.Fatal(err)
	}
	return before, after
}

func TestForwardCopyPrototypeAddressAliasDomain(t *testing.T) {
	before, after := forwardCopyModules(t)
	scenarios := 0
	for _, heap := range []int{16, 65536} {
		initial := graceSumMemory(heap)
		for n := int64(0); n < 256; n++ {
			pairs := [][2]int64{{0, 0}, {1, 0}, {0, 1}, {7, 3}, {3, 7}, {15, 0}, {0, 15}, {65535 - n, 0}, {0, 65535 - n}, {65536 - n, 0}, {0, 65536 - n}, {65535, 0}, {0, 65535}, {65535, 65535}, {65534, 65535}, {65535, 65534}}
			for _, pair := range pairs {
				wantMemory := append([]byte(nil), initial...)
				wantError := ""
				for i := int64(0); i < n; i++ {
					src := int64(uint16(pair[1] + i))
					dst := int64(uint16(pair[0] + i))
					if src >= int64(heap) {
						wantError = fmt.Sprintf("load: out-of-bounds address %d (heap size %d)", src, heap)
						break
					}
					value := wantMemory[src]
					if dst >= int64(heap) {
						wantError = fmt.Sprintf("store: out-of-bounds address %d (heap size %d)", dst, heap)
						break
					}
					wantMemory[dst] = value
				}
				for j, module := range []*mir2.Module{before, after} {
					vm := mir2.NewVM(module)
					vm.EnsureHeap(int64(heap))
					vm.WriteHeapSlice(0, initial)
					result, err := vm.Call("copy_forward", []mir2.Value{{I: pair[0]}, {I: pair[1]}, {I: n}})
					if graceSumError(err) != wantError {
						t.Fatalf("heap%d n%d dst%d src%d variant%d error%s want%s", heap, n, pair[0], pair[1], j, graceSumError(err), wantError)
					}
					if wantError == "" && !reflect.DeepEqual(result, []mir2.Value{{I: n}}) {
						t.Fatal("finalindex oracle", result, n)
					}
					if !bytes.Equal(vm.ReadHeap(0, heap), wantMemory) {
						t.Fatal("partial/sequential memory oracle", heap, n, pair, j)
					}
				}
				scenarios++
			}
		}
	}
	t.Logf("Verified%d sequentialcopy alias/address scenarios on rawVM beforeafter withfirsttrap/partialmemory oracle", scenarios)
}

func TestForwardCopyPrototypeLegalityAndOptIn(t *testing.T) {
	untouched := graceMeasureParse(t, forwardCopySource)
	before := untouched.Dump()
	if r, err := hir.RunGrace(untouched, 0); err != nil || r.Applied != 0 || before != untouched.Dump() {
		t.Fatal("copy must require explicit opt-in", r, err)
	}
	for name, source := range map[string]string{
		"nonzero_index":  strings.Replace(forwardCopySource, "i:u8=0", "i:u8=1", 1),
		"stride":         strings.Replace(forwardCopySource, "i=i+1", "i=i+2", 1),
		"reversed_index": strings.Replace(forwardCopySource, "src[i]", "src[n-i]", 1),
		"extra_store":    strings.Replace(forwardCopySource, "i=i+1", "dst[0]=0\n i=i+1", 1),
		"mutated_source": strings.Replace(forwardCopySource, "i=i+1", "src=dst\n i=i+1", 1),
		"wrong_return":   strings.Replace(forwardCopySource, "return i", "return n", 1),
	} {
		t.Run(name, func(t *testing.T) {
			m := graceMeasureParse(t, source)
			m.GraceRulePolicy = map[string]bool{hir.ForwardCopyRule: true}
			if r, err := hir.RunGrace(m, 0); err != nil || r.Applied != 0 {
				t.Fatal("unsafe copy admitted", r, err)
			}
		})
	}
}

func TestForwardCopyPrototypeZ80Boundary(t *testing.T) {
	for _, n := range []int64{0, 1, 5, 255} {
		for _, pair := range [][2]int64{{0x5000, 0x4000}, {0x4001, 0x4000}, {0x4000, 0x4001}, {65535 - n, 0x4000}, {65536 - n, 0x4000}, {0x5000, 65535 - n}, {0x5000, 65536 - n}, {65535, 65535}} {

			memory := map[uint16]byte{}
			for i := int64(0); i < n; i++ {
				memory[uint16(pair[0]+i)] = byte(i*17 + 3)
				memory[uint16(pair[1]+i)] = byte(i*37 + 11)
			}
			expected := map[uint16]byte{}
			for addr, value := range memory {
				expected[addr] = value
			}
			for i := int64(0); i < n; i++ {
				expected[uint16(pair[0]+i)] = expected[uint16(pair[1]+i)]
			}
			t.Logf("copy dst%d src%d n%d", pair[0], pair[1], n)
			for _, enabled := range []bool{false, true} {
				m := graceMeasureParse(t, forwardCopySource)
				m.GraceRulePolicy = map[string]bool{hir.ForwardCopyRule: enabled}
				measureForwardCopyZ80Memory(t, m, "copy_forward", Options{AssertMode: "none", UseHIRGrace: true}, []int64{pair[0], pair[1], n}, memory, n, expected)
			}
		}
	}
}
