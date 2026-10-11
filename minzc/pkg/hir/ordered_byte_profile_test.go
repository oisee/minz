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

const orderedByteSource = `fun read_word(input:^u8,index:u16)->u16 {var twice:u16=index*2
return u16(input[twice])+u16(input[twice+1])*256}
fun ordinary(n:u8)->u8 {var i:u8=0
while i<n {i=i+1}
return i}
fun same_a()->u8 {return 7}
fun same_b()->u8 {return 7}`

func orderedByteModule(t *testing.T) (*hir.Module, *hir.ProfileCompatibility, string) {
	t.Helper()
	m, e := nanz.Parse(orderedByteSource, "ordered.nanz")
	if e != nil {
		t.Fatal(e)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(orderedByteSource)))
	c, e := hir.BuildProfileCompatibility(m, hir.ProfileContext{CompilerID: "test", SemanticsID: "test", TargetID: "test", WorkloadSHA256: hash, HostsSHA256: hash, Dependencies: map[string]string{"ordered.nanz": hash}})
	if e != nil {
		t.Fatal(e)
	}
	ids, e := hir.EligibleOrderedByteHelpers(m)
	if e != nil || len(ids) != 1 {
		t.Fatal(ids, e)
	}
	return m, c, ids[0]
}
func TestOrderedByteHelperProofChoices(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		source, _, id := orderedByteModule(t)
		actual, c, _ := orderedByteModule(t)
		choices := map[string]bool{id: enabled}
		if e := hir.ApplyOrderedByteHelperChoices(actual, c, choices); e != nil {
			t.Fatal(e)
		}
		p, e := hir.ValidateOrderedByteHelperChoices(source, actual, choices)
		if e != nil || len(p) != 1 || p[0].Enabled != enabled || p[0].FunctionSiteID != id {
			t.Fatal(p, e)
		}
	}
}
func TestOrderedByteHelperProofForgeries(t *testing.T) {
	mutants := map[string]func(*hir.Module){
		"lowwide": func(m *hir.Module) {
			sum := m.FuncByName("read_word").Body.Body[2].(*hir.ReturnStmt).Val.(*hir.BinExpr)
			sum.L.(*hir.CastExpr).X.(*hir.LoadExpr).Ty = mir2.TyU16
		},
		"reverse": func(m *hir.Module) {
			sum := m.FuncByName("read_word").Body.Body[2].(*hir.ReturnStmt).Val.(*hir.BinExpr)
			low := sum.L.(*hir.CastExpr).X.(*hir.LoadExpr)
			high := sum.R.(*hir.BinExpr).L.(*hir.CastExpr).X.(*hir.LoadExpr)
			low.Ptr, high.Ptr = high.Ptr, low.Ptr
		},
		"base": func(m *hir.Module) { m.FuncByName("read_word").Body.Body[0].(*hir.VarDeclStmt).Init = hir.U16(65535) },
		"highoffset": func(m *hir.Module) {
			sum := m.FuncByName("read_word").Body.Body[2].(*hir.ReturnStmt).Val.(*hir.BinExpr)
			sum.R.(*hir.BinExpr).L.(*hir.CastExpr).X.(*hir.LoadExpr).Ptr.(*hir.BinExpr).R = hir.U16(2)
		},
		"metadata":     func(m *hir.Module) { m.FuncByName("read_word").Params[0].SMC = true },
		"ordinarybody": func(m *hir.Module) { m.FuncByName("same_a").Body.Body[0].(*hir.ReturnStmt).Val = hir.U8(8) },
		"rule":         func(m *hir.Module) { m.GraceRulePolicy[hir.RegionCompareRule] = true },
		"unknownfalse": func(m *hir.Module) { m.OrderedBytePolicy["unknown"] = false },
		"originmissing": func(m *hir.Module) {
			for s := range m.SourceLocations {
				delete(m.SourceLocations, s)
				break
			}
		},
		"originorphan": func(m *hir.Module) {
			for s, o := range m.SourceLocations {
				delete(m.SourceLocations, s)
				m.SourceLocations[&hir.ReturnStmt{Val: hir.U8(7)}] = o
				break
			}
		},
		"originswap": func(m *hir.Module) {
			a := m.FuncByName("same_a").Body.Body[0]
			b := m.FuncByName("same_b").Body.Body[0]
			hash := m.SourceUnits["ordered.nanz"]
			m.SourceLocations[a] = hir.SourceLocation{Unit: "ordered.nanz", SourceSHA256: hash, Line: 6, Column: 1, ByteOffset: strings.Index(orderedByteSource, "fun same_b")}
			m.SourceLocations[b] = hir.SourceLocation{Unit: "ordered.nanz", SourceSHA256: hash, Line: 5, Column: 1, ByteOffset: strings.Index(orderedByteSource, "fun same_a")}
		},
		"lineagemissing": func(m *hir.Module) {
			for s := range m.LoopLineage {
				delete(m.LoopLineage, s)
				break
			}
		},
		"lineageorphan": func(m *hir.Module) {
			for s, l := range m.LoopLineage {
				delete(m.LoopLineage, s)
				m.LoopLineage[&hir.ReturnStmt{Val: hir.U8(7)}] = l
				break
			}
		},
	}
	for name, mutate := range mutants {
		t.Run(name, func(t *testing.T) {
			source, _, id := orderedByteModule(t)
			actual, c, _ := orderedByteModule(t)
			if e := hir.ApplyOrderedByteHelperChoices(actual, c, map[string]bool{id: true}); e != nil {
				t.Fatal(e)
			}
			mutate(actual)
			if _, e := hir.ValidateOrderedByteHelperChoices(source, actual, map[string]bool{id: true}); e == nil {
				t.Fatal("forged proof accepted")
			}
		})
	}
}
func TestOrderedByteHelperFreshIdentityAndCap(t *testing.T) {
	source, c, id := orderedByteModule(t)
	before := source.Dump()
	if e := hir.ApplyOrderedByteHelperChoices(source, c, map[string]bool{"unknown": false}); e == nil || source.Dump() != before || source.OrderedBytePolicy != nil {
		t.Fatal("unknown false not transactional")
	}
	source.OrderedBytePolicy = map[string]bool{}
	if _, e := hir.EligibleOrderedByteHelpers(source); e == nil {
		t.Fatal("prior policy accepted")
	}
	a, _, _ := orderedByteModule(t)
	b, _, _ := orderedByteModule(t)
	a.SourceLocations[&hir.ReturnStmt{Val: hir.U8(7)}] = hir.SourceLocation{Unit: "ordered.nanz", SourceSHA256: a.SourceUnits["ordered.nanz"], Line: 1, Column: 1}
	if _, e := hir.BuildFunctionSiteMap(a); e == nil {
		t.Fatal("detached source accepted")
	}
	base, _, _ := orderedByteModule(t)
	sites, e := hir.BuildFunctionSiteMap(base)
	if e != nil {
		t.Fatal(e)
	}
	b.FuncByName("read_word").Name = "different"
	other, e := hir.BuildFunctionSiteMap(b)
	if e != nil {
		t.Fatal(e)
	}
	if fmt.Sprint(sites) == fmt.Sprint(other) {
		t.Fatal("function identity failed to expire")
	}
	capSource := ""
	for i := 0; i < 5; i++ {
		capSource += fmt.Sprintf("fun r%d(p:^u8,k:u16)->u16 {var b:u16=k*2\nreturn u16(p[b])+u16(p[b+1])*256}\n", i)
	}
	m, e := nanz.Parse(capSource, "cap.nanz")
	if e != nil {
		t.Fatal(e)
	}
	ids, e := hir.EligibleOrderedByteHelpers(m)
	if e != nil || len(ids) != 5 {
		t.Fatal(ids, e)
	}
	h := fmt.Sprintf("%x", sha256.Sum256([]byte(capSource)))
	cc, e := hir.BuildProfileCompatibility(m, hir.ProfileContext{CompilerID: "test", SemanticsID: "test", TargetID: "test", WorkloadSHA256: h, HostsSHA256: h, Dependencies: map[string]string{"cap.nanz": h}})
	if e != nil {
		t.Fatal(e)
	}
	if e = hir.ApplyOrderedByteHelperChoices(m, cc, map[string]bool{}); e == nil || m.OrderedBytePolicy != nil {
		t.Fatal("fifth dimension not refused before mutation", e)
	}
	_ = id
}

