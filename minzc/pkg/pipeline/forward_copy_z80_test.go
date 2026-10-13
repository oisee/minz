package pipeline

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func measureForwardCopyZ80Memory(t *testing.T, h *hir.Module, name string, options Options, args []int64, memory map[uint16]byte, want int64, expected map[uint16]byte) (int, int) {
	t.Helper()
	s, err := CompileHIRSteps(h, options)
	if err != nil {
		t.Fatal(err)
	}
	f := s.MIR2Module.FuncByName(name)
	a := hir.Assert{FuncName: name, Args: args, Expected: want, Via: "z80"}
	boot := buildAssertBootstrap(assertLoadAddr, a, f, s.Allocation)
	r, err := z80asm.NewAssembler().AssembleString(boot + "\n" + s.Assembly)
	if err != nil || len(r.Errors) > 0 {
		t.Fatalf("assemble: %v %v", err, r.Errors)
	}
	pc, ok := r.Symbols[name]
	if !ok {
		pc, ok = r.Symbols[strings.ToUpper(name)]
	}
	if !ok {
		t.Fatalf("function symbol missing: %v", r.Symbols)
	}
	platform := emulator.NewRemogattoZ80WithScreen()
	platform.SetROMEnd(0) // CPU conformance fixture uses an all-RAM address space.
	z := platform.RemogattoZ80
	// Pure CPU conformance: the screen wrapper's SMC callback treats writes over
	// RST-shaped data as hooks and can overwrite input at 0xffff. These fixtures
	// execute ordinary RAM code/data with no screen/RST interception.
	z.SetSMCTracker(nil)
	if err := z.LoadMemory(assertLoadAddr, r.Binary); err != nil {
		t.Fatal(err)
	}
	for addr, v := range memory {
		z.SetMemory(addr, v)
	}
	z.SetPC(assertLoadAddr)
	for i := 0; z.GetPC() != uint16(pc); i++ {
		if i > 1000 {
			t.Fatal("bootstrap did not reach function")
		}
		z.Step()
	}
	entrySP := z.GetSP()
	returnPC := uint16(z.GetMemory(entrySP)) | uint16(z.GetMemory(entrySP+1))<<8
	start := z.GetCycles()
	for i := 0; ; i++ {
		if i > 100000 {
			t.Fatalf("function did not return: PC=%04x SP=%04x wantPC=%04x SP=%04x\n%s\n%s", z.GetPC(), z.GetSP(), returnPC, entrySP+2, s.MIR2Opt, s.Assembly)
		}
		z.Step()
		if z.GetPC() == returnPC && z.GetSP() == entrySP+2 {
			break
		}
	}
	for addr, value := range expected {
		if z.GetMemory(addr) != value {
			t.Fatalf("copy target memory %04x got%02x want%02x", addr, z.GetMemory(addr), value)
		}
	}
	cycles := z.GetCycles() - start
	if err := checkAssertZ80Result(z, a, f, map[string]*hir.Func{name: h.FuncByName(name)}); err != nil {
		t.Fatalf("%v\n%s", err, s.Assembly)
	}
	size, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if err != nil || len(size.Errors) > 0 {
		t.Fatalf("size assembly: %v %v", err, size.Errors)
	}
	return cycles, len(size.Binary)
}
