package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lizp"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"testing"
)

// Diagnostic-only separate synthetic source, never part of original corpus claim.
func TestLizpErasedCallbackSignatureDiagnostic(t *testing.T) {
	if os.Getenv("LIZP_CALLBACK_DIAGNOSTIC") != "1" {
		t.Skip("opt-in unsupported erased callback signature audit")
	}
	source := `(defun apply ((f ptr) (x u8)) -> u8 (return (funcall f x)))
 (defun word ((x u16)) -> u16 (return (+ x 256)))
 (defun probe ((x u8)) -> u8 (return (apply #'word x)))`
	h, e := lizp.Compile(source, "erased-callback.lizp")
	if e != nil {
		t.Fatal(e)
	}
	if e = hir.Verify(h); e != nil {
		t.Log("HIR rejects", e)
		return
	}
	s, e := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
	if e != nil {
		t.Log("pipeline rejects", e)
		return
	}
	rawH, _ := lizp.Compile(source, "erased-callback.lizp")
	raw, final := mir2.NewVM(hir.LowerModule(rawH)), mir2.NewVM(s.MIR2Module)
	for _, vm := range []*mir2.VM{raw, final} {
		v, e := vm.Call("probe", []mir2.Value{{I: 5}})
		t.Logf("VM result%v err%v", v, e)
	}
	b, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if e != nil || len(b.Errors) > 0 {
		t.Fatal(e)
	}
	f := s.MIR2Module.FuncByName("probe")
	loc := s.Allocation.Locs[f.Contract.Params[0].Reg].Name
	driver, e := z80asm.NewAssembler().AssembleString(fmt.Sprintf("ORG 0x7000\nLD %s,5\nCALL %d\nHALT", loc, b.Symbols["probe"]))
	if e != nil {
		t.Fatal(e)
	}
	z := emulator.NewRemogattoZ80()
	z.LoadMemory(0x8000, b.Binary)
	z.LoadMemory(0x7000, driver.Binary)
	z.SetPC(0x7000)
	z.SetSP(0xf000)
	for n := 0; n < 10000 && !z.IsHalted(); n++ {
		z.Step()
	}
	t.Logf("accepted nativeA%d regs%v ASM\n%s", z.GetRegisters().A, z.GetRegisters(), s.Assembly)
	if dir := os.Getenv("LIZP_CALLBACK_EXPORT"); dir != "" {
		os.MkdirAll(dir, 0755)
		os.WriteFile(dir+"/source.lizp", []byte(source), 0644)
		os.WriteFile(dir+"/accepted.a80", []byte(s.Assembly), 0644)
		os.WriteFile(dir+"/accepted.bin", b.Binary, 0644)
		os.WriteFile(dir+"/accepted.mir", []byte(s.MIR2Module.Dump()), 0644)
	}
}
