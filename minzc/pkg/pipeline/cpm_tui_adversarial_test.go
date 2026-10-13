package pipeline

import (
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"strings"
	"testing"
)

func cpmTetrisSteps(t *testing.T) Steps {
	t.Helper()
	src, e := os.ReadFile("../../../examples/nanz/tetris_cpm.nanz")
	if e != nil {
		t.Fatal(e)
	}
	h, e := nanz.ParseWithOpts(string(src), "tetris_cpm.nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
	if e != nil {
		t.Fatal(e)
	}
	s, e := CompileHIRSteps(h, Options{AssertMode: "none", ContractOpt: true})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestCPMTUICanonicalHostileInputs(t *testing.T) {
	for _, kind := range []string{"body", "injected-call", "clobber", "input-order", "return-width", "unused-unknown", "readline-call", "readline-address", "hidden-asm", "external", "physical-home", "physical-kind"} {
		t.Run(kind, func(t *testing.T) {
			s := cpmTetrisSteps(t)
			m := s.MIR2Module
			f := m.FuncByName("tui__render__tui_goto")
			i := f.Entry().Insts[0]
			switch kind {
			case "body":
				i.Asm.Template = strings.Replace(i.Asm.Template, "LD C, 2", "LD C, 9", 1)
			case "injected-call":
				i.Asm.Template += "\nCALL malicious"
			case "clobber":
				i.Asm.Clobbers = mir2.ClassSet(0)
			case "input-order":
				i.Asm.Ins[0], i.Asm.Ins[1] = i.Asm.Ins[1], i.Asm.Ins[0]
			case "return-width":
				m.FuncByName("tui__render__tui_width").Contract.Returns[0].Ty = mir2.TyU16
			case "unused-unknown":
				m.FuncByName("tui__render__tui_read_line").Entry().Insts[0].Asm.Template += "\nCALL malicious"
			case "readline-call":
				m.FuncByName("main").Entry().Insts = append(m.FuncByName("main").Entry().Insts, &mir2.Inst{Op: mir2.OpCall, Sym: "tui__render__tui_read_line"})
			case "hidden-asm":
				i.Asm.Template += "\nCALL malicious"
				f.Attrs.HasAsm = false
			case "physical-home":
				p := f.Contract.Params[0]
				loc := s.Allocation.Locs[p.Reg]
				loc.Name = "B"
				s.Allocation.Locs[p.Reg] = loc
			case "physical-kind":
				p := f.Contract.Params[0]
				loc := s.Allocation.Locs[p.Reg]
				loc.Kind = mir2.LocMem
				s.Allocation.Locs[p.Reg] = loc
			case "external":
				m.FuncByName("tui__render__tui_read_line").Attrs.IsExtern = true
			case "readline-address":
				m.FuncByName("main").Entry().Insts = append(m.FuncByName("main").Entry().Insts, &mir2.Inst{Op: mir2.OpAddrOf, Dst: 60000, Ty: mir2.TyPtr, Sym: "tui__render__tui_read_line"})
			}
			text, e := mir2.Z80Codegen(m, s.Allocation, mir2.Z80CodegenOptions{Runtime: mir2.Z80RuntimeCPM, RuntimeStackBytes: 1024})
			if e == nil || text != "" {
				t.Fatal("hostile canonical/runtime closure admitted", kind, e)
			}
		})
	}
}
