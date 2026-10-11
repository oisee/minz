package pipeline

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

const contractFlagSource = `fun arithmetic(n:u8,ptr:^u8)->u8 {
 var i:u8=0
 var s:u8=0
 while i<n {s=s+ptr[i]
 i=i+1}
 return s
}`

func TestCompileHIRStepsRespectsContractOption(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		h, err := nanz.Parse(contractFlagSource, "contract-flag.nanz")
		if err != nil {
			t.Fatal(err)
		}
		baseline := hir.LowerModule(h).FuncByName("arithmetic").Contract.Params[0].Class
		steps, err := CompileHIRSteps(h, Options{ContractOpt: enabled, AssertMode: "none"})
		if err != nil {
			t.Fatal(err)
		}
		if !enabled && steps.MIR2Module.FuncByName("arithmetic").Contract.Params[0].Class != baseline {
			t.Fatal("ContractOpt false still optimized parameter class")
		}
		if enabled && steps.MIR2Module.FuncByName("arithmetic").Contract.Params[0].Class == baseline {
			t.Fatal("fixture no longer exercises optimization")
		}
		h, err = nanz.Parse(contractFlagSource, "contract-flag.nanz")
		if err != nil {
			t.Fatal(err)
		}
		asm, err := CompileHIRWithOptions(h, Options{ContractOpt: enabled, AssertMode: "none"})
		if err != nil {
			t.Fatal(err)
		}
		vm := mir2.NewVM(steps.MIR2Module)
		vm.EnsureHeap(65536)
		vm.WriteHeapSlice(0x4000, []byte{1, 2, 3})
		values, vmErr := vm.Call("arithmetic", []mir2.Value{{I: 3}, {I: 0x4000}})
		if vmErr != nil || len(values) != 1 || values[0].I != 6 {
			t.Fatalf("enabled %v: VM got %v, want 6; error %v", enabled, values, vmErr)
		}
		for _, assembly := range []string{steps.Assembly, asm} {
			a := hir.Assert{FuncName: "arithmetic", Args: []int64{3, 0x4000}, Expected: 6}
			f := steps.MIR2Module.FuncByName("arithmetic")
			boot := buildAssertBootstrap(assertLoadAddr, a, f, steps.Allocation)
			r, err := z80asm.NewAssembler().AssembleString(boot + "\n" + assembly)
			if err != nil || len(r.Errors) > 0 {
				t.Fatal(err)
			}
			z := emulator.NewRemogattoZ80()
			z.MaxCycles = 10000
			z.LoadMemory(assertLoadAddr, r.Binary)
			z.SetMemory(0x4000, 1)
			z.SetMemory(0x4001, 2)
			z.SetMemory(0x4002, 3)
			z.SetPC(assertLoadAddr)
			if err = z.Run(); err != nil {
				t.Fatal(err)
			}
			if z.GetRegisters().A != 6 {
				t.Fatalf("enabled %v: native got %d, want 6; assembly %s", enabled, z.GetRegisters().A, assembly)
			}
			if !strings.Contains(assembly, "arithmetic:") {
				t.Fatal("missing function")
			}
		}
	}
	if !DefaultOptions().ContractOpt {
		t.Fatal("production default disabled")
	}
}
