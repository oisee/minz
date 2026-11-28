package mir2

import (
	"strings"
	"testing"
)

// d = add x, 31 feeds only the next compare, so byteResultStaysInA would keep
// d in A. A also holds v, live after both, and every scratch byte is taken
// (x in B, y in C, pairs DE/HL/IX/IY live): saveAccIfLive must preserve v on
// the stack, and its deferred restore rewrites A after the add. The result
// must then go to its home, not stay in A.
func TestByteResultNotLeftInAAcrossStackRestore(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("busy")
	add := &Inst{Op: OpAdd, Dst: 3, Src: [2]Reg{1, 9}, Ty: TyU8}
	cmp := &Inst{Op: OpCmp, Dst: 4, Src: [2]Reg{3, 2}, Ty: TyBool, SrcTy: TyU8, Cond: CmpUlt}
	b := &Block{Label: "entry", Insts: []*Inst{add, cmp}, Term: &TermRet{Vals: []Reg{5, 4, 6, 7, 8, 10}}}
	f.Blocks = []*Block{b}
	a := &AllocResult{Locs: map[Reg]PhysLoc{
		1: {Name: "B"}, 2: {Name: "C"}, 3: {Name: "B"}, 4: {Name: "F", Kind: LocFlag}, 5: {Name: "A"},
		6: {Name: "DE"}, 7: {Name: "HL"}, 8: {Name: "IX", Kind: LocIXY}, 10: {Name: "IY", Kind: LocIXY},
		9: {Kind: LocRemat, Name: RematLocName}}}
	var out strings.Builder
	g := &z80cg{fn: f, mod: m, curBlock: b, curInstIdx: 0, ar: a, sb: &out, constVals: map[Reg]int64{9: 31},
		deadConsts: map[Reg]bool{9: true}, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}}
	if g.freeScratch8(add) != "" {
		t.Fatal("scratch not exhausted")
	}
	g.genBinOp("ADD", add)
	code := out.String()
	if !strings.Contains(code, "POP") {
		t.Fatalf("expected a stack restore of A:\n%s", code)
	}
	if g.physOverride[3] == "A" {
		t.Fatalf("result left in A although the deferred restore rewrites A:\n%s", code)
	}
	if !strings.Contains(code, "LD B, A") {
		t.Fatalf("result not copied to its home B:\n%s", code)
	}
}
