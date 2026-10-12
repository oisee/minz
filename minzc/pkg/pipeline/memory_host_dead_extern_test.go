package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/remogatto/z80"
	"sort"
	"strings"
	"testing"
)

func TestMemoryHostUnboundDeadExternNativeFetchDenied(t *testing.T) {
	const source = `@extern fun peek(addr:u16)->u8
@extern fun unknown(v:u8)->u16
fun entry(addr:u16)->u8 {return peek(addr)}
fun never()->u16 {return unknown(99)}
`
	factory := func() *hir.Module {
		h, e := nanz.Parse(source, "native-dead-extern.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8}, Regions: map[string]hir.MemoryHostRegion{"ram": {Address: 0x9000, Bytes: []byte{173}}}, Calls: []hir.MemoryHostInvocation{{Function: "entry", Arguments: []int64{0x9000}}}}
	h := factory()
	raw, e := hir.RunMemoryHostBaseline(h, w)
	if e != nil {
		t.Fatal(e)
	}
	steps, e := CompileHIRSteps(factory(), Options{AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	target, e := runZ80MemoryHostWorkload(steps, h, w, *raw)
	if e != nil || target.Invocations[0].Results[0].I != 173 {
		t.Fatal(target, e)
	}
	a, e := z80asm.NewAssembler().AssembleString("ORG 0x400\n" + target.Assembly)
	if e != nil || len(a.Errors) != 0 {
		t.Fatal(e, a.Errors)
	}
	reach, e := hir.MemoryHostReachableFunctions(steps.MIR2Module, w)
	if e != nil {
		t.Fatal(e)
	}
	var ranges []memoryHostFunctionRange
	for _, f := range steps.MIR2Module.Funcs {
		if p, ok := workloadSymbol(a.Symbols, f.Name); ok {
			ranges = append(ranges, memoryHostFunctionRange{f.Name, p})
		}
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })
	// Force a fetch of actual assembled unknown stub and never body: neither
	// label existence nor a RET-shaped body confers executable authority.
	for _, name := range []string{"unknown", "never"} {
		p, ok := workloadSymbol(a.Symbols, name)
		if !ok {
			t.Fatalf("expected retained actual %s label", name)
		}
		if memoryHostReachableInstruction(ranges, reach, p) {
			t.Fatalf("dead %s is executable", name)
		}
		m := &memoryHostZ80Memory{lengths: map[uint16]int{}, active: true}
		copy(m.data[0x400:], a.Binary)
		m.cpu = z80.NewZ80(m, memoryHostNoPorts{})
		m.cpu.SetPC(uint16(p))
		m.cpu.SetSP(0xeff0)
		m.cpu.A = 77
		if _, e := memoryHostNativeStep(m); e == nil || !strings.Contains(e.Error(), "execution outside instructions") || m.cpu.A != 77 || m.cpu.SP() != 0xeff0 {
			t.Fatalf("dead fetch effects %s %v", name, e)
		}
	}
}