func TestOrderedByteHelperExactFalsePolicy(t *testing.T) {
	for _, extra := range []bool{false, true} {
		source, _, id := orderedByteModule(t)
		actual, c, _ := orderedByteModule(t)
		choices := map[string]bool{id: false}
		if extra {
			choices = map[string]bool{}
		}
		if e := hir.ApplyOrderedByteHelperChoices(actual, c, choices); e != nil {
			t.Fatal(e)
		}
		if extra {
			actual.OrderedBytePolicy[id] = false
		} else {
			delete(actual.OrderedBytePolicy, id)
		}
		if _, e := hir.ValidateOrderedByteHelperChoices(source, actual, choices); e == nil {
			t.Fatal("nonexact false policy accepted", extra)
		}
	}
}
func TestOrderedByteHelperSourceInvalidRepresentations(t *testing.T) {
	for name, mutate := range map[string]func(*hir.Module){
		"nilfunc": func(m *hir.Module) { m.Funcs[0] = nil },
		"cycle": func(m *hir.Module) {
			x := &hir.CastExpr{Ty: mir2.TyU16}
			x.X = x
			m.Funcs[0].Body.Body[0].(*hir.VarDeclStmt).Init = x
		},
		"sharedstmt": func(m *hir.Module) { m.FuncByName("same_b").Body.Body[0] = m.FuncByName("same_a").Body.Body[0] },
	} {
		t.Run(name, func(t *testing.T) {
			m, _, _ := orderedByteModule(t)
			mutate(m)
			if _, e := hir.BuildFunctionSiteMap(m); e == nil {
				t.Fatal("invalid representation accepted")
			}
		})
	}
}

