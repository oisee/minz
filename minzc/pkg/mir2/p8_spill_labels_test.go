package mir2

import (
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestP8OrphanSpillDefinition(t *testing.T) {
	m := &Module{Name: "p8"}
	f := m.AddFunc("arith")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	a := b.Param("a", TyU8, ClassAcc)
	x := b.Const(7, TyU8, ClassGeneral)
	out := b.Add(a, x, TyU8, ClassAcc)
	b.Ret(out)
	ar := &AllocResult{Locs: map[Reg]PhysLoc{a: {Kind: LocReg, Name: "A"}, x: {Kind: LocMem}, out: {Kind: LocReg, Name: "A"}}}
	asm := mustZ80Asm(Z80Codegen(m, ar))
	res, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(res.Errors) > 0 {
		t.Fatalf("%v %v\n%s", err, res.Errors, asm)
	}
	// The immediate ALU path does not emit the TSMC reload, so the
	// constant's patch store must become an ordinary, defined spill store.
	if strings.Contains(asm, "LD (_spill_") && !strings.Contains(asm, "_spill_arith_r2:") {
		t.Fatalf("missing spill definition:\n%s", asm)
	}
}
