package hir_test

import (
	"crypto/sha256"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"reflect"
	"strings"
	"testing"
)

const orderedByteRWSource = orderedByteSource + `
fun write_word(output:^u8,index:u16,value:u16){var twice:u16=index*2
output[twice]=u8(value)
output[twice+1]=u8(value/256)}`

func orderedByteRWModule(t *testing.T) (*hir.Module, *hir.ProfileCompatibility, []hir.OrderedByteRWFunctionSite) {
	t.Helper()
	m, e := nanz.Parse(orderedByteRWSource, "rw.nanz")
	if e != nil {
		t.Fatal(e)
	}
	h := fmt.Sprintf("%x", sha256.Sum256([]byte(orderedByteRWSource)))
	c, e := hir.BuildProfileCompatibility(m, hir.ProfileContext{CompilerID: "test", SemanticsID: "test", TargetID: "test", WorkloadSHA256: h, HostsSHA256: h, Dependencies: map[string]string{"rw.nanz": h}})
	if e != nil {
		t.Fatal(e)
	}
	sites, e := hir.EligibleOrderedByteRWHelpers(m)
	if e != nil || len(sites) != 2 {
		t.Fatal(sites, e)
	}
	return m, c, sites
}
func rwChoices(sites []hir.OrderedByteRWFunctionSite, mask int) map[string]hir.FunctionChoice {
	out := map[string]hir.FunctionChoice{}
	for j, s := range sites {
		out[s.ID] = hir.FunctionChoice{Kind: s.Kind, Enabled: mask&(1<<j) != 0}
	}
	return out
}
func TestOrderedByteRWFourChoicesAndReadonlyStrict(t *testing.T) {
	for mask := 0; mask < 4; mask++ {
		source, _, sites := orderedByteRWModule(t)
		actual, c, _ := orderedByteRWModule(t)
		choices := rwChoices(sites, mask)
		if e := hir.ApplyOrderedByteRWChoices(actual, c, choices); e != nil {
			t.Fatal(mask, e)
		}
		proof, e := hir.ValidateOrderedByteRWChoices(source, actual, choices)
		if e != nil || len(proof) != 2 {
			t.Fatal(mask, proof, e)
		}
		for _, p := range proof {
			if p.Kind != choices[p.FunctionSiteID].Kind || p.Enabled != choices[p.FunctionSiteID].Enabled {
				t.Fatal("proof kind/choice", p)
			}
		}
		if _, e = hir.ValidateOrderedByteHelperChoices(source, actual, map[string]bool{}); e == nil {
			t.Fatal("readonlyproof accepted RWpolicy")
		}
	}
	m, _, _ := orderedByteRWModule(t)
	ids, e := hir.EligibleOrderedByteHelpers(m)
	if e != nil || len(ids) != 1 {
		t.Fatal("readonly eligibility expanded", ids, e)
	}
}
func TestOrderedByteRWWriteForgeries(t *testing.T) {
	mutations := map[string]func(*hir.Module){
		"reverse": func(m *hir.Module) { b := m.FuncByName("write_word").Body; b.Body[2], b.Body[3] = b.Body[3], b.Body[2] },
		"wide": func(m *hir.Module) {
			m.FuncByName("write_word").Body.Body[2].(*hir.StoreStmt).Val = hir.Var("value", mir2.TyU16)
		},
		"highoffset": func(m *hir.Module) {
			m.FuncByName("write_word").Body.Body[3].(*hir.StoreStmt).Ptr.(*hir.BinExpr).R = hir.U16(2)
		},
		"highvalue": func(m *hir.Module) {
			m.FuncByName("write_word").Body.Body[3].(*hir.StoreStmt).Val = hir.Cast(hir.Var("value", mir2.TyU16), mir2.TyU8)
		},
		"hoistedvalue": func(m *hir.Module) {
			b := m.FuncByName("write_word").Body
			b.Body = append(b.Body[:2], append([]hir.Stmt{hir.Decl("high", mir2.TyU8, b.Body[3].(*hir.StoreStmt).Val)}, b.Body[2:]...)...)
		},
		"base": func(m *hir.Module) { m.FuncByName("write_word").Body.Body[0].(*hir.VarDeclStmt).Init = hir.U16(65535) },
		"SMC":  func(m *hir.Module) { m.FuncByName("write_word").Params[2].SMC = true },
		"originorphan": func(m *hir.Module) {
			for s, o := range m.SourceLocations {
				delete(m.SourceLocations, s)
				m.SourceLocations[&hir.ReturnStmt{Val: hir.U8(7)}] = o
				break
			}
		},
		"ordinary": func(m *hir.Module) { m.FuncByName("same_a").Body.Body[0].(*hir.ReturnStmt).Val = hir.U8(8) },
		"kind": func(m *hir.Module) {
			for id, c := range m.OrderedByteRWPolicy {
				c.Kind = "wrong"
				m.OrderedByteRWPolicy[id] = c
				break
			}
		},
		"foreignreadonly": func(m *hir.Module) { m.OrderedBytePolicy = map[string]bool{} },
		"missingfalse": func(m *hir.Module) {
			for id := range m.OrderedByteRWPolicy {
				delete(m.OrderedByteRWPolicy, id)
				break
			}
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			source, _, sites := orderedByteRWModule(t)
			actual, c, _ := orderedByteRWModule(t)
			choices := rwChoices(sites, 3)
			if name == "missingfalse" {
				choices = rwChoices(sites, 0)
			}
			if e := hir.ApplyOrderedByteRWChoices(actual, c, choices); e != nil {
				t.Fatal(e)
			}
			mutate(actual)
			if _, e := hir.ValidateOrderedByteRWChoices(source, actual, choices); e == nil {
				t.Fatal("writeforgery accepted")
			}
		})
	}
}
func TestOrderedByteRWUnknownKindsAndSticky(t *testing.T) {
	for _, kind := range []string{"", hir.OrderedByteWriteKind, "unknown"} {
		m, c, sites := orderedByteRWModule(t)
		var id string
		for _, s := range sites {
			if s.Kind == hir.OrderedByteReadKind {
				id = s.ID
			}
		}
		before := m.Dump()
		if e := hir.ApplyOrderedByteRWChoices(m, c, map[string]hir.FunctionChoice{id: {Kind: kind, Enabled: false}}); e == nil {
			t.Fatal("false kind mismatch accepted")
		}
		if m.Dump() != before || m.OrderedByteRWPolicy != nil {
			t.Fatal("refused input mutated")
		}
	}
	m, c, _ := orderedByteRWModule(t)
	m.OrderedByteRWPolicy = map[string]hir.FunctionChoice{}
	if e := hir.ApplyOrderedByteHelperChoices(m, c, map[string]bool{}); e == nil {
		t.Fatal("readonly applied overstickyRW")
	}
	if _, e := hir.EligibleOrderedByteHelpers(m); e == nil {
		t.Fatal("readonlysource acceptedstickyRW")
	}
}
func TestOrderedByteRWRawStoreDomain(t *testing.T) {
	source, _, sites := orderedByteRWModule(t)
	actual, c, _ := orderedByteRWModule(t)
	if e := hir.ApplyOrderedByteRWChoices(actual, c, rwChoices(sites, 3)); e != nil {
		t.Fatal(e)
	}
	mods := []*mir2.Module{hir.LowerModule(source), hir.LowerModule(actual)}
	minimum := 0
	for _, f := range source.Funcs {
		minimum += len(f.Name) + 1
	}
	count := 0
	for _, heap := range []int{minimum, minimum + 1, 65536} {
		memory := make([]byte, heap)
		for j := range memory {
			memory[j] = byte(j*37 + 11)
		}
		vms := []*mir2.VM{}
		for _, m := range mods {
			vm := mir2.NewVM(m)
			vm.EnsureHeap(int64(heap))
			vm.WriteHeapSlice(0, memory)
			vms = append(vms, vm)
		}
		check := func(arr, idx, val uint16) {
			low := int(arr + idx*2)
			high := int(arr + idx*2 + 1)
			events := []mir2.VMDataAccess{}
			err := ""
			for j, address := range []int{low, high} {
				if address >= heap {
					err = fmt.Sprintf("store: out-of-bounds address %d (heap size %d)", address, heap)
					break
				}
				v := byte(val)
				if j == 1 {
					v = byte(val >> 8)
				}
				events = append(events, mir2.VMDataAccess{Op: mir2.OpStore, Addr: int64(address), Value: v})
			}
			for variant, vm := range vms {
				observed := []mir2.VMDataAccess{}
				vm.ObserveDataAccess = func(a mir2.VMDataAccess) { observed = append(observed, a) }
				_, e := vm.Call("write_word", []mir2.Value{{I: int64(arr)}, {I: int64(idx)}, {I: int64(val)}})
				gotErr := ""
				if e != nil {
					gotErr = e.Error()
					if j := strings.Index(gotErr, "store: "); j >= 0 {
						gotErr = gotErr[j:]
					}
				}
				if gotErr != err || !reflect.DeepEqual(observed, events) {
					t.Fatal(heap, arr, idx, val, variant, e, err, observed, events)
				}
				for _, event := range events {
					if vm.ReadHeap(event.Addr, 1)[0] != event.Value {
						t.Fatal("partialmemory", heap, arr, idx, val)
					}
				}
				for _, event := range events {
					vm.WriteHeap(event.Addr, memory[event.Addr])
				}
			}
			count++
		}
		for _, idx := range []uint16{0, 1, 255, 256, 32767, 32768, 32769, 65534, 65535} {
			for _, arr := range []uint16{0, 1, uint16(minimum - 1), uint16(minimum), 65534, 65535} {
				for _, v := range []uint16{0, 1, 255, 256, 0xABCD, 65535} {
					check(arr, idx, v)
				}
			}
		}
		if heap == 65536 {
			for idx := 0; idx < 65536; idx++ {
				check(0, uint16(idx), uint16(idx*257))
				check(65535, uint16(idx), uint16(idx*257))
			}
		}
		for _, vm := range vms {
			if !reflect.DeepEqual(vm.ReadHeap(0, heap), memory) {
				t.Fatal("unexpectedmemory")
			}
		}
	}
	t.Logf("RWcontract orderedstore/partialmemory oracle %d casespervariant", count)
}

