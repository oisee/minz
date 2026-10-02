package nanz_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestExternClobberContract(t *testing.T) {
	for _, tc := range []struct {
		attr string
		regs []string
		addr uint16
	}{
		{`@extern`, nil, 0},
		{`@extern(clobbers: "")`, []string{}, 0},
		{`@extern(clobbers: "a, HL, F, IXH")`, []string{"A", "HL", "F", "IXH"}, 0},
		{`@extern(0x1234, clobbers: "DE")`, []string{"DE"}, 0x1234},
		{`@extern(0x10)`, nil, 0x10},
	} {
		t.Run(tc.attr, func(t *testing.T) {
			m, err := nanz.Parse(tc.attr+` fun ext(@z80_hl addr: u16, value: u8) -> u8`, "test")
			if err != nil {
				t.Fatal(err)
			}
			check := func(m *hir.Module) {
				t.Helper()
				f := hir.LowerModule(m).FuncByName("ext")
				if !f.Attrs.IsExtern || f.Attrs.ExternAddr != tc.addr || !reflect.DeepEqual(f.Contract.ExternClobbers, tc.regs) {
					t.Fatalf("lost extern contract: %+v", f)
				}
				if len(f.Blocks) != 0 || len(f.Contract.Params) != 2 || f.Contract.Params[0].Class != mir2.ClassPointer || len(f.Contract.Returns) != 1 || f.Contract.Returns[0].Class != mir2.ClassAcc {
					t.Fatalf("lost extern ABI: %+v", f.Contract)
				}
			}
			check(m)
			// Use ordinary parameters for printer round-trip (register annotations
			// are not currently printed by the source formatter).
			m.Funcs[0].Params[0].RegClass = 0
			round, err := nanz.Parse(nanz.Print(m), "round")
			if err != nil {
				t.Fatal(err)
			}
			check(round)
		})
	}
}

func TestExternClobbersRejectUnknown(t *testing.T) {
	for _, reg := range []string{"QQ", "SP", "PC", "A'", "I", "R", "A,", "mem0"} {
		_, err := nanz.Parse(`@extern(clobbers: "`+reg+`") fun ext()`, "test")
		if err == nil || !strings.Contains(err.Error(), "clobber register") {
			t.Errorf("%q: expected invalid register error, got %v", reg, err)
		}
	}
}

func TestExternRedeclarationConflicts(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"body after extern", `@extern(clobbers: "") fun g(a: u8) -> u8
fun g(a: u8) -> u8 { return a + 2 }
fun main() -> u8 { return g(1) + 12 }`, "shares a name with a function body"},
		{"extern after body", `fun g(a: u8) -> u8 { return a }
@extern(clobbers: "") fun g(a: u8) -> u8`, "shares a name with a function body"},
		{"extern overload", `@extern(clobbers: "") fun f(a: u8) -> void
@extern fun f(a: u16) -> void
fun main() -> void { f(0x1234) }`, "conflicting @extern redeclarations"},
		{"contract", `@extern(clobbers: "") fun f()
@extern(clobbers: "C") fun f()`, "conflicting @extern redeclarations"},
		{"address", `@extern(0x10) fun f()
@extern(0x18) fun f()`, "conflicting @extern redeclarations"},
		{"return", `@extern fun f() -> u8
@extern fun f() -> u16`, "conflicting @extern redeclarations"},
		{"parameter ABI", `@extern fun f(@z80_a a: u8)
@extern fun f(@z80_c a: u8)`, "conflicting @extern redeclarations"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := nanz.Parse(tc.source, "test")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
	// Repeated imports may supply identical declarations with different parameter names.
	if _, err := nanz.Parse("@extern fun f(a: u8)\n@extern fun f(b: u8)", "test"); err != nil {
		t.Fatal(err)
	}
}

func TestExternClobberSyntaxDiagnostics(t *testing.T) {
	for _, attr := range []string{`@extern(clobbers "A")`, `@extern(0x10 clobbers: "A")`, `@extern(clobbers: A)`, `@extern(writes: "A")`} {
		_, err := nanz.Parse(attr+" fun f()", "test")
		if err == nil || !strings.Contains(err.Error(), `@extern(clobbers: "A, HL")`) || !strings.Contains(err.Error(), `@extern(0x1234, clobbers: "...")`) || strings.Contains(err.Error(), "token kind") {
			t.Errorf("%s: got %v", attr, err)
		}
	}
	for _, regs := range []string{"A,", ",HL", "A,,HL", "A, ,HL"} {
		_, err := nanz.Parse(`@extern(clobbers: "`+regs+`") fun f()`, "test")
		if err == nil || !strings.Contains(err.Error(), "empty clobber register entry") {
			t.Errorf("%q: got %v", regs, err)
		}
	}
}
