package mir2

import "testing"

func TestPropagateConstantsNormalizesBlockWidth(t *testing.T) {
	for _, tc := range []struct {
		name   string
		value  int64
		narrow Ty
		want   int64
	}{
		{"byte_zero", 256, TyU8, 0},
		{"byte_one", 257, TyU8, 1},
		{"bool", 3, TyBool, 1},
		{"pointer", 65536, TyPtr, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &Module{}
			f := m.AddFunc("width_chain")
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			v := b.Const(tc.value, TyU32, ClassGeneral)
			b.Jmp("narrow", v)
			narrow := b.SwitchToNewBlock("narrow")
			p := b.BlockParam(narrow, tc.narrow, ClassGeneral)
			b.Jmp("wide", p)
			wide := b.SwitchToNewBlock("wide")
			q := b.BlockParam(wide, TyU32, ClassGeneral)
			b.Ret(q)
			assertConstPropValue(t, m, f, nil, tc.want)
			if !PropagateConstants(f) {
				t.Fatal("expected propagation")
			}
			assertConstPropValue(t, m, f, nil, tc.want)
		})
	}
}

func TestPropagateConstantsJoinsAfterTruncation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		a, b       int64
		shouldFold bool
	}{
		{"equal_zero", 256, 512, true},
		{"equal_one", 1, 257, true},
		{"different", 1, 258, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &Module{}
			f := m.AddFunc("width_join")
			cond := f.AllocReg()
			f.Contract.Params = []Param{{Name: "cond", Reg: cond, Ty: TyBool}}
			b := NewBuilder(f)
			entry := b.SwitchToNewBlock("entry")
			a := b.Const(tc.a, TyU16, ClassGeneral)
			other := b.Const(tc.b, TyU16, ClassGeneral)
			entry.Term = &TermBrIf{Cond: cond, Then: "join", Else: "join", ThenArgs: []Reg{a}, ElseArgs: []Reg{other}}
			join := b.SwitchToNewBlock("join")
			p := b.BlockParam(join, TyU8, ClassGeneral)
			b.Ret(p)
			for flag := int64(0); flag <= 1; flag++ {
				want := tc.b & 255
				if flag == 1 {
					want = tc.a & 255
				}
				assertConstPropValue(t, m, f, []Value{{I: flag}}, want)
			}
			if got := PropagateConstants(f); got != tc.shouldFold {
				t.Fatalf("propagation=%v want=%v", got, tc.shouldFold)
			}
			for flag := int64(0); flag <= 1; flag++ {
				want := tc.b & 255
				if flag == 1 {
					want = tc.a & 255
				}
				assertConstPropValue(t, m, f, []Value{{I: flag}}, want)
			}
		})
	}
}

func assertConstPropValue(t *testing.T, m *Module, f *Func, args []Value, want int64) {
	t.Helper()
	got, err := NewVM(m).Call(f.Name, args)
	if err != nil || len(got) != 1 || got[0].I != want {
		t.Fatalf("VM result=%v error=%v want=%d", got, err, want)
	}
}
