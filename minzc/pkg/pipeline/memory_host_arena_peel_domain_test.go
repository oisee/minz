package pipeline

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type arenaDomainCase struct {
	Name                           string
	Length                         int
	Arena, SourceStart, SourceBase uint16
	Regions                        map[string]hir.MemoryHostRegion
}
type arenaDomainSummary struct {
	Cases, Invocations, ByteReads, Denied int
	Families                              map[string]int
}

// This oracle deliberately uses integer modular address arithmetic and an
// ordered byte read model, independent of HIR lowering or the peel helper.
func arenaDomainOracle(c arenaDomainCase) ([]mir2.Value, []hir.MemoryHostAccess, *hir.MemoryHostAccessError) {
	events := []hir.MemoryHostAccess{}
	read := func(address uint16) (byte, *hir.MemoryHostAccessError) {
		for _, r := range c.Regions {
			offset := int(address) - int(r.Address)
			if offset >= 0 && offset < len(r.Bytes) {
				value := r.Bytes[offset]
				events = append(events, hir.MemoryHostAccess{Kind: "read", Address: address, Value: value})
				return value, nil
			}
		}
		return 0, &hir.MemoryHostAccessError{Kind: "undeclared", Address: address}
	}
	for i := 0; i < c.Length; i++ {
		left, denied := read(uint16(uint32(c.Arena) + 1 + uint32(i)))
		if denied != nil {
			return []mir2.Value{}, events, denied
		}
		right, denied := read(uint16(uint32(c.SourceBase) + uint32(c.SourceStart) + uint32(i)))
		if denied != nil {
			return []mir2.Value{}, events, denied
		}
		if left != right {
			return []mir2.Value{{I: 0}}, events, nil
		}
	}
	return []mir2.Value{{I: 1}}, events, nil
}
func arenaDomainWorkload(cases []arenaDomainCase) hir.MemoryHostWorkload {
	first := cases[0]
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8, "poke": hir.MemoryHostWriteU8}, Regions: first.Regions}
	for _, c := range cases {
		call := hir.MemoryHostInvocation{Function: "region_match", Arguments: []int64{int64(c.Arena), int64(c.SourceStart), int64(c.Length)}, Globals: map[string][]byte{"src_buf": {byte(c.SourceBase), byte(c.SourceBase >> 8)}}}
		for name, r := range c.Regions {
			call.Patches = append(call.Patches, hir.MemoryHostPatch{Region: name, Bytes: r.Bytes, Clear: true})
		}
		w.Calls = append(w.Calls, call)
	}
	return w
}
func checkArenaDomainBatch(t *testing.T, source, peeled *hir.Module, originalMIR, peeledMIR *mir2.Module, cases []arenaDomainCase, summary *arenaDomainSummary, family string) {
	t.Helper()
	w := arenaDomainWorkload(cases)
	for _, variant := range []struct {
		Name   string
		Source *hir.Module
		MIR    *mir2.Module
	}{{"original", source, originalMIR}, {"peeled", peeled, peeledMIR}} {
		run, err := hir.RunMemoryHostMIRBaseline(variant.Source, variant.MIR, w)
		_, _, lastDenied := arenaDomainOracle(cases[len(cases)-1])
		if run == nil || (err != nil) != (lastDenied != nil) || len(run.Invocations) != len(cases) {
			t.Fatalf("%s %s batch refused: %v", family, variant.Name, err)
		}
		for j, c := range cases {
			wantResult, wantEvents, wantFailure := arenaDomainOracle(c)
			got := run.Invocations[j]
			if !reflect.DeepEqual(got.Results, wantResult) || !reflect.DeepEqual(got.Events, wantEvents) || !reflect.DeepEqual(got.Failure, wantFailure) {
				t.Fatalf("%s %s %s result/events/failure differ: got%+v wantFailure%+v", family, variant.Name, c.Name, got, wantFailure)
			}
			for name, r := range c.Regions {
				if !reflect.DeepEqual(got.Regions[name], r.Bytes) {
					t.Fatalf("%s %s changed region %s", family, variant.Name, name)
				}
			}
			for name, initial := range run.Invocations[0].Globals {
				want := initial
				if name == "src_buf" {
					want = []byte{byte(c.SourceBase), byte(c.SourceBase >> 8)}
				}
				if !reflect.DeepEqual(got.Globals[name], want) {
					t.Fatalf("%s %s changed global %s", family, variant.Name, name)
				}
			}
			summary.Invocations++
			summary.ByteReads += len(got.Events)
		}
	}
	summary.Cases += len(cases)
	summary.Families[family] += len(cases)
	for _, c := range cases {
		_, _, denied := arenaDomainOracle(c)
		if denied != nil {
			summary.Denied++
		}
	}
}
func TestMemoryHostArenaPeelRawDomain(t *testing.T) {
	source, _ := memoryHostParserFixture(t)
	peeled, w := memoryHostParserFixture(t)
	if e := peelMemoryHostArenaFirstByte(peeled, w); e != nil {
		t.Fatal(e)
	}
	originalMIR, peeledMIR := hir.LowerModule(source), hir.LowerModule(peeled)
	summary := arenaDomainSummary{Families: map[string]int{}}
	constant := func(n int) []byte {
		b := make([]byte, n)
		for j := range b {
			b[j] = 0x5a
		}
		return b
	}
	for n := 0; n <= 255; n++ {
		cases := []arenaDomainCase{}
		for mismatch := -1; mismatch < n; mismatch++ {
			right := constant(256)
			if mismatch >= 0 {
				right[mismatch] = 0xa5
			}
			cases = append(cases, arenaDomainCase{Name: fmt.Sprintf("len%d_mismatch%d", n, mismatch), Length: n, Arena: 0x7fff, SourceBase: 0x9000, Regions: map[string]hir.MemoryHostRegion{"left": {Address: 0x8000, Bytes: constant(256)}, "right": {Address: 0x9000, Bytes: right}}})
		}
		checkArenaDomainBatch(t, source, peeled, originalMIR, peeledMIR, cases, &summary, "all_lengths_and_mismatches")
	}
	for _, offset := range []int{-1, 0, 1} {
		cases := []arenaDomainCase{}
		for n := 0; n <= 255; n++ {
			cases = append(cases, arenaDomainCase{Name: fmt.Sprintf("alias%d_len%d", offset, n), Length: n, Arena: 0x8000, SourceBase: uint16(0x8001 + offset), Regions: map[string]hir.MemoryHostRegion{"shared": {Address: 0x8000, Bytes: constant(258)}}})
		}
		checkArenaDomainBatch(t, source, peeled, originalMIR, peeledMIR, cases, &summary, fmt.Sprintf("alias_offset_%d", offset))
	}
	wrapped := []arenaDomainCase{}
	for n := 0; n <= 255; n++ {
		wrapped = append(wrapped, arenaDomainCase{Name: fmt.Sprintf("source_wrap_len%d", n), Length: n, Arena: 0x7fff, SourceBase: 0xff00, SourceStart: 0x9100, Regions: map[string]hir.MemoryHostRegion{"left": {Address: 0x8000, Bytes: constant(256)}, "right": {Address: 0x9000, Bytes: constant(256)}}})
	}
	checkArenaDomainBatch(t, source, peeled, originalMIR, peeledMIR, wrapped, &summary, "source_modular_wrap")
	failures := []arenaDomainCase{
		{Name: "first_left_wrap_zero", Length: 1, Arena: 65535, SourceBase: 0x8000, Regions: map[string]hir.MemoryHostRegion{"right": {Address: 0x8000, Bytes: constant(4)}}},
		{Name: "first_right_denied", Length: 1, Arena: 0x7fff, SourceBase: 0x9000, Regions: map[string]hir.MemoryHostRegion{"left": {Address: 0x8000, Bytes: constant(4)}}},
		{Name: "tail_left_wrap_zero", Length: 3, Arena: 0xfffd, SourceBase: 0x8000, Regions: map[string]hir.MemoryHostRegion{"left": {Address: 0xfffe, Bytes: constant(2)}, "right": {Address: 0x8000, Bytes: constant(4)}}},
		{Name: "tail_right_wrap_zero", Length: 3, Arena: 0x7fff, SourceBase: 0xfffe, Regions: map[string]hir.MemoryHostRegion{"left": {Address: 0x8000, Bytes: constant(4)}, "right": {Address: 0xfffe, Bytes: constant(2)}}},
	}
	for _, c := range failures {
		checkArenaDomainBatch(t, source, peeled, originalMIR, peeledMIR, []arenaDomainCase{c}, &summary, "typed_denial_prefix")
	}
	t.Logf("raw domain cases%d invocations%d exactbyteReads%d typedDenials%d", summary.Cases, summary.Invocations, summary.ByteReads, summary.Denied)
	if dir := os.Getenv("MINZ_ARENA_PEEL_ARTIFACTS"); dir != "" {
		b, e := json.MarshalIndent(summary, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, "raw-domain-summary.json"), b, 0644); e != nil {
			t.Fatal(e)
		}
	}
}
