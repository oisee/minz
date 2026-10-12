package pipeline

import (
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pascal"
	"os"
	"strings"
	"testing"
)

func TestPascalDecimalAdapterRejectsMutations(t *testing.T) {
	source, e := os.ReadFile("../../../reports/pascal-decimal-runtime/factorial.pas")
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"wrong sign", "lost IX", "bad divisor", "wrong helper body", "helper public CALL", "bad ABI", "wrongclobber"} {
		t.Run(name, func(t *testing.T) {
			h, e := pascal.Compile(string(source), "examples/pascal/factorial.pas")
			if e != nil {
				t.Fatal(e)
			}
			s, e := CompileHIRSteps(h, Options{ContractOpt: true})
			if e != nil {
				t.Fatal(e)
			}
			f := s.MIR2Module.FuncByName("WriteI16")
			i := f.Entry().Insts[0]
			switch name {
			case "wrong sign":
				i.Asm.Template = strings.Replace(i.Asm.Template, "E, 45", "E, 43", 1)
			case "lost IX":
				i.Asm.Template = strings.Replace(i.Asm.Template, "PUSH IX", "NOP", 1)
			case "bad divisor":
				i.Asm.Template = strings.Replace(i.Asm.Template, "BC, 10000", "BC, 1000", 1)
			case "wrong helper body":
				s.MIR2Module.FuncByName("_wr16_dig").Entry().Insts[0].Asm.Template += "\nCALL 5"
			case "helper public CALL":
				for _, i := range s.MIR2Module.FuncByName("main").Entry().Insts {
					if i.Op == mir2.OpCall {
						i.Sym = "_wr16_dig"
						i.Args = nil
						break
					}
				}
			case "bad ABI":
				s.Allocation.Locs[f.Contract.Params[0].Reg] = mir2.PhysLoc{Name: "DE"}
			case "wrongclobber":
				i.Asm.Clobbers = 0
			}
			if _, e := mir2.Z80Codegen(s.MIR2Module, s.Allocation, mir2.Z80CodegenOptions{Runtime: mir2.Z80RuntimeCPM, RuntimeStackBytes: 1024}); e == nil {
				t.Fatal("mutateddecimal admitted")
			}
		})
	}
}
