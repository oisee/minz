package mir2

import (
	"strings"
	"testing"
)

func TestDisjointBytePackingRejectsUnprovedBoundaries(t *testing.T) {
	for _, kind := range []string{"accepted", "flags", "alias", "index", "nonzero-low", "signed-ext", "use-after", "live-half", "pending-flag", "symmetric"} {
		t.Run(kind, func(t *testing.T) {
			m := &Module{}
			f := m.AddFunc("packing")
			b := f.NewBlock("entry")
			b.Insts = []*Inst{{Op: OpExt, Dst: 2, Src: [2]Reg{1}, SrcTy: TyU8, Ty: TyU16}, {Op: OpConst, Dst: 3, Imm: 8, Ty: TyU16}, {Op: OpShl, Dst: 4, Src: [2]Reg{2, 3}, Ty: TyU16}, {Op: OpExt, Dst: 6, Src: [2]Reg{5}, SrcTy: TyU8, Ty: TyU16}, {Op: OpOr, Dst: 7, Src: [2]Reg{4, 6}, Ty: TyU16}}
			target := b.Insts[4]
			b.Term = &TermRet{Vals: []Reg{7}}
			ar := &AllocResult{Locs: map[Reg]PhysLoc{4: {Name: "HL"}, 6: {Name: "BC"}, 7: {Name: "HL"}}}
			switch kind {
			case "flags":
				ar.Locs[8] = PhysLoc{Name: "F"}
				b.Term = &TermRet{Vals: []Reg{7, 8}}
			case "alias":
				ar.Locs[6] = PhysLoc{Name: "HL"}
			case "index":
				ar.Locs[4] = PhysLoc{Name: "IX", Kind: LocIXY}
				ar.Locs[7] = ar.Locs[4]
			case "nonzero-low":
				b.Insts[2].Op = OpOr
			case "signed-ext":
				b.Insts[0].SrcTy = TyI8
			case "use-after":
				b.Term = &TermRet{Vals: []Reg{4, 7}}
			case "live-half":
				ar.Locs[8] = PhysLoc{Name: "L"}
				b.Term = &TermRet{Vals: []Reg{7, 8}}
			case "symmetric":
				target.Src = [2]Reg{6, 4}
			}
			var out strings.Builder
			g := &z80cg{fn: f, curBlock: b, ar: ar, sb: &out, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}}
			if kind == "pending-flag" {
				g.pendingFlagReg = 8
			}
			got := g.genDisjointByteOr(target)
			want := kind == "accepted" || kind == "symmetric"
			if got != want {
				t.Fatalf("accepted=%v want%v output%s", got, want, out.String())
			}
		})
	}
}
