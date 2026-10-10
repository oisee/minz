package pipeline

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func measureProfileZ80(t *testing.T, h *hir.Module, name string, options Options, args []int64, memory map[uint16]byte, want int64, directory string) (int, int) {
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
	cycles := z.GetCycles() - start
	for addr, v := range memory {
		if z.GetMemory(addr) != v {
			t.Fatalf("Z80 input memory changed at %04x", addr)
		}
	}
	if err := checkAssertZ80Result(z, a, f, map[string]*hir.Func{name: h.FuncByName(name)}); err != nil {
		t.Fatalf("%v\n%s", err, s.Assembly)
	}
	size, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if err != nil || len(size.Errors) > 0 {
		t.Fatalf("size assembly: %v %v", err, size.Errors)
	}
	if directory != "" {
		if err := os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string][]byte{"input.hir": []byte(s.HIR), "raw.mir": []byte(s.MIR2Raw), "optimized.mir": []byte(s.MIR2Opt), "program.a80": []byte(s.Assembly), "execution.a80": []byte(boot + "\n" + s.Assembly), "program.bin": size.Binary, "execution.bin": r.Binary} {
			if err := os.WriteFile(filepath.Join(directory, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return cycles, len(size.Binary)
}
