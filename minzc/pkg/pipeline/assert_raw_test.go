package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/mir2llvm"
)

func TestAssertRejectsMalformedRawResults(t *testing.T) {
	for _, c := range []struct {
		ty        mir2.Ty
		raw, want int64
		pass      bool
	}{
		{mir2.TyBool, 2, 0, false}, {mir2.TyBool, 2, 1, false}, {mir2.TyBool, 3, 1, false},
		{mir2.TyU8, 300, 44, false}, {mir2.TyI16, 65041, -495, true}, {mir2.TyI16, 65041, 65041, true},
		{mir2.TyI16, 65041, -494, false},
	} {
		for _, tuple := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%d/%d/tuple=%v", c.ty, c.raw, c.want, tuple), func(t *testing.T) {
				m := &mir2.Module{}
				f := m.AddFunc("entry")
				f.Contract.Returns = []mir2.Return{{Ty: c.ty, Class: mir2.ClassAcc}}
				b := mir2.NewBuilder(f)
				b.SwitchToNewBlock("entry")
				// Return a deliberately malformed VM value without an arithmetic truncation.
				b.Ret(b.Const(c.raw, mir2.TyU16, mir2.ClassPointer))
				a := hir.Assert{FuncName: "entry", Expected: c.want}
				if tuple {
					a.ExpectedMulti = []int64{c.want}
				}
				check := func(backend string, err error) {
					t.Helper()
					if (err == nil) != c.pass {
						t.Errorf("%s: pass=%v: %v", backend, c.pass, err)
					}
					if !c.pass && c.ty == mir2.TyI16 && err != nil && !strings.Contains(err.Error(), "got -495 (raw 65041)") {
						t.Errorf("%s: missing signed diagnostic: %v", backend, err)
					}
				}
				check("mir2", runOneAssertMIR2(mir2.NewVM(m), a))
				if c.raw <= 255 || c.ty == mir2.TyI16 {
					z := emulator.NewRemogattoZ80()
					z.SetRegisters(emulator.Registers{A: byte(c.raw), HL: uint16(c.raw)})
					if c.ty == mir2.TyI16 {
						f.Contract.Returns[0].Class = mir2.ClassPointer
					}
					check("z80", checkAssertZ80Result(z, a, f, nil))
				}
			})
		}
	}
}

func TestAssertMissingReturnContracts(t *testing.T) {
	for _, backend := range []string{"mir2", "llvm"} {
		for _, kind := range []string{"missing_function", "missing_scalar", "short_tuple"} {
			t.Run(backend+"/"+kind, func(t *testing.T) {
				m := &mir2.Module{}
				a := hir.Assert{FuncName: "entry"}
				if kind != "missing_function" {
					f := m.AddFunc("entry")
					b := mir2.NewBuilder(f)
					b.SwitchToNewBlock("entry")
					r := b.Const(0, mir2.TyU8, mir2.ClassAcc)
					b.Ret(r, r)
					if kind == "short_tuple" {
						f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
						a.ExpectedMulti = []int64{0, 0}
					}
				}
				var err error
				if backend == "mir2" {
					err = runOneAssertMIR2(mir2.NewVM(m), a)
				} else {
					err = mir2llvm.RunAsserts(&hir.Module{Asserts: []hir.Assert{a}}, m, true)
				}
				if err == nil {
					t.Fatal("missing contract accepted")
				}
			})
		}
	}
}
