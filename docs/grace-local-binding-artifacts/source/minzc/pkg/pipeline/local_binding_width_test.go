package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

const localBindingWidthSource = `global evaluations:u8
fun effect(v:u16)->u16 {evaluations=evaluations+1
return v}
fun decl(v:u16)->u16 {let x:u8=v
if x==0{return 1000}
return u16(x)}
fun variable(v:u16)->u16 {var x:u8=v
if x==0{return 1000}
return u16(x)}
fun assign(v:u16)->u16 {var x:u8=7
x=v
if x==0{return 1000}
return u16(x)}
fun once(v:u16)->u16 {var x:u8=effect(v)
return u16(x)+u16(evaluations)*256}
fun wide(v:u8)->u16 {var x:u16=v
return x+256}
fun signed(v:i8)->i16 {var x:i16=v
return x+256}
fun signedassign(v:i8)->i16 {var x:i16=0
x=v
return x+256}
fun use(x:u8)->u16 {if x==0{return 1000}
return u16(x)}
fun caller(v:u16)->u16 {var x:u8=v
return use(x)}`

func localWidthModule(t *testing.T) *hir.Module {
	t.Helper()
	m, e := nanz.Parse(localBindingWidthSource, "local-width.nanz")
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestLocalIntegerBindingWidths(t *testing.T) {
	cases := []struct {
		name   string
		values []int64
		oracle func(int64) int64
	}{
		{"decl", []int64{0, 255, 256, 511, 65535}, func(v int64) int64 {
			x := v & 255
			if x == 0 {
				return 1000
			}
			return x
		}},
		{"variable", []int64{0, 255, 256, 511, 65535}, func(v int64) int64 {
			x := v & 255
			if x == 0 {
				return 1000
			}
			return x
		}},
		{"assign", []int64{0, 255, 256, 511, 65535}, func(v int64) int64 {
			x := v & 255
			if x == 0 {
				return 1000
			}
			return x
		}},
		{"caller", []int64{0, 255, 256, 511, 65535}, func(v int64) int64 {
			x := v & 255
			if x == 0 {
				return 1000
			}
			return x
		}},
		{"once", []int64{0, 255, 256, 511, 65535}, func(v int64) int64 { return (v & 255) + 256 }},
		{"wide", []int64{0, 1, 127, 128, 255}, func(v int64) int64 { return v + 256 }},
		{"signed", []int64{-128, -1, 0, 1, 127}, func(v int64) int64 { return v + 256 }},
		{"signedassign", []int64{-128, -1, 0, 1, 127}, func(v int64) int64 { return v + 256 }},
	}
	for _, tc := range cases {
		for _, v := range tc.values {
			t.Run(fmt.Sprintf("%s_%d", tc.name, v), func(t *testing.T) {
				want := tc.oracle(v)
				raw := hir.LowerModule(localWidthModule(t))
				if e := mir2.Verify(raw); e != nil {
					t.Fatal(e)
				}
				r, e := mir2.NewVM(raw).Call(tc.name, []mir2.Value{{I: v}})
				if e != nil || len(r) != 1 || r[0].I != want {
					t.Fatal("raw", r, e, want)
				}
				for _, enabled := range []bool{false, true} {
					m := localWidthModule(t)
					steps, e := CompileHIRSteps(m, Options{UseHIRGrace: enabled, AssertMode: "none"})
					if e != nil {
						t.Fatal(e)
					}
					r, e := mir2.NewVM(steps.MIR2Module).Call(tc.name, []mir2.Value{{I: v}})
					if e != nil || len(r) != 1 || r[0].I != want {
						t.Fatal("optimized", enabled, r, e, want)
					}
					measureGraceZ80HIR(t, localWidthModule(t), tc.name, Options{UseHIRGrace: enabled}, []int64{v}, nil, want)
				}
			})
		}
	}
}
func TestLetInDeclaredWidthAndFoldBindings(t *testing.T) {
	for _, value := range []int64{0, 255, 256, 511, 65535} {
		f := &hir.Func{Name: "letin", Params: []hir.Param{{Name: "value", Ty: mir2.TyU16}}, RetTy: mir2.TyU16, Body: hir.Blk(&hir.ReturnStmt{Val: &hir.LetInExpr{Name: "narrow", Ty: mir2.TyU8, Init: hir.Var("value", mir2.TyU16), Body: hir.Add(hir.Cast(hir.Var("narrow", mir2.TyU8), mir2.TyU16), hir.U16(256), mir2.TyU16)}})}
		m := &hir.Module{Name: "letin", Funcs: []*hir.Func{f}}
		if e := hir.Verify(m); e != nil {
			t.Fatal(e)
		}
		want := (value & 255) + 256
		r, e := mir2.NewVM(hir.LowerModule(m)).Call("letin", []mir2.Value{{I: value}})
		if e != nil || r[0].I != want {
			t.Fatal("letin", value, r, e, want)
		}
		measureGraceZ80HIR(t, m, "letin", Options{}, []int64{value}, nil, want)
	}
	src := `fun cb(acc:u16,x:u8)->u16 {return acc+256+u16(x)}
fun folddecl(n:u8)->u16 {var x:u8=range(0..n).fold(u16(0),cb)
return u16(x)+256}
fun narrowcb(acc:u8,x:u8)->u8 {return acc+x}
fun foldwide(n:u8)->u16 {var x:u16=range(0..n).fold(0,narrowcb)
return x+256}
fun foldwideassign(n:u8)->u16 {var x:u16=7
x=range(0..n).fold(0,narrowcb)
return x+256}
fun foldassign(n:u8)->u16 {var x:u8=7
x=range(0..n).fold(u16(0),cb)
return u16(x)+256}`
	for _, name := range []string{"folddecl", "foldassign", "foldwide", "foldwideassign"} {
		for _, n := range []int64{0, 1, 2, 16} {
			m, e := nanz.Parse(src, "fold-width.nanz")
			if e != nil {
				t.Fatal(e)
			}
			want := (n * (n + 1) / 2) & 255
			want += 256
			r, e := mir2.NewVM(hir.LowerModule(m)).Call(name, []mir2.Value{{I: n}})
			if e != nil || r[0].I != want {
				t.Fatal(name, n, r, e, want)
			}
			measureGraceZ80HIR(t, m, name, Options{}, []int64{n}, nil, want)
		}
	}
}

func TestLetInShadowRestoresOuterBinding(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		m := &hir.Module{Name: "shadow", Funcs: []*hir.Func{{Name: "shadow", RetTy: mir2.TyU16, Body: hir.Blk(hir.Decl("x", mir2.TyU16, hir.U16(7)), hir.Decl("y", mir2.TyU8, &hir.LetInExpr{Name: "x", Ty: mir2.TyU8, Init: hir.U16(257), Body: hir.Var("x", mir2.TyU8)}), &hir.ReturnStmt{Val: hir.Add(hir.Var("x", mir2.TyU16), hir.Cast(hir.Var("y", mir2.TyU8), mir2.TyU16), mir2.TyU16)})}}}
		if e := hir.Verify(m); e != nil {
			t.Fatal(e)
		}
		r, e := mir2.NewVM(hir.LowerModule(m)).Call("shadow", nil)
		if e != nil || r[0].I != 8 {
			t.Fatal(r, e)
		}
		measureGraceZ80HIR(t, m, "shadow", Options{UseHIRGrace: enabled}, nil, nil, 8)
	}
}
