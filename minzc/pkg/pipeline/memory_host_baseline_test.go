package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoryHostFactoryOriginalParser(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	factory := func() (*hir.Module, error) { m, _ := memoryHostParserFixture(t); return m, nil }
	r, e := RunZ80MemoryHostBaseline(factory, w)
	if e != nil {
		t.Fatal(e)
	}
	exportMemoryHostReferenceBeforeAssert(t, "factory", r.Target, map[string]any{"factory-baseline.json": r})
	// This fixture binds the current known adapter binary and cycle reference.
	// A legitimate backend change must remeasure this reference while keeping
	// the independent parser state/event oracle; production has no fixed cost.
	assertMemoryHostParserIndependentOracle(t, r.Raw.Invocations, r.Target.Invocations)
	assertCurrentMemoryHostParserReference(t, r.Target)
	if dir := os.Getenv("MINZ_MEMORY_HOST_ARTIFACTS"); dir != "" {
		os.MkdirAll(dir, 0755)
		b, e := json.MarshalIndent(r, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, "factory-baseline.json"), b, 0644); e != nil {
			t.Fatal(e)
		}
	}
}
func TestMemoryHostFactoryRefusals(t *testing.T) {
	const source = `@extern fun peek(addr:u16)->u8
fun load(addr:u16)->u8{return peek(addr)}
fun same_one()->u16{return 7}
fun same_two()->u16{return 7}`
	parse := func() (*hir.Module, error) { return nanz.Parse(source, "factory.nanz") }
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8}, Regions: map[string]hir.MemoryHostRegion{"data": {Address: 0x8000, Bytes: []byte{7}}}, Calls: []hir.MemoryHostInvocation{{Function: "load", Arguments: []int64{0x8000}}}}
	for _, mode := range []string{"nil", "shared", "shared_ast", "source_changed", "prior_only_origin", "fresh_and_prior_origin", "prior_policy", "third_call_origin", "caller_bytes_mutated"} {
		t.Run(mode, func(t *testing.T) {
			count := 0
			prior := []*hir.Module{}
			shared, _ := parse()
			work := w
			work.Regions = map[string]hir.MemoryHostRegion{"data": {Address: 0x8000, Bytes: []byte{7}}}
			factory := func() (*hir.Module, error) {
				m, e := parse()
				if e != nil {
					return nil, e
				}
				count++
				hash := m.SourceUnits["factory.nanz"]
				location := func(name string) hir.SourceLocation {
					start := strings.Index(source, "fun "+name)
					off := start + strings.Index(source[start:], "return")
					return hir.SourceLocation{Unit: "factory.nanz", ByteOffset: off, Line: strings.Count(source[:off], "\n") + 1, Column: off - strings.LastIndex(source[:off], "\n"), SourceSHA256: hash}
				}
				one, two := location("same_one"), location("same_two")
				attach := func(x *hir.Module, swap bool) {
					a, b := one, two
					if swap {
						a, b = b, a
					}
					x.SourceLocations[x.FuncByName("same_one").Body.Body[0]] = a
					x.SourceLocations[x.FuncByName("same_two").Body.Body[0]] = b
				}
				attach(m, false)
				switch mode {
				case "nil":
					return nil, nil
				case "shared":
					return shared, nil
				case "shared_ast":
					m.Funcs = shared.Funcs
				case "source_changed":
					if count > 1 {
						m.Name = "changed"
					}
				case "third_call_origin":
					if count == 3 {
						attach(prior[0], true)
					}
				case "prior_policy":
					if count > 1 {
						prior[0].GraceRulePolicy = map[string]bool{}
					}
				case "prior_only_origin", "fresh_and_prior_origin":
					if count > 1 {
						for _, x := range prior {
							attach(x, true)
						}
						if mode == "fresh_and_prior_origin" {
							attach(m, true)
						}
					}
				case "caller_bytes_mutated":
					work.Regions["data"].Bytes[0] = 99
				}
				prior = append(prior, m)
				return m, nil
			}
			r, e := RunZ80MemoryHostBaseline(factory, work)
			if mode == "caller_bytes_mutated" {
				if e != nil || r.Raw.Invocations[0].Results[0].I != 7 {
					t.Fatal("caller mutation changed owned workload", e)
				}
				return
			}
			if e == nil || r != nil {
				t.Fatal("unsafe factory accepted", mode)
			}
		})
	}
}

// This checkpoint was refreshed only after exact source/host state/event
// equivalence and factory/instrumented native artifact equality were proven.
// The relocated unit-RHS correction (2dbe4355) changes only native emission;
// reports/hardware-parser-reference retains unchanged source/MIR/allocation,
// old/new ASM/BIN, and all three route comparisons.
// Refreshed 2026-10-20 (-3844 T / -14 bytes, oracle unchanged): clean
// origin/main dde023f4 already emitted this exact binary; the byte spill slot
// width fix leaves it unchanged.
// Refreshed 2026-10-21 for pair-move and word-step codegen (-23,858 T /
// -54 bytes). Source, MIR, allocation, independent state/event oracle, and
// equality of the direct/factory/instrumented routes were checked unchanged.
// Refreshed 2026-10-22 for the general ordered-copy idiom: the retained
// copy helper shrinks by 11 bytes; this workload's T states and independent
// state/event oracle are unchanged.
// Refreshed against clean origin/main 76e8070d4: -782 T / -20 bytes.
// Direct, factory and instrumented routes retain the independent parser oracle.
func assertCurrentMemoryHostParserReference(t *testing.T, target *MemoryHostTargetEvidence) {
	t.Helper()
	if target.TotalTStates != 1260288 || len(target.Binary) != 2269 || target.BinarySHA256 != "9209f1d325bd81d808a5dcd28c20ef5de73189a4180c88397de49029be3b052b" {
		t.Fatal("current parser target drift", target.TotalTStates, len(target.Binary), target.BinarySHA256)
	}
	if fmt.Sprintf("%x", sha256.Sum256(target.Binary)) != target.BinarySHA256 {
		t.Fatal("parser binary payload hash mismatch")
	}
}
