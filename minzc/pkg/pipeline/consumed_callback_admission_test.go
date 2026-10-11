package pipeline

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lizp"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"strings"
	"testing"
)

func TestConsumedCallbackPipelineRefusesErasedWordABI(t *testing.T) {
	source := `(defun apply ((f ptr) (x u8)) -> u8 (return (funcall f x)))
 (defun word ((x u16)) -> u16 (return (+ x 256)))
 (defun probe ((x u8)) -> u8 (return (apply #'word x)))`
	h, e := lizp.Compile(source, "erased-callback.lizp")
	if e != nil {
		t.Fatal(e)
	}
	s, e := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
	if e == nil || !strings.Contains(e.Error(), "callback") {
		t.Fatalf("expected callback refusal got%v asm%s", e, s.Assembly)
	}
	if s.Assembly != "" {
		t.Fatal("assembly emitted after refusal")
	}
	t.Log(e)
	if dir := os.Getenv("CONSUMED_CALLBACK_EXPORT"); dir != "" {
		os.MkdirAll(dir, 0755)
		os.WriteFile(dir+"/refused-source.lizp", []byte(source), 0644)
		os.WriteFile(dir+"/refusal.txt", []byte(e.Error()), 0644)
	}
}

func TestConsumedCallbackTypedWordNative(t *testing.T) {
	u16 := mir2.TyU16
	ptr := mir2.TyPtr
	target := &hir.Func{Name: "word", Params: []hir.Param{{Name: "x", Ty: u16}}, RetTy: u16, Body: hir.Blk(hir.Ret(hir.Add(hir.Var("x", u16), hir.U16(256), u16)))}
	apply := &hir.Func{Name: "apply_word", Params: []hir.Param{{Name: "f", Ty: ptr}, {Name: "x", Ty: u16}}, RetTy: u16, Body: hir.Blk(hir.Ret(&hir.CallIndirectExpr{FnPtr: hir.Var("f", ptr), Args: []hir.Expr{hir.Var("x", u16)}, Ty: u16}))}
	caller := &hir.Func{Name: "caller", RetTy: u16, Body: hir.Blk(hir.Ret(&hir.CallExpr{Fn: "apply_word", Args: []hir.Expr{&hir.AddrOfExpr{Sym: "word"}, hir.U16(5)}, Ty: u16}))}
	m := &hir.Module{Funcs: []*hir.Func{caller, apply, target}}
	s, e := CompileHIRSteps(m, Options{ContractOpt: true, AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	b, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if e != nil {
		t.Fatal(e)
	}
	if len(b.Errors) > 0 {
		t.Fatal(b.Errors)
	}
	vm := mir2.NewVM(s.MIR2Module)
	out, e := vm.Call("caller", nil)
	if e != nil || len(out) != 1 || out[0].I != 261 {
		t.Fatal(out, e)
	}
	z := emulator.NewRemogattoZ80()
	z.LoadMemory(0x8000, b.Binary)
	z.LoadMemory(0xeffe, []byte{0, 0xf1})
	z.SetPC(uint16(b.Symbols["caller"]))
	z.SetSP(0xeffe)
	for i := 0; i < 1000 && z.GetPC() != 0xf100; i++ {
		z.Step()
	}
	if z.GetRegisters().HL != 261 || z.GetPC() != 0xf100 || z.GetSP() != 0xf000 {
		t.Fatal(z.GetRegisters(), s.Assembly)
	}
}
