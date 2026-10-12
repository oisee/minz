package pipeline

import (
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pascal"
	"os"
	"strings"
	"testing"
)

func TestPascalCPMAdapterNearMisses(t *testing.T) {
	s, e := os.ReadFile("../../../reports/pascal-cpm-main/hello.pas")
	if e != nil {
		t.Fatal(e)
	}
	for _, variant := range []string{"wrong BDOS", "extra call", "wrong input", "missing clobber", "fake label", "wrong CR"} {
		t.Run(variant, func(t *testing.T) {
			h, e := pascal.Compile(string(s), "examples/pascal/hello.pas")
			if e != nil {
				t.Fatal(e)
			}
			steps, e := CompileHIRSteps(h, Options{ContractOpt: true})
			if e != nil {
				t.Fatal(e)
			}
			f := steps.MIR2Module.FuncByName("WriteStr")
			i := f.Entry().Insts[0]
			switch variant {
			case "wrong BDOS":
				i.Asm.Template = strings.Replace(i.Asm.Template, "C, 9", "C, 10", 1)
			case "extra call":
				i.Asm.Template += "\nCALL 5"
			case "wrong input":
				steps.Allocation.Locs[f.Contract.Params[0].Reg] = mir2.PhysLoc{Name: "DE"}
			case "missing clobber":
				i.Asm.Clobbers = 0
			case "fake label":
				f.Name = "fake_WriteStr"
			case "wrong CR":
				steps.MIR2Module.FuncByName("WriteCrLf").Entry().Insts[0].Asm.Template = strings.Replace(steps.MIR2Module.FuncByName("WriteCrLf").Entry().Insts[0].Asm.Template, "E, 13", "E, 12", 1)
			}
			if _, e := mir2.Z80Codegen(steps.MIR2Module, steps.Allocation, mir2.Z80CodegenOptions{Runtime: mir2.Z80RuntimeCPM, RuntimeStackBytes: 1024}); e == nil {
				t.Fatal("near miss admitted")
			}
		})
	}
}
