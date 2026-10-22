package mir2

import (
	"github.com/minz/minzc/pkg/emulator"
	"strings"
	"testing"
)

// A copy in C lets the emitter reuse A for the ALU input. The original A
// value is still needed through a successor with no explicit edge arguments.
func TestByteALUCopyAliasPreservesLiveAccumulator(t *testing.T) {
	for _, op := range []Op{OpAdd, OpSub, OpAnd, OpOr, OpXor} {
		t.Run(op.String(), func(t *testing.T) {
			k := int64(3)
			if op == OpAdd || op == OpSub {
				k = 4
			}
			m := &Module{}
			f := m.AddFunc("copy_alias")
			f.Contract.Params = []Param{{Reg: 1, Ty: TyU8, Class: ClassAcc}}
			f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}, {Ty: TyU8, Class: ClassGeneral}}
			f.Blocks = []*Block{
				{Label: "entry", Insts: []*Inst{
					{Op: OpMove, Dst: 2, Src: [2]Reg{1}, Ty: TyU8, SrcTy: TyU8},
					{Op: OpConst, Dst: 3, Ty: TyU8, Imm: k},
					{Op: op, Dst: 4, Src: [2]Reg{2, 3}, Ty: TyU8},
				}, Term: &TermJmp{Target: "after"}},
				{Label: "after", Term: &TermRet{Vals: []Reg{1, 4}}},
			}
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: "A"}, 2: {Kind: LocReg, Name: "C"}, 3: {Kind: LocReg, Name: "D"}, 4: {Kind: LocReg, Name: "C"}}}
			asm, err := Z80Codegen(m, ar)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(asm, "LD C, A") < 2 {
				t.Fatal("copy/ALU shape missing", asm)
			}
			for x := int64(0); x < 256; x++ {
				want := byte(x + k)
				switch op {
				case OpSub:
					want = byte(x - k)
				case OpAnd:
					want = byte(x & k)
				case OpOr:
					want = byte(x | k)
				case OpXor:
					want = byte(x ^ k)
				}
				got, err := NewVM(m).Call(f.Name, []Value{{I: x}})
				z := runRAFragment(t, asm, emulator.Registers{A: byte(x)})
				if err != nil || len(got) != 2 || got[0].I != x || got[1].I != int64(want) || z.A != byte(x) || byte(z.BC) != want {
					t.Fatal(x, got, err, z, want, asm)
				}
			}
		})
	}
}
