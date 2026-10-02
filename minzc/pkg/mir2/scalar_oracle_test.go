package mir2_test

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// This test oracle uses Go's byte and signed-byte arithmetic, not MIR2's VM,
// constant-folder helpers or predicates. Values are compared as stored bits.
type scalarCase struct {
	op   mir2.Op
	cond mir2.CmpCond
	aty  mir2.Ty
	a, b uint8
}

func scalarOracle(tc scalarCase) (uint8, bool) {
	a, b := tc.a, tc.b
	switch tc.op {
	case mir2.OpCmp:
		var result bool
		switch tc.cond {
		case mir2.CmpLt:
			result = int8(a) < int8(b)
		case mir2.CmpLe:
			result = int8(a) <= int8(b)
		case mir2.CmpGt:
			result = int8(a) > int8(b)
		case mir2.CmpGe:
			result = int8(a) >= int8(b)
		case mir2.CmpUlt:
			result = a < b
		default:
			panic("oracle condition outside admitted subset")
		}
		if result {
			return 1, false
		}
		return 0, false
	case mir2.OpShl:
		return uint8(uint64(a) << uint(b)), false
	case mir2.OpShr:
		return uint8(uint64(a) >> uint(b)), false
	case mir2.OpSar:
		return uint8(int64(int8(a)) >> uint(b)), false
	case mir2.OpSDiv, mir2.OpSMod:
		if b == 0 {
			return 0, true
		}
		if tc.op == mir2.OpSMod {
			return uint8(int32(int8(a)) % int32(int8(b))), false
		}
		return uint8(int32(int8(a)) / int32(int8(b))), false
	default:
		panic("oracle opcode outside admitted subset")
	}
}

func buildScalarCase(tc scalarCase) (*mir2.Module, *mir2.Func) {
	m := &mir2.Module{Name: "scalar_oracle"}
	f := m.AddFunc("scalar")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	lhs := b.Const(int64(tc.a), tc.aty, mir2.ClassGeneral)
	rhs := b.Const(int64(tc.b), tc.aty, mir2.ClassGeneral)
	var result mir2.Reg
	if tc.op == mir2.OpCmp {
		result = b.CmpWithSrcTy(tc.cond, lhs, rhs, mir2.ClassGeneral, false, tc.aty)
	} else {
		result = b.BinOp(tc.op, lhs, rhs, tc.aty, mir2.ClassGeneral)
	}
	b.Ret(result)
	return m, f
}

func scalarMatchesOracle(t *testing.T, tc scalarCase, m *mir2.Module, f *mir2.Func, phase string) bool {
	t.Helper()
	want, wantTrap := scalarOracle(tc)
	got, err := mir2.NewVM(m).CallFunc(f, nil)
	if wantTrap {
		if err == nil {
			t.Errorf("%s %v(%d,%d): expected division trap, got %v", phase, tc.op, tc.a, tc.b, got)
			return false
		}
		return true
	}
	if err != nil || len(got) != 1 {
		t.Errorf("%s %v(%d,%d): VM error %v, returns %d", phase, tc.op, tc.a, tc.b, err, len(got))
		return false
	}
	if uint8(got[0].I) != want {
		t.Errorf("%s %v(%d,%d): got %d, oracle %d", phase, tc.op, tc.a, tc.b, uint8(got[0].I), want)
		return false
	}
	return true
}

func checkScalarCase(t *testing.T, tc scalarCase) {
	t.Helper()
	m, f := buildScalarCase(tc)
	if !scalarMatchesOracle(t, tc, m, f, "raw") {
		return
	}
	mir2.FoldConstants(f)
	if !scalarMatchesOracle(t, tc, m, f, "folded") {
		return
	}
}

func TestScalarOracleSignedCompare(t *testing.T) {
	// Exhaust every i8 pair, including all sign-boundary crossings.
	for a := 0; a < 256; a++ {
		for b := 0; b < 256; b++ {
			checkScalarCase(t, scalarCase{op: mir2.OpCmp, cond: mir2.CmpLt, aty: mir2.TyI8, a: uint8(a), b: uint8(b)})
		}
	}
	for _, cond := range []mir2.CmpCond{mir2.CmpLe, mir2.CmpGt, mir2.CmpGe, mir2.CmpUlt} {
		for _, a := range []uint8{0, 1, 127, 128, 254, 255} {
			for _, b := range []uint8{0, 1, 127, 128, 254, 255} {
				checkScalarCase(t, scalarCase{op: mir2.OpCmp, cond: cond, aty: mir2.TyI8, a: a, b: b})
			}
		}
	}
}

func TestScalarOracleShiftsAndSignedDivision(t *testing.T) {
	// Every pair of byte bit patterns is checked against both VM paths.
	for a := 0; a < 256; a++ {
		for count := 0; count < 256; count++ {
			for _, op := range []mir2.Op{mir2.OpShl, mir2.OpShr} {
				checkScalarCase(t, scalarCase{op: op, aty: mir2.TyU8, a: uint8(a), b: uint8(count)})
			}
			checkScalarCase(t, scalarCase{op: mir2.OpSar, aty: mir2.TyI8, a: uint8(a), b: uint8(count)})
			checkScalarCase(t, scalarCase{op: mir2.OpSDiv, aty: mir2.TyI8, a: uint8(a), b: uint8(count)})
			checkScalarCase(t, scalarCase{op: mir2.OpSMod, aty: mir2.TyI8, a: uint8(a), b: uint8(count)})
		}
	}
}

func TestScalarOracleRejectsWrongFold(t *testing.T) {
	tc := scalarCase{op: mir2.OpCmp, cond: mir2.CmpLt, aty: mir2.TyI8, a: 255, b: 1}
	m, f := buildScalarCase(tc)
	if !mir2.FoldConstants(f) {
		t.Fatal("expected comparison to fold")
	}
	insts := f.Entry().Insts
	last := insts[len(insts)-1]
	if last.Op != mir2.OpConst {
		t.Fatalf("expected folded const, got %v", last.Op)
	}
	last.Imm ^= 1 // deliberate bad rewrite: detector must find the wrong answer
	want, _ := scalarOracle(tc)
	got, err := mir2.NewVM(m).CallFunc(f, nil)
	if err != nil || len(got) != 1 || uint8(got[0].I) == want {
		t.Fatal(fmt.Sprintf("negative control failed: got %v, err %v, oracle %d", got, err, want))
	}
}

func TestFoldConstantsLeavesSyntheticCarryUnknown(t *testing.T) {
	_, f := buildScalarCase(scalarCase{op: mir2.OpCmp, cond: mir2.CmpLt, aty: mir2.TyU8, a: 1, b: 2})
	cmp := f.Entry().Insts[2]
	cmp.Cond = mir2.CmpSubCarry
	if mir2.FoldConstants(f) || cmp.Op != mir2.OpCmp {
		t.Fatal("synthetic carry condition must not fold without its subtraction context")
	}
}
