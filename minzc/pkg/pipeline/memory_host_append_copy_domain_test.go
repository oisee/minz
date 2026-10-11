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

type appendDomainCase struct {
	Name                    string
	Length                  int
	Bump, SourceBase, Start uint16
	Regions                 map[string]hir.MemoryHostRegion
}
type appendDomainExpected struct {
	Results   []mir2.Value
	Events    []hir.MemoryHostAccess
	Failure   *hir.MemoryHostAccessError
	Regions   map[string][]byte
	FinalBump uint16
}

// Sequential arithmetic/memory model: header first, then read before write.
// Aliases observe preceding writes, including the header byte.
func appendDomainOracle(c appendDomainCase) appendDomainExpected {
	o := appendDomainExpected{Results: []mir2.Value{}, Events: []hir.MemoryHostAccess{}, Regions: map[string][]byte{}, FinalBump: c.Bump}
	for name, r := range c.Regions {
		o.Regions[name] = append([]byte{}, r.Bytes...)
	}
	access := func(addr uint16, write bool, value byte) (byte, bool) {
		for name, r := range c.Regions {
			i := int(addr) - int(r.Address)
			if i < 0 || i >= len(r.Bytes) {
				continue
			}
			if write && !r.Writable {
				o.Failure = &hir.MemoryHostAccessError{Kind: "read-only", Address: addr, Write: true}
				return 0, false
			}
			if write {
				o.Regions[name][i] = value
				o.Events = append(o.Events, hir.MemoryHostAccess{Kind: "write", Address: addr, Value: value})
				return value, true
			}
			v := o.Regions[name][i]
			o.Events = append(o.Events, hir.MemoryHostAccess{Kind: "read", Address: addr, Value: v})
			return v, true
		}
		o.Failure = &hir.MemoryHostAccessError{Kind: "undeclared", Address: addr, Write: write}
		return 0, false
	}
	if _, ok := access(c.Bump, true, byte(c.Length)); !ok {
		return o
	}
	for j := 0; j < c.Length; j++ {
		v, ok := access(uint16(uint32(c.SourceBase)+uint32(c.Start)+uint32(j)), false, 0)
		if !ok {
			return o
		}
		if _, ok = access(uint16(uint32(c.Bump)+1+uint32(j)), true, v); !ok {
			return o
		}
	}
	o.FinalBump = uint16(uint32(c.Bump) + 1 + uint32(c.Length))
	o.Results = []mir2.Value{{I: int64(c.Bump)}}
	return o
}
func appendDomainWorkload(cases []appendDomainCase) hir.MemoryHostWorkload {
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8, "poke": hir.MemoryHostWriteU8}, Regions: cases[0].Regions}
	word := func(v uint16) []byte { return []byte{byte(v), byte(v >> 8)} }
	for _, c := range cases {
		call := hir.MemoryHostInvocation{Function: "intern_region", Arguments: []int64{int64(c.Start), int64(c.Length)}, Globals: map[string][]byte{"str_arena": word(c.Bump), "str_bump": word(c.Bump), "src_buf": word(c.SourceBase)}}
		for name, r := range c.Regions {
			call.Patches = append(call.Patches, hir.MemoryHostPatch{Region: name, Bytes: r.Bytes, Clear: true})
		}
		w.Calls = append(w.Calls, call)
	}
	return w
}
func checkAppendDomainObservation(t *testing.T, source *hir.Module, w hir.MemoryHostWorkload, c appendDomainCase, index int, got hir.MemoryHostObservation) {
	t.Helper()
	o := appendDomainOracle(c)
	if !reflect.DeepEqual(got.Results, o.Results) || !reflect.DeepEqual(got.Events, o.Events) || !reflect.DeepEqual(got.Failure, o.Failure) || !reflect.DeepEqual(got.Regions, o.Regions) {
		t.Fatalf("%s result/ordered effects/failure/fullregions differ: failure%+v want%+v", c.Name, got.Failure, o.Failure)
	}
	globals := map[string][]byte{}
	for _, g := range source.Globals {
		b := make([]byte, max(mir2.ByteWidth(g.Ty), len(g.Init)))
		copy(b, g.Init)
		globals[g.Name] = b
	}
	for n, b := range w.Globals {
		globals[n] = append([]byte{}, b...)
	}
	for n, b := range w.Calls[index].Globals {
		globals[n] = append([]byte{}, b...)
	}
	globals["str_bump"] = []byte{byte(o.FinalBump), byte(o.FinalBump >> 8)}
	if !reflect.DeepEqual(got.Globals, globals) {
		t.Fatalf("%s independent source globals differ", c.Name)
	}
}
func TestMemoryHostAppendCopyRawDomain(t *testing.T) {
	source, w := memoryHostParserFixture(t)
	actual, _ := memoryHostParserFixture(t)
	if e := hoistMemoryHostAppendBases(actual, w); e != nil {
		t.Fatal(e)
	}
	originalMIR, actualMIR := hir.LowerModule(source), hir.LowerModule(actual)
	count, events, denials := 0, 0, 0
	check := func(cases []appendDomainCase) {
		w := appendDomainWorkload(cases)
		for _, v := range []struct {
			Source *hir.Module
			MIR    *mir2.Module
		}{{source, originalMIR}, {actual, actualMIR}} {
			run, e := hir.RunMemoryHostMIRBaseline(v.Source, v.MIR, w)
			last := appendDomainOracle(cases[len(cases)-1])
			if run == nil || (e != nil) != (last.Failure != nil) || len(run.Invocations) != len(cases) {
				t.Fatal("raw domain refused", e)
			}
			for j, c := range cases {
				checkAppendDomainObservation(t, source, w, c, j, run.Invocations[j])
			}
		}
		for _, c := range cases {
			o := appendDomainOracle(c)
			count++
			events += len(o.Events)
			if o.Failure != nil {
				denials++
			}
		}
	}
	fill := func(n int) []byte {
		b := make([]byte, n)
		for j := range b {
			b[j] = byte(j*37 + 19)
		}
		return b
	}
	for _, offset := range []int{-1, 0, 1, 2} {
		cs := []appendDomainCase{}
		for n := 0; n <= 255; n++ {
			cs = append(cs, appendDomainCase{Name: fmt.Sprintf("alias%d_len%d", offset, n), Length: n, Bump: 0x8000, SourceBase: uint16(0x8000 + offset), Regions: map[string]hir.MemoryHostRegion{"shared": {Address: 0x7fff, Bytes: fill(260), Writable: true}}})
		}
		check(cs)
	}
	cs := []appendDomainCase{}
	for n := 0; n <= 255; n++ {
		cs = append(cs, appendDomainCase{Name: fmt.Sprintf("source_modular_len%d", n), Length: n, Bump: 0x8000, SourceBase: 0xff00, Start: 0x9100, Regions: map[string]hir.MemoryHostRegion{"dest": {Address: 0x8000, Bytes: fill(257), Writable: true}, "src": {Address: 0x9000, Bytes: fill(256)}}})
	}
	check(cs)
	failures := []appendDomainCase{
		{Name: "header_readonly", Length: 2, Bump: 0x8000, SourceBase: 0x9000, Regions: map[string]hir.MemoryHostRegion{"dest": {Address: 0x8000, Bytes: fill(4)}, "src": {Address: 0x9000, Bytes: fill(4)}}},
		{Name: "first_source_denied", Length: 2, Bump: 0x8000, SourceBase: 0x9000, Regions: map[string]hir.MemoryHostRegion{"dest": {Address: 0x8000, Bytes: fill(4), Writable: true}}},
		{Name: "tail_source_denied", Length: 3, Bump: 0x8000, SourceBase: 0xfffe, Regions: map[string]hir.MemoryHostRegion{"dest": {Address: 0x8000, Bytes: fill(4), Writable: true}, "src": {Address: 0xfffe, Bytes: fill(2)}}},
		{Name: "first_destination_readonly", Length: 2, Bump: 0x8000, SourceBase: 0x9000, Regions: map[string]hir.MemoryHostRegion{"header": {Address: 0x8000, Bytes: fill(1), Writable: true}, "dest": {Address: 0x8001, Bytes: fill(3)}, "src": {Address: 0x9000, Bytes: fill(4)}}},
		{Name: "tail_destination_wrap_denied", Length: 3, Bump: 0xfffe, SourceBase: 0x9000, Regions: map[string]hir.MemoryHostRegion{"dest": {Address: 0xfffe, Bytes: fill(2), Writable: true}, "src": {Address: 0x9000, Bytes: fill(4)}}},
		{Name: "destination_base_wrap_denied", Length: 1, Bump: 0xffff, SourceBase: 0x9000, Regions: map[string]hir.MemoryHostRegion{"dest": {Address: 0xffff, Bytes: fill(1), Writable: true}, "src": {Address: 0x9000, Bytes: fill(2)}}},
	}
	for _, c := range failures {
		check([]appendDomainCase{c})
	}
	t.Logf("raw append domain cases%d verifiedInvocations%d exactEvents%d typedDenials%d", count, count*2, events*2, denials)
	if d := os.Getenv("MINZ_APPEND_COPY_ARTIFACTS"); d != "" {
		b, _ := json.MarshalIndent(map[string]int{"cases": count, "invocations": count * 2, "events": events * 2, "denials": denials}, "", "  ")
		if e := os.WriteFile(filepath.Join(d, "raw-domain-summary.json"), b, 0644); e != nil {
			t.Fatal(e)
		}
	}
}
