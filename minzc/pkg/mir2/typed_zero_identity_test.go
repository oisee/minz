package mir2

import (
	"reflect"
	"testing"
)

func zeroIdentityFixture(op Op, ty Ty, reverse bool) (*Module, *Func, *Inst) {
	m := &Module{}
	f := m.AddFunc("zero")
	f.Contract.Params = []Param{{Reg: 1, Ty: ty, Class: ClassGeneral}}
	f.Contract.Returns = []Return{{Ty: ty, Class: ClassGeneral}}
	src := [2]Reg{1, 2}
	if reverse {
		src = [2]Reg{2, 1}
	}
	x := &Inst{Op: op, Dst: 3, Src: src, Ty: ty, Cls: ClassGeneral, SrcFile: "original.nanz", SrcLine: 12}
	f.Blocks = []*Block{{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 2, Ty: ty}, x}, Term: &TermRet{Vals: []Reg{3}}}}
	return m, f, x
}
func TestTypedZeroIdentitiesModels(t *testing.T) {
	for _, ty := range []Ty{TyU8, TyI8, TyU16, TyI16} {
		for _, op := range []Op{OpOr, OpXor, OpSub, OpShl, OpAnd, OpMul} {
			for _, reverse := range []bool{false, true} {
				if op == OpMul && ty.Width() == 8 {
					continue
				}
				if reverse && (op == OpSub || op == OpShl) {
					continue
				}
				m, f, _ := zeroIdentityFixture(op, ty, reverse)
				if !simplifyTypedZeroIdentities(f) {
					t.Fatal("no rewrite", op, ty)
				}
				x := f.Entry().Insts[1]
				if x.SrcLine != 12 || x.SrcFile != "original.nanz" {
					t.Fatal("source metadata")
				}
				vm := NewVM(m)
				vm.MaxSteps = 0
				for v := int64(0); v < 65536; v++ {
					if ty.Width() == 8 && v >= 256 {
						break
					}
					out, e := vm.Call("zero", []Value{{I: v}})
					want := v
					if op == OpAnd || op == OpMul {
						want = 0
					}
					if e != nil || len(out) != 1 || out[0].I != want {
						t.Fatal(op, ty, v, out, want, e)
					}
				}
			}
		}
	}
}
func TestTypedZeroIdentitiesRefusals(t *testing.T) {
	for _, kind := range []string{"byte-mul", "hard", "hard-constant", "constant-metadata", "cmp", "flag-return", "flag-block", "flag-param", "mixed", "wide", "bool", "later", "flag", "asm", "CY", "callcond", "metadata", "nonzero", "wrong-sub"} {
		t.Run(kind, func(t *testing.T) {
			m, f, x := zeroIdentityFixture(OpOr, TyU8, false)
			switch kind {
			case "byte-mul":
				x.Op = OpMul
			case "hard":
				x.ClsHard = true
			case "hard-constant":
				f.Entry().Insts[0].ClsHard = true
			case "constant-metadata":
				f.Entry().Insts[0].SequentialBytes = true
			case "cmp":
				f.Entry().Insts = append(f.Entry().Insts, &Inst{Op: OpCmp, Dst: 4, Src: [2]Reg{1, 2}, Ty: TyBool})
			case "flag-return":
				f.Contract.Returns[0].Class = ClassFlag
			case "flag-param":
				f.Contract.Params[0].Class = ClassFlag
			case "flag-block":
				f.Entry().Params = []BlockParam{{Dst: 5, Ty: TyBool, Class: ClassFlag}}
			case "mixed":
				f.Contract.Params[0].Ty = TyU16
			case "wide":
				x.Ty = TyU32
			case "bool":
				x.Ty = TyBool
			case "later":
				f.Entry().Insts = []*Inst{x, f.Entry().Insts[0]}
			case "flag":
				x.Cls = ClassFlag
			case "asm":
				f.Entry().Insts = append(f.Entry().Insts, &Inst{Op: OpAsm, Asm: &AsmExtra{Template: "ADC A,0"}})
			case "CY":
				f.Attrs.FallibleCY = true
			case "callcond":
				f.Entry().Insts = append(f.Entry().Insts, &Inst{Op: OpCallCond})
			case "metadata":
				x.SequentialBytes = true
			case "nonzero":
				f.Entry().Insts[0].Imm = 1
			case "wrong-sub":
				x.Op = OpSub
				x.Src = [2]Reg{2, 1}
			}
			before := m.Dump()
			copy := *x
			if simplifyTypedZeroIdentities(f) || m.Dump() != before || !reflect.DeepEqual(*x, copy) {
				t.Fatal("unsafe rewrite", kind)
			}
		})
	}
}

func TestTypedZeroIdentitiesStoredConstant(t *testing.T) {
	for _, tc := range []struct {
		ty  Ty
		imm int64
	}{{TyU8, 256}, {TyI8, -256}, {TyU16, 65536}, {TyI16, -65536}} {
		_, f, _ := zeroIdentityFixture(OpOr, tc.ty, false)
		f.Entry().Insts[0].Imm = tc.imm
		if !simplifyTypedZeroIdentities(f) || f.Entry().Insts[1].Op != OpMove {
			t.Fatal(tc)
		}
	}
}
