package nanz

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

func TestStructuredMetaDiagnostics(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"if-condition", `emit_hir(hir_if(hir_int(1,c"u8"),hir_block(),hir_block()))`, "if needs boolean"},
		{"if-branch", `emit_hir(hir_if(hir_int(1,c"bool"),hir_int(2,c"u8"),hir_block()))`, "if needs boolean"},
		{"if-cycle-then", `let b:u16=hir_block() hir_append(b,hir_if(hir_int(1,c"bool"),b,hir_block()))`, "cyclic block"},
		{"if-cycle-else", `let b:u16=hir_block() hir_append(b,hir_if(hir_int(1,c"bool"),hir_block(),b))`, "cyclic block"},
		{"bitwise-bool", `emit_hir(hir_binary(c"&",hir_int(1,c"bool"),hir_int(0,c"bool")))`, "matching integer types"},
		{"bitwise-mismatch", `emit_hir(hir_binary(c"|",hir_int(1,c"u8"),hir_int(0,c"u16")))`, "matching integer types"},
		{"load-type", `emit_hir(hir_load(hir_int(1,c"u8")))`, "load needs byte pointer"},
		{"handle", `emit_hir(0)`, "invalid node handle"},
		{"kind", `emit_hir(hir_int(1,c"u8"))`, "expected statement"},
		{"overflow", `emit_hir(hir_int(256,c"u8"))`, "integer outside"},
		{"type", `emit_hir(hir_var(c"x",c"u8",hir_int(1,c"u16")))`, "initializer type mismatch"},
		{"cycle", `let b:u16=hir_block() hir_append(b,b) emit_hir(b)`, "cyclic block"},
		{"mixed", `emit(c"var x:u8=1") emit_hir(hir_block())`, "cannot mix textual and structured"},
		{"identifier", `emit_hir(hir_var(c"x;return",c"u8",hir_int(1,c"u8")))`, "invalid identifier"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse("fun @build() { "+tc.body+" }\nfun main() { @build() }", "bad-meta.nanz")
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "@build") {
				t.Fatalf("error=%v want %s", err, tc.want)
			}
		})
	}
}
func TestStructuredMetaBuildsTypedHIR(t *testing.T) {
	h, err := Parse(`fun @build() {
 let b:u16=hir_block()
 hir_append(b,hir_var(c"x",c"u8",hir_int(7,c"u8")))
 emit_hir(b)
 }
 fun main() { @build() }`, "typed-meta.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if err = hir.Verify(h); err != nil {
		t.Fatal(err)
	}
	b, ok := h.Funcs[0].Body.Body[0].(*hir.Block)
	if !ok || len(b.Body) != 1 {
		t.Fatalf("body=%#v", h.Funcs[0].Body)
	}
}

func TestStructuredMetaSnapshotsAndInvocationIsolation(t *testing.T) {
	h, err := Parse(`fun @build() {
        let b:u16=hir_block()
        hir_append(b,hir_var(c"x",c"u8",hir_int(7,c"u8")))
        emit_hir(b)
        hir_append(b,hir_assign(hir_ref(c"x",c"u8"),hir_int(9,c"u8")))
        emit_hir(b)
    }
    fun main() { @build() @build() }`, "snapshot-meta.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if err = hir.Verify(h); err != nil {
		t.Fatal(err)
	}
	for _, call := range h.Funcs[0].Body.Body {
		block := call.(*hir.Block)
		if len(block.Body) != 2 || len(block.Body[0].(*hir.Block).Body) != 1 || len(block.Body[1].(*hir.Block).Body) != 2 {
			t.Fatal("emitted code changed after builder mutation", block)
		}
		first := block.Body[0].(*hir.Block).Body[0]
		second := block.Body[1].(*hir.Block).Body[0]
		if first == second {
			t.Fatal("shared mutable HIR statements")
		}
	}
}
func TestStructuredMetaHandleLimit(t *testing.T) {
	b := metaHIRBuilder{nodes: make([]any, 65534)}
	values, err := b.add(&hir.IntLitExpr{Val: 1, Ty: mir2.TyU8})
	if err != nil || values[0].I != 65535 {
		t.Fatal(values, err)
	}
	if _, err = b.add(&hir.Block{}); err == nil || !strings.Contains(err.Error(), "exhausted") {
		t.Fatal("handle overflow accepted", err)
	}
}

func TestStructuredIfSnapshotsBothBranches(t *testing.T) {
	h, err := Parse(`fun @build() {
   let yes:u16=hir_block()
   let no:u16=hir_block()
   hir_append(yes,hir_store(hir_int(24576,c"ptr"),hir_int(7,c"u8")))
   hir_append(no,hir_store(hir_int(24576,c"ptr"),hir_int(9,c"u8")))
   let branch:u16=hir_if(hir_ref(c"visible",c"bool"),yes,no)
   emit_hir(branch)
   hir_append(yes,hir_store(hir_int(24577,c"ptr"),hir_int(1,c"u8")))
   hir_append(no,hir_store(hir_int(24577,c"ptr"),hir_int(2,c"u8")))
   emit_hir(branch)
 }
 fun main(visible:bool) { @build() }`, "if-snapshots.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if err = hir.Verify(h); err != nil {
		t.Fatal(err)
	}
	block := h.Funcs[0].Body.Body[0].(*hir.Block)
	first := block.Body[0].(*hir.IfStmt)
	second := block.Body[1].(*hir.IfStmt)
	if len(first.Then.Body) != 1 || len(first.Else.Body) != 1 || len(second.Then.Body) != 2 || len(second.Else.Body) != 2 {
		t.Fatal("conditional snapshots changed after builder mutation")
	}
	if first.Then.Body[0] == second.Then.Body[0] || first.Else.Body[0] == second.Else.Body[0] {
		t.Fatal("shared conditional statements")
	}
}

