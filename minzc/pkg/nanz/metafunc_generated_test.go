package nanz_test

import (
	"os"
	"path/filepath"
	"strconv"
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

// Numeric block hosts must carry a signed type through assignment and bounds
// checks, rather than being an undeclared void call with an incidental VM value.
func TestMetafunctionSignedNumericAssignment(t *testing.T) {
	const src = `
fun @validate() {
    var n: i32 = 0
    n = node_arg_int(0, 0)
    if n < 0 { meta_error(c"negative numeric argument") }
    if n > 65535 { meta_error(c"address out of range") }
    emit(c"fun generated() -> u8 { return 42 }")
}
@validate() { value 65535 }
fun main() -> u8 { return generated() }
`
	if _, err := nanz.Parse(src, "numeric_assignment.nanz"); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"-1", "65536", "2147483648", "-2147483649"} {
		bad := strings.Replace(src, "value 65535", "value "+strconv.Quote(value), 1)
		if _, err := nanz.Parse(bad, "numeric_assignment_bad.nanz"); err == nil {
			t.Fatalf("invalid numeric argument %s accepted", value)
		}
	}
}

func TestDistinctMetafunctionInvocationsHaveDistinctSourceUnits(t *testing.T) {
	const src = `fun @constant(name: ^u8) -> void {
 let value: u16 = node_arg_int(0, 0)
 emit(str_concat(c"fun ", str_concat(name, str_concat(c"() -> u16 { return ", str_concat(str_from_int(value), c" }")))))
 }
 @constant("one") { value 1 }
 @constant("two") { value 2 }
 fun main() -> u16 { return one() + two() }
 `
	m, err := nanz.Parse(src, "distinct-meta.nanz")
	if err != nil {
		t.Fatal(err)
	}
	got, err := mir2.NewVM(hir.LowerModule(m)).Call("main", nil)
	if err != nil || len(got) != 1 || got[0].I != 3 {
		t.Fatalf("got %v err %v", got, err)
	}
	again, err := nanz.Parse(src, "distinct-meta.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.SourceUnits) != 3 || len(again.SourceUnits) != 3 {
		t.Fatalf("source units: %v", m.SourceUnits)
	}
	for name, hash := range m.SourceUnits {
		if again.SourceUnits[name] != hash {
			t.Fatal("unstable source identity", name)
		}
	}
}

func TestGeneratedStructMethodsAreVisible(t *testing.T) {
	const source = `fun @counter(name:^u8) {
 emit(str_concat(str_concat(c"struct ",name),c" {value:u8}"))
 emit(str_concat(str_concat(c"fun ",name),str_concat(c".add(self:^",str_concat(name,c",n:u8)->u8 {return self.value+n}"))))
}
@counter("First")
@counter("Second")
fun main()->u8 {
 let a:First=First{value:3}
 let b:Second=Second{value:4}
 return a.add(2)+b.add(3)
}`
	m, err := nanz.Parse(source, "generated-methods.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if err = hir.Verify(m); err != nil {
		t.Fatal(err)
	}
	got, err := mir2.NewVM(hir.LowerModule(m)).Call("main", nil)
	if err != nil || len(got) != 1 || got[0].I != 12 {
		t.Fatalf("methods: %v %v want12", got, err)
	}
	for _, name := range []string{"First_add", "Second_add"} {
		if !strings.Contains(m.Dump(), "call @"+name) {
			t.Fatalf("missing concrete method %s", name)
		}
	}
}

func TestGeneratedSharedImportDeclarations(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "shared.nanz"), []byte(`struct Item {value:u8}
global base:u8=2
fun read(x:u8)->u8{return x+base}
`), 0644); err != nil {
		t.Fatal(err)
	}
	const meta = `fun @maker(name:^u8){
 emit(c"import shared")
 emit(str_concat(str_concat(c"fun ",name),c"()->u8{return read(1)}"))
}
`
	m, err := nanz.ParseWithOpts(meta+`@maker("One")
@maker("Two")
fun main()->u8{return One()+Two()}`, "shared-generators.nanz", nanz.ParseOpts{BaseDir: directory})
	if err != nil {
		t.Fatal(err)
	}
	if err = hir.Verify(m); err != nil {
		t.Fatal(err)
	}
	got, err := mir2.NewVM(hir.LowerModule(m)).Call("main", nil)
	if err != nil || len(got) != 1 || got[0].I != 6 {
		t.Fatal(got, err)
	}
	if len(m.Globals) != 1 || len(m.Structs) != 1 {
		t.Fatalf("reimported declarations: globals%d structs%d", len(m.Globals), len(m.Structs))
	}
	_, err = nanz.ParseWithOpts(meta+`@maker("Same")
@maker("Same")`, "duplicate-generator.nanz", nanz.ParseOpts{BaseDir: directory})
	if err == nil || !strings.Contains(err.Error(), "duplicate generated function") {
		t.Fatalf("ordinary collision accepted: %v", err)
	}
}
