package pipeline

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

func TestZ80TupleChecksEveryReturn(t *testing.T) {
	z := emulator.NewRemogattoZ80()
	z.SetRegisters(emulator.Registers{A: 42, BC: 43})
	f := &mir2.Func{}
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}, {Ty: mir2.TyU8, Class: mir2.ClassRegC}}
	a := hir.Assert{ExpectedMulti: []int64{42, 43}}
	if err := checkAssertZ80Result(z, a, f, nil); err != nil {
		t.Fatal(err)
	}
	a.ExpectedMulti[1] = 44
	if err := checkAssertZ80Result(z, a, f, nil); err == nil || !strings.Contains(err.Error(), "return[1] got 43, want 44") {
		t.Fatalf("second tuple value unchecked: %v", err)
	}
	f.Contract.Returns = f.Contract.Returns[:1]
	if err := checkAssertZ80Result(z, a, f, nil); err == nil {
		t.Fatal("missing return unchecked")
	}
}
