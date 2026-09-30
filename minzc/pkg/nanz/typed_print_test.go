package nanz_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80validate"
)

func TestTypedPrintLibrary(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	source, err := os.ReadFile(filepath.Join(root, "examples", "nanz", "typed_print.nanz"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := nanz.ParseWithOpts(string(source), "typed_print.nanz", nanz.ParseOpts{
		BaseDir:   filepath.Join(root, "examples", "nanz"),
		StdlibDir: filepath.Join(root, "stdlib"),
	})
	if err != nil {
		t.Fatal(err)
	}
	vm := mir2.NewVM(hir.LowerModule(m))
	result, err := vm.Call("main", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0].I != 18 {
		t.Fatalf("written bytes = %v, want 18", result)
	}
	addr := vm.GlobalAddr("output")
	if addr < 0 {
		t.Fatal("missing output global")
	}
	if got := string(vm.ReadHeap(addr, 19)); got != "score=65535 u8=175\x00" {
		t.Fatalf("flat output = %q", got)
	}
	result, err = vm.Call("probe_zero", nil)
	if err != nil || len(result) != 1 || result[0].I != 1 {
		t.Fatalf("zero formatting: result=%v err=%v", result, err)
	}
	if got := string(vm.ReadHeap(addr, 2)); got != "0\x00" {
		t.Fatalf("zero bytes = %q", got)
	}
	result, err = vm.Call("probe_overflow", nil)
	if err != nil || len(result) != 1 || result[0].I != 1 {
		t.Fatalf("overflow flag: result=%v err=%v", result, err)
	}
	if got := string(vm.ReadHeap(addr, 3)); got != "100" {
		t.Fatalf("bounded bytes = %q", got)
	}
	asm, err := pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("Z80 compile: %v", err)
	}
	if strings.Contains(asm, "CALL write_to") || strings.Contains(asm, "JP (HL)") {
		t.Fatal("typed print introduced runtime interface dispatch")
	}
	for _, name := range []string{"CText_write_to", "Dec8_write_to", "Dec16_write_to"} {
		if !strings.Contains(asm, "CALL text__print__"+name) {
			t.Fatalf("missing direct call to %s", name)
		}
	}
	if errs := z80validate.Validate(asm); len(errs) != 0 {
		t.Fatalf("invalid Z80: %v", errs)
	}
}
