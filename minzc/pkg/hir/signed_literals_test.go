package hir_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"strings"
	"testing"
)

func TestSignedLiteralContexts(t *testing.T) {
	type literalCase struct {
		name, src string
		args      []int64
		want      int64
	}
	cases := []literalCase{
		{"loop", `fun run(a:i16)->u16{var i:i16=a;var n:u16=0;while i < -2 {i=i+1;n=n+1};return n}`, []int64{-5}, 3},
		{"loop_zero", `fun run(a:i16)->u16{var i:i16=a;var n:u16=0;while i < -2 {i=i+1;n=n+1};return n}`, []int64{0}, 0},
		{"add", `fun run(a:i16)->i16{return a + -3}`, []int64{1}, 65534},
		{"multiply", `fun run(a:i16)->i16{return a * -1}`, []int64{3}, 65533},
		{"divide", `fun run(a:i16)->i16{return a / -2}`, []int64{7}, 65533},
		{"modulo", `fun run(a:i16)->i16{return a % -2}`, []int64{-7}, 65535},
		{"binding", `fun run(a:i16)->i16{var x:i16=-2;return x+a}`, []int64{0}, 65534},
		{"assignment", `fun run(a:i16)->i16{var x:i16=a;x=-2;return x}`, []int64{0}, 65534},
		{"argument", `fun ident(a:i16)->i16{return a} fun run(a:i16)->i16{return ident(-2)+a}`, []int64{0}, 65534},
		{"return", `fun run(a:i16)->i16{return -2}`, []int64{0}, 65534},
		// Implicit signed widening preserves the mathematical value.
		{"mixed_variable_signed", `fun run(a:i16,b:i8)->bool{return a < b}`, []int64{0, -2}, 0},
		{"narrow_return", `fun run(a:i8)->i16{return a}`, []int64{-2}, 65534},
		{"narrow_binding", `fun run(a:i8)->i16{var x:i16=a;return x}`, []int64{-2}, 65534},
		{"narrow_assignment", `fun run(a:i8)->i16{var x:i16=0;x=a;return x}`, []int64{-2}, 65534},
		{"narrow_global_assignment", `global x:i16 fun run(a:i8)->i16{x=a;return x}`, []int64{-2}, 65534},
		{"narrow_argument", `fun ident(a:i16)->i16{return a} fun run(a:i8)->i16{return ident(a)}`, []int64{-2}, 65534},
		{"narrow_cross_return", `fun ident(a:i8)->i16{return a} fun run(a:i8)->i16{return ident(a)}`, []int64{-2}, 65534},
		{"narrow_conditional", `fun run(a:i8,b:i16)->i16{let x:i16=if b==0 {b} else {a};return x}`, []int64{-2, 1}, 65534},
		{"narrow_field_initializer", `struct Cell {v:i16} fun run(a:i8)->i16{let x:Cell=Cell{v:a};return x.v}`, []int64{-2}, 65534},
		{"narrow_field_assignment", `struct Cell {v:i16} fun run(a:i8)->i16{var x:Cell=Cell{v:0};x.v=a;return x.v}`, []int64{-2}, 65534},
		{"mixed_equal", `fun run(a:i16,b:i8)->bool{return a == b}`, []int64{-2, -2}, 1},
		{"mixed_narrow_left", `fun run(a:i8,b:i16)->bool{return a < b}`, []int64{-2, 0}, 1},
		{"mixed_narrow_left_word", `fun run(a:i8,b:i16)->bool{return a < b}`, []int64{0, 256}, 1},
		{"mixed_add", `fun run(a:i16,b:i8)->i16{return a+b}`, []int64{1, -2}, 65535},
		{"mixed_subtract", `fun run(a:i16,b:i8)->i16{return a-b}`, []int64{1, -2}, 3},
		{"signed_variable_cast", `fun run(a:i16,b:i8)->bool{return a < (b as i16)}`, []int64{0, -2}, 0},
		{"unsigned_large", `fun run(a:u16)->bool{return a < 1000}`, []int64{999}, 1},
		{"unsigned_large_left", `fun run(a:u16)->bool{return 1000 > a}`, []int64{999}, 1},
	}
	for _, op := range []string{"<", "<=", ">", ">=", "==", "!="} {
		for _, pair := range [][2]int64{{-2, -2}, {0, -2}, {256, 0}, {-129, -128}, {32767, 127}, {-32768, -128}} {
			a, b := pair[0], pair[1]
			for _, reverse := range []bool{false, true} {
				left, right := a, b
				params := "a:i16,b:i8"
				args := []int64{a, b}
				if reverse {
					left, right = b, a
					params = "a:i8,b:i16"
					args = []int64{b, a}
				}
				want := map[string]bool{"<": left < right, "<=": left <= right, ">": left > right, ">=": left >= right, "==": left == right, "!=": left != right}[op]
				w := int64(0)
				if want {
					w = 1
				}
				cases = append(cases, literalCase{fmt.Sprintf("mixed/%s/%d/%d/%t", op, a, b, reverse), fmt.Sprintf("fun run(%s)->bool{return a %s b}", params, op), args, w})
			}
		}
	}
	for _, ty := range []string{"i8", "i16"} {
		for _, op := range []string{"<", "<=", ">", ">=", "==", "!="} {
			for _, a := range []int64{-5, -2, 0, 127} {
				want := false
				switch op {
				case "<":
					want = a < -2
				case "<=":
					want = a <= -2
				case ">":
					want = a > -2
				case ">=":
					want = a >= -2
				case "==":
					want = a == -2
				case "!=":
					want = a != -2
				}
				w := int64(0)
				if want {
					w = 1
				}
				cases = append(cases, literalCase{fmt.Sprintf("%s/%s/%d", ty, op, a), fmt.Sprintf("fun run(a:%s)->bool{return a %s -2}", ty, op), []int64{a}, w})
				reverse := map[string]string{"<": ">", "<=": ">=", ">": "<", ">=": "<=", "==": "==", "!=": "!="}[op]
				cases = append(cases, literalCase{fmt.Sprintf("%s/left/%s/%d", ty, op, a), fmt.Sprintf("fun run(a:%s)->bool{return -2 %s a}", ty, reverse), []int64{a}, w})
			}
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hm, err := nanz.Parse(strings.ReplaceAll(tc.src, ";", "\n"), "signed_literals")
			if err != nil {
				t.Fatal(err)
			}
			raw := hir.LowerModule(hm)
			fix := compileProductionHIRFixture(t, hm)
			values := make([]mir2.Value, len(tc.args))
			for i, a := range tc.args {
				values[i].I = a & ((1 << hm.FuncByName("run").Params[i].Ty.Width()) - 1)
			}
			for name, m := range map[string]*mir2.Module{"raw": raw, "optimized": fix.module} {
				got, e := mir2.NewVM(m).Call("run", values)
				if e != nil || len(got) != 1 || got[0].I != tc.want {
					t.Errorf("%s got %v %v want %d", name, got, e, tc.want)
				}
			}
			got, e := runHIRZ80(t, fix, "run", tc.args)
			if e != nil || got != tc.want {
				t.Errorf("Z80 got %d %v want %d", got, e, tc.want)
			}
		})
	}
}
