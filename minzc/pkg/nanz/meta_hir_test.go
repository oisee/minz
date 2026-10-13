package nanz

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

func TestStructuredMetaDiagnostics(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
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
