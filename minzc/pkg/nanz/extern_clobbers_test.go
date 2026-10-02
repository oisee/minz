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
