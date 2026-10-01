package nanz_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestMetaFormatSpike(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	path := filepath.Join(root, "stdlib", "text", "spikes", "meta_format_spike.nanz")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := nanz.ParseWithOpts(string(src), path, nanz.ParseOpts{
		BaseDir: filepath.Dir(path), StdlibDir: filepath.Join(root, "stdlib"),
	})
	if err != nil {
		t.Fatal(err)
	}
	vm := mir2.NewVM(hir.LowerModule(m))
	got, err := vm.Call("main", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].I != 19 {
		t.Fatalf("formatted length = %v, want 19", got)
	}
	addr := vm.GlobalAddr("output")
	if bytes := string(vm.ReadHeap(addr, 20)); bytes != "score=65535 moves=7\x00" {
		t.Fatalf("formatted bytes = %q", bytes)
	}
	hirText := m.Dump()
	for _, name := range []string{"CText_write_to", "Dec16_write_to"} {
		if !strings.Contains(hirText, "call @text__print__"+name) {
			t.Fatalf("missing concrete formatter call %s", name)
		}
	}
}

func TestInlineMetaPrintSpike(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	path := filepath.Join(root, "stdlib", "text", "spikes", "meta_inline_print_spike.nanz")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := nanz.ParseWithOpts(string(src), path, nanz.ParseOpts{
		BaseDir: filepath.Dir(path), StdlibDir: filepath.Join(root, "stdlib"),
	})
	if err != nil {
		t.Fatal(err)
	}
	vm := mir2.NewVM(hir.LowerModule(m))
	got, err := vm.Call("main", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].I != 19 {
		t.Fatalf("formatted length = %v, want 19", got)
	}
	addr := vm.GlobalAddr("output")
	if bytes := string(vm.ReadHeap(addr, 20)); bytes != "score=65535 moves=7\x00" {
		t.Fatalf("formatted bytes = %q", bytes)
	}
	hirText := m.Dump()
	for _, name := range []string{"CText_write_to", "Dec16_write_to"} {
		if !strings.Contains(hirText, "call @text__print__"+name) {
			t.Fatalf("missing concrete formatter call %s", name)
		}
	}
}

func TestTemplateMetaPrintSpike(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	path := filepath.Join(root, "stdlib", "text", "spikes", "meta_template_print_spike.nanz")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := nanz.ParseWithOpts(string(src), path, nanz.ParseOpts{
		BaseDir: filepath.Dir(path), StdlibDir: filepath.Join(root, "stdlib"),
	})
	if err != nil {
		t.Fatal(err)
	}
	vm := mir2.NewVM(hir.LowerModule(m))
	got, err := vm.Call("main", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "score=65535 game=Game(7)\n"
	if len(got) != 1 || got[0].I != int64(len(want)) {
		t.Fatalf("formatted length = %v, want %d", got, len(want))
	}
	addr := vm.GlobalAddr("output")
	if bytes := string(vm.ReadHeap(addr, len(want)+1)); bytes != want+"\x00" {
		t.Fatalf("formatted bytes = %q", bytes)
	}
	hirText := m.Dump()
	for _, name := range []string{"CText_write_to", "Dec16_write_to"} {
		if !strings.Contains(hirText, "call @text__print__"+name) {
			t.Fatalf("missing concrete formatter call %s", name)
		}
	}
	if !strings.Contains(hirText, "call @Game_write_to") {
		t.Fatal("missing direct object formatter call")
	}
	for _, tc := range []struct {
		name, source, message string
	}{
		{"bad specifier", strings.Replace(string(src), "#{game:show}", "#{game:oct}", 1), "unknown print format specifier"},
		{"unclosed interpolation", strings.Replace(string(src), "#{game:show}", "#{game:show", 1), "unclosed print expression"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := nanz.ParseWithOpts(tc.source, path, nanz.ParseOpts{
				BaseDir: filepath.Dir(path), StdlibDir: filepath.Join(root, "stdlib"),
			})
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("want diagnostic %q, got %v", tc.message, err)
			}
		})
	}
}

func TestImportedStatementMetafunction(t *testing.T) {
	dir := t.TempDir()
	module := `fun @answer() -> void { emit(c"return 42") }`
	if err := os.WriteFile(filepath.Join(dir, "macro.nanz"), []byte(module), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, importLine, call string }{
		{"glob", "import macro", "@answer()"},
		{"selected alias", "import macro { answer as result }", "@result()"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := tc.importLine + "\nfun main() -> u16 { " + tc.call + " }\n"
			m, err := nanz.ParseWithOpts(src, "main.nanz", nanz.ParseOpts{BaseDir: dir})
			if err != nil {
				t.Fatal(err)
			}
			got, err := mir2.NewVM(hir.LowerModule(m)).Call("main", nil)
			if err != nil || len(got) != 1 || got[0].I != 42 {
				t.Fatalf("main() = %v, error %v", got, err)
			}
		})
	}
}

func TestUserMetafunctionGeneratedReturnType(t *testing.T) {
	const src = `
fun @make_value() -> void {
    emit(c"fun generated() -> u8 { return 46 }")
}
@make_value()
fun main() -> u8 { return generated() }
`
	m, err := nanz.Parse(src, "generated_return.nanz")
	if err != nil {
		t.Fatal(err)
	}
	vm := mir2.NewVM(hir.LowerModule(m))
	values, err := vm.Call("main", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].I != 46 {
		t.Fatalf("generated u8 result = %v, want 46", values)
	}
}

func TestUserMetafunctionNumericBlockArguments(t *testing.T) {
	const src = `
fun @make_value() -> void {
    let n: u8 = node_arg_int(0, 0)
    emit(str_concat(c"fun generated() -> u8 { return ", str_concat(str_from_int(n), c" }")))
}
@make_value() { value 46 }
fun main() -> u8 { return generated() }
`
	m, err := nanz.Parse(src, "numeric_block.nanz")
	if err != nil {
		t.Fatal(err)
	}
	vm := mir2.NewVM(hir.LowerModule(m))
	values, err := vm.Call("main", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].I != 46 {
		t.Fatalf("numeric block result = %v, want 46", values)
	}

	bad := strings.Replace(src, "value 46", `value "forty-six"`, 1)
	if _, err := nanz.Parse(bad, "bad_numeric_block.nanz"); err == nil || !strings.Contains(err.Error(), "node_arg_int") {
		t.Fatalf("invalid numeric argument should fail at compile time, got %v", err)
	}
}
