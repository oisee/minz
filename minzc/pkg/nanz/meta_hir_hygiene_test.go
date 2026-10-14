package nanz

import (
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// Independent lexical oracle: inner if x=8, loop x=9, outer x=7.
// Caller x becomes 8+9+7=24; the two expansions never bind caller x.
func TestStructuredMetaNestedCallerHygiene(t *testing.T) {
	const source = `fun @build() {
 let caller:u16=hir_caller_ref(c"x",c"u8")
 let x:u16=hir_ref(c"x",c"u8")
 let i:u16=hir_ref(c"i",c"u8")
 let b:u16=hir_block()
 hir_append(b,hir_var(c"x",c"u8",hir_int(7,c"u8")))
 hir_append(b,hir_var(c"i",c"u8",hir_int(0,c"u8")))
 let yes:u16=hir_block()
 hir_append(yes,hir_var(c"x",c"u8",hir_binary(c"+",x,hir_int(1,c"u8"))))
 hir_append(yes,hir_assign(caller,x))
 hir_append(b,hir_if(hir_int(1,c"bool"),yes,hir_block()))
 let loop:u16=hir_block()
 hir_append(loop,hir_var(c"x",c"u8",hir_binary(c"+",x,hir_int(2,c"u8"))))
 hir_append(loop,hir_assign(caller,hir_binary(c"+",caller,x)))
 hir_append(loop,hir_assign(i,hir_binary(c"+",i,hir_int(1,c"u8"))))
 hir_append(b,hir_while(hir_binary(c"!=",i,hir_int(1,c"u8")),loop))
 hir_append(b,hir_assign(caller,hir_binary(c"+",caller,x)))
 emit_hir(b)
 emit_hir(b)
}
fun main(x:u8)->u8 { @build() @build() return x }
`
	first, err := Parse(source, "hygiene.nanz")
	if err != nil {
		t.Fatal(err)
	}
	second, err := Parse(source, "hygiene.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Funcs, second.Funcs) {
		t.Fatal("non-deterministic private bindings")
	}
	if err = hir.Verify(first); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	var visit func(hir.Stmt)
	visit = func(s hir.Stmt) {
		switch n := s.(type) {
		case *hir.Block:
			for _, c := range n.Body {
				visit(c)
			}
		case *hir.VarDeclStmt:
			if !strings.HasPrefix(n.Name, "$meta_") || names[n.Name] {
				t.Fatalf("capturable or reused generated binding %q", n.Name)
			}
			names[n.Name] = true
		case *hir.IfStmt:
			visit(n.Then)
			visit(n.Else)
		case *hir.WhileStmt:
			visit(n.Body)
		}
	}
	visit(first.Funcs[0].Body)
	// Four snapshots, each with x, i and two nested x declarations.
	if len(names) != 16 {
		t.Fatalf("private declarations=%d, want16", len(names))
	}
	vm := mir2.NewVM(hir.LowerModule(first))
	for x := 0; x < 256; x++ {
		got, err := vm.Call("main", []mir2.Value{{I: int64(x)}})
		if err != nil || len(got) != 1 || got[0].I != 24 {
			t.Fatalf("input%d result%v err%v want24", x, got, err)
		}
	}
}

func TestStructuredMetaGlobalCallerHygiene(t *testing.T) {
	h, err := Parse(`global x:u8=3
fun @build() {
 let b:u16=hir_block()
 let caller:u16=hir_caller_ref(c"x",c"u8")
 hir_append(b,hir_var(c"x",c"u8",hir_int(7,c"u8")))
 hir_append(b,hir_assign(caller,hir_binary(c"+",hir_ref(c"x",c"u8"),hir_int(2,c"u8"))))
 emit_hir(b)
}
fun main()->u8 { @build() return x }`, "global-hygiene.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if err = hir.Verify(h); err != nil {
		t.Fatal(err)
	}
	got, err := mir2.NewVM(hir.LowerModule(h)).Call("main", nil)
	if err != nil || len(got) != 1 || got[0].I != 9 {
		t.Fatalf("global caller result%v err%v want9", got, err)
	}
}
