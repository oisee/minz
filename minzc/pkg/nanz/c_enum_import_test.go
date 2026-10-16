package nanz_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCEnumImportNativeVM(t *testing.T) {
	dir := t.TempDir()
	source := `enum Wide{Big=65536UL};enum Wide value=Big;unsigned long constant(void){return Big;}`
	if err := os.WriteFile(filepath.Join(dir, "enums.c"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	parse := func() *hir.Module {
		h, err := nanz.ParseWithOpts("import enums {value,constant}\nfun run()->u32{return u32(value)+constant()}", "caller.nanz", nanz.ParseOpts{BaseDir: dir})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	for _, mode := range []string{"default", "grace", "size"} {
		t.Run(mode, func(t *testing.T) {
			h := parse()
			h.Asserts = append(h.Asserts, hir.Assert{FuncName: "run", Expected: 131072, Via: "z80"})
			steps, err := pipeline.CompileHIRSteps(h, pipeline.Options{AssertMode: "z80", ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
				got, err := mir2.NewVM(m).Call("run", nil)
				if err != nil || len(got) != 1 || got[0].I != 131072 {
					t.Fatalf("VM got=%v err=%v", got, err)
				}
			}
		})
	}
}
func TestCBitfieldImportDiagnostic(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bits.c"), []byte("struct B{unsigned int value:3;};struct B bits;"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := nanz.ParseWithOpts("import bits", "caller.nanz", nanz.ParseOpts{BaseDir: dir}); err == nil || !strings.Contains(err.Error(), "bit-field") {
		t.Fatalf("expected bit-field refusal: %v", err)
	}
}