func TestStructuredCallerReferenceTypes(t *testing.T) {
	for _, tc := range []struct{ name, decl, ref, want string }{
		{"parameter", `fun main(frame:^u8) { @build() }`, `hir_caller_ref(c"frame",c"ptr")`, ""},
		{"global", `global frame:ptr=0 fun main() { @build() }`, `hir_caller_ref(c"frame",c"ptr")`, ""},
		{"local-shadows-global", `global frame:u16=0 fun main(frame:^u8) { @build() }`, `hir_caller_ref(c"frame",c"ptr")`, ""},
		{"wrong-type", `fun main(frame:u16) { @build() }`, `hir_caller_ref(c"frame",c"ptr")`, "type u16, requested ptr"},
		{"missing-even-without-emit", `fun main() { @build() }`, `hir_caller_ref(c"frame",c"ptr")`, "unknown caller variable"},
		{"builder-local-is-not-caller", `fun main() { @build() }`, `hir_caller_ref(c"generated",c"u8")`, "unknown caller variable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := `fun @build() { let local:u16=hir_var(c"generated",c"u8",hir_int(1,c"u8")) let ref:u16=` + tc.ref + ` } ` + tc.decl
			_, e := Parse(source, "caller-ref.nanz")
			if tc.want == "" {
				if e != nil {
					t.Fatal(e)
				}
			} else if e == nil || !strings.Contains(e.Error(), tc.want) {
				t.Fatalf("got %v want %s", e, tc.want)
			}
		})
	}
}

func TestStructuredMetaCallerArrayHygieneAndSnapshots(t *testing.T) {
	h, err := Parse(`global values:[u8;3]=[1,2,3]
 fun @build() {
  let b:u16=hir_block()
  hir_append(b,hir_var(c"values",c"u8",hir_int(99,c"u8")))
  hir_append(b,hir_var(c"index",c"u8",hir_int(1,c"u8")))
  let element:u16=hir_caller_index(c"values",hir_ref(c"index",c"u8"))
  hir_append(b,hir_assign(element,hir_int(7,c"u8")))
  emit_hir(b)
  hir_append(b,hir_assign(hir_caller_index(c"values",hir_int(2,c"u8")),hir_int(9,c"u8")))
  emit_hir(b)
 }
 fun main() { @build() @build() }
 `, "caller-array.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if err = hir.Verify(h); err != nil {
		t.Fatal(err)
	}
	v := mir2.NewVM(hir.LowerModule(h))
	if _, err = v.Call("main", nil); err != nil {
		t.Fatal(err)
	}
	got := v.ReadHeap(v.GlobalAddr("values"), 3)
	if len(got) != 3 || got[0] != 1 || got[1] != 7 || got[2] != 9 {
		t.Fatal("caller array captured by generated local", got)
	}
	// Both emitted blocks retain independent expression trees.
	outer := h.Funcs[0].Body.Body[0].(*hir.Block)
	first := outer.Body[0].(*hir.Block).Body[2].(*hir.AssignStmt).Target.(*hir.IndexExpr)
	second := outer.Body[1].(*hir.Block).Body[2].(*hir.AssignStmt).Target.(*hir.IndexExpr)
	if first == second || first.Base == second.Base || first.Idx == second.Idx {
		t.Fatal("mutable index nodes shared by snapshots")
	}
}

func TestStructuredMetaCallerArrayDiagnostics(t *testing.T) {
	for _, tc := range []struct{ name, global, index, want string }{
		{"unknown", "global other:[u8;3]", `hir_int(0,c"u8")`, "needs byte array"},
		{"scalar", "global values:u8", `hir_int(0,c"u8")`, "needs byte array"},
		{"word-array", "global values:[u16;3]", `hir_int(0,c"u8")`, "needs byte array"},
		{"bool-index", "global values:[u8;3]", `hir_int(0,c"bool")`, "index needs u8 or u16"},
		{"pointer-index", "global values:[u8;3]", `hir_int(0,c"ptr")`, "index needs u8 or u16"},
		{"constant-bound", "global values:[u8;3]", `hir_int(3,c"u8")`, "index out of bounds"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.global+`
fun @build() { emit_hir(hir_assign(hir_caller_index(c"values",`+tc.index+`),hir_int(7,c"u8"))) }
fun main() { @build() }`, "bad-caller-array.nanz")
			// Use actual newlines in the constructed source.
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v want %s", err, tc.want)
			}
		})
	}
}