func TestOrderedByteHelperContractRawDomain(t *testing.T) {
	source, _, id := orderedByteModule(t)
	actual, c, _ := orderedByteModule(t)
	if e := hir.ApplyOrderedByteHelperChoices(actual, c, map[string]bool{id: true}); e != nil {
		t.Fatal(e)
	}
	mods := []*mir2.Module{hir.LowerModule(source), hir.LowerModule(actual)}
	// NewVM reserves all function-name tags, so calculate the real minimum heap.
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
		check := func(arr, idx uint16) {
			low := int(arr + idx*2)
			high := int(arr + idx*2 + 1)
			want := int64(0)
			wantErr := ""
			events := []mir2.VMDataAccess{}
			for j, address := range []int{low, high} {
				if address >= heap {
					wantErr = fmt.Sprintf("load: out-of-bounds address %d (heap size %d)", address, heap)
					break
				}
				value := memory[address]
				events = append(events, mir2.VMDataAccess{Op: mir2.OpLoad, Addr: int64(address), Value: value})
				if j == 0 {
					want = int64(value)
				} else {
					want += int64(value) * 256
				}
			}
			for variant, vm := range vms {
				observed := []mir2.VMDataAccess{}
				vm.ObserveDataAccess = func(a mir2.VMDataAccess) { observed = append(observed, a) }
				got, e := vm.Call("read_word", []mir2.Value{{I: int64(arr)}, {I: int64(idx)}})
				err := ""
				if e != nil {
					err = e.Error()
					if j := strings.Index(err, "load: "); j >= 0 {
						err = err[j:]
					}
				}
				if err != wantErr || !reflect.DeepEqual(observed, events) || (e == nil && !reflect.DeepEqual(got, []mir2.Value{{I: want}})) {
					t.Fatal(heap, arr, idx, variant, got, e, want, wantErr, observed, events)
				}
			}
			count++
		}
		for _, idx := range []uint16{0, 1, 255, 256, 32767, 32768, 32769, 65534, 65535} {
			for _, arr := range []uint16{0, 1, uint16(minimum - 1), uint16(minimum), 65534, 65535} {
				check(arr, idx)
			}
		}
		if heap == 65536 {
			for idx := 0; idx < 65536; idx++ {
				check(0, uint16(idx))
				check(65535, uint16(idx))
			}
		}
		for _, vm := range vms {
			if !reflect.DeepEqual(vm.ReadHeap(0, heap), memory) {
				t.Fatal("memory changed")
			}
		}
	}
	t.Logf("contract raw ordered-byte oracle %d cases per variant", count)
}

