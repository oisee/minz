package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIImportedMetafunctionSizeGoal(t *testing.T) {
	hirProfileTestState(t)
	assertMode = "all"
	dir := t.TempDir()
	definitions := `fun @choose() -> void {
 if meta_opt_size() {emit(c"fun chosen()->u8 {return 9}")}
 else {emit(c"fun chosen()->u8 {return 7}")}
}`
	if err := os.WriteFile(filepath.Join(dir, "hints.nanz"), []byte(definitions), 0600); err != nil {
		t.Fatal(err)
	}
	for index, size := range []bool{false, true, false, true} {
		optSize = size
		want := 7
		if size {
			want = 9
		}
		source := filepath.Join(dir, fmt.Sprintf("goal-%d.nanz", index))
		text := fmt.Sprintf("import hints\n@choose()\nassert chosen() == %d via mir2\nassert chosen() == %d via z80\n", want, want)
		if err := os.WriteFile(source, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		outputFile = filepath.Join(dir, fmt.Sprintf("goal-%d.a80", index))
		if err := compileViaHIR(source); err != nil {
			t.Fatalf("size=%v: %v", size, err)
		}
		assembly, err := os.ReadFile(outputFile)
		if err != nil || len(assembly) == 0 {
			t.Fatal("no selected assembly", err)
		}
		if strings.Contains(string(assembly), "meta_opt_size") {
			t.Fatal("compile-time goal leaked into Z80 output")
		}
	}
}
