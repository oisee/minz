package nanz_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"path/filepath"
	"testing"
)

func TestImportedSimpleEnumNative(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "flags.nanz"), []byte("enum State { READY = 1, DONE = 4 }\nfun get() -> u8 { return 7 }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, src := range []string{
		"import flags\nfun probe() -> State { var state:State = State.READY\nif state != State.READY {return 9}\nreturn State.DONE }",
		"import flags { State as Status }\nfun probe() -> Status { return Status.DONE }",
		"import flags { * }\nfun probe() -> u8 { return State.DONE }",
		"import flags\nfun probe() -> u8 { return flags.State.DONE }",
	} {
		m, err := nanz.ParseWithOpts(src, "import-enum.nanz", nanz.ParseOpts{BaseDir: dir})
		if err != nil {
			t.Fatal(err)
		}
		m.Asserts = []hir.Assert{{FuncName: "probe", Expected: 4, Source: "imported simple enum metadata"}}
		if err := hir.Verify(m); err != nil {
			t.Fatal(err)
		}
		if err := pipeline.RunAssertsMIR2(m, hir.LowerModule(hir.CloneModule(m))); err != nil {
			t.Fatal(err)
		}
		if _, err := pipeline.CompileHIRSteps(m, pipeline.Options{DisableVariantSearch: true}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := nanz.ParseWithOpts("import flags { get }\nfun probe() -> State {return State.DONE}", "hidden-enum.nanz", nanz.ParseOpts{BaseDir: dir}); err == nil {
		t.Fatal("selective import exposed unselected enum")
	}
}
