package mir2

import (
	"fmt"
	"testing"
)

// A folded RHS does not remove the register use of the same constant on the
// LHS. Exercise every opcode recognized by computeDeadConsts at each width,
// and check that RHS-only folding remains available where supported.
func TestComputeDeadConstsOperandPositions(t *testing.T) {
	for _, ty := range []Ty{TyU8, TyU16, TyU32} {
		for _, op := range []Op{OpAdd, OpSub, OpAnd, OpOr, OpXor, OpCmp, OpDiv, OpSDiv, OpMod, OpSMod, OpStore, OpMul, OpShl, OpShr, OpSar} {
			for _, self := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/self=%t", ty, op, self), func(t *testing.T) {
					lhs := Reg(2)
					if self {
						lhs = 1
					}
					inst := &Inst{Op: op, Dst: 3, Src: [2]Reg{lhs, 1}, Ty: ty, SrcTy: ty}
					if op == OpCmp {
						inst.Ty = TyBool
					}
					f := &Func{Blocks: []*Block{{Insts: []*Inst{{Op: OpConst, Dst: 1, Ty: ty, Imm: 1}, inst}}}}
					loc := "A"
					if ty.Width() > 8 {
						loc = "HL"
					}
					ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: loc}, 2: {Kind: LocReg, Name: loc}, 3: {Kind: LocReg, Name: loc}}}
					wantDead := !self
					switch op {
					case OpCmp, OpStore:
						wantDead = wantDead && ty.Width() <= 8
					case OpAdd, OpSub, OpAnd, OpOr, OpXor, OpDiv, OpSDiv, OpMod, OpSMod:
						wantDead = wantDead && ty.Width() <= 16
					}
					if got := computeDeadConsts(f, ar)[1]; got != wantDead {
						t.Fatalf("constant load suppressed = %t, want %t", got, wantDead)
					}
				})
			}
		}
	}
}

// genBinOp emits every 8-bit ADD/SUB of a g.constVals RHS as INC/DEC or
// ADD A,n / SUB n, so such a constant is dead whatever its value. A constant
// merged with a block parameter is not in g.constVals (BUG-014) and keeps its
// load.
func TestComputeDeadConstsByteAddSubAnyValue(t *testing.T) {
	for _, op := range []Op{OpAdd, OpSub} {
		for _, k := range []int64{4, 31, 100, 255} {
			for _, param := range []bool{false, true} {
				inst := &Inst{Op: op, Dst: 3, Src: [2]Reg{2, 1}, Ty: TyU8}
				entry := &Block{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 1, Ty: TyU8, Imm: k}, inst}}
				if param {
					entry.Params = []BlockParam{{Dst: 1, Ty: TyU8}}
				}
				f := &Func{Blocks: []*Block{entry}}
				ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: "D"}, 2: {Kind: LocReg, Name: "C"}, 3: {Kind: LocReg, Name: "A"}}}
				if got := computeDeadConsts(f, ar)[1]; got == param {
					t.Fatalf("%s k=%d block-param=%t: constant load suppressed = %t", op, k, param, got)
				}
			}
		}
	}
}
