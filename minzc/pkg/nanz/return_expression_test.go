package nanz

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

func TestParenthesizedReturnExpression(t *testing.T) {
	for _, tc := range []struct {
		expr string
		want int64
	}{
		{"(b+b)-(a+a)", 44238}, {"(b+b)+(a+a)", 21302},
		{"(b+b)*3", 6}, {"(b+b)&3", 2}, {"(b+b) xor 3", 1},
		{"(b+b)<<1", 4}, {"(b+b) as u8 + 3", 5},
		{"((b+b)-(a+a))", 44238}, {"(b+b)", 2},
		{"2-(a+1)", 22119}, {"2-(a<<1)", 44238},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			h, err := Parse("fun f(a:u16,b:u16)->u16{return "+tc.expr+"}", "return.nanz")
			if err != nil {
				t.Fatal(err)
			}
			if body := h.FuncByName("f").Body.Body; len(body) != 1 {
				t.Errorf("return split into %d statements: %s", len(body), h.Dump())
			}
			out, err := mir2.NewVM(hir.LowerModule(h)).Call("f", []mir2.Value{{I: 43418}, {I: 1}})
			if err != nil || len(out) != 1 || out[0].I != tc.want {
				t.Fatalf("got %v, err=%v; want %d", out, err, tc.want)
			}
		})
	}
}

func TestParenthesizedReturnPostfixAndTuple(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want []int64
	}{
		{`fun f(a:u16,b:u16)->u16{var xs:[u16;2]=[a,b] return (xs)[1]}`, []int64{1}},
		{`fun f(a:u16,b:u16)->(u16,u16){return ((b+b)-(a+a), (a)+b)}`, []int64{44238, 43419}},
	} {
		h, err := Parse(tc.src, "return.nanz")
		if err != nil {
			t.Fatal(err)
		}
		out, err := mir2.NewVM(hir.LowerModule(h)).Call("f", []mir2.Value{{I: 43418}, {I: 1}})
		if err != nil || len(out) != len(tc.want) {
			t.Fatalf("got %v, err=%v", out, err)
		}
		for i, w := range tc.want {
			if out[i].I != w {
				t.Errorf("result %d got %d want %d", i, out[i].I, w)
			}
		}
	}
}