func TestOrderedByteRWNilEmptyAndLegacyRefusals(t *testing.T) {
	for _, choices := range []map[string]hir.FunctionChoice{nil, {}} {
		source, _, _ := orderedByteRWModule(t)
		actual, c, _ := orderedByteRWModule(t)
		if e := hir.ApplyOrderedByteRWChoices(actual, c, choices); e != nil {
			t.Fatal(e)
		}
		if _, e := hir.ValidateOrderedByteRWChoices(source, actual, choices); e != nil {
			t.Fatal("nil/emptychoices", e)
		}
	}
	for name, apply := range map[string]func(*hir.Module, *hir.ProfileCompatibility) error{
		"readonly": func(m *hir.Module, c *hir.ProfileCompatibility) error {
			return hir.ApplyOrderedByteHelperChoices(m, c, nil)
		},
		"copy": func(m *hir.Module, c *hir.ProfileCompatibility) error {
			return hir.ApplyForwardCopySiteChoices(m, c, nil)
		},
		"region": func(m *hir.Module, c *hir.ProfileCompatibility) error {
			return hir.ApplyRegionCompareSiteChoices(m, c, nil)
		},
		"copyregion": func(m *hir.Module, c *hir.ProfileCompatibility) error {
			return hir.ApplyMixedCopyRegionChoices(m, c, hir.MixedCopyRegionChoices{})
		},
		"copyweighted": func(m *hir.Module, c *hir.ProfileCompatibility) error {
			return hir.ApplyMixedCopyWeightedChoices(m, c, hir.MixedCopyWeightedChoices{})
		},
		"loop": func(m *hir.Module, c *hir.ProfileCompatibility) error { return hir.ApplyProfileDecisions(m, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			m, c, _ := orderedByteRWModule(t)
			m.OrderedByteRWPolicy = map[string]hir.FunctionChoice{}
			before := m.Dump()
			if e := apply(m, c); e == nil {
				t.Fatal("legacyacceptedRWsticky")
			}
			if m.Dump() != before || m.LoopLineage != nil {
				t.Fatal("legacyrefusalmutated")
			}
		})
	}
}