func TestOrderedByteHelperStickyPolicyRefusesLegacyApplications(t *testing.T) {
	for name, apply := range map[string]func(*hir.Module, *hir.ProfileCompatibility) error{
		"loop": func(m *hir.Module, c *hir.ProfileCompatibility) error { return hir.ApplyProfileDecisions(m, nil) },
		"copy": func(m *hir.Module, c *hir.ProfileCompatibility) error {
			return hir.ApplyForwardCopySiteChoices(m, c, map[string]bool{})
		},
		"region": func(m *hir.Module, c *hir.ProfileCompatibility) error {
			return hir.ApplyRegionCompareSiteChoices(m, c, map[string]bool{})
		},
		"copyregion": func(m *hir.Module, c *hir.ProfileCompatibility) error {
			return hir.ApplyMixedCopyRegionChoices(m, c, hir.MixedCopyRegionChoices{})
		},
		"copyweighted": func(m *hir.Module, c *hir.ProfileCompatibility) error {
			return hir.ApplyMixedCopyWeightedChoices(m, c, hir.MixedCopyWeightedChoices{})
		},
	} {
		t.Run(name, func(t *testing.T) {
			m, c, _ := orderedByteModule(t)
			m.OrderedBytePolicy = map[string]bool{}
			before := m.Dump()
			if e := apply(m, c); e == nil {
				t.Fatal("legacy API accepted sticky helper policy")
			}
			if m.Dump() != before || m.LoopLineage != nil {
				t.Fatal("refused legacy API mutated source")
			}
		})
	}
}

func TestOrderedByteFunctionSiteBindsOriginPaths(t *testing.T) {
	m, _, _ := orderedByteModule(t)
	// Equal statements with genuine source coordinates formerly collided in the
	// legacy origin multiset. FunctionSiteMap binds each one to its function/path.
	a := m.FuncByName("same_a").Body.Body[0]
	b := m.FuncByName("same_b").Body.Body[0]
	hash := m.SourceUnits["ordered.nanz"]
	m.SourceLocations[a] = hir.SourceLocation{Unit: "ordered.nanz", SourceSHA256: hash, Line: 5, Column: 1, ByteOffset: strings.Index(orderedByteSource, "fun same_a")}
	m.SourceLocations[b] = hir.SourceLocation{Unit: "ordered.nanz", SourceSHA256: hash, Line: 6, Column: 1, ByteOffset: strings.Index(orderedByteSource, "fun same_b")}
	before, e := hir.BuildFunctionSiteMap(m)
	if e != nil {
		t.Fatal(e)
	}
	m.SourceLocations[a], m.SourceLocations[b] = m.SourceLocations[b], m.SourceLocations[a]
	after, e := hir.BuildFunctionSiteMap(m)
	if e != nil {
		t.Fatal(e)
	}
	if before.OriginsSHA256 == after.OriginsSHA256 || reflect.DeepEqual(before.Sites, after.Sites) {
		t.Fatal("origin path association not bound")
	}
}
