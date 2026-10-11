package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestCPMRecursiveLiveSpillRestoration(t *testing.T) {
	for _, n := range []int{0, 1, 2, 7, 12, 31} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			src := fmt.Sprintf("global depth:u16=%d\nglobal result:u16=0\nfun rec(n:u16)->u16{if n==0{return 0}\nreturn n+rec(n-1)*3}\nfun main()->void{result=rec(depth)}", n)
			h, e := nanz.Parse(src, "recursive.nanz")
			if e != nil {
				t.Fatal(e)
			}
			steps, e := CompileHIRSteps(h, Options{AssertMode: "none"})
			if e != nil {
				t.Fatal(e)
			}
			f := steps.MIR2Module.FuncByName("rec")
			r := f.Contract.Params[0].Reg
			// Force the parameter, live across the recursive call, into the same
			// named home at every activation. The recursive child overwrites it.
			steps.Allocation.Locs[r] = mir2.PhysLoc{Kind: mir2.LocMem, Name: "$F000", Offset: 0xf000}
			steps.Allocation.Spilled = append(steps.Allocation.Spilled, r)
			code, e := mir2.Z80Codegen(steps.MIR2Module, steps.Allocation, mir2.Z80CodegenOptions{Runtime: mir2.Z80RuntimeCPM, RuntimeStackBytes: 1024})
			if e != nil {
				t.Fatal(e)
			}
			if !strings.Contains(code, "CALL rec") || !strings.Contains(code, mir2.Z80SpillLabel("rec", r)) {
				t.Fatal("fixture lost real recursion or shared home")
			}
			a, e := z80asm.NewAssembler().AssembleString(code)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			z.MaxCycles = 10000000
			z.LoadMemory(0x100, a.Binary)
			z.SetMemory(0xfdfe, 0)
			z.SetMemory(0xfdff, 0)
			z.SetRegisters(emulator.Registers{PC: 0x100, SP: 0xfdfe})
			if e := z.Run(); e != nil {
				t.Fatal(e)
			}
			var want uint16
			for i := 1; i <= n; i++ {
				want = uint16(i) + want*3
			}
			p := uint16(a.Symbols["result"])
			got := uint16(z.GetMemory(p)) | uint16(z.GetMemory(p+1))<<8
			vm, e := mir2.NewVM(steps.MIR2Module).Call("rec", []mir2.Value{{I: int64(n)}})
			if e != nil || len(vm) != 1 || uint16(vm[0].I) != want {
				t.Fatal("VM oracle", vm, e, want)
			}
			if got != want || z.GetRegisters().SP != 0xfe00 || z.GetMemory(uint16(a.Symbols["__cpm_status"])) != 0 || z.GetMemory(uint16(a.Symbols["__cpm_stack_guard"])) != 0xa5 {
				t.Fatal("recursive shared live home", got, want, z.GetRegisters())
			}
		})
	}
}
