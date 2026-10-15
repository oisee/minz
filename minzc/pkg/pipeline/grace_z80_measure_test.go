package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/z80asm"
)

// Measures function entry through RET; excludes bootstrap argument setup,
// CALL, DI and HALT. Size includes function code and generated spill storage.
func measureGraceZ80(t *testing.T, src, name string, enabled bool, args []int64, memory map[uint16]byte, want int64) (int, int) {
	t.Helper()
	return measureGraceZ80HIR(t, graceMeasureParse(t, src), name, Options{UseHIRGrace: enabled}, args, memory, want)
}

func measureGraceZ80HIR(t *testing.T, h *hir.Module, name string, options Options, args []int64, memory map[uint16]byte, want int64) (int, int) {
	t.Helper()
	s, err := CompileHIRSteps(h, options)
	if err != nil {
		t.Fatal(err)
	}
	return measureGraceZ80Compiled(t, h, s, name, args, memory, want, 100000)
}

// Reuse one compiled candidate across a workload matrix. The instruction
// limit remains explicit so longer traces cannot silently hang the suite.
func measureGraceZ80Compiled(t *testing.T, h *hir.Module, s Steps, name string, args []int64, memory map[uint16]byte, want int64, instructionLimit int) (int, int) {
	t.Helper()
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
		if i > instructionLimit {
			t.Fatalf("function did not return: PC=%04x SP=%04x wantPC=%04x SP=%04x\n%s\n%s", z.GetPC(), z.GetSP(), returnPC, entrySP+2, s.MIR2Opt, s.Assembly)
		}
		z.Step()
		if z.GetPC() == returnPC && z.GetSP() == entrySP+2 {
			break
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

func TestGraceZ80MeasuredExamples(t *testing.T) {
	for _, name := range []string{"sum_array", "region_match"} {
		for _, n := range []int{0, 1, 5, 16, 64, 200, 255} {
			t.Run(fmt.Sprintf("%s/n%d", name, n), func(t *testing.T) {
				mem := map[uint16]byte{}
				src := graceSumSource
				args := []int64{0x4000, int64(n)}
				want := int64(0)
				for i := 0; i < n; i++ {
					v := byte(i*37 + 11)
					mem[uint16(0x4000+i)] = v
					want = (want + int64(v)) & 255
				}
				if name == "region_match" {
					src = graceRegionSource
					args = []int64{0x4000, 32, 0x5000, int64(n)}
					want = 1
					for i := 0; i < n; i++ {
						v := byte(i*37 + 11)
						mem[uint16(0x4020+i)] = v
						mem[uint16(0x5000+i)] = v
					}
				}
				before, bsize := measureGraceZ80(t, src, name, false, args, mem, want)
				after, asize := measureGraceZ80(t, src, name, true, args, mem, want)
				t.Logf("HIR Grace: T-states %d -> %d (%+.1f%%); bytes %d -> %d", before, after, 100*float64(after-before)/float64(before), bsize, asize)
			})
		}
	}
}
